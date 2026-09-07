// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

func toJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}
	return string(b)
}

var zeroPosition diagnostics.Position

// normalizeForComparison despoja del AST todo lo que el texto no retiene
// (posiciones) o que el formatter reconstruye desde campos estructurados en
// vez de copiar verbatim (*HTML, FrontMatter.Raw) — comparar esto crudo con
// reflect.DeepEqual fallaría incluso con un formatter perfecto, porque el
// layout de texto cambió (ver advisor: "AST equality must exclude
// positional metadata").
func normalizeForComparison(doc *ast.AST) *ast.AST {
	cp := *doc
	cp.Position = zeroPosition
	cp.EndPosition = zeroPosition
	if cp.FrontMatter != nil {
		fm := *cp.FrontMatter
		fm.Position = zeroPosition
		fm.EndPosition = zeroPosition
		fm.Raw = ""
		cp.FrontMatter = &fm
	}
	cp.ContentBlocks = make([]ast.ContentBlock, len(doc.ContentBlocks))
	for i, b := range doc.ContentBlocks {
		cp.ContentBlocks[i] = normalizeBlock(b)
	}
	return &cp
}

func normalizeBlock(b ast.ContentBlock) ast.ContentBlock {
	nb := b
	nb.Position = zeroPosition
	nb.EndPosition = zeroPosition
	nb.TitleHTML = ""
	nb.HeadingHTML = ""
	nb.SubtitleHTML = ""
	nb.Elements = make([]ast.Element, len(b.Elements))
	for i, el := range b.Elements {
		nb.Elements[i] = normalizeElement(el)
	}
	return nb
}

func normalizeElement(el ast.Element) ast.Element {
	switch e := el.(type) {
	case *ast.TextElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.ContentHTML = ""
		return &c
	case *ast.PointsElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.Items = normalizePointItems(e.Items)
		return &c
	case *ast.CodeElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.ContentHTML = ""
		return &c
	case *ast.ImageElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.AltHTML, c.CaptionHTML = "", ""
		return &c
	case *ast.TableElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.HeadersHTML, c.RowsHTML, c.CaptionHTML = nil, nil, ""
		return &c
	case *ast.SpecialBlockElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.TitleHTML, c.ContentHTML = "", ""
		return &c
	case *ast.CodeGroupElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		blocks := make([]ast.CodeBlock, len(e.CodeBlocks))
		for i, cb := range e.CodeBlocks {
			cb.LabelHTML, cb.ContentHTML = "", ""
			blocks[i] = cb
		}
		c.CodeBlocks = blocks
		return &c
	case *ast.MermaidElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.TitleHTML = ""
		return &c
	case *ast.PlantUMLElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.TitleHTML = ""
		return &c
	case *ast.ChartElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.TitleHTML = ""
		// RawJSON preserva el byte-order original del texto fuente (encoding/json
		// no reordena claves de un json.RawMessage al reindentar); el formatter
		// canonicaliza vía unmarshal+marshal (orden alfabético). Ambos son el
		// mismo documento JSON — normalizar antes de comparar, igual que con
		// los campos *HTML.
		if len(c.RawJSON) > 0 {
			var v interface{}
			if err := json.Unmarshal(c.RawJSON, &v); err == nil {
				if canon, err := json.Marshal(v); err == nil {
					c.RawJSON = canon
				}
			}
		}
		return &c
	case *ast.MapElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.TitleHTML = ""
		return &c
	case *ast.DirectiveNode:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		return &c
	case *ast.MediaElement:
		// issue #21: no *HTML fields of its own to clear, just position.
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		return &c
	case *ast.QuoteElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.ContentHTML, c.AuthorHTML, c.SourceHTML = "", "", ""
		return &c
	case *ast.ChecklistElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.Items = normalizeChecklistItems(e.Items)
		return &c
	case *ast.GridElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.ContentHTML = ""
		cols := make([]ast.ColumnElement, len(e.Columns))
		for i, col := range e.Columns {
			col.Position, col.EndPosition = zeroPosition, zeroPosition
			col.ContentHTML = ""
			cols[i] = col
		}
		c.Columns = cols
		return &c
	case *ast.MathElement:
		// Sin case, MathElement conservaba su Position igual que quiz/poll: el
		// hueco existía desde el issue #239-B y no se había notado porque
		// ningún fixture del corpus formateaba una ecuación en una posición
		// que el formateo moviera. Lo encontró el test de cobertura de abajo.
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.CaptionHTML = ""
		return &c
	case *ast.QuizElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.QuestionHTML, c.OptionsHTML, c.ExplanationHTML = "", nil, ""
		c.LangRuns, c.DiscardedLangRuns = nil, nil
		return &c
	case *ast.PollElement:
		c := *e
		c.Position, c.EndPosition = zeroPosition, zeroPosition
		c.QuestionHTML, c.OptionsHTML = "", nil
		c.LangRuns, c.DiscardedLangRuns = nil, nil
		return &c
	default:
		// Un tipo sin case conserva su Position, y entonces el round-trip
		// falla por el corrimiento de líneas que el propio formateo produce
		// — no por una pérdida de datos. Pasó al agregar quiz/poll (issue
		// #198): el harness los comparaba con posición y el fallo parecía de
		// contenido. TestNormalizeElementCoversAllImplementers, abajo, es lo
		// que hace que la próxima vez se sepa desde el nombre del test.
		return el
	}
}

