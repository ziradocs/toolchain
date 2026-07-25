// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

func parseDocument(t *testing.T, content string) (*ast.AST, error) {
	t.Helper()
	p := parser.NewDocumentFlexParserWithNormalization(content, util.NewNoop())
	doc, diags := p.Parse()
	for _, d := range diags {
		if d.Severity == "error" {
			return doc, &parseErr{msg: d.Message}
		}
	}
	return doc, nil
}

type parseErr struct{ msg string }

func (e *parseErr) Error() string { return e.msg }

// knownNormalizerBugs documentó (issue #204) 2 fixtures cuyo texto
// reformateado era válido/canónico pero corrompía en el REPARSE por bugs
// pre-existentes del AI normalizer compartido
// (internal/normalize/normalizer/rules/*), no del formatter. Ambos quedaron
// arreglados y el mapa está vacío — se deja declarado (en vez de borrar el
// mecanismo) porque el harness de round-trip sobre el corpus completo es la
// forma más barata de detectar la PRÓXIMA regresión de este tipo:
//
//   - webp_test.doclang: un bloque ```chart/```map con JSON "clave": valor
//     era reescrito como tabla Markdown por
//     rules/enhancement/tables.go (TablesRule.Apply), cuyo guard contra
//     bloques especiales (isInSpecialBlock) no reconocía code fences
//     (```). Arreglado: tables.go ahora rastrea el estado de fence (abierto
//     / cerrado) al escanear líneas vía TablesRule.isInCodeFence y trata
//     cualquier línea dentro de un fence abierto como "bloque especial",
//     sin importar el lenguaje declarado tras los backticks.
//
//   - maps_offline_test.doclang: un <<map type=... zoom=...>> con varias
//     líneas "marker: lat, lng, ..." en la PRIMERA de tres secciones "##"
//     del documento perdía markers completos (3 de 4) en el parseo
//     original, y el marker restante perdía su campo lng en el reparse
//     (quedaba en 0). Root cause real (no lo que sugería el comentario
//     original de "3+ mapas consecutivos"): YamlEscapingRule.Apply
//     (rules/frontmatter/yaml_escaping.go) buscaba los primeros DOS "---"
//     en CUALQUIER parte del documento como delimitadores de frontmatter,
//     en vez de exigir que el primero fuera la línea 0 del documento (la
//     semántica correcta, ya usada por
//     base.DocumentAnalyzer.SkipFrontmatter y
//     base.FrontmatterParser.HasFrontmatter). Este fixture no tiene
//     frontmatter real — empieza con "# Maps Offline..." — así que los
//     primeros dos separadores de sección "---" del body (antes y después
//     de la primera sección "##") eran tratados como si delimitaran un
//     frontmatter YAML, y cada línea "key: value" dentro (incluyendo
//     "marker: lat, lng, ..." y, en el reparse, "lng: ...") se le aplicaba
//     el escaping de frontmatter. Un bug secundario en
//     needsEscaping/numericPrefixWithTextPattern (cualquier decimal
//     positivo como "40.7128" o "151.2093" matchea `\d+[^\d\s]` porque el
//     punto decimal cuenta como "texto tras dígitos") causaba que esas
//     líneas se envolvieran entre comillas y escaparan sus comillas
//     internas, dejando valores que ni parseInlineMarker ni
//     parseLatLng/strconv.ParseFloat podían interpretar (marker
//     descartado, o lng->0.0 por el fallback de error de ParseFloat).
//     Arreglado acotando YamlEscapingRule.Apply a exigir
//     TrimSpace(lines[0]) == "---" antes de buscar el cierre del
//     frontmatter; el bug de numericPrefixWithTextPattern con decimales no
//     se tocó (no se manifiesta si nunca se entra al bloque de
//     frontmatter con contenido que no lo es) y queda fuera de este fix
//     por ser una preocupación separada dentro de frontmatter real.
//
// Es un gap del normalizer, no del dialecto de fmt — fuera del scope de
// esta feature (ver el comentario sobre "no expandir el parser" en
// formatStrictElement). Documentado aquí en vez de silenciado: si se
// reintroduce un bug de este tipo, este mapa es el lugar para anotarlo de
// nuevo mientras se investiga.
var knownNormalizerBugs = map[string]string{}

