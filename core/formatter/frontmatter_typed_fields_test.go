// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	"reflect"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// Este archivo cubre el issue #230: frontMatterOverrides solo conocía
// mode/title/author/date/theme/lang/variables, así que Numbering,
// HeaderFooter, TOC, Page y Watermark desaparecían del texto emitido —sin
// error— cuando FrontMatterNode.Raw estaba vacío (un nodo armado en código, o
// decodificado desde --format json, donde Raw es json:"-"). Es el mismo hueco
// que ya se había tapado a mano para Lang (frontmatter_lang_test.go); el
// guard estructural está al final del archivo.

func newEmptyRawFM(t *testing.T) *ast.FrontMatterNode {
	t.Helper()
	fm := ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	if fm.Raw != "" {
		t.Fatalf("ast.NewFrontMatterNode ya no arranca con Raw vacío (%q); este archivo entero asume ese punto de partida", fm.Raw)
	}
	return fm
}

// docWithFrontMatter arma el AST mínimo que los dos formatters aceptan.
func docWithFrontMatter(fm *ast.FrontMatterNode) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = fm
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "title")
	block.Heading = "Sección"
	block.Elements = append(block.Elements, ast.NewTextElement(pos, "Contenido."))
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

// TestTypedFrontMatterFieldsSurviveEmptyRaw es el repro directo del issue:
// cada campo tipado, seteado sobre un FrontMatterNode sin Raw, tiene que
// aparecer en el texto que emiten FormatDocument y FormatStrict.
func TestTypedFrontMatterFieldsSurviveEmptyRaw(t *testing.T) {
	opacity := 0.5
	rotation := -30.0

	cases := []struct {
		name string
		set  func(fm *ast.FrontMatterNode)
		want []string
	}{
		{
			name: "numbering false",
			set:  func(fm *ast.FrontMatterNode) { fm.Numbering = boolPtr(false) },
			want: []string{"numbering: false"},
		},
		{
			name: "numbering true",
			set:  func(fm *ast.FrontMatterNode) { fm.Numbering = boolPtr(true) },
			want: []string{"numbering: true"},
		},
		{
			name: "toc enabled y depth",
			set: func(fm *ast.FrontMatterNode) {
				fm.TOC = &ast.TOCConfig{Enabled: boolPtr(true), Depth: intPtr(3)}
			},
			// depth: 3, no "depth: 3.0": asYAMLMap va por yaml y no por JSON
			// justamente para que un int siga siendo int.
			want: []string{"toc:", "enabled: true", "depth: 3"},
		},
		{
			name: "page size y margins por lado",
			set: func(fm *ast.FrontMatterNode) {
				fm.Page = &ast.PageConfig{
					Size:    "A4",
					Margins: &ast.PageMargins{Top: "2cm", Right: "1cm", Bottom: "2cm", Left: "1cm"},
				}
			},
			want: []string{"page:", "size: A4", "margins:", "top: 2cm", "right: 1cm", "bottom: 2cm", "left: 1cm"},
		},
		{
			name: "watermark con enabled false explícito",
			set: func(fm *ast.FrontMatterNode) {
				fm.Watermark = &ast.WatermarkConfig{
					Enabled:  false,
					Text:     "BORRADOR",
					Opacity:  &opacity,
					Rotation: &rotation,
					FontSize: "72pt",
					Repeat:   boolPtr(true),
				}
			},
			// font_size, no fontsize: sin el tag yaml: espejo, yaml.v3
			// emitiría strings.ToLower(field.Name).
			want: []string{"watermark:", "enabled: false", "text: BORRADOR", "font_size: 72pt", "repeat: true", "opacity: 0.5", "rotation: -30"},
		},
		{
			name: "header y footer como llaves de nivel superior",
			set: func(fm *ast.FrontMatterNode) {
				fm.HeaderFooter = &ast.HeaderFooterConfig{
					Header: &ast.HeaderConfig{
						Enabled: true,
						Height:  "60px",
						Text:    &ast.HeaderFooterText{Center: "Título"},
						Logo:    &ast.LogoConfig{Source: "./logo.png", Position: "left"},
					},
					Footer: &ast.FooterConfig{
						Enabled: true,
						PageNumbers: &ast.PageNumbersConfig{
							Enabled:            true,
							Format:             "{{current}} / {{total}}",
							ExcludeTitleSlides: true,
							StartFrom:          2,
						},
					},
				}
			},
			// exclude_title_slides/start_from/page_numbers son los nombres
			// multi-palabra: son la prueba de que los tags yaml: espejo
			// funcionan contra la dependencia real, no contra lo que yaml.v3
			// inventaría solo.
			want: []string{"header:", "footer:", "page_numbers:", "exclude_title_slides: true", "start_from: 2"},
		},
		{
			name: "layout_defaults como llave de nivel superior",
			set: func(fm *ast.FrontMatterNode) {
				fm.HeaderFooter = &ast.HeaderFooterConfig{
					LayoutDefaults: map[string]*ast.LayoutHeaderFooterConfig{
						"title": {Header: &ast.HeaderConfig{Enabled: false}},
					},
				}
			},
			want: []string{"layout_defaults:", "title:", "header:"},
		},
	}

	formatters := []struct {
		name string
		fn   func(*ast.AST) (string, error)
	}{
		{"FormatDocument", FormatDocument},
		{"FormatStrict", FormatStrict},
	}

	for _, tc := range cases {
		for _, f := range formatters {
			t.Run(tc.name+"/"+f.name, func(t *testing.T) {
				fm := newEmptyRawFM(t)
				fm.Mode = "strict"
				tc.set(fm)

				out, err := f.fn(docWithFrontMatter(fm))
				if err != nil {
					t.Fatalf("%s: %v", f.name, err)
				}
				for _, want := range tc.want {
					if !strings.Contains(out, want) {
						t.Errorf("falta %q en el frontmatter emitido:\n%s", want, out)
					}
				}
				// header_footer: no es una llave de frontmatter — el parser
				// arma HeaderFooterConfig desde header/footer/layout_defaults.
				if strings.Contains(out, "header_footer:") {
					t.Errorf("se emitió header_footer:, una clave que el parser nunca lee:\n%s", out)
				}
			})
		}
	}
}

