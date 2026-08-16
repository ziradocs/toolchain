#!/usr/bin/env python3
# Copyright 2026 Misael Monterroca
# SPDX-License-Identifier: Apache-2.0
#
# Guard against examples/ referencing local image assets that don't exist on
# disk — the failure mode that let a broken `![TechFlow Logo]
# (assets/techflow-logo.png)` (no assets/ dir at all in that example's
# directory) and nine more in a second example sit in the repo undetected,
# in a directory a README named as the recommended starting point. A build
# doesn't fail on this: the generator emits a broken-image reference and
# moves on (see #167's `<div class="image-error">` path for the sanitizer
# rejection case, which is a different failure — this script catches the
#"never existed" case, upstream of that).
#
# Scoped to the markdown image syntax (`![alt](path)`), which is the only
# one any file under examples/ actually uses today (verified: zero hits for
# strict mode's `IMAGE "path"` keyword or `<<image>>` block syntax across
# the whole tree). If a future example introduces one of those, extend
# IMAGE_REF_RE rather than adding a second pass.
#
# check-example-assets-baseline.txt: when this check was first written, a
# full sweep of examples/ (not just the two files a specific bug report
# named) turned up 52 more broken references across 17 other files — a
# separate, much larger finding than the two this change actually fixed.
# Fixing all 52 wasn't in scope for that change, and shipping a check that
# immediately fails CI on 52 pre-existing violations is worse than not
# having the check (someone would just disable it). So: real references are
# baselined, not silently dropped from the check's data set — an entry
# LEAVES the baseline by being fixed, not by being declared exempt forever.

import re
import sys
import urllib.parse
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
EXAMPLES_DIR = REPO_ROOT / "examples"
BASELINE_FILE = Path(__file__).resolve().parent / "check-example-assets-baseline.txt"

IMAGE_REF_RE = re.compile(r"!\[[^\]]*\]\(([^)]+)\)")

REMOTE_SCHEMES = ("http://", "https://", "data:")


def load_baseline():
    if not BASELINE_FILE.is_file():
        return set()
    entries = set()
    for line in BASELINE_FILE.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        path, _, raw_ref = line.partition("\t")
        if not raw_ref:
            print(f"warning: malformed baseline line (expected 'path<TAB>ref'): {line!r}", file=sys.stderr)
            continue
        entries.add((path, raw_ref))
    return entries


def is_local_reference(path: str) -> bool:
    if path.startswith(REMOTE_SCHEMES):
        return False
    # A path built from an unresolved {{variable}} can't be checked
    # statically — skip it rather than false-positive.
    if "{{" in path:
        return False
    return True


def find_missing_assets():
    missing = []
    for pattern in ("*.slidelang", "*.doclang"):
        for source_file in sorted(EXAMPLES_DIR.rglob(pattern)):
            content = source_file.read_text(encoding="utf-8")
            rel_path = str(source_file.relative_to(REPO_ROOT))
            for match in IMAGE_REF_RE.finditer(content):
                raw_path = match.group(1).strip()
                if not is_local_reference(raw_path):
                    continue
                # Local image references are never URL-encoded in this
                # codebase's examples, but decode defensively rather than
                # false-flagging a literal %20 in a filename.
                decoded_path = urllib.parse.unquote(raw_path)
                resolved = (source_file.parent / decoded_path).resolve()
                if not resolved.is_file():
                    line_no = content.count("\n", 0, match.start()) + 1
                    missing.append((rel_path, line_no, raw_path))
    return missing


def main():
    if not EXAMPLES_DIR.is_dir():
        print(f"error: {EXAMPLES_DIR} not found", file=sys.stderr)
        return 1

    baseline = load_baseline()
    missing = find_missing_assets()

    new_failures = [m for m in missing if (m[0], m[2]) not in baseline]
    baselined_hits = [m for m in missing if (m[0], m[2]) in baseline]
    seen_baseline_keys = {(rel_path, raw_path) for rel_path, _, raw_path in baselined_hits}
    stale_baseline = sorted(baseline - seen_baseline_keys)

    if baselined_hits:
        print(f"{len(baselined_hits)} reference(s) matched the pre-existing baseline (not failing):\n")
        for rel_path, line_no, raw_path in baselined_hits:
            print(f"  {rel_path}:{line_no}: {raw_path}")
        print()

    if stale_baseline:
        print(
            f"note: {len(stale_baseline)} baseline entr{'y is' if len(stale_baseline) == 1 else 'ies are'} "
            "stale (fixed, or no longer referenced) — remove from "
            f"{BASELINE_FILE.name} to keep it honest:\n"
        )
        for rel_path, raw_path in stale_baseline:
            print(f"  {rel_path}: {raw_path}")
        print()

    if not new_failures:
        print("OK: no local image reference outside the baseline is broken.")
        return 0

    print(f"Found {len(new_failures)} local image reference(s) that don't resolve to a file:\n", file=sys.stderr)
    for rel_path, line_no, raw_path in new_failures:
        print(f"  {rel_path}:{line_no}: {raw_path}", file=sys.stderr)
    print(
        "\nEither add the missing asset under the example's own directory, "
        "or fix the reference — a broken image in a published example is "
        "exactly the defect this check exists to catch (issue #163/#167 "
        "follow-up). If this is pre-existing debt you're deliberately not "
        f"fixing right now, add it to {BASELINE_FILE.name} instead of "
        "ignoring this failure.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
