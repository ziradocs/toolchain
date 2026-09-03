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

// parseFlexBody parsea un cuerpo flex (sin frontmatter) directo con
// FlexParser, saltándose el normalizador: lo que se prueba acá es la
// gramática, no las reglas de normalización.
func parseFlexBody(t *testing.T, lines ...string) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	return NewFlexParser(strings.Join(lines, "\n"), util.NewNoop()).Parse()
}

func findDiag(diags []diagnostics.Diagnostic, ruleID string) *diagnostics.Diagnostic {
	for i := range diags {
		if diags[i].RuleID == ruleID || diags[i].Code == ruleID {
			return &diags[i]
		}
	}
	return nil
}

// Issue #239: el bloque `---\nlayout: X\n---` se consumía y se tiraba, así
// que en flex todo slide era `title` o `content` — aunque el linter trae 19
// schemas por layout y las plantillas los estilan. No había forma de decir
// "este slide es una comparación".
func TestFlexParser_LayoutBlockTypesTheNextSlide(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: stats", "---",
		"## Metrics", "Texto.",
	)

	if len(astNode.ContentBlocks) != 2 {
		t.Fatalf("bloques = %d, want 2", len(astNode.ContentBlocks))
	}
	if got := astNode.ContentBlocks[1].BlockType; got != "stats" {
		t.Errorf("BlockType = %q, want %q", got, "stats")
	}
	if got := astNode.ContentBlocks[1].Title; got != "Metrics" {
		t.Errorf("Title = %q, want %q", got, "Metrics")
	}
}

// Un `layout:` explícito le gana a la heurística posicional ("solo el primer
// # es title"). Sin esto no habría forma de que un slide del medio fuera de
// título, ni de que el primero no lo fuera.
func TestFlexParser_LayoutBeatsThePositionalHeuristic(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: title", "---",
		"## Cierre",
	)

	if len(astNode.ContentBlocks) != 2 {
		t.Fatalf("bloques = %d, want 2", len(astNode.ContentBlocks))
	}
	second := astNode.ContentBlocks[1]
	if second.BlockType != "title" {
		t.Errorf("BlockType = %q, want %q", second.BlockType, "title")
	}
	// Un slide de título se titula con Heading, que es lo que la plantilla y
	// el schema `title` esperan; con Title quedaría sin heading y LAYOUT001
	// lo marcaría.
	if second.Heading != "Cierre" {
		t.Errorf("Heading = %q, want %q — un layout de título se titula con Heading", second.Heading, "Cierre")
	}
	if second.Title != "" {
		t.Errorf("Title = %q, want vacío", second.Title)
	}
}

// El caso que destapó este feature y que había que arreglar aparte: el
// return final de parseContentBlock era un allowlist de tipos, así que un
// slide con layout propio y sin elementos todavía —un deck a medio
// escribir— se caía del AST sin ningún diagnóstico.
func TestFlexParser_LayoutSlideWithoutElementsSurvives(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: stats", "---",
		"## Métricas", "",
		"---", "layout: comparison", "---",
		"## Comparación", "",
	)

	if len(astNode.ContentBlocks) != 3 {
		t.Fatalf("bloques = %d, want 3 — un slide con heading y sin elementos no puede desaparecer", len(astNode.ContentBlocks))
	}
	for i, want := range []string{"title", "stats", "comparison"} {
		if got := astNode.ContentBlocks[i].BlockType; got != want {
			t.Errorf("bloque %d: BlockType = %q, want %q", i, got, want)
		}
	}
}

// Las llaves que no son `layout` siguen siendo inertes, pero ahora se ven.
// Una sola vez por documento: el corpus repite header:/footer: en cada
// slide, y un aviso por ocurrencia serían decenas por deck.
func TestFlexParser_InertMetadataKeysAreReportedOncePerDocument(t *testing.T) {
	_, diags := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: stats", "header: uno", "---",
		"## A", "Texto.", "",
		"---", "layout: stats", "header: dos", "---",
		"## B", "Texto.",
	)

	n := 0
	for _, d := range diags {
		if d.RuleID == "FLEX002" || d.Code == "FLEX002" {
			n++
			if d.Severity != diagnostics.Info {
				t.Errorf("FLEX002 severity = %v, want Info — las llaves inertes no pueden ensuciar el conteo de warnings", d.Severity)
			}
		}
	}
	if n != 1 {
		t.Errorf("FLEX002 = %d veces, want 1 (una por llave por documento)", n)
	}
}

