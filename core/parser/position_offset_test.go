// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// Estos tests fijan el arreglo de issue #245: toda posición del CUERPO que
// un parser escribe en el AST cuenta desde la primera línea del ARCHIVO,
// frontmatter incluido — no desde la primera línea del cuerpo (que es lo
// que hacía antes).
//
// El fixture estándar que comparten T1/T2/T9/T10 es un frontmatter de 4
// líneas (---, mode:, title:, ---), una línea en blanco, y el contenido a
// partir de la línea 6 del archivo.

// TestParse_BodyPositionsAreFileAbsolute es T1: el ejemplo literal del issue
// (slides flex, "- alpha") y su equivalente en los otros tres dialectos.
func TestParse_BodyPositionsAreFileAbsolute(t *testing.T) {
	t.Run("slides strict via Parse", func(t *testing.T) {
		content := "---\n" +
			"mode: strict\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"SLIDE content\n" +
			"  TEXT\n" +
			"    Hello\n"

		astNode, diags := New(util.NewNoop()).Parse(content, "deck.slidelang")
		failOnError(t, diags)
		if len(astNode.ContentBlocks) != 1 {
			t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
		}
		block := astNode.ContentBlocks[0]
		if got := block.Position.Line; got != 6 {
			t.Errorf("block.Position.Line = %d, want 6 (file line of SLIDE)", got)
		}
		if len(block.Elements) != 1 {
			t.Fatalf("block.Elements = %d, want 1", len(block.Elements))
		}
		el := block.Elements[0]
		if got := el.GetPosition().Line; got != 7 {
			t.Errorf("TEXT element position.Line = %d, want 7 (file line of TEXT)", got)
		}
		if got := el.GetPosition().Column; got != 1 {
			t.Errorf("TEXT element position.Column = %d, want 1", got)
		}
	})

	t.Run("slides flex via Parse (issue #245 literal example)", func(t *testing.T) {
		content := "---\n" +
			"mode: flex\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"# Title\n" +
			"- alpha\n"

		astNode, diags := New(util.NewNoop()).Parse(content, "deck.slidelang")
		failOnError(t, diags)
		if len(astNode.ContentBlocks) != 1 {
			t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
		}
		block := astNode.ContentBlocks[0]
		if got := block.Position.Line; got != 6 {
			t.Errorf("block.Position.Line = %d, want 6 (file line of '# Title')", got)
		}
		if len(block.Elements) != 1 {
			t.Fatalf("block.Elements = %d, want 1 (the points list)", len(block.Elements))
		}
		el := block.Elements[0]
		if got := el.GetPosition().Line; got != 7 {
			t.Errorf("points element position.Line = %d, want 7 (file line of '- alpha')", got)
		}
	})

	t.Run("doclang flex via ParseDocument", func(t *testing.T) {
		content := "---\n" +
			"mode: flex\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"# Intro\n" +
			"Hello world.\n"

		astNode, diags := New(util.NewNoop()).ParseDocument(content, "doc.doclang")
		failOnError(t, diags)
		if len(astNode.ContentBlocks) != 1 {
			t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
		}
		block := astNode.ContentBlocks[0]
		if got := block.Position.Line; got != 6 {
			t.Errorf("block.Position.Line = %d, want 6 (file line of '# Intro')", got)
		}
		if len(block.Elements) != 1 {
			t.Fatalf("block.Elements = %d, want 1", len(block.Elements))
		}
		el := block.Elements[0]
		if got := el.GetPosition().Line; got != 7 {
			t.Errorf("prose element position.Line = %d, want 7 (file line of 'Hello world.')", got)
		}
	})

	t.Run("doclang strict via ParseDocument", func(t *testing.T) {
		content := "---\n" +
			"mode: strict\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"SECTION \"Intro\"\n" +
			"  TEXT\n" +
			"    Hello\n"

		astNode, diags := New(util.NewNoop()).ParseDocument(content, "doc.doclang")
		failOnError(t, diags)
		if len(astNode.ContentBlocks) != 1 {
			t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
		}
		block := astNode.ContentBlocks[0]
		if got := block.Position.Line; got != 6 {
			t.Errorf("block.Position.Line = %d, want 6 (file line of SECTION)", got)
		}
		if len(block.Elements) != 1 {
			t.Fatalf("block.Elements = %d, want 1", len(block.Elements))
		}
		el := block.Elements[0]
		if got := el.GetPosition().Line; got != 7 {
			t.Errorf("TEXT element position.Line = %d, want 7 (file line of TEXT)", got)
		}
	})
}

