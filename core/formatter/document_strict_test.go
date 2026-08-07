// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter_test

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/formatter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

func parseDoc(t *testing.T, src string) string {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(src, "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("fixture has parse errors: %s", d.String())
		}
	}
	out, err := formatter.FormatDocumentStrict(doc)
	if err != nil {
		t.Fatalf("FormatDocumentStrict failed: %v", err)
	}
	return out
}

// La propiedad que hace usable a un formatter: formatear un documento ya
// canónico no lo cambia, y volver a formatear el resultado tampoco.
func TestFormatDocumentStrict_RoundTripIsStable(t *testing.T) {
	src := `---
mode: strict
title: "Policy"
---

SECTION "Overview"

  TEXT
    Plain prose that survives a round trip.

SECTION "Details"
  level: 2

  POINTS
    - First
    - Second

SECTION "Appendix"

  TEXT
    Closing notes.
`
	once := parseDoc(t, src)
	twice := parseDoc(t, once)

	if once != twice {
		t.Errorf("formatting is not idempotent:\n--- first ---\n%s\n--- second ---\n%s", once, twice)
	}
	if !strings.Contains(once, `SECTION "Details"`) || !strings.Contains(once, "level: 2") {
		t.Errorf("expected the subsection to round-trip as a level-2 SECTION, got:\n%s", once)
	}
}

// El `id:` solo se re-emite cuando no es derivable del título — si no,
// cada subsección arrastraría un id redundante.
func TestFormatDocumentStrict_OmitsDerivableIDs(t *testing.T) {
	out := parseDoc(t, `---
mode: strict
title: "T"
---

SECTION "Guide"

  TEXT
    Intro.

SECTION "Install Steps"
  level: 2
`)
	if strings.Contains(out, "id:") {
		t.Errorf("expected no redundant id: for a title-derived anchor, got:\n%s", out)
	}
}

// Y al revés: un id que NO se deriva del título tiene que sobrevivir, o el
// formateo cambiaría a dónde apunta una referencia.
func TestFormatDocumentStrict_KeepsNonDerivableIDs(t *testing.T) {
	src := `---
mode: strict
title: "T"
---

SECTION "Guide"

  TEXT
    Intro.

SECTION "Installation Steps"
  level: 2
  id: install
`
	once := parseDoc(t, src)
	if !strings.Contains(once, "id: install") {
		t.Fatalf("the explicit id was dropped by the formatter:\n%s", once)
	}
	if twice := parseDoc(t, once); once != twice {
		t.Errorf("explicit ids break idempotence:\n--- first ---\n%s\n--- second ---\n%s", once, twice)
	}
}

// El parser corta el título en la ÚLTIMA comilla precisamente para que un
// título con comillas adentro sobreviva sin escapes. El formatter aplicaba
// la guarda genérica de campos entrecomillados y RECHAZABA lo que el parser
// acepta y conserva — un round-trip roto en un caso que el parser ya tenía
// testeado. Se cubren los dos lugares donde se emite un título: el bloque y
// la subsección.
func TestFormatDocumentStrict_TitlesWithInnerQuotesRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "bloque",
			src:  "---\nmode: strict\n---\n\nSECTION \"El informe \"final\"\"\n\n  TEXT\n    x\n",
			want: `SECTION "El informe "final""`,
		},
		{
			name: "subsección",
			src: "---\nmode: strict\n---\n\nSECTION \"Guide\"\n\n  TEXT\n    x\n\n" +
				"SECTION \"La opción \"rápida\"\"\n  level: 2\n",
			want: `SECTION "La opción "rápida""`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			once := parseDoc(t, tc.src)
			if !strings.Contains(once, tc.want) {
				t.Fatalf("expected %s in the output, got:\n%s", tc.want, once)
			}
			// Lo que prueba que la regla de última comilla es reversible: el
			// texto emitido vuelve a parsear al mismo documento.
			if twice := parseDoc(t, once); once != twice {
				t.Errorf("inner quotes break idempotence:\n--- first ---\n%s\n--- second ---\n%s", once, twice)
			}
		})
	}
}

// Un salto de línea sí es irrepresentable —un SECTION vive en una sola
// línea— así que ahí el formatter debe seguir fallando en vez de emitir
// texto que no re-parsea. Es inalcanzable desde el parser, pero no desde un
// AST construido a mano o tocado por un filtro externo.
func TestFormatDocumentStrict_RejectsMultilineTitles(t *testing.T) {
	doc, diags := parser.New(util.NewNoop()).ParseDocument(
		"---\nmode: strict\n---\n\nSECTION \"Fine\"\n\n  TEXT\n    x\n", "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("fixture has parse errors: %s", d.String())
		}
	}
	doc.ContentBlocks[0].Heading = "linea uno\nlinea dos"

	if _, err := formatter.FormatDocumentStrict(doc); err == nil {
		t.Error("expected a multiline section title to be rejected")
	}
}

// Transpilar flex → strict es el flujo que promueve un borrador a artefacto
// auditable. El resultado tiene que parsear como strict y conservar la
// estructura.
func TestFormatDocumentStrict_TranspilesFlex(t *testing.T) {
	out := parseDoc(t, `---
mode: flex
title: "Draft"
---

# Guide

Intro paragraph.

## Install

Run it.
`)

	if !strings.Contains(out, "mode: strict") {
		t.Errorf("expected the transpiled document to declare strict mode, got:\n%s", out)
	}
	if !strings.Contains(out, `SECTION "Guide"`) || !strings.Contains(out, `SECTION "Install"`) {
		t.Errorf("expected both sections to survive transpilation, got:\n%s", out)
	}

	// Y lo que de verdad importa: re-parsea a la misma forma.
	doc, diags := parser.New(util.NewNoop()).ParseDocument(out, "d.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("the transpiled document does not parse: %s", d.String())
		}
	}
	if len(doc.ContentBlocks) != 1 {
		t.Errorf("expected 1 block after transpiling, got %d", len(doc.ContentBlocks))
	}
}

// Un título con formato inline pierde el énfasis pero NO se corrompe: el
// documento resultante re-parsea limpio. Es la misma canonicalización que
// FormatDocument ya documenta para `## **bold**`, pineada acá para que
// nadie la descubra como bug en producción.
func TestFormatDocumentStrict_InlineFormattingInTitlesDegradesCleanly(t *testing.T) {
	out := parseDoc(t, `---
mode: strict
title: "T"
---

SECTION "Guide"

  TEXT
    Intro.

SECTION "The **important** part"
  level: 2
`)

	if strings.Contains(out, "<strong>") {
		t.Errorf("raw HTML leaked into the source form:\n%s", out)
	}
	if !strings.Contains(out, `SECTION "The important part"`) {
		t.Errorf("expected the title to degrade to plain text, got:\n%s", out)
	}
	if twice := parseDoc(t, out); out != twice {
		t.Errorf("the degraded title is not stable:\n--- first ---\n%s\n--- second ---\n%s", out, twice)
	}
}
