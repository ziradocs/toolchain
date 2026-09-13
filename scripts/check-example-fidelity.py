#!/usr/bin/env python3
# Copyright 2026 Misael Monterroca
# SPDX-License-Identifier: Apache-2.0
#
# Guard against the corpus regressing from "builds without error" to
# "builds, but silently drops or garbles content" — a class of bug that
# html-validate.yml, the linter, and every existing example check miss
# entirely, because a chart with a collapsed axis, a dropped image, or a
# code-group whose panels are stacked, all still produce valid, buildable
# HTML. Only counting structural elements in the SOURCE against the same
# elements in the GENERATED HTML catches that class of defect.
#
# Same baseline pattern as check-example-assets.py: this repo's corpus
# already has real fidelity bugs at the time this check was written (see the
# toolchain audit dated 2026-09-11/12 — combo chart axes, single-row chart
# data, slidelang code-group CSS, nested content inside `:::`/grid columns,
# etc). Failing CI on all of those immediately would just get the check
# disabled. So: every mismatch this script finds on first run is baselined
# by exact (file, check) key with its current counts, and a mismatch only
# fails the build if it's NEW or WORSE than what's baselined. Fixing the
# underlying bug removes rows from the baseline as "stale, remove me" — an
# entry leaves by being fixed, never by being declared exempt forever.
#
# What this checks (source counts vs. rendered-HTML counts, per file):
#   - chart / map / mermaid / plantuml block count (native tag + fenced form)
#   - table count and total data-row count (<tr>, minus header/separator)
#   - image count (all `![...](...)` occurrences on a line, not just the first)
#   - checklist item count
#   - plain code-fence count vs. rendered <pre> count
#   - special-block count per known alert type (info/warning/danger/success/tip)
#   - grid and code-group block count
#
# This is a STRUCTURAL count check, not a semantic/visual one — two `<tr>`
# tags with swapped cell content still match. It exists to catch silent
# content loss (an element parsed in the source that never makes it into the
# HTML at all, or renders as the wrong count), not to replace the visual
# review a human did for the canonical gallery/new examples during the audit.

import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
EXAMPLES_DIR = REPO_ROOT / "examples"
BASELINE_FILE = Path(__file__).resolve().parent / "check-example-fidelity-baseline.json"

# unknown_mode_test.slidelang deliberately has an invalid `mode:` value to
# exercise the parser's error path (see its own header comment) — it's not
# expected to build at all, so it's not part of this corpus.
SKIP = {"unknown_mode_test.slidelang"}

TABLE_SEP_RE = re.compile(r"^\|[\s:|\-]+\|$")
IMAGE_RE = re.compile(r"!\[[^\]]*\]\([^)]+\)")
CHECKLIST_RE = re.compile(r"^\s*-\s*\[[ xX]\]")
SPECIAL_OPEN_RE = re.compile(r"^:{3,4}\s*([a-zA-Z][\w-]*)")
NATIVE_TAG_RE = re.compile(r"<<(chart|map|mermaid|plantuml)\b", re.IGNORECASE)

ALERT_TYPES = ("info", "tip", "warning", "danger", "success")
STRUCTURAL_TYPES = {"grid", "column", "columns", "code-group"}


def find_sources():
    out = []
    for root, _dirs, files in os.walk(EXAMPLES_DIR):
        for f in files:
            if f.endswith(".slidelang") or f.endswith(".doclang"):
                if f in SKIP:
                    continue
                out.append(Path(root) / f)
    return sorted(out)