// TestParse_BodyDiagnosticsAreFileAbsolute es T2: los diagnósticos que el
// parser ancla a una línea del CUERPO (no solo las posiciones de los
// elementos del AST) también tienen que contar desde el archivo.
func TestParse_BodyDiagnosticsAreFileAbsolute(t *testing.T) {
	t.Run("strict slides: unexpected content", func(t *testing.T) {
		content := "---\n" +
			"mode: strict\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"garbage line\n"

		_, diags := New(util.NewNoop()).Parse(content, "deck.slidelang")
		d := findDiagContaining(diags, "unexpected content")
		if d == nil {
			t.Fatalf("expected an 'unexpected content' diagnostic, got: %+v", diags)
		}
		if got := d.Position.Line; got != 6 {
			t.Errorf("position.Line = %d, want 6 (file line of 'garbage line')", got)
		}
	})

	t.Run("doclang strict: level 2 cannot be first section", func(t *testing.T) {
		content := "---\n" +
			"mode: strict\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"SECTION \"Sub\"\n" +
			"  level: 2\n"

		_, diags := New(util.NewNoop()).ParseDocument(content, "doc.doclang")
		d := findDiagContaining(diags, "cannot be the first section")
		if d == nil {
			t.Fatalf("expected a 'cannot be the first section' diagnostic, got: %+v", diags)
		}
		if got := d.Position.Line; got != 6 {
			t.Errorf("position.Line = %d, want 6 (file line of SECTION)", got)
		}
	})

	t.Run("flex slides: FLEX001 failsafe (issue #192)", func(t *testing.T) {
		content := "---\n" +
			"mode: flex\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"# Title\n" +
			"\n" +
			"<<charrt>>\n"

		_, diags := New(util.NewNoop()).Parse(content, "deck.slidelang")
		d := findDiag(diags, "FLEX001")
		if d == nil {
			t.Fatalf("expected a FLEX001 diagnostic, got: %+v", diags)
		}
		if got := d.Position.Line; got != 8 {
			t.Errorf("position.Line = %d, want 8 (file line of '<<charrt>>')", got)
		}
	})

	t.Run("flex slides: FLEX002 unknown layout option", func(t *testing.T) {
		content := "---\n" +
			"mode: flex\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"# Title\n" +
			"\n" +
			"---\n" +
			"layout: stats\n" +
			"bogus: 1\n" +
			"---\n" +
			"## Metrics\n"

		_, diags := New(util.NewNoop()).Parse(content, "deck.slidelang")
		d := findDiag(diags, "FLEX002")
		if d == nil {
			t.Fatalf("expected a FLEX002 diagnostic, got: %+v", diags)
		}
		if got := d.Position.Line; got != 10 {
			t.Errorf("position.Line = %d, want 10 (file line of 'bogus: 1')", got)
		}
	})
}

