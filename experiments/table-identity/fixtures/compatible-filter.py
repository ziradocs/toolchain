#!/usr/bin/env python3
"""Synthetic identity-preserving filter for CLI negotiation probes."""

import json
import sys


if len(sys.argv) == 2 and sys.argv[1] == "--ziradocs-capabilities":
    print(json.dumps({"astSchemaVersions": ["2.15.0"], "features": ["table-rows-v1"]}))
else:
    json.dump(json.load(sys.stdin), sys.stdout)
