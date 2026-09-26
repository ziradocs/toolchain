#!/usr/bin/env python3
"""Synthetic external filter: injects unsupported row records to observe loss.

`99.0.0` is only a test sentinel. Never use this as a production filter.
"""

import json
import os
import sys


doc = json.load(sys.stdin)
doc["schemaVersion"] = "99.0.0"
for block in doc["contentBlocks"]:
    for element in block["elements"]:
        if element["type"] == "table":
            element["tableRows"] = [
                {"nodeId": "HeaderRow", "section": "header", "cells": [
                    {"nodeId": "KeyHeader", "content": "Key", "header": True},
                    {"nodeId": "ValueHeader", "content": "Value", "header": True},
                ]},
                {"nodeId": "DataRowA", "section": "body", "cells": [
                    {"nodeId": "KeyA", "content": "A"},
                    {"nodeId": "ValueA", "content": "1"},
                ]},
            ]
            break
capture = os.environ.get("TABLE_ID_FILTER_OUTPUT")
if capture:
    with open(capture, "w", encoding="utf-8") as output:
        json.dump(doc, output, indent=2)
        output.write("\n")
json.dump(doc, sys.stdout)