// TestParse_FrontMatterDiagnosticsStayFileAbsolute es T3: los diagnósticos
// que pertenecen al FRONTMATTER en sí (FRONT001/002, el delimitador
// faltante) siempre fueron relativos al archivo — frontmatter.go no lo toca
// este fix — pero no había ningún test que lo fijara. Sin este test, un
// offset mal aplicado a esos diagnósticos (sumándolo donde no corresponde)
// pasaría en verde.
func TestParse_FrontMatterDiagnosticsStayFileAbsolute(t *testing.T) {
	t.Run("FRONT001 missing mode, via Parse", func(t *testing.T) {
		content := "---\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"# Title\n" +
			"content\n"

		_, diags := New(util.NewNoop()).Parse(content, "deck.slidelang")
		d := findDiag(diags, "FRONT001")
		if d == nil {
			t.Fatalf("expected a FRONT001 diagnostic, got: %+v", diags)
		}
		if got := d.Position.Line; got != 2 {
			t.Errorf("position.Line = %d, want 2", got)
		}
	})

	t.Run("FRONT002 invalid mode, via Parse", func(t *testing.T) {
		content := "---\n" +
			"mode: nope\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"# Title\n" +
			"content\n"

		_, diags := New(util.NewNoop()).Parse(content, "deck.slidelang")
		d := findDiag(diags, "FRONT002")
		if d == nil {
			t.Fatalf("expected a FRONT002 diagnostic, got: %+v", diags)
		}
		if got := d.Position.Line; got != 2 {
			t.Errorf("position.Line = %d, want 2", got)
		}
	})

	t.Run("FRONT001 missing mode, via ParseDocument", func(t *testing.T) {
		content := "---\n" +
			"title: \"T\"\n" +
			"---\n" +
			"\n" +
			"# Intro\n" +
			"content\n"

		_, diags := New(util.NewNoop()).ParseDocument(content, "doc.doclang")
		d := findDiag(diags, "FRONT001")
		if d == nil {
			t.Fatalf("expected a FRONT001 diagnostic, got: %+v", diags)
		}
		if got := d.Position.Line; got != 2 {
			t.Errorf("position.Line = %d, want 2", got)
		}
	})

	t.Run("Missing FrontMatter delimiter, via Parse", func(t *testing.T) {
		content := "# Title\ncontent\n"

		_, diags := New(util.NewNoop()).Parse(content, "deck.slidelang")
		d := findDiagContaining(diags, "Missing FrontMatter delimiter")
		if d == nil {
			t.Fatalf("expected a 'Missing FrontMatter delimiter' diagnostic, got: %+v", diags)
		}
		if got := d.Position.Line; got != 1 {
			t.Errorf("position.Line = %d, want 1", got)
		}
	})
}

// TestParseDocument_NoFrontMatter_PositionsAreFileAbsolute es T4: un
// documento SIN frontmatter, pasado por la normalización real (mode "").
// Es guardia, no regresión — H1 estableció que en producción el
// normalizador nunca INYECTA ni ALARGA un frontmatter (bodyContentConfig
// filtra toda regla "frontmatter"), así que este escenario no puede exhibir
// el bug que #245 corrige; lo que fija es que, sin frontmatter, bodyOffset
// es 0 y las posiciones ya salían correctas antes del fix también. El
// discriminador real que SÍ distingue "offset explícito" de "offset
// derivado del propio recorte" es T6.
func TestParseDocument_NoFrontMatter_PositionsAreFileAbsolute(t *testing.T) {
	content := "# Title\n\nprose\n"

	astNode, diags := New(util.NewNoop()).ParseDocument(content, "doc.doclang")
	failOnError(t, diags)
	if len(astNode.ContentBlocks) != 1 {
		t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
	}
	block := astNode.ContentBlocks[0]
	if got := block.Position.Line; got != 1 {
		t.Errorf("block.Position.Line = %d, want 1", got)
	}
	if len(block.Elements) != 1 {
		t.Fatalf("block.Elements = %d, want 1", len(block.Elements))
	}
	if got := block.Elements[0].GetPosition().Line; got != 3 {
		t.Errorf("prose element position.Line = %d, want 3", got)
	}
}

