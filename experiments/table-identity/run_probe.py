#!/usr/bin/env python3
"""Synthetic, read-only probe of the *current* slidelang CLI.

Run on an isolated exact-SHA Ubuntu checkout. Results go outside the checkout:
    python3 experiments/table-identity/run_probe.py /path/to/slidelang /path/to/results
This is an observation harness, not a proposed grammar or compiler patch.
"""

import hashlib
import json
import pathlib
import subprocess
import sys


PREFIX = "---\nmode: strict\ntitle: Probe\n---\nSLIDE content\n  title: \"A\"\n"

CASES = {
    "regular_duplicates": PREFIX + "  | Key | Value |\n  |---|---|\n  | A | 1 |\n  | A | 1 |\n",
    "regular_reorder": PREFIX + "  | Key | Value |\n  |---|---|\n  | A | 1 |\n  | B | 2 |\n",
    "regular_reorder_changed": PREFIX + "  | Key | Value |\n  |---|---|\n  | B | 2 |\n  | A | 1 |\n",
    "regular_insert": PREFIX + "  | Key | Value |\n  |---|---|\n  | X | 0 |\n  | A | 1 |\n  | B | 2 |\n",
    "regular_edit": PREFIX + "  | Key | Value |\n  |---|---|\n  | A | 9 |\n  | B | 2 |\n",
    "regular_delete": PREFIX + "  | Key | Value |\n  |---|---|\n  | B | 2 |\n",
    "regular_table_id": PREFIX + "  <!-- node-id: TableA -->\n  | Key | Value |\n  |---|---|\n  | A | 1 |\n  | A | 1 |\n",
    "regular_row_marker": PREFIX + "  | Key | Value |\n  |---|---|\n  <!-- node-id: RowA -->\n  | A | 1 |\n",
    "regular_duplicate_node_id": PREFIX + "  <!-- node-id: Reused -->\n  | Key | Value |\n  |---|---|\n  | A | 1 |\n  <!-- node-id: Reused -->\n  TEXT\n    A\n",
    "slide_table_collision": "---\nmode: strict\ntitle: Probe\n---\n<!-- node-id: Reused -->\nSLIDE content\n  title: \"A\"\n  <!-- node-id: Reused -->\n  | Key | Value |\n  |---|---|\n  | A | 1 |\n",
    "regular_orphan_node_id": PREFIX + "  | Key | Value |\n  |---|---|\n  | A | 1 |\n  <!-- node-id: Missing -->\n",
    "merged": PREFIX + "  TABLE\n    cells:\n      - [{content: Key, header: true, scope: col}, {content: Value, header: true, scope: col}]\n      - [{content: A, rowspan: 2}, {content: 1}]\n      - [{content: 2}]\n",
    "merged_reorder_breaks_span": PREFIX + "  TABLE\n    cells:\n      - [{content: Key, header: true}, {content: Value, header: true}]\n      - [{content: 2}]\n      - [{content: A, rowspan: 2}, {content: 1}]\n",
    "merged_insert_during_span": PREFIX + "  TABLE\n    cells:\n      - [{content: Key, header: true}, {content: Value, header: true}]\n      - [{content: A, rowspan: 2}, {content: 1}]\n      - [{content: X}, {content: 0}]\n      - [{content: 2}]\n",
    "merged_delete_covered_row": PREFIX + "  TABLE\n    cells:\n      - [{content: Key, header: true}, {content: Value, header: true}]\n      - [{content: A, rowspan: 2}, {content: 1}]\n",
    "merged_duplicate_content": PREFIX + "  TABLE\n    cells:\n      - [{content: Key, header: true}, {content: Value, header: true}]\n      - [{content: A}, {content: 1}]\n      - [{content: A}, {content: 1}]\n",
    "merged_unknown_id_keys": PREFIX + "  TABLE\n    cells:\n      - [{content: Key, header: true, id: HeaderKey}, {content: Value, header: true}]\n      - [{content: A, id: CellA}, {content: 1}]\n",
    "merged_cell_marker": PREFIX + "  TABLE\n    cells:\n      <!-- node-id: CellA -->\n      - [{content: A}, {content: 1}]\n",
}


def sha(data):
    return hashlib.sha256(data).hexdigest()


def run(argv):
    p = subprocess.run(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return {"command": argv, "exitCode": p.returncode,
            "stdout": p.stdout.decode("utf-8", "replace"),
            "stderr": p.stderr.decode("utf-8", "replace")}


def tables(value):
    found = []
    if isinstance(value, dict):
        if value.get("type") == "table":
            found.append(value)
        for child in value.values():
            found.extend(tables(child))
    elif isinstance(value, list):
        for child in value:
            found.extend(tables(child))
    return found


def main():
    if len(sys.argv) != 3:
        raise SystemExit("usage: run_probe.py CLI RESULTS_DIRECTORY")
    cli, root = pathlib.Path(sys.argv[1]).resolve(), pathlib.Path(sys.argv[2]).resolve()
    root.mkdir(parents=True, exist_ok=True)
    manifest = {"gitHead": run(["git", "rev-parse", "HEAD"])["stdout"].strip(),
                "cliSha256": sha(cli.read_bytes()), "cases": {}}
    for name, source in CASES.items():
        case = root / name
        case.mkdir(exist_ok=True)
        src = case / "input.slidelang"
        src.write_text(source)
        record = {"sourceSha256": sha(src.read_bytes())}
        record["build"] = run([str(cli), "build", str(src), "--format", "json",
                               "--output", str(case / "build"), "--no-colors"])
        outputs = list((case / "build").rglob("*.json")) if (case / "build").exists() else []
        record["jsonOutputs"] = []
        for path in outputs:
            data = path.read_bytes()
            try:
                parsed = json.loads(data)
                summary = [{"nodeId": t.get("nodeId"), "headers": t.get("headers"),
                            "rows": t.get("rows"), "cells": t.get("cells"),
                            "rowPositions": t.get("rowPositions")}
                           for t in tables(parsed)]
            except json.JSONDecodeError:
                summary = None
            record["jsonOutputs"].append({"path": str(path.relative_to(root)),
                                          "sha256": sha(data), "tables": summary})
        record["fmt"] = run([str(cli), "fmt", str(src)])
        if record["fmt"]["exitCode"] == 0:
            formatted = case / "formatted.slidelang"
            formatted.write_text(record["fmt"]["stdout"])
            record["formattedSha256"] = sha(formatted.read_bytes())
            record["reparse"] = run([str(cli), "build", str(formatted), "--format", "json",
                                     "--output", str(case / "reparse"), "--no-colors"])
            reparse_outputs = list((case / "reparse").rglob("*.json")) if (case / "reparse").exists() else []
            record["reparseOutputs"] = []
            for path in reparse_outputs:
                data = path.read_bytes()
                record["reparseOutputs"].append({"path": str(path.relative_to(root)),
                                                  "sha256": sha(data),
                                                  "tables": tables(json.loads(data))})
        manifest["cases"][name] = record
    (root / "manifest.json").write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n")


if __name__ == "__main__":
    main()