def parse_source(path):
    lines = path.read_text(encoding="utf-8", errors="replace").split("\n")

    counts = {
        "chart": 0,
        "map": 0,
        "mermaid": 0,
        "plantuml": 0,
        "table": 0,
        "table_pipe_lines": 0,
        "image": 0,
        "checklist": 0,
        "code_plain_fence": 0,
        "special_blocks": {},
    }

    in_fence = False
    fence_lang = None

    for line in lines:
        stripped = line.strip()

        if not in_fence:
            m = re.match(r"^(\s*)```(\S*)", line)
            if m:
                in_fence = True
                fence_lang = m.group(2).strip().lower()
                continue
            for nm in NATIVE_TAG_RE.finditer(line):
                counts[nm.group(1).lower()] += 1
            if TABLE_SEP_RE.match(stripped) and "-" in stripped:
                counts["table"] += 1
            if stripped.startswith("|") and stripped.endswith("|") and len(stripped) > 1:
                counts["table_pipe_lines"] += 1
            counts["image"] += len(list(IMAGE_RE.finditer(line)))
            if CHECKLIST_RE.match(line):
                counts["checklist"] += 1
            sm = SPECIAL_OPEN_RE.match(stripped)
            if sm:
                typ = sm.group(1).lower()
                counts["special_blocks"][typ] = counts["special_blocks"].get(typ, 0) + 1
        else:
            if re.match(r"^\s*```\s*$", line):
                in_fence = False
                if fence_lang in ("chart", "map", "mermaid"):
                    counts[fence_lang] += 1
                else:
                    counts["code_plain_fence"] += 1
                fence_lang = None

    return counts


SCRIPT_STYLE_RE = re.compile(
    r"<script\b[^>]*>.*?</script>|<style\b[^>]*>.*?</style>", re.DOTALL | re.IGNORECASE
)


def read_html(path):
    if not path.exists():
        return None
    raw = path.read_text(encoding="utf-8", errors="replace")
    # Strip <script>/<style> so CSS rules (".chart-container {") and JS
    # string literals mentioning these class names aren't counted as real
    # rendered elements.
    return SCRIPT_STYLE_RE.sub("", raw)


def count_sub(html, s):
    return html.count(s) if html else 0


def count_regex(html, pattern):
    return len(re.findall(pattern, html)) if html else 0


def parse_html(dialect, html):
    if html is None:
        return None
    r = {}
    if dialect == "slidelang":
        r["chart"] = count_sub(html, "slidelang-chart-container")
        r["map"] = count_sub(html, "slidelang-map-container")
        r["mermaid"] = count_sub(html, "slidelang-element slidelang-mermaid ")
        r["plantuml"] = count_sub(html, "plantuml-container")
        r["table"] = count_sub(html, "<table")
        r["tr"] = count_sub(html, "<tr")
        r["image"] = count_regex(html, r"<img\s[^>]*src=")
        r["checklist"] = count_sub(html, "checklist-checkbox") or count_regex(
            html, r'type="checkbox"'
        )
        r["pre"] = count_sub(html, "<pre")
        r["alerts"] = {}
        for m in re.finditer(r"slidelang-special-block slidelang-([\w-]+)\s", html):
            typ = m.group(1).lower()
            r["alerts"][typ] = r["alerts"].get(typ, 0) + 1
        r["code_group"] = count_sub(html, "slidelang-element slidelang-code-group ")
        r["grid"] = count_sub(html, "slidelang-element slidelang-grid ")
    else:
        r["chart"] = count_sub(html, "chart-container")
        r["map"] = count_sub(html, "map-wrapper")
        r["mermaid"] = count_sub(html, "mermaid-container")
        r["plantuml"] = count_sub(html, "plantuml-container")
        r["table"] = count_sub(html, "<table")
        r["tr"] = count_sub(html, "<tr")
        r["image"] = count_regex(html, r"<img\s[^>]*src=")
        r["checklist"] = count_regex(html, r'type="checkbox"')
        r["pre"] = count_sub(html, "<pre")
        r["alerts"] = {}
        for m in re.finditer(r'class="alert alert-([\w-]+)"', html):
            typ = m.group(1).lower()
            r["alerts"][typ] = r["alerts"].get(typ, 0) + 1
        r["code_group"] = count_sub(html, 'class="code-group"')
        r["grid"] = count_regex(html, r'class="grid"')
    return r


def build_binaries(build_dir):
    binaries = {}
    for dialect, module in (("slidelang", "slidelang"), ("doclang", "doclang")):
        out = build_dir / dialect
        subprocess.run(
            ["go", "build", "-o", str(out), f"./cmd/{dialect}"],
            cwd=REPO_ROOT / module,
            check=True,
        )
        binaries[dialect] = out
    return binaries