// TestTypedFrontMatterFieldsReparse cierra lo que una aserción de substring no
// puede: que el valor emitido vuelva al MISMO AST. Es lo único que detecta un
// omitempty mal puesto — `toc: false` y `watermark: {enabled: false}` son los
// dos casos donde omitir la llave no rompe el YAML, solo cambia el valor de
// vuelta (watermark arranca en Enabled: true cuando la llave está declarada).
func TestTypedFrontMatterFieldsReparse(t *testing.T) {
	cases := []struct {
		name  string
		set   func(fm *ast.FrontMatterNode)
		check func(t *testing.T, fm *ast.FrontMatterNode)
	}{
		{
			name: "toc enabled false",
			set:  func(fm *ast.FrontMatterNode) { fm.TOC = &ast.TOCConfig{Enabled: boolPtr(false)} },
			check: func(t *testing.T, fm *ast.FrontMatterNode) {
				if fm.TOC == nil || fm.TOC.Enabled == nil || *fm.TOC.Enabled {
					t.Errorf("se perdió `toc: false` en el reparse: %#v", fm.TOC)
				}
			},
		},
		{
			name: "numbering false",
			set:  func(fm *ast.FrontMatterNode) { fm.Numbering = boolPtr(false) },
			check: func(t *testing.T, fm *ast.FrontMatterNode) {
				if fm.Numbering == nil || *fm.Numbering {
					t.Errorf("se perdió `numbering: false` en el reparse: %#v", fm.Numbering)
				}
			},
		},
		{
			name: "watermark enabled false",
			set: func(fm *ast.FrontMatterNode) {
				fm.Watermark = &ast.WatermarkConfig{Enabled: false, Text: "BORRADOR"}
			},
			check: func(t *testing.T, fm *ast.FrontMatterNode) {
				if fm.Watermark == nil {
					t.Fatal("se perdió `watermark:` entero en el reparse")
				}
				if fm.Watermark.Enabled {
					t.Error("`watermark.enabled: false` volvió como true — el omitempty del tag yaml no espeja al json")
				}
				if fm.Watermark.Text != "BORRADOR" {
					t.Errorf("watermark.text = %q, se esperaba %q", fm.Watermark.Text, "BORRADOR")
				}
			},
		},
		{
			name: "page con margins",
			set: func(fm *ast.FrontMatterNode) {
				fm.Page = &ast.PageConfig{Size: "A4", Margins: &ast.PageMargins{Top: "2cm", Right: "2cm", Bottom: "2cm", Left: "2cm"}}
			},
			check: func(t *testing.T, fm *ast.FrontMatterNode) {
				if fm.Page == nil || fm.Page.Size != "A4" || fm.Page.Margins == nil || fm.Page.Margins.Left != "2cm" {
					t.Errorf("se perdió `page:` en el reparse: %#v", fm.Page)
				}
			},
		},
		{
			name: "header y footer completos",
			set: func(fm *ast.FrontMatterNode) {
				fm.HeaderFooter = &ast.HeaderFooterConfig{
					Header: &ast.HeaderConfig{Enabled: true, Text: &ast.HeaderFooterText{Center: "Título"}},
					Footer: &ast.FooterConfig{
						Enabled:     true,
						PageNumbers: &ast.PageNumbersConfig{Enabled: true, ExcludeTitleSlides: true, StartFrom: 2, Format: "{{current}}"},
					},
				}
			},
			check: func(t *testing.T, fm *ast.FrontMatterNode) {
				hf := fm.HeaderFooter
				if hf == nil || hf.Header == nil || hf.Footer == nil {
					t.Fatalf("se perdió header/footer en el reparse: %#v", hf)
				}
				if hf.Header.Text == nil || hf.Header.Text.Center != "Título" {
					t.Errorf("header.text.center no sobrevivió: %#v", hf.Header.Text)
				}
				pn := hf.Footer.PageNumbers
				if pn == nil || !pn.ExcludeTitleSlides || pn.StartFrom != 2 {
					t.Errorf("footer.page_numbers no sobrevivió (¿nombres de llave multi-palabra?): %#v", pn)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm := newEmptyRawFM(t)
			tc.set(fm)

			out, err := FormatDocument(docWithFrontMatter(fm))
			if err != nil {
				t.Fatalf("FormatDocument: %v", err)
			}
			reparsed := reparseFrontMatter(t, out)
			tc.check(t, reparsed)
		})
	}
}

// TestFormatDocument_IdempotentWithEmptyRaw es el guard del helper asYAMLMap.
// header/footer/layout_defaults son fill-if-absent, así que las dos pasadas
// toman caminos distintos: la primera emite el valor desde el struct (orden de
// declaración de campos), la segunda desde el Raw ya reparseado (orden
// alfabético de mapa). Sin normalizar a mapa en la inserción, el mismo AST
// produce dos textos y `fmt` deja de ser idempotente. Ni las aserciones de
// substring ni las de reparse de arriba ven ese bug.
func TestFormatDocument_IdempotentWithEmptyRaw(t *testing.T) {
	fm := newEmptyRawFM(t)
	fm.Title = "Doc"
	fm.HeaderFooter = &ast.HeaderFooterConfig{
		Header: &ast.HeaderConfig{
			Enabled:    true,
			Height:     "60px",
			Background: "#fff",
			Text:       &ast.HeaderFooterText{Center: "Título"},
			Logo:       &ast.LogoConfig{Source: "./logo.png"},
			Border:     &ast.BorderConfig{Enabled: true, Color: "#ccc"},
		},
		Footer: &ast.FooterConfig{
			Enabled:     true,
			Height:      "40px",
			PageNumbers: &ast.PageNumbersConfig{Enabled: true, StartFrom: 2},
		},
		LayoutDefaults: map[string]*ast.LayoutHeaderFooterConfig{
			"title": {Header: &ast.HeaderConfig{Enabled: false}},
		},
	}
	fm.TOC = &ast.TOCConfig{Enabled: boolPtr(true), Depth: intPtr(2)}
	fm.Page = &ast.PageConfig{Size: "A4", Margins: &ast.PageMargins{Top: "2cm"}}
	fm.Numbering = boolPtr(false)
	fm.Watermark = &ast.WatermarkConfig{Enabled: true, Text: "BORRADOR"}

	out, err := FormatDocument(docWithFrontMatter(fm))
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}

	doc2 := parseDocumentOrFail(t, out)
	out2, err := FormatDocument(doc2)
	if err != nil {
		t.Fatalf("FormatDocument (2ª pasada): %v", err)
	}
	if out != out2 {
		t.Fatalf("fmt no es idempotente arrancando de Raw vacío\n--- 1ª ---\n%s\n--- 2ª ---\n%s", out, out2)
	}
}

