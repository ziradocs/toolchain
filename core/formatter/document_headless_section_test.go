// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"reflect"
	"strings"
	"testing"
)

// TestFormatDocument_HeadlessLeadingSection_RoundTrips: la sección inicial
// sin encabezado (contenido antes del primer `#`, o un archivo con solo
// `##`) se escribe sin línea `# ` y vuelve a parsear al mismo AST. Antes el
// formatter siempre escribía `# <título>`, y con título vacío dejaba un `#`
// suelto que el reparse convertía en contenido.
func TestFormatDocument_HeadlessLeadingSection_RoundTrips(t *testing.T) {
	inputs := map[string]string{
		"preamble": "Intro antes del título.\n\n## Antes\n\n# Título\n\nCuerpo.\n\n# Otra\n\nMás.\n",
		"only-h2":  "## Solo dos\n\nTexto.\n\n### Tres\n\nMás.\n",
		"with-fm":  "---\nmode: flex\n---\n\nIntro.\n\n# Título\n\nCuerpo.\n",
	}
	for name, src := range inputs {
		t.Run(name, func(t *testing.T) {
			doc, err := parseDocument(t, src)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			out, err := FormatDocument(doc)
			if err != nil {
				t.Fatalf("FormatDocument: %v", err)
			}
			for _, line := range strings.Split(out, "\n") {
				if strings.TrimSpace(line) == "#" {
					t.Fatalf("encabezado vacío en la salida:\n%s", out)
				}
			}
			if strings.HasPrefix(out, "\n") {
				t.Errorf("la salida no debe abrir con una línea en blanco:\n%q", out)
			}
			reparsed, err := parseDocument(t, out)
			if err != nil {
				t.Fatalf("reparse: %v\n%s", err, out)
			}
			if want, got := normalizeForComparison(doc), normalizeForComparison(reparsed); !reflect.DeepEqual(want, got) {
				t.Fatalf("round-trip mismatch\n--- out ---\n%s\nwant %s\ngot  %s", out, toJSON(t, want), toJSON(t, got))
			}
			out2, err := FormatDocument(reparsed)
			if err != nil || out2 != out {
				t.Fatalf("no idempotente (err=%v)\n--- 1st ---\n%s\n--- 2nd ---\n%s", err, out, out2)
			}
		})
	}
}