// TestParse_FrontMatterWithoutMode_OffsetIsOriginalLength es T5: un
// frontmatter de 3 líneas (sin `mode:`, que dispara FRONT001 y cae a modo
// "auto") sigue produciendo un offset correcto — 3, no 4. Guardia igual que
// T4: el "modo auto" no interactúa con H1, pero confirma que el offset se
// deriva del LARGO REAL del frontmatter (vía bodyLineOffset/EndPosition), no
// de un tamaño fijo asumido.
//
// Normalización deshabilitada a propósito: el modo "auto" corre su propia
// detección/normalización EN EL CUERPO (createAutoParser →
// normalize.ProcessContent con ContentModeFull) — un camino DISTINTO del
// ProcessWithDetection compartido que H1 verificó que nunca toca el
// frontmatter. Ese camino de "auto" sí puede insertar contenido sintético
// en el cuerpo (metadata inferida), lo cual desplazaría "# Title" por una
// razón ajena a #245 — la misma clase de deriva pre-existente que
// include.Expand, documentada como fuera de alcance. Deshabilitar
// normalización aísla lo que este test SÍ debe fijar: el offset del
// frontmatter en sí.
func TestParse_FrontMatterWithoutMode_OffsetIsOriginalLength(t *testing.T) {
	content := "---\n" +
		"title: \"T\"\n" +
		"---\n" +
		"\n" +
		"# Title\n"

	p := New(util.NewNoop())
	p.SetNormalization(false)
	astNode, diags := p.Parse(content, "deck.slidelang")
	if d := findDiag(diags, "FRONT001"); d == nil {
		t.Fatalf("expected FRONT001 (no mode declared), got: %+v", diags)
	}
	if len(astNode.ContentBlocks) != 1 {
		t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
	}
	if got := astNode.ContentBlocks[0].Position.Line; got != 5 {
		t.Errorf("block.Position.Line = %d, want 5 (file line of '# Title')", got)
	}
}

// TestDocumentParsers_ExplicitOffsetWinsOverOwnStrip es T6, el
// discriminador real de #245 (H1): un offset explícito, pasado por
// newDocumentFlexParserAt/newDocumentStrictParserAt, tiene que ganarle a lo
// que el parser derivaría de su PROPIO recorte del frontmatter si se lo
// llamara con el documento completo. Es el único test que efectivamente
// falla si alguien vuelve a derivar el offset del recorte normalizado en
// vez de honrar el explícito — T4/T5 no pueden hacerlo porque ese camino es
// inalcanzable en producción (bodyContentConfig filtra las reglas de
// frontmatter), así que solo un test white-box, construyendo el parser
// directo con un offset deliberadamente DISTINTO del que su propio strip
// calcularía, lo ejercita.
func TestDocumentParsers_ExplicitOffsetWinsOverOwnStrip(t *testing.T) {
	// El frontmatter propio de este input mide 3 líneas (---, mode: flex,
	// ---), así que NewDocumentFlexParser (offset derivado de su propio
	// recorte) reporta "# T" en la línea de body 1 + 3 = 4. Un offset
	// explícito de 10 tiene que ganarle a eso: 10 + 0 + 1 = 11.
	input := "---\nmode: flex\n---\n# T\n"

	t.Run("flex", func(t *testing.T) {
		explicit := newDocumentFlexParserAt(input, 10, util.NewNoop())
		astNode, diags := explicit.Parse()
		failOnError(t, diags)
		if len(astNode.ContentBlocks) != 1 {
			t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
		}
		if got := astNode.ContentBlocks[0].Position.Line; got != 11 {
			t.Errorf("with explicit offset: block.Position.Line = %d, want 11", got)
		}

		derived := NewDocumentFlexParser(input, util.NewNoop())
		astNode2, diags2 := derived.Parse()
		failOnError(t, diags2)
		if len(astNode2.ContentBlocks) != 1 {
			t.Fatalf("ContentBlocks = %d, want 1", len(astNode2.ContentBlocks))
		}
		if got := astNode2.ContentBlocks[0].Position.Line; got != 4 {
			t.Errorf("with own-strip-derived offset: block.Position.Line = %d, want 4", got)
		}
	})

	t.Run("strict", func(t *testing.T) {
		strictInput := "---\nmode: strict\n---\nSECTION \"T\"\n"

		explicit := newDocumentStrictParserAt(strictInput, 10, util.NewNoop())
		astNode, diags := explicit.Parse()
		failOnError(t, diags)
		if len(astNode.ContentBlocks) != 1 {
			t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
		}
		if got := astNode.ContentBlocks[0].Position.Line; got != 11 {
			t.Errorf("with explicit offset: block.Position.Line = %d, want 11", got)
		}

		derived := NewDocumentStrictParser(strictInput, util.NewNoop())
		astNode2, diags2 := derived.Parse()
		failOnError(t, diags2)
		if len(astNode2.ContentBlocks) != 1 {
			t.Fatalf("ContentBlocks = %d, want 1", len(astNode2.ContentBlocks))
		}
		if got := astNode2.ContentBlocks[0].Position.Line; got != 4 {
			t.Errorf("with own-strip-derived offset: block.Position.Line = %d, want 4", got)
		}
	})
}