def build_html(binaries, sources, html_dir):
    built = {}
    for src in sources:
        dialect = "slidelang" if src.suffix == ".slidelang" else "doclang"
        base = src.stem
        out_dir = html_dir / dialect / base
        result = subprocess.run(
            [str(binaries[dialect]), "build", str(src), "--format", "html", "--output", str(out_dir)],
            capture_output=True,
            text=True,
        )
        html_path = out_dir / f"{base}.html"
        built[src] = html_path if result.returncode == 0 and html_path.exists() else None
    return built


def compute_mismatches(dialect, scounts, hcounts):
    mismatches = {}

    def check(name, s_val, h_val):
        if s_val != h_val:
            mismatches[name] = {"source": s_val, "html": h_val}

    check("chart", scounts["chart"], hcounts["chart"])
    check("map", scounts["map"], hcounts["map"])
    check("mermaid", scounts["mermaid"], hcounts["mermaid"])
    check("plantuml", scounts["plantuml"], hcounts["plantuml"])
    check("table", scounts["table"], hcounts["table"])
    check("image", scounts["image"], hcounts["image"])
    check("checklist", scounts["checklist"], hcounts["checklist"])
    check("code_pre", scounts["code_plain_fence"], hcounts["pre"])

    pipe_lines = scounts["table_pipe_lines"]
    if pipe_lines > 0:
        expected_tr = pipe_lines - scounts["table"]
        if expected_tr != hcounts["tr"]:
            mismatches["table_rows"] = {"source": expected_tr, "html": hcounts["tr"]}

    for typ in ALERT_TYPES:
        s_val = scounts["special_blocks"].get(typ, 0)
        h_val = hcounts["alerts"].get(typ, 0)
        if s_val != h_val:
            mismatches[f"special_block[{typ}]"] = {"source": s_val, "html": h_val}

    grid_src = scounts["special_blocks"].get("grid", 0)
    if grid_src != hcounts["grid"]:
        mismatches["grid"] = {"source": grid_src, "html": hcounts["grid"]}

    cg_src = scounts["special_blocks"].get("code-group", 0)
    if cg_src != hcounts["code_group"]:
        mismatches["code_group"] = {"source": cg_src, "html": hcounts["code_group"]}

    return mismatches


def load_baseline():
    if not BASELINE_FILE.is_file():
        return {}
    return json.loads(BASELINE_FILE.read_text(encoding="utf-8"))


def signed_gap(vals):
    """html - source: negative means html is MISSING elements (loss),
    positive means html has EXTRA elements (duplication). None for
    HTML_MISSING, treated as infinitely bad so it always counts as a
    regression unless it was already baselined as missing.

    Kept signed on purpose (hallazgo de revisión independiente): comparar
    solo abs(source - html) trataba una pérdida (1 -> 0, gap 1) y una
    duplicación (1 -> 2, gap 1) como el mismo "gap", así que un renderer que
    cambiara de perder un elemento a DUPLICARLO pasaba el check con el mismo
    baseline — una regresión real, solo que en la dirección opuesta,
    disfrazada de "sin cambio". Comparar el par con signo exige que una
    entrada "mejorada" reduzca la magnitud SIN cambiar de signo: acercarse a
    0 desde el mismo lado en el que ya estaba, no cruzar al lado opuesto.
    """
    if vals == "build failed":
        return None
    return vals["html"] - vals["source"]


def sign(n):
    return (n > 0) - (n < 0)