// Un nombre con forma inválida no se acepta como tipo de bloque. De un
// nombre bien formado pero inexistente se encarga el linter (LAYOUT_UNKNOWN),
// que es quien tiene la lista de schemas.
func TestFlexParser_MalformedLayoutNameIsRejected(t *testing.T) {
	for _, bad := range []string{"2col", "stats!", "con espacio", "", "  "} {
		t.Run(bad, func(t *testing.T) {
			astNode, diags := parseFlexBody(t,
				"# Deck", "",
				"---", "layout: "+bad, "---",
				"## Slide", "Texto.",
			)

			if len(astNode.ContentBlocks) != 2 {
				t.Fatalf("bloques = %d, want 2", len(astNode.ContentBlocks))
			}
			if got := astNode.ContentBlocks[1].BlockType; got != "content" {
				t.Errorf("BlockType = %q, want %q — un nombre inválido se ignora", got, "content")
			}
			// Con valor vacío la línea ni siquiera es una llave `layout`
			// utilizable; lo que importa es que no tipe el bloque.
			if strings.TrimSpace(bad) != "" && findDiag(diags, "FLEX003") == nil {
				t.Errorf("se esperaba FLEX003 para el nombre %q", bad)
			}
		})
	}
}

// Un bloque de metadata sin `layout` no tipa nada, y un `layout` no se
// arrastra más allá del slide al que le toca.
func TestFlexParser_LayoutDoesNotLeakToLaterSlides(t *testing.T) {
	astNode, _ := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: stats", "---",
		"## Con layout", "Texto.", "",
		"## Sin layout", "Texto.",
	)

	if len(astNode.ContentBlocks) != 3 {
		t.Fatalf("bloques = %d, want 3", len(astNode.ContentBlocks))
	}
	if got := astNode.ContentBlocks[1].BlockType; got != "stats" {
		t.Errorf("bloque 1: BlockType = %q, want %q", got, "stats")
	}
	if got := astNode.ContentBlocks[2].BlockType; got != "content" {
		t.Errorf("bloque 2: BlockType = %q, want %q — el layout no se arrastra", got, "content")
	}
}

// El separador de slide de verdad ("---" seguido de un heading) no puede
// confundirse con un bloque de metadata, ni al revés. Es la distinción que
// metadataBlockCloseIndex ya hacía y que este feature no puede romper.
func TestFlexParser_RealSeparatorIsNotAMetadataBlock(t *testing.T) {
	astNode, diags := parseFlexBody(t,
		"# Deck", "", "---", "", "## Segundo", "Texto.",
	)

	if len(astNode.ContentBlocks) != 2 {
		t.Fatalf("bloques = %d, want 2", len(astNode.ContentBlocks))
	}
	if got := astNode.ContentBlocks[1].BlockType; got != "content" {
		t.Errorf("BlockType = %q, want %q", got, "content")
	}
	if d := findDiag(diags, "FLEX002"); d != nil {
		t.Errorf("un separador real no puede reportar llaves inertes: %+v", d)
	}
}

// La mayúscula se tolera a propósito: `layout: Stats` quiere decir `stats`,
// y los nombres de schema son todos minúsculas. Ignorarlo en silencio sería
// peor que normalizarlo.
func TestFlexParser_LayoutNameIsCaseInsensitive(t *testing.T) {
	astNode, diags := parseFlexBody(t,
		"# Deck", "",
		"---", "layout: Stats", "---",
		"## Metrics", "Texto.",
	)

	if got := astNode.ContentBlocks[1].BlockType; got != "stats" {
		t.Errorf("BlockType = %q, want %q", got, "stats")
	}
	if d := findDiag(diags, "FLEX003"); d != nil {
		t.Errorf("la mayúscula no es un nombre inválido: %+v", d)
	}
}
