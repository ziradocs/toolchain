// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/util"
)

// Los IDs que el parser dejó de compartir con el linter quedan fijos: el
// aviso de `mode:` faltante es FRONT009 (FRONT001 es del linter) y el YAML
// inválido de un POLL es POLL006 (POLL003 es del linter, sobre `results`).
// Una política del frontmatter se escribe con estos IDs.
func TestParserRuleIDs_FRONT009AndPOLL006(t *testing.T) {
	ids := func(src string) map[string]bool {
		_, diags := New(util.NewNoop()).Parse(src, "d.slidelang")
		out := map[string]bool{}
		for _, d := range diags {
			out[d.RuleID] = true
		}
		return out
	}
	got := ids("---\ntitle: \"D\"\n---\n\nSLIDE content\n  title: \"T\"\n  TEXT\n    hola\n")
	if !got["FRONT009"] || got["FRONT001"] {
		t.Errorf("missing mode: want FRONT009 and not FRONT001, got %v", got)
	}
	got = ids("---\nmode: flex\ntitle: \"D\"\n---\n\n# A\n\n<<poll>>\nquestion: \"sin cerrar\n  - [{\n<<end>>\n")
	if !got["POLL006"] || got["POLL003"] {
		t.Errorf("invalid poll YAML: want POLL006 and not POLL003, got %v", got)
	}
}