// TestNewFlexParser_WholeDocumentStaysFileAbsolute es T7: NewFlexParser
// llamado con el documento COMPLETO (frontmatter incluido, el uso que hace
// cualquier caller externo o un test que no pase por Parser.Parse) ya era
// exacto antes de #245 — parseFrontMatter avanza currentLine sobre el mismo
// array en vez de re-cortarlo — y tiene que seguir siéndolo: lineOffset es
// 0 en este camino, así que sumarlo no debe doblar el conteo.
func TestNewFlexParser_WholeDocumentStaysFileAbsolute(t *testing.T) {
	content := "---\nmode: flex\n---\n\n# T\n- alpha\n"

	astNode, diags := NewFlexParser(content, util.NewNoop()).Parse()
	failOnError(t, diags)
	if len(astNode.ContentBlocks) != 1 {
		t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
	}
	block := astNode.ContentBlocks[0]
	if got := block.Position.Line; got != 5 {
		t.Errorf("block.Position.Line = %d, want 5 (file line of '# T')", got)
	}
	if len(block.Elements) != 1 {
		t.Fatalf("block.Elements = %d, want 1", len(block.Elements))
	}
	if got := block.Elements[0].GetPosition().Line; got != 6 {
		t.Errorf("points element position.Line = %d, want 6 (file line of '- alpha')", got)
	}
}

// TestNewDocumentFlexParserWithNormalization_PositionsAreFileAbsolute es T8:
// el constructor que usa el build WASM de slidelang (el único caller externo
// de NewDocumentFlexParserWithNormalization) también tiene que devolver
// posiciones file-absolute — capturando el offset ANTES de que la
// normalización reasigne `input`.
func TestNewDocumentFlexParserWithNormalization_PositionsAreFileAbsolute(t *testing.T) {
	parser := NewDocumentFlexParserWithNormalization(unindentedMermaidDoc, util.NewNoop())
	if !parser.normalized {
		t.Fatal("precondición: se esperaba que este fixture disparara la normalización (ver normalize_dialect_test.go)")
	}

	astNode, diags := parser.Parse()
	failOnError(t, diags)

	lines := strings.Split(unindentedMermaidDoc, "\n")
	wantLine := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "# ") {
			wantLine = i + 1
			break
		}
	}
	if wantLine == -1 {
		t.Fatal("fixture sin ningún '# ' — no se puede calcular la línea esperada")
	}

	if len(astNode.ContentBlocks) == 0 {
		t.Fatal("ContentBlocks vacío")
	}
	if got := astNode.ContentBlocks[0].Position.Line; got != wantLine {
		t.Errorf("block.Position.Line = %d, want %d (file line of the first '# ' heading)", got, wantLine)
	}
}

// TestDocumentStrict_HeaderLinePositions es T9: buildHeadingElement recibe
// la posición YA resuelta con el `headerLine` capturado ANTES de que
// parseSection consuma el cuerpo de la sección — necesario porque
// p.currentLine ya avanzó para cuando se construye el heading de un nivel
// 2-6 anidado.
func TestDocumentStrict_HeaderLinePositions(t *testing.T) {
	content := "---\n" +
		"mode: strict\n" +
		"title: \"T\"\n" +
		"---\n" +
		"\n" +
		"SECTION \"Intro\"\n" +
		"  TEXT\n" +
		"    Hello\n" +
		"\n" +
		"SECTION \"Details\"\n" +
		"  level: 2\n"

	astNode, diags := New(util.NewNoop()).ParseDocument(content, "doc.doclang")
	failOnError(t, diags)
	if len(astNode.ContentBlocks) != 1 {
		t.Fatalf("ContentBlocks = %d, want 1 (level-2 SECTION nests under level-1)", len(astNode.ContentBlocks))
	}
	block := astNode.ContentBlocks[0]
	if got := block.Position.Line; got != 6 {
		t.Errorf("block.Position.Line = %d, want 6 (file line of SECTION \"Intro\")", got)
	}
	if len(block.Elements) != 2 {
		t.Fatalf("block.Elements = %d, want 2 (TEXT + nested heading)", len(block.Elements))
	}
	heading := block.Elements[1]
	if got := heading.GetPosition().Line; got != 10 {
		t.Errorf("nested heading position.Line = %d, want 10 (file line of SECTION \"Details\")", got)
	}
}

