// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/util"
)

// Issue #255: las opciones se leen del mismo bloque que el `layout:`.
func TestFlexParser_ReadsLayoutOptions(t *testing.T) {
	// El "# Deck" inicial no es decorativo: NewFlexParser trata un `---` en la
	// primera línea como frontmatter (su contrato público, ver issue #239), así
	// que el bloque de metadata tiene que venir después de algo.
	astNode, _ := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: comparison", "columns: 2", "---",
		"## Comparación", "", "Texto.",
	)

	if len(astNode.ContentBlocks) != 2 {
		t.Fatalf("se esperaban 2 bloques, hay %d", len(astNode.ContentBlocks))
	}
	cfg := astNode.ContentBlocks[1].LayoutConfig
	if cfg == nil {
		t.Fatal("LayoutConfig es nil")
	}
	if cfg.Columns != 2 {
		t.Errorf("Columns = %d, se esperaba 2", cfg.Columns)
	}
}

// El orden dentro del bloque no puede importar: nada obliga a que `layout:`
// venga primero, y por eso la lectura es en dos pasadas.
func TestFlexParser_LayoutOptionBeforeLayoutKey(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"# Deck", "",
		"---", "columns: 3", "layout: comparison", "---",
		"## X", "", "Texto.",
	)

	cfg := astNode.ContentBlocks[1].LayoutConfig
	if cfg == nil || cfg.Columns != 3 {
		t.Fatalf("una opción declarada ANTES de layout: se perdió: %+v", cfg)
	}
}

// Un slide sin opciones no debe arrastrar un objeto vacío por el contrato.
func TestFlexParser_NoOptionsLeavesLayoutConfigNil(t *testing.T) {
	astNode, _ := parseFlexBody(t, "# Deck", "", "---", "layout: comparison", "---", "## X", "", "Texto.")
	if astNode.ContentBlocks[1].LayoutConfig != nil {
		t.Errorf("LayoutConfig debería ser nil sin opciones: %+v", astNode.ContentBlocks[1].LayoutConfig)
	}
}

func TestFlexParser_LayoutOptionDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		lines   []string
		ruleID  string
		wantMsg string
	}{
		{
			"valor fuera de rango",
			[]string{"# Deck", "", "---", "layout: comparison", "columns: 9", "---", "## X", "", "T."},
			"FLEX004", "between 1 and 4",
		},
		{
			"opción de otro layout",
			[]string{"# Deck", "", "---", "layout: hero", "columns: 2", "---", "## X", "", "T."},
			"FLEX002", `not an option of layout "hero"`,
		},
		{
			"typo",
			[]string{"# Deck", "", "---", "layout: comparison", "colums: 2", "---", "## X", "", "T."},
			"FLEX002", "has no effect",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			astNode, diags := parseFlexBody(t, tc.lines...)

			d := findDiag(diags, tc.ruleID)
			if d == nil {
				t.Fatalf("falta %s; diagnósticos: %+v", tc.ruleID, diags)
			}
			if !strings.Contains(d.Message, tc.wantMsg) {
				t.Errorf("mensaje %q no contiene %q", d.Message, tc.wantMsg)
			}
			// Un valor rechazado no se escribe.
			if cfg := astNode.ContentBlocks[1].LayoutConfig; cfg != nil && cfg.Columns != 0 {
				t.Errorf("un valor rechazado llegó al AST: %+v", cfg)
			}
		})
	}
}

// En strict las opciones van como propiedades bajo el SLIDE.
func TestStrictParser_ReadsLayoutOptions(t *testing.T) {
	content := strings.Join([]string{
		"---", "mode: strict", `title: "T"`, "---", "",
		"SLIDE title", `  heading: "H"`, "",
		"SLIDE comparison", `  title: "C"`, "  columns: 2",
		"  TEXT", "    A", "  TEXT", "    B",
	}, "\n")

	astNode, diags := New(util.NewNoop()).Parse(content, "t.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("error inesperado: %s", d.String())
		}
	}

	var found bool
	for i := range astNode.ContentBlocks {
		if astNode.ContentBlocks[i].BlockType != "comparison" {
			continue
		}
		found = true
		cfg := astNode.ContentBlocks[i].LayoutConfig
		if cfg == nil || cfg.Columns != 2 {
			t.Errorf("LayoutConfig = %+v, se esperaba Columns 2", cfg)
		}
	}
	if !found {
		t.Fatal("no se encontró el slide comparison")
	}
}