// TestFormatDocument_PreservesUnmodeledHeaderSubKeys es el guard de la mitad
// "fill-if-absent" de formatFrontMatter. rawHeaderConfig/rawFooterConfig
// decodean a structs tagueados, así que descartan toda sub-llave desconocida
// SIN diagnóstico; el corpus las trae de verdad
// (18.4_headers_footers_advanced_flex.slidelang: logo.link, text.content,
// divider, page_numbers.prefix, social_links). Si header/footer pasaran a
// pisar como el resto de los campos tipados, `fmt` borraría configuración del
// autor y el harness de round-trip seguiría verde: compara ASTs, y estas
// llaves nunca estuvieron en el AST.
func TestFormatDocument_PreservesUnmodeledHeaderSubKeys(t *testing.T) {
	raw := strings.Join([]string{
		"title: Doc",
		"header:",
		"  enabled: true",
		"  divider: true",
		"  logo:",
		"    source: ./logo.png",
		"    link: https://example.com",
		"  text:",
		"    content: \"{{department}}\"",
		"    style: subtitle",
		"footer:",
		"  enabled: true",
		"  social_links: true",
		"  page_numbers:",
		"    enabled: true",
		"    prefix: \"Page \"",
	}, "\n")

	fm := newEmptyRawFM(t)
	fm.Title = "Doc"
	fm.Raw = raw
	// Poblado como lo dejaría el parser: solo lo que el AST modela.
	fm.HeaderFooter = &ast.HeaderFooterConfig{
		Header: &ast.HeaderConfig{Enabled: true, Logo: &ast.LogoConfig{Source: "./logo.png"}, Text: &ast.HeaderFooterText{}},
		Footer: &ast.FooterConfig{Enabled: true, PageNumbers: &ast.PageNumbersConfig{Enabled: true}},
	}

	out, err := FormatDocument(docWithFrontMatter(fm))
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	for _, want := range []string{"divider: true", "link: https://example.com", "content: '{{department}}'", "style: subtitle", "social_links: true", "prefix: 'Page '"} {
		if !strings.Contains(out, want) {
			t.Errorf("se borró la sub-llave no modelada %q del header/footer del autor:\n%s", want, out)
		}
	}
}