func normalizeChecklistItems(items []ast.ChecklistItem) []ast.ChecklistItem {
	out := make([]ast.ChecklistItem, len(items))
	for i, it := range items {
		it.Position, it.EndPosition = zeroPosition, zeroPosition
		it.ContentHTML = ""
		it.SubItems = normalizeChecklistItems(it.SubItems)
		out[i] = it
	}
	return out
}

func normalizePointItems(items []ast.PointItem) []ast.PointItem {
	out := make([]ast.PointItem, len(items))
	for i, it := range items {
		it.Position, it.EndPosition = zeroPosition, zeroPosition
		it.ContentHTML = ""
		it.SubPoints = normalizePointItems(it.SubPoints)
		out[i] = it
	}
	return out
}

func parseStrict(t *testing.T, content string) *ast.AST {
	t.Helper()
	doc, err := tryParseStrict(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return doc
}

// tryParseStrict no falla el test: se usa en el filtrado inicial del
// corpus, que incluye fixtures flex/flex-full (alias deprecado flex-ai)/
// inválidas a propósito fuera del scope de FormatStrict (que solo cubre
// el dialecto strict).
func tryParseStrict(content string) (*ast.AST, error) {
	p := parser.New(util.NewNoop())
	doc, diags := p.Parse(content, "roundtrip.slidelang")
	for _, d := range diags {
		if d.Severity == "error" {
			return nil, fmt.Errorf("%s", d.Message)
		}
	}
	return doc, nil
}

func TestFormatStrict_RoundTrip_Corpus(t *testing.T) {
	root := "../../examples"
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".slidelang" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if len(files) == 0 {
		t.Fatal("no .slidelang fixtures found under examples/")
	}

	tested := 0
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		doc, perr := tryParseStrict(string(content))
		if perr != nil {
			continue // fixture no relevante para el scope de FormatStrict (p.ej. flex-full roto a propósito)
		}
		if doc.FrontMatter == nil || doc.FrontMatter.Mode != "strict" {
			continue // solo el dialecto strict es el scope de FormatStrict
		}

		t.Run(f, func(t *testing.T) {
			out, err := FormatStrict(doc)
			if err != nil {
				var uerr *UnsupportedElementError
				if errors.As(err, &uerr) {
					skipIfDeclaredGap(t, allowedUnsupportedInStrict, uerr)
				}
				t.Fatalf("FormatStrict: %v", err)
			}

			reparsed := parseStrict(t, out)
			want := normalizeForComparison(doc)
			got := normalizeForComparison(reparsed)
			if !reflect.DeepEqual(want, got) {
				wantJSON, gotJSON := toJSON(t, want), toJSON(t, got)
				tag := strings.ReplaceAll(filepath.Base(f), string(filepath.Separator), "_")
				_ = os.WriteFile("/tmp/rt_"+tag+"_want.json", []byte(wantJSON), 0644)
				_ = os.WriteFile("/tmp/rt_"+tag+"_got.json", []byte(gotJSON), 0644)
				_ = os.WriteFile("/tmp/rt_"+tag+"_out.slidelang", []byte(out), 0644)
				t.Fatalf("round-trip AST mismatch for %s (wrote /tmp/rt_%s_{want,got}.json, /tmp/rt_%s_out.slidelang)", f, tag, tag)
			}

			// Idempotencia: Format(Parse(Format(ast))) == Format(ast).
			out2, err := FormatStrict(reparsed)
			if err != nil {
				t.Fatalf("FormatStrict (2nd pass): %v", err)
			}
			if out != out2 {
				t.Fatalf("fmt no es idempotente para %s\n--- 1st ---\n%s\n--- 2nd ---\n%s", f, out, out2)
			}

			// Determinismo: reformatear el MISMO AST debe dar bytes idénticos.
			out3, err := FormatStrict(doc)
			if err != nil {
				t.Fatalf("FormatStrict (3rd pass): %v", err)
			}
			if out != out3 {
				t.Fatalf("fmt no es determinista para %s", f)
			}
			tested++
		})
	}
}

// TestNormalizeElementCoversAllImplementers cierra el hueco que destapó el
// issue #198: normalizeElement es un type switch escrito a mano, y un tipo sin
// case cae al default, que devuelve el elemento CON su Position.
//
// El síntoma es engañoso. El round-trip de corpus compara ASTs completos, así
// que el elemento nuevo falla por el corrimiento de líneas que el propio
// formateo produce —el formatter reescribe el documento y las líneas se
// mueven— y el error se lee como una pérdida de datos que no ocurrió. Se
// perdió un rato entendiendo eso; este test hace que la próxima vez el fallo
// diga el nombre del tipo que falta.
func TestNormalizeElementCoversAllImplementers(t *testing.T) {
	implementers, err := findElementImplementers(filepath.Join("..", "ast"))
	if err != nil {
		t.Fatalf("findElementImplementers: %v", err)
	}
	if len(implementers) == 0 {
		t.Fatal("no se encontró ningún implementador de element() en ../ast")
	}

	cases, err := findSwitchCaseTypes("strict_roundtrip_test.go", "normalizeElement")
	if err != nil {
		t.Fatalf("findSwitchCaseTypes: %v", err)
	}

	// ColumnElement no aparece suelto en block.Elements: vive dentro de
	// GridElement.Columns, y el case de Grid ya normaliza sus columnas.
	skip := map[string]bool{"ColumnElement": true}

	var missing []string
	for name := range implementers {
		if !cases[name] && !skip[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("normalizeElement no tiene case para: %v\n"+
			"→ sin case, el tipo conserva su Position y el round-trip de corpus falla por el "+
			"corrimiento de líneas del formateo, no por una pérdida real de datos", missing)
	}
}
