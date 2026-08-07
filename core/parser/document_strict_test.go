// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

func parseStrictDoc(t *testing.T, input string) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	return NewDocumentStrictParser(input, util.NewNoop()).Parse()
}

func errorMessages(diags []diagnostics.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		if d.IsError() {
			out = append(out, d.Message)
		}
	}
	return out
}

func assertNoErrors(t *testing.T, diags []diagnostics.Diagnostic) {
	t.Helper()
	if msgs := errorMessages(diags); len(msgs) > 0 {
		t.Fatalf("expected no parse errors, got: %v", msgs)
	}
}

// assertOneErrorContaining exige exactamente un error y que mencione substr.
func assertOneErrorContaining(t *testing.T, diags []diagnostics.Diagnostic, substr string) {
	t.Helper()
	msgs := errorMessages(diags)
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 error, got %d: %v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0], substr) {
		t.Errorf("expected error mentioning %q, got: %s", substr, msgs[0])
	}
}

// El caso base: dos secciones de nivel 1 se vuelven dos ContentBlocks, con
// la misma regla posicional que flex (el primero es el bloque "title" del
// documento y su texto va a Heading; los demás son "content" y van a Title).
func TestDocumentStrictParser_TopLevelSections(t *testing.T) {
	doc, diags := parseStrictDoc(t, `SECTION "Introduction"

  TEXT
    Hello world.

SECTION "Background"

  TEXT
    Some context.
`)
	assertNoErrors(t, diags)

	if len(doc.ContentBlocks) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(doc.ContentBlocks))
	}

	first := doc.ContentBlocks[0]
	if first.BlockType != "title" {
		t.Errorf("expected the first section to be the document title block, got type %q", first.BlockType)
	}
	if first.Heading != "Introduction" {
		t.Errorf("expected Heading %q, got %q", "Introduction", first.Heading)
	}
	if len(first.Elements) != 1 {
		t.Errorf("expected 1 element in the first section, got %d", len(first.Elements))
	}

	second := doc.ContentBlocks[1]
	if second.BlockType != "content" {
		t.Errorf("expected the second section to be a content block, got type %q", second.BlockType)
	}
	if second.Title != "Background" {
		t.Errorf("expected Title %q, got %q", "Background", second.Title)
	}
}

// El contrato de forma que hace que renderer/TOC/xref funcionen sin ramas
// por dialecto: los niveles 2-6 NO son bloques, son elementos <hN id=...>
// dentro del bloque abierto.
func TestDocumentStrictParser_SubsectionsBecomeHeadingElements(t *testing.T) {
	doc, diags := parseStrictDoc(t, `SECTION "Guide"

  TEXT
    Intro paragraph.

SECTION "Install"
  level: 2

  TEXT
    Run the installer.
`)
	assertNoErrors(t, diags)

	if len(doc.ContentBlocks) != 1 {
		t.Fatalf("expected the level-2 section to stay inside the level-1 block, got %d blocks", len(doc.ContentBlocks))
	}

	els := doc.ContentBlocks[0].Elements
	if len(els) != 3 {
		t.Fatalf("expected 3 elements (text, heading, text), got %d", len(els))
	}

	heading, ok := els[1].(*ast.TextElement)
	if !ok {
		t.Fatalf("expected the heading to be a TextElement, got %T", els[1])
	}
	if heading.Level != 2 {
		t.Errorf("expected Level 2 on the heading element, got %d", heading.Level)
	}
	if want := `<h2 id="install">Install</h2>`; heading.Content != want {
		t.Errorf("heading HTML drifted from the flex dialect's contract:\n got: %s\nwant: %s", heading.Content, want)
	}

	// El orden importa (contrato de orden de elementos, issue #62): el
	// encabezado va ANTES del contenido que introduce.
	if _, isText := els[2].(*ast.TextElement); !isText {
		t.Errorf("expected the subsection's body to follow its heading, got %T", els[2])
	}
}

// Un `id:` explícito reemplaza el anchor derivado del título — es lo que
// hace que un `\ref` estable sobreviva a que el título cambie.
func TestDocumentStrictParser_ExplicitID(t *testing.T) {
	doc, diags := parseStrictDoc(t, `SECTION "Guide"

  TEXT
    Intro.

SECTION "Installation Steps"
  level: 2
  id: install
`)
	assertNoErrors(t, diags)

	heading := doc.ContentBlocks[0].Elements[1].(*ast.TextElement)
	if want := `<h2 id="install">Installation Steps</h2>`; heading.Content != want {
		t.Errorf("got: %s\nwant: %s", heading.Content, want)
	}
}