func reparseFrontMatter(t *testing.T, content string) *ast.FrontMatterNode {
	t.Helper()
	doc := parseDocumentOrFail(t, content)
	if doc.FrontMatter == nil {
		t.Fatalf("el texto reformateado no tiene frontmatter:\n%s", content)
	}
	return doc.FrontMatter
}

func parseDocumentOrFail(t *testing.T, content string) *ast.AST {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(content, "test.doclang")
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("el texto reformateado no re-parsea: %s\n%s", d.Message, content)
		}
	}
	return doc
}

// frontMatterFieldsNotEmitted documenta, campo por campo, por qué un campo
// exportado de ast.FrontMatterNode deliberadamente no se lee en
// frontMatterOverrides ni en frontMatterFallbacks.
var frontMatterFieldsNotEmitted = map[string]string{
	"BaseNode": "posición/tipo de nodo, no contenido del frontmatter",
	"Raw":      "es la ENTRADA de formatFrontMatter, no un campo a emitir",
	"Mode":     "sale del parámetro `mode` de frontMatterOverrides, no de fm. Consecuencia conocida y fuera del scope del issue #230: FormatDocument pasa \"\", así que un fm.Mode seteado sobre un nodo sin Raw no llega a la salida por ese camino (FormatStrict/FormatDocumentStrict sí fuerzan \"strict\")",
}