def main():
    write_baseline = "--write-baseline" in sys.argv
    sources = find_sources()

    with tempfile.TemporaryDirectory(prefix="fidelity-") as tmp:
        tmp_path = Path(tmp)
        binaries = build_binaries(tmp_path / "bin")
        html_by_src = build_html(binaries, sources, tmp_path / "html")

        baseline = load_baseline()
        new_failures = []
        improved = []
        seen_baseline_keys = set()
        current = {}

        for src in sources:
            rel = str(src.relative_to(REPO_ROOT))
            dialect = "slidelang" if src.suffix == ".slidelang" else "doclang"
            html_path = html_by_src[src]

            if html_path is None:
                key = f"{rel}::HTML_MISSING"
                current[key] = "build failed"
                if key in baseline:
                    seen_baseline_keys.add(key)
                else:
                    new_failures.append((rel, "HTML_MISSING", "build failed", None))
                continue

            scounts = parse_source(src)
            hcounts = parse_html(dialect, read_html(html_path))
            mismatches = compute_mismatches(dialect, scounts, hcounts)

            for name, vals in mismatches.items():
                key = f"{rel}::{name}"
                current[key] = vals

                if key not in baseline:
                    # No mismatch existed here before (gap was 0) — any gap
                    # now is strictly a new regression.
                    new_failures.append((rel, name, vals, None))
                    continue

                base_vals = baseline[key]
                base_signed = signed_gap(base_vals)
                new_signed = signed_gap(vals)

                # Una entrada solo cuenta como "no peor" si se mueve hacia 0
                # DESDE EL MISMO LADO en el que ya estaba baselineada (mismo
                # signo) — nunca cruzando de pérdida a duplicación o
                # viceversa, que es un defecto distinto disfrazado del mismo
                # |gap| (ver signed_gap). base_signed == 0 no puede pasar en
                # la práctica (una entrada solo entra al baseline con un
                # mismatch real), pero sign(0) == 0 igual exige signo 0 en
                # new_signed, que tampoco puede pasar — correcto por
                # construcción, sin caso especial.
                is_regression = (
                    base_signed is None
                    or new_signed is None
                    or sign(new_signed) != sign(base_signed)
                    or abs(new_signed) > abs(base_signed)
                )

                if is_regression:
                    new_failures.append((rel, name, vals, base_vals))
                else:
                    # Same or smaller gap than baselined: not a regression,
                    # even if the exact (source, html) pair changed (e.g. the
                    # fixture itself grew a line). Flag as "improved" so the
                    # baseline can be tightened, rather than silently going
                    # stale forever.
                    seen_baseline_keys.add(key)
                    if vals != base_vals:
                        improved.append((key, base_vals, vals))

        if write_baseline:
            BASELINE_FILE.write_text(
                json.dumps(dict(sorted(current.items())), indent=2) + "\n", encoding="utf-8"
            )
            print(f"Wrote {len(current)} entries to {BASELINE_FILE.name}.")
            return 0

        failing_keys = {f"{rel}::{name}" for rel, name, _vals, _base in new_failures}
        stale = sorted(set(baseline) - seen_baseline_keys - {k for k, _, _ in improved} - failing_keys)

        if stale:
            print(f"note: {len(stale)} baseline entr{'y is' if len(stale) == 1 else 'ies are'} "
                  f"stale (fixed outright) — remove from {BASELINE_FILE.name}:\n")
            for key in stale:
                print(f"  {key}: {baseline[key]}")
            print()

        if improved:
            print(f"note: {len(improved)} baseline entr{'y has' if len(improved) == 1 else 'ies have'} "
                  f"a smaller gap than before (not failing, but the baseline could be tightened "
                  f"with --write-baseline):\n")
            for key, base_vals, vals in improved:
                print(f"  {key}: was {base_vals} -> now {vals}")
            print()

        if not new_failures:
            print(f"OK: {len(sources)} example(s) checked ({len(SKIP)} skipped by design), "
                  "no fidelity regression outside the baseline.")
            return 0

        print(f"Found {len(new_failures)} fidelity mismatch(es) not covered by the baseline:\n",
              file=sys.stderr)
        for rel, name, vals, base_vals in new_failures:
            print(f"  {rel} [{name}]: {vals}" + (f" (baseline was {base_vals})" if base_vals else ""),
                  file=sys.stderr)
        print(
            f"\nEither fix the underlying rendering bug, or if this is pre-existing debt "
            f"you're deliberately not fixing right now, add it to {BASELINE_FILE.name} "
            "with its current counts instead of ignoring this failure.",
            file=sys.stderr,
        )
        return 1


if __name__ == "__main__":
    sys.exit(main())