// El `id:` lo escribe el autor y se interpola dentro de un atributo HTML:
// tiene que pasar por el mismo saneado de lista blanca que el anchor
// derivado, o es una inyección directa.
func TestDocumentStrictParser_ExplicitIDCannotEscapeTheAttribute(t *testing.T) {
	doc, diags := parseStrictDoc(t, `SECTION "Guide"

  TEXT
    Intro.

SECTION "Payload"
  level: 2
  id: x"><script>alert(1)</script>
`)
	assertNoErrors(t, diags)

	heading := doc.ContentBlocks[0].Elements[1].(*ast.TextElement)

	// El valor del id, aislado: no puede contener nada capaz de cerrar el
	// atributo ni de abrir una etiqueta. Los caracteres alfanuméricos del
	// payload sobreviven (el saneado es lista blanca, no borrado de
	// palabras) y eso está bien — lo que importa es que no quede sintaxis.
	idValue := heading.Content[len(`<h2 id="`):strings.Index(heading.Content[len(`<h2 id="`):], `"`)+len(`<h2 id="`)]
	for _, forbidden := range []string{`"`, "<", ">", "'", "="} {
		if strings.Contains(idValue, forbidden) {
			t.Errorf("explicit id kept %q and can escape the attribute: id=%q (full: %s)",
				forbidden, idValue, heading.Content)
		}
	}
	if !strings.HasSuffix(heading.Content, `>Payload</h2>`) {
		t.Errorf("expected a well-formed heading, got: %s", heading.Content)
	}
}

// El `id:` se NORMALIZA a su forma de anchor, y esa forma es la canónica.
// Importa porque el anchor saneado es lo único que sobrevive en el AST: un
// `fmt` que reconstruya el documento solo puede emitir esa forma. Que la
// normalización sea idempotente es lo que hace ese round-trip estable en
// vez de que el id derive un poco en cada pasada.
func TestDocumentStrictParser_ExplicitIDIsNormalizedIdempotently(t *testing.T) {
	const header = "SECTION \"Guide\"\n\n  TEXT\n    Intro.\n\n"

	doc, diags := parseStrictDoc(t, header+"SECTION \"Steps\"\n  level: 2\n  id: Install_Steps\n")
	assertNoErrors(t, diags)

	heading := doc.ContentBlocks[0].Elements[1].(*ast.TextElement)
	if want := `<h2 id="install_steps">Steps</h2>`; heading.Content != want {
		t.Fatalf("got: %s\nwant: %s", heading.Content, want)
	}

	// Segunda pasada con la forma ya normalizada: mismo anchor. Sin esto,
	// `fmt` movería el id un poco en cada corrida.
	again, diags := parseStrictDoc(t, header+"SECTION \"Steps\"\n  level: 2\n  id: install_steps\n")
	assertNoErrors(t, diags)
	if got := again.ContentBlocks[0].Elements[1].(*ast.TextElement).Content; got != heading.Content {
		t.Errorf("id normalization is not idempotent:\nfirst:  %s\nsecond: %s", heading.Content, got)
	}
}

// Un id que se sanea hasta quedar vacío no es utilizable como anchor, y
// dejarlo pasar produciría `id=""` — un `\ref` roto sin explicación.
func TestDocumentStrictParser_ExplicitIDWithNoUsableCharacters(t *testing.T) {
	_, diags := parseStrictDoc(t, `SECTION "Guide"

  TEXT
    Intro.

SECTION "Emoji"
  level: 2
  id: "🎉"
`)
	assertOneErrorContaining(t, diags, "no usable characters")
}

func TestDocumentStrictParser_SectionHeaderErrors(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"sin título", "SECTION\n", "requires a quoted title"},
		{"sin comillas", "SECTION Introduction\n", "must be quoted"},
		{"comilla sin cerrar", "SECTION \"Introduction\n", "closing quote"},
		{"propiedades en línea", "SECTION \"Intro\" level: 2\n", "properties go on indented lines"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := parseStrictDoc(t, tc.input)
			assertOneErrorContaining(t, diags, tc.want)
		})
	}
}

func TestDocumentStrictParser_PropertyErrors(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"nivel no numérico", "SECTION \"A\"\n  level: two\n", "must be an integer"},
		{"nivel fuera de rango", "SECTION \"A\"\n  level: 7\n", "must be between 1 and 6"},
		{"propiedad desconocida", "SECTION \"A\"\n  numbered: true\n", "Unknown SECTION property"},
		{"propiedad de SLIDE", "SECTION \"A\"\n  subtitle: Nope\n", "is a SLIDE property"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := parseStrictDoc(t, tc.input)
			assertOneErrorContaining(t, diags, tc.want)
		})
	}
}

// `id:` en nivel 1 se rechaza en vez de aceptarse-e-ignorarse: el AST no
// tiene dónde guardarlo, así que el anchor seguiría derivándose del título
// y el `\ref` del autor apuntaría a otro lado.
func TestDocumentStrictParser_IDRejectedOnTopLevelSection(t *testing.T) {
	_, diags := parseStrictDoc(t, "SECTION \"Intro\"\n  id: intro\n")
	assertOneErrorContaining(t, diags, "only supported on level 2-6")
}