// TestChecklist_EndPositionIsLastConsumedFileLine es T10: la aritmética de
// checklist.go's EndPosition ("línea 1-based de la última consumida", ver
// H2 en el plan) se preserva EXACTA, solo desplazada por el offset — no se
// "corrige" a un valor más intuitivo. Se fija comparando dos frontmatters de
// largo distinto (4 y 5 líneas) sobre el MISMO cuerpo: el delta entre los
// dos EndPosition tiene que ser exactamente el delta de largo del
// frontmatter (1), cualquiera sea el valor absoluto que la aritmética
// interna produzca.
func TestChecklist_EndPositionIsLastConsumedFileLine(t *testing.T) {
	body := "\nSLIDE content\n  CHECKLIST\n    [x] one\n    [ ] two\n"

	checklistOf := func(t *testing.T, frontMatter string) *ast.ChecklistElement {
		t.Helper()
		astNode, diags := New(util.NewNoop()).Parse(frontMatter+body, "deck.slidelang")
		failOnError(t, diags)
		if len(astNode.ContentBlocks) != 1 {
			t.Fatalf("ContentBlocks = %d, want 1", len(astNode.ContentBlocks))
		}
		block := astNode.ContentBlocks[0]
		if len(block.Elements) != 1 {
			t.Fatalf("block.Elements = %d, want 1", len(block.Elements))
		}
		checklist, ok := block.Elements[0].(*ast.ChecklistElement)
		if !ok {
			t.Fatalf("element type = %T, want *ast.ChecklistElement", block.Elements[0])
		}
		return checklist
	}

	fourLines := "---\nmode: strict\ntitle: \"T\"\n---\n"
	fiveLines := "---\nmode: strict\ntitle: \"T\"\nauthor: \"A\"\n---\n"

	shortChecklist := checklistOf(t, fourLines)
	longChecklist := checklistOf(t, fiveLines)

	// Todas las demás posiciones (Position, Items[*].Position) también deben
	// desplazarse por exactamente 1 — no solo EndPosition.
	if delta := shortChecklist.EndPosition.Line - longChecklist.EndPosition.Line; delta != -1 {
		t.Errorf("EndPosition.Line delta = %d, want -1 (frontmatter de 5 líneas vs 4)", delta)
	}
	if delta := shortChecklist.Position.Line - longChecklist.Position.Line; delta != -1 {
		t.Errorf("Position.Line delta = %d, want -1", delta)
	}
	if len(shortChecklist.Items) != 2 || len(longChecklist.Items) != 2 {
		t.Fatalf("Items = %d/%d, want 2/2", len(shortChecklist.Items), len(longChecklist.Items))
	}
	for i := range shortChecklist.Items {
		if delta := shortChecklist.Items[i].Position.Line - longChecklist.Items[i].Position.Line; delta != -1 {
			t.Errorf("Items[%d].Position.Line delta = %d, want -1", i, delta)
		}
	}
}

// findDiagContaining busca el primer diagnóstico cuyo Message contiene
// substr.
func findDiagContaining(diags []diagnostics.Diagnostic, substr string) *diagnostics.Diagnostic {
	for i := range diags {
		if strings.Contains(diags[i].Message, substr) {
			return &diags[i]
		}
	}
	return nil
}

// failOnError falla el test si diags trae algún diagnóstico de severidad
// Error — los tests de este archivo prueban posiciones, no manejo de
// errores, así que un error inesperado es una señal de que el fixture está
// mal armado.
func failOnError(t *testing.T, diags []diagnostics.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("diagnóstico de error inesperado: %+v", d)
		}
	}
}