func TestFormatDocument_RoundTrip_Corpus(t *testing.T) {
	root := "../../examples"
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".doclang" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if len(files) == 0 {
		t.Fatal("no .doclang fixtures found under examples/")
	}

	for _, f := range files {
		if reason, skip := knownNormalizerBugs[filepath.Base(f)]; skip {
			t.Run(f, func(t *testing.T) { t.Skip(reason) })
			continue
		}
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		doc, perr := parseDocument(t, string(content))
		if perr != nil {
			continue // fixture con errores de parseo, fuera del scope de este harness
		}

		t.Run(f, func(t *testing.T) {
			out, err := FormatDocument(doc)
			if err != nil {
				var uerr *UnsupportedElementError
				if reflect.TypeOf(err) == reflect.TypeOf(uerr) {
					t.Skipf("elemento no soportado por el dialecto de DocLang: %v", err)
				}
				t.Fatalf("FormatDocument: %v", err)
			}

			reparsed, perr := parseDocument(t, out)
			if perr != nil {
				tag := strings.ReplaceAll(filepath.Base(f), string(filepath.Separator), "_")
				_ = os.WriteFile("/tmp/rtd_"+tag+"_out.doclang", []byte(out), 0644)
				t.Fatalf("el output formateado no re-parsea para %s: %v (wrote /tmp/rtd_%s_out.doclang)", f, perr, tag)
			}
			want := normalizeForComparison(doc)
			got := normalizeForComparison(reparsed)
			if !reflect.DeepEqual(want, got) {
				wantJSON, gotJSON := toJSON(t, want), toJSON(t, got)
				tag := strings.ReplaceAll(filepath.Base(f), string(filepath.Separator), "_")
				_ = os.WriteFile("/tmp/rtd_"+tag+"_want.json", []byte(wantJSON), 0644)
				_ = os.WriteFile("/tmp/rtd_"+tag+"_got.json", []byte(gotJSON), 0644)
				_ = os.WriteFile("/tmp/rtd_"+tag+"_out.doclang", []byte(out), 0644)
				t.Fatalf("round-trip AST mismatch for %s (wrote /tmp/rtd_%s_{want,got}.json, /tmp/rtd_%s_out.doclang)", f, tag, tag)
			}

			out2, err := FormatDocument(reparsed)
			if err != nil {
				t.Fatalf("FormatDocument (2nd pass): %v", err)
			}
			if out != out2 {
				t.Fatalf("fmt no es idempotente para %s\n--- 1st ---\n%s\n--- 2nd ---\n%s", f, out, out2)
			}

			out3, err := FormatDocument(doc)
			if err != nil {
				t.Fatalf("FormatDocument (3rd pass): %v", err)
			}
			if out != out3 {
				t.Fatalf("fmt no es determinista para %s", f)
			}
		})
	}
}

// TestFormatDocument_TableWithLabel_NotFlattenedToPipe is the direct
// regression for a code-review finding: doclang's TableElement case used to
// check only Caption before falling back to formatPipeTable, so a table
// with a Label (issue #239's cross-reference anchor) but no Caption was
// silently downgraded to the pipe form on `doclang fmt --write`, which has
// no way to carry Label — the label just disappeared, with no error.
func TestFormatDocument_TableWithLabel_NotFlattenedToPipe(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Headers = []string{"Q1", "Q2"}
	table.Rows = [][]string{{"100", "200"}}
	table.Cells = ast.DeriveCellsFromFlat(table.Headers, table.Rows)
	table.Label = "tbl:sales"

	block := ast.NewContentBlock(pos, "content")
	block.Heading = "Sales" // the first block's H1 comes from Heading, not Title (see formatDocumentSection)
	block.Elements = append(block.Elements, table)
	doc := ast.NewAST(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: unexpected error: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "|") {
		t.Fatalf("table with a Label was flattened to the pipe form (label would be lost):\n%s", out)
	}

	reparsed, err := parseDocument(t, out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	got := findFirstTable(t, reparsed)
	if got.Label != "tbl:sales" {
		t.Errorf("Label did not round-trip: got %q, want %q\nformatted:\n%s", got.Label, "tbl:sales", out)
	}
}

// TestFormatDocument_TableWithRowScopedHeader_NotFlattenedToPipe is the
// other half of the same finding: a table with row-scoped headers (a real
// ast.TableElement.Cells shape the pipe/flat Headers-Rows view can't
// express at all, issue #20) must not be silently downgraded to the pipe
// form either — `<th scope="row">` would disappear with no error, exactly
// the accessibility information this feature exists to preserve.
func TestFormatDocument_TableWithRowScopedHeader_NotFlattenedToPipe(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Cells = [][]ast.TableCell{
		{
			{Content: "Region", IsHeader: true, Scope: "col"},
			{Content: "Sales", IsHeader: true, Scope: "col"},
		},
		{
			{Content: "West", IsHeader: true, Scope: "row"},
			{Content: "100"},
		},
	}
	table.Headers, table.Rows = ast.FlattenCellsToRows(table.Cells)

	block := ast.NewContentBlock(pos, "content")
	block.Heading = "Sales" // the first block's H1 comes from Heading, not Title (see formatDocumentSection)
	block.Elements = append(block.Elements, table)
	doc := ast.NewAST(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: unexpected error: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "|") {
		t.Fatalf("table with a row-scoped header was flattened to the pipe form (scope=\"row\" would be lost):\n%s", out)
	}

	reparsed, err := parseDocument(t, out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	got := findFirstTable(t, reparsed)
	found := false
	for _, row := range got.Cells {
		for _, c := range row {
			if c.Content == "West" && c.IsHeader && c.Scope == "row" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf(`scope="row" header did not round-trip: got Cells=%+v`+"\nformatted:\n%s", got.Cells, out)
	}
}

func findFirstTable(t *testing.T, doc *ast.AST) *ast.TableElement {
	t.Helper()
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if table, ok := el.(*ast.TableElement); ok {
				return table
			}
		}
	}
	t.Fatalf("no TableElement found in parsed document")
	return nil
}