// Y una opción que ese layout no acepta sigue siendo un error de strict, pero
// el mensaje ahora dice cuáles sí acepta.
func TestStrictParser_UnknownPropertyNamesTheLayoutOptions(t *testing.T) {
	content := strings.Join([]string{
		"---", "mode: strict", `title: "T"`, "---", "",
		"SLIDE title", `  heading: "H"`, "",
		"SLIDE hero", `  title: "H"`, "  columns: 2",
	}, "\n")

	_, diags := New(util.NewNoop()).Parse(content, "t.slidelang")

	var msg string
	for _, d := range diags {
		if d.IsError() && strings.Contains(d.Message, "columns") {
			msg = d.Message
		}
	}
	if msg == "" {
		t.Fatalf("se esperaba un error nombrando columns; diagnósticos: %+v", diags)
	}
	if !strings.Contains(msg, "align") {
		t.Errorf("el mensaje no dice qué opciones acepta hero: %q", msg)
	}
}

// Una opción mal puesta tiene que reportarse en CADA slide donde aparece.
//
// El parser deduplica las llaves inertes por nombre —`header:`/`footer:` viven
// en 17 ejemplos cada una y repetirlas llenaría el reporte de ruido—, pero el
// mensaje de una opción de layout nombra el layout de ESE slide. Con la
// deduplicación puesta, el segundo slide quedaba sin una sola línea: el
// parser tampoco escribe la opción, así que el AST no tiene LayoutConfig y el
// linter no tiene nada que mirar. Silencio completo, en el caso exacto que
// #255 existe para atrapar.
func TestFlexParser_InapplicableOptionIsReportedOnEverySlide(t *testing.T) {
	_, diags := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: hero", "columns: 2", "---",
		"## A", "", "Texto.", "",
		"---", "layout: testimonial", "columns: 3", "---",
		"## B", "", "Texto.",
	)

	var reported []string
	for _, d := range diags {
		if strings.Contains(d.Message, `"columns"`) {
			reported = append(reported, d.Message)
		}
	}
	if len(reported) != 2 {
		t.Fatalf("se esperaban 2 diagnósticos por `columns`, hay %d: %v", len(reported), reported)
	}
	if !strings.Contains(reported[0], "hero") {
		t.Errorf("el primero no nombra el layout del primer slide: %q", reported[0])
	}
	if !strings.Contains(reported[1], "testimonial") {
		t.Errorf("el segundo no nombra el layout del segundo slide: %q", reported[1])
	}
}

// Las llaves que NO son opciones de layout sí se siguen deduplicando: es para
// lo que existe el mapa, y subirlas a un diagnóstico por slide llenaría de
// ruido decks que hoy compilan limpios.
func TestFlexParser_InertKeyIsReportedOnce(t *testing.T) {
	_, diags := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: comparison", "header: A", "---",
		"## A", "", "Texto.", "",
		"---", "layout: comparison", "header: B", "---",
		"## B", "", "Texto.",
	)

	n := 0
	for _, d := range diags {
		if strings.Contains(d.Message, `"header"`) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("se esperaba 1 diagnóstico por `header`, hay %d", n)
	}
}

// Un segundo bloque de metadata no puede borrar las opciones del primero.
//
// Dos bloques seguidos son legales, y la asignación de lo pendiente era
// incondicional: el segundo pisaba con un Config vacío opciones bien escritas,
// en el bloque correcto, sobre un layout que las acepta — y desaparecían sin
// un solo diagnóstico.
func TestFlexParser_SecondMetadataBlockDoesNotEraseOptions(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: comparison", "columns: 3", "---",
		"---", "foo: bar", "---",
		"## X", "", "Texto.",
	)

	last := astNode.ContentBlocks[len(astNode.ContentBlocks)-1]
	if last.BlockType != "comparison" {
		t.Fatalf("BlockType = %q, se esperaba comparison", last.BlockType)
	}
	if last.LayoutConfig == nil {
		t.Fatal("LayoutConfig es nil: el segundo bloque de metadata borró las opciones del primero")
	}
	if last.LayoutConfig.Columns != 3 {
		t.Errorf("Columns = %d, se esperaba 3", last.LayoutConfig.Columns)
	}
}

// Y un segundo bloque que SÍ declara layout se queda con el suyo: las opciones
// pertenecen al layout del bloque que las declaró, no se arrastran.
func TestFlexParser_SecondMetadataBlockWithLayoutReplacesOptions(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: comparison", "columns: 3", "---",
		"---", "layout: stats", "---",
		"## X", "", "Texto.",
	)

	last := astNode.ContentBlocks[len(astNode.ContentBlocks)-1]
	if last.BlockType != "stats" {
		t.Fatalf("BlockType = %q, se esperaba stats", last.BlockType)
	}
	if last.LayoutConfig != nil {
		t.Errorf("LayoutConfig = %+v, se esperaba nil: el layout nuevo no declaró opciones", last.LayoutConfig)
	}
}
