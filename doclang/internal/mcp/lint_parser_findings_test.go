// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"testing"
)

// Una imagen de bloque con ruta suelta con espacios dentro de un :::note la
// avisa el parser (en la línea de la imagen) y el linter desde el texto del
// bloque (en la línea del ":::"). La tool lint juntaba los dos: el mismo
// LINK001 salía dos veces, en líneas distintas.
func TestLintTool_BlockImageLink001Once(t *testing.T) {
	session, ctx := newTestSession(t)
	src := "# T\n\n![a](sub dir/a.png)\n\n:::note\n![c](sub dir/c.png)\n:::\n"
	res := callTool(t, session, ctx, "lint", map[string]any{"source": src})
	if res.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, res))
	}
	var out lintOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var lines []int
	for _, d := range out.Diagnostics {
		if d.RuleID == "LINK001" {
			lines = append(lines, d.Position.Line)
		}
	}
	if len(lines) != 2 || lines[0] != 3 || lines[1] != 6 {
		t.Fatalf("LINK001 lines = %v, want [3 6] (diagnostics: %+v)", lines, out.Diagnostics)
	}
}