// Sin bloque padre no hay dónde colgar el encabezado: se reporta, pero el
// contenido se promueve en vez de desaparecer.
func TestDocumentStrictParser_SubsectionWithoutParentIsReportedAndPromoted(t *testing.T) {
	doc, diags := parseStrictDoc(t, `SECTION "Orphan"
  level: 2

  TEXT
    Body text.
`)
	assertOneErrorContaining(t, diags, "cannot be the first section")

	if len(doc.ContentBlocks) != 1 {
		t.Fatalf("expected the orphan section to survive as a block, got %d blocks", len(doc.ContentBlocks))
	}
	if len(doc.ContentBlocks[0].Elements) != 1 {
		t.Error("the orphan section's content was dropped along with the error")
	}
}

func TestDocumentStrictParser_TopLevelContentMustBeASection(t *testing.T) {
	_, diags := parseStrictDoc(t, "# A Markdown heading\n")
	assertOneErrorContaining(t, diags, "unexpected content in strict mode")
}

// La jerarquía se declara con `level:`, no indentando. Decirlo explícito
// evita que el error salga como "propiedad desconocida" del bloque anterior.
func TestDocumentStrictParser_IndentedSectionIsRejected(t *testing.T) {
	_, diags := parseStrictDoc(t, "SECTION \"Parent\"\n\n  SECTION \"Child\"\n")
	msgs := errorMessages(diags)
	if len(msgs) == 0 {
		t.Fatal("expected an error for an indented SECTION")
	}
	if !strings.Contains(msgs[0], "must start at column 1") {
		t.Errorf("expected the error to name the real problem, got: %s", msgs[0])
	}
}

// Espeja la guarda del parser de slides (issue #45): "SECTIONfoo" pasa un
// HasPrefix pelado pero no abre una sección, y tratarlo como si lo hiciera
// es la receta del loop infinito.
func TestDocumentStrictParser_KeywordRequiresWordBoundary(t *testing.T) {
	_, diags := parseStrictDoc(t, "SECTIONfoo\n")
	assertOneErrorContaining(t, diags, "unexpected content in strict mode")
}

// Una sección declarada sin cuerpo se conserva. Es la divergencia
// deliberada con flex, que descarta un `#` sin elementos: acá el autor la
// escribió a propósito.
func TestDocumentStrictParser_EmptySectionIsKept(t *testing.T) {
	doc, diags := parseStrictDoc(t, "SECTION \"Intro\"\n\nSECTION \"Empty\"\n\nSECTION \"Last\"\n")
	assertNoErrors(t, diags)

	if len(doc.ContentBlocks) != 3 {
		t.Fatalf("expected all 3 declared sections to survive, got %d", len(doc.ContentBlocks))
	}
	if doc.ContentBlocks[1].Title != "Empty" {
		t.Errorf("expected the empty section to keep its title, got %q", doc.ContentBlocks[1].Title)
	}
}

// Un título con comillas adentro sobrevive sin necesitar escapes: el parser
// corta en la ÚLTIMA comilla, no en la segunda.
func TestDocumentStrictParser_QuotedTitleWithInnerQuotes(t *testing.T) {
	doc, diags := parseStrictDoc(t, "SECTION \"El informe \"final\"\"\n")
	assertNoErrors(t, diags)
	if want := `El informe "final"`; doc.ContentBlocks[0].Heading != want {
		t.Errorf("got %q, want %q", doc.ContentBlocks[0].Heading, want)
	}
}

func TestDocumentStrictParser_ParsesFrontMatter(t *testing.T) {
	doc, diags := parseStrictDoc(t, `---
title: "Spec"
mode: strict
theme: technical
---

SECTION "Intro"

  TEXT
    Hello.
`)
	assertNoErrors(t, diags)

	if doc.FrontMatter == nil {
		t.Fatal("expected the frontmatter to be attached to the AST")
	}
	if doc.FrontMatter.Mode != "strict" {
		t.Errorf("expected mode strict, got %q", doc.FrontMatter.Mode)
	}
	if doc.FrontMatter.Theme != "technical" {
		t.Errorf("expected theme technical, got %q", doc.FrontMatter.Theme)
	}
	if len(doc.ContentBlocks) != 1 {
		t.Errorf("expected 1 section after the frontmatter, got %d", len(doc.ContentBlocks))
	}
}

// Ningún input puede colgar el parser: toda rama del bucle superior tiene
// que avanzar. Es la misma clase de bug que el fuzzer encontró en el parser
// de slides (issues #45/#155), acá cerrada por construcción.
func TestDocumentStrictParser_AlwaysMakesForwardProgress(t *testing.T) {
	inputs := []string{
		"", "\n", "   ", "SECTION", "SECTION\"", "SECTIONfoo", "  SECTION \"x\"",
		"SECTION \"a\"\n  level:\n", "SECTION \"a\"\n  \n\n", "|", ":::", "@x",
		"SECTION \"a\"\n  level: 2\n  id:\n", "---\n", "---\nmode: strict\n",
	}
	for _, in := range inputs {
		done := make(chan struct{})
		go func() {
			defer close(done)
			NewDocumentStrictParser(in, util.NewNoop()).Parse()
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("parser hung on input %q", in)
		}
	}
}