// TestFrontMatterOverridesCoverAllTypedFields es el guard estructural contra
// la tercera repetición del mismo bug. Lang fue la primera
// (frontmatter_lang_test.go), Numbering/HeaderFooter/TOC/Page/Watermark la
// segunda (issue #230): en ambos casos un campo nuevo de FrontMatterNode se
// agregó al parser y al renderer, nadie lo agregó a frontMatterOverrides, y
// desaparecía en silencio del texto emitido cuando Raw estaba vacío. Mismo
// patrón que TestFormattersCoverAllElementImplementers (element_coverage_test.go):
// el hueco tiene que fallar un test, no esperar a que alguien inspeccione el
// archivo emitido a mano.
func TestFrontMatterOverridesCoverAllTypedFields(t *testing.T) {
	read, err := frontMatterFieldsRead("frontmatter.go", "frontMatterOverrides", "frontMatterFallbacks")
	if err != nil {
		t.Fatalf("frontMatterFieldsRead: %v", err)
	}
	if len(read) == 0 {
		t.Fatal("no se encontró ningún acceso `fm.X` en frontmatter.go; ¿cambiaron los nombres de las funciones?")
	}

	var missing []string
	fmType := reflect.TypeOf(ast.FrontMatterNode{})
	for i := 0; i < fmType.NumField(); i++ {
		f := fmType.Field(i)
		if f.PkgPath != "" {
			continue // campo privado
		}
		if read[f.Name] {
			continue
		}
		if _, excused := frontMatterFieldsNotEmitted[f.Name]; excused {
			continue
		}
		missing = append(missing, f.Name)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("ast.FrontMatterNode.%s no se emite en frontMatterOverrides/frontMatterFallbacks ni está en frontMatterFieldsNotEmitted.\n"+
			"Ese es exactamente el issue #230: el campo se puede setear en el AST, el formatter no da error, y el valor nunca llega al texto emitido "+
			"cuando Raw está vacío. Agregalo a la función que corresponda (ver el doc comment de formatFrontMatter para el criterio pisa-vs-rellena) "+
			"o documentá acá por qué no va.", strings.Join(missing, ", ast.FrontMatterNode."))
	}

	for name := range frontMatterFieldsNotEmitted {
		if _, ok := fmType.FieldByName(name); !ok {
			t.Errorf("frontMatterFieldsNotEmitted menciona %q, que ya no es un campo de ast.FrontMatterNode — borrá la entrada", name)
		}
	}
}

// frontMatterFieldsRead junta los nombres X de todo selector `fm.X` que
// aparezca en las funciones nombradas de file.
func frontMatterFieldsRead(file string, funcNames ...string) (map[string]bool, error) {
	fset := token.NewFileSet()
	parsed, err := goparser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(funcNames))
	for _, n := range funcNames {
		wanted[n] = true
	}

	read := map[string]bool{}
	found := 0
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*goast.FuncDecl)
		if !ok || !wanted[fn.Name.Name] {
			continue
		}
		found++
		goast.Inspect(fn.Body, func(n goast.Node) bool {
			sel, ok := n.(*goast.SelectorExpr)
			if !ok {
				return true
			}
			if ident, ok := sel.X.(*goast.Ident); ok && ident.Name == "fm" {
				read[sel.Sel.Name] = true
			}
			return true
		})
	}
	if found != len(funcNames) {
		return nil, fmt.Errorf("se buscaron las funciones %s en %s y solo se encontraron %d; ¿las renombraron?", strings.Join(funcNames, "/"), file, found)
	}
	return read, nil
}
