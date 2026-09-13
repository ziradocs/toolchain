// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/formatter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// El corpus de round-trip de core (formatter/document_roundtrip_test.go)
// recorre examples/**/*.doclang, y la salida de `doclang init` no está ahí.
// Por eso las plantillas pudieron shipear durante todo su ciclo de vida
// produciendo documentos que no sobrevivían al propio toolchain, con
// TestInit_TemplateThenBuild en verde: ese test solo exige que el build no
// devuelva error, y las tres corrupciones eran silenciosas.
//
// Lo que se rompía, todo en `doclang build` y no solo en `doclang fmt`:
//
//   - `## 1.2 Scope` y `## 2.2 Components` de la plantilla `technical`
//     salían como texto en negrita, fuera de la jerarquía de headings y
//     fuera del TOC. Cadena completa: el <<mermaid>> con el contenido sin
//     indentar puntuaba 0.8 en el detector de "contenido AI" del
//     normalizador, eso encendía el set entero de reglas, y HeadersRule
//     (heurística de slidelang) degradaba el segundo "##" de cada sección.
//   - El `>>` que cerraba ese <<mermaid>> (y el <<chart>> de `report`) no es
//     un terminador del lenguaje, así que quedaba como una cita anidada y
//     se renderizaba como un "> >" suelto.
//   - `numbering:` encendido sobre títulos que ya traían su número escrito
//     producía "1. 1. Introduction".
//
// Ver el comentario arriba de generateFlexTemplate (init.go) para las reglas
// que las plantillas tienen que respetar.

// zeroPos neutraliza las posiciones al comparar dos ASTs: un reformateo
// mueve líneas y columnas por definición.
var zeroPos = diagnostics.Position{}

// templateCases enumera lo que acepta `doclang init --template` junto con lo
// que se espera de cada plantilla. Las expectativas van acá y no inferidas del
// contenido: un guard que se apaga solo cuando no encuentra nada que revisar
// es exactamente el skip ciego que escondió el issue #78 durante todo su ciclo
// de vida (ver el comentario de allowedUnsupportedInDocument en
// core/formatter/document_roundtrip_test.go). Si `stripFrontMatter` o el regex
// de headings se rompen, esto tiene que fallar, no pasar en verde.
var templateCases = []struct {
	name string
	// hasSubsections: la plantilla usa "##"/"###". Falso solo para `strict`,
	// que declara bloques SECTION en vez de headings Markdown.
	hasSubsections bool
	// selfNumberedHeadings: los títulos ya traen su número escrito
	// ("# 1. Introduction"), así que la plantilla TIENE que declarar
	// `numbering: false` — si no, el build emite "1. 1. Introduction".
	selfNumberedHeadings bool
	// chartTitle: la plantilla trae un <<chart>> y este es el título que
	// tiene que llegar al AST. Vacío = la plantilla no trae chart.
	chartTitle string
}{
	{name: "default", hasSubsections: true},
	{name: "strict"},
	{name: "technical", hasSubsections: true, selfNumberedHeadings: true},
	{name: "report", hasSubsections: true, selfNumberedHeadings: true, chartTitle: "Performance Metrics"},
}

// initTemplate corre `doclang init doc --template tmpl` en un tempdir y
// devuelve el contenido generado.
func initTemplate(t *testing.T, tmpl string) string {
	t.Helper()

	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir failed: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origWd); err != nil {
			t.Fatalf("os.Chdir (restore) failed: %v", err)
		}
	})

	cmd := NewInitCommand()
	cmd.SetArgs([]string{"doc", "--template", tmpl})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doclang init --template %s: %v", tmpl, err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "doc.doclang"))
	if err != nil {
		t.Fatalf("no se pudo leer la plantilla generada: %v", err)
	}
	return string(content)
}

func parseTemplate(t *testing.T, content, label string) *ast.AST {
	t.Helper()
	doc, _ := parseTemplateWithDiagnostics(t, content, label)
	return doc
}

func parseTemplateWithDiagnostics(t *testing.T, content, label string) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	doc, diags := parser.New(util.NewNoop()).ParseDocument(content, "doc.doclang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("%s no parsea: %v\n%s", label, d, content)
		}
	}
	if doc == nil {
		t.Fatalf("%s: ParseDocument devolvió nil", label)
	}
	return doc, diags
}

var headingLineRe = regexp.MustCompile(`(?m)^(#{2,6}) (.+)$`)

// TestInit_TemplatesSurviveTheirOwnToolchain es el test que las tres
// corrupciones tenían que fallar y no fallaban.
func TestInit_TemplatesSurviveTheirOwnToolchain(t *testing.T) {
	for _, tc := range templateCases {
		t.Run(tc.name, func(t *testing.T) {
			content := initTemplate(t, tc.name)
			doc, diags := parseTemplateWithDiagnostics(t, content, "la plantilla "+tc.name)

			assertNoStrayQuotes(t, doc)
			assertSubsectionHeadingsSurvive(t, doc, content, tc.hasSubsections)
			assertNoParserWarnings(t, diags)
			if tc.selfNumberedHeadings {
				assertNumberingDisabled(t, doc)
			}
			if tc.chartTitle != "" {
				assertChartTitle(t, doc, tc.chartTitle)
			}
		})
	}
}

// assertSubsectionHeadingsSurvive exige que cada línea "## "/"### " de la
// plantilla siga siendo un heading en el AST. Es la única forma de ver el
// daño de HeadersRule: ocurre ANTES del primer parseo, así que un harness de
// round-trip (parsear → formatear → reparsear → comparar) lo compara consigo
// mismo y pasa en verde.
func assertSubsectionHeadingsSurvive(t *testing.T, doc *ast.AST, content string, hasSubsections bool) {
	t.Helper()

	var want []string
	for _, m := range headingLineRe.FindAllStringSubmatch(stripFrontMatter(content), -1) {
		want = append(want, strings.TrimSpace(m[2]))
	}
	if !hasSubsections {
		if len(want) > 0 {
			t.Errorf("la plantilla ganó subsecciones (%s) pero templateCases la declara sin ellas — actualizá la tabla", strings.Join(want, ", "))
		}
		return
	}
	if len(want) == 0 {
		t.Fatalf("no se encontró ninguna línea \"## \" en la plantilla, que templateCases declara con subsecciones — "+
			"si la plantilla no cambió, lo roto es stripFrontMatter o headingLineRe, no la plantilla:\n%s", content)
	}

	got := map[string]bool{}
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			text, ok := el.(*ast.TextElement)
			if !ok || !text.IsRawHTML {
				continue
			}
			if inner := headingInnerText(text.Content); inner != "" {
				got[inner] = true
			}
		}
	}

	for _, h := range want {
		if !got[h] {
			t.Errorf("el subtítulo %q dejó de ser un heading al parsear.\n"+
				"Suele ser HeadersRule del normalizador degradándolo a **negrita**, y solo se enciende si el detector "+
				"marca el documento como \"contenido AI\" — revisá que el contenido de todo bloque <<...>> esté indentado "+
				"2 espacios (ver el comentario de generateFlexTemplate en init.go).", h)
		}
	}
}

// assertNoStrayQuotes caza el `>>` suelto: al no ser un terminador del
// lenguaje, el bloque <<...>> termina por dedent y el `>>` sobrante se parsea
// como una cita anidada, o sea un QuoteElement cuyo contenido es exactamente
// ">".
//
// Asertar esa forma exacta, y no "no hay ninguna cita", es deliberado: la
// versión amplia necesitaba un escape hatch ("si la plantilla declara citas,
// no revisar"), y ese hatch se habría apagado solo el día que alguien agregue
// una cita legítima a una plantilla. Una cita cuyo único contenido es ">" no
// es algo que un autor escriba, así que este guard no necesita apagarse nunca.
func assertNoStrayQuotes(t *testing.T, doc *ast.AST) {
	t.Helper()
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			q, ok := el.(*ast.QuoteElement)
			if !ok || strings.Trim(strings.TrimSpace(q.Content), ">") != "" {
				continue
			}
			t.Errorf("quedó un QuoteElement vacío (%q): es un `>>` suelto cerrando un bloque <<...>>, que no es un "+
				"terminador del lenguaje. El terminador es <<end>>.", strings.TrimSpace(q.Content))
		}
	}
}

// assertNumberingDisabled cubre la tercera regla de generateFlexTemplate: una
// plantilla cuyos títulos ya traen su número escrito tiene que declarar
// `numbering: false`, o el build emite "1. 1. Introduction". Sin esta
// aserción, volver `numbering:` a su forma legacy encendida no rompía ningún
// test.
func assertNumberingDisabled(t *testing.T, doc *ast.AST) {
	t.Helper()
	fm := doc.FrontMatter
	if fm == nil {
		t.Fatal("la plantilla no tiene frontmatter")
	}
	if fm.Numbering == nil {
		t.Error("la plantilla no declara `numbering:`, y sus títulos ya traen su propio número — el build va a emitir " +
			"\"1. 1. Introduction\". Declarar `numbering: false`.")
		return
	}
	if *fm.Numbering {
		t.Error("la plantilla declara `numbering: true` sobre títulos que ya traen su propio número — el build emite " +
			"\"1. 1. Introduction\". Debe ser `numbering: false`.")
	}
}

// assertNoParserWarnings exige que una plantilla recién generada parsee
// LIMPIA. Es el guard general contra la clase de defecto que ya apareció tres
// veces en estas plantillas: escribir sintaxis que el parser no lee y que
// desaparece sin que nadie se entere. Cada vez que el parser aprenda a avisar
// de algo nuevo (CHART005 fue el último), este test lo aprovecha gratis.
func assertNoParserWarnings(t *testing.T, diags []diagnostics.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		t.Errorf("la plantilla generó un diagnóstico del parser [%s]: %s\n"+
			"Una plantilla de `init` es lo primero que alguien construye; tiene que parsear limpia.", d.RuleID, d.Message)
	}
}

// assertChartTitle cubre lo que ni el round-trip ni el conteo de headings ven:
// el <<chart>> de `report` declaraba su título como atributo de la apertura
// (`<<chart:bar title="...">>`), que no es sintaxis del lenguaje, así que el
// título no llegaba al AST y el build renderizaba el chart sin título. La
// forma que el parser lee es la llave del cuerpo, `title:`.
func assertChartTitle(t *testing.T, doc *ast.AST, want string) {
	t.Helper()
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			chart, ok := el.(*ast.ChartElement)
			if !ok {
				continue
			}
			if chart.Title != want {
				t.Errorf("el título del chart no llegó al AST: chart.Title = %q, se esperaba %q.\n"+
					"El título va como llave del cuerpo (`title:`), no como atributo de la apertura.", chart.Title, want)
			}
			if len(chart.Data) == 0 {
				t.Error("el chart quedó sin datos: revisá que `data:` esté a la sangría base del bloque y no anidado " +
					"dentro de otra llave.")
			}
			return
		}
	}
	t.Errorf("la plantilla declara un chart con título %q pero no se encontró ningún ChartElement en el AST", want)
}

// TestInit_TemplatesRoundTripThroughFmt cierra la otra mitad: que `fmt` sobre
// una plantilla recién generada no pierda ni cambie nada del AST, y que sea
// idempotente. Espeja TestFormatDocument_RoundTrip_Corpus de core, que no
// puede ver estas plantillas porque solo recorre examples/.
func TestInit_TemplatesRoundTripThroughFmt(t *testing.T) {
	for _, tc := range templateCases {
		tmpl := tc.name
		t.Run(tmpl, func(t *testing.T) {
			content := initTemplate(t, tmpl)
			doc := parseTemplate(t, content, "la plantilla "+tmpl)

			format := formatter.FormatDocument
			if isStrictDocument(doc) {
				format = formatter.FormatDocumentStrict
			}

			out, err := format(doc)
			if err != nil {
				t.Fatalf("format: %v", err)
			}
			reparsed := parseTemplate(t, out, "el resultado de fmt sobre "+tmpl)

			want, got := normalizeAST(doc), normalizeAST(reparsed)
			if !astEqualIgnoringStrippedKeys(t, want, got) {
				t.Errorf("fmt sobre la plantilla %s no round-trippea: el AST reparseado difiere del original\n--- fmt ---\n%s", tmpl, out)
			}

			out2, err := format(reparsed)
			if err != nil {
				t.Fatalf("format (2ª pasada): %v", err)
			}
			if out != out2 {
				t.Errorf("fmt sobre la plantilla %s no es idempotente\n--- 1ª ---\n%s\n--- 2ª ---\n%s", tmpl, out, out2)
			}
		})
	}
}

func isStrictDocument(doc *ast.AST) bool {
	return doc.FrontMatter != nil && doc.FrontMatter.Mode == "strict"
}

// normalizeAST descarta lo que no sobrevive a un round-trip por diseño
// (posiciones, el YAML crudo del frontmatter, y el HTML pre-renderizado que
// el parser cachea) para poder comparar el resto con DeepEqual. Mismo
// criterio que normalizeForComparison en core/formatter.
func normalizeAST(doc *ast.AST) *ast.AST {
	cp := *doc
	cp.Position, cp.EndPosition = zeroPos, zeroPos
	if cp.FrontMatter != nil {
		fm := *cp.FrontMatter
		fm.Position, fm.EndPosition = zeroPos, zeroPos
		fm.Raw = ""
		cp.FrontMatter = &fm
	}
	cp.ContentBlocks = make([]ast.ContentBlock, len(doc.ContentBlocks))
	for i, b := range doc.ContentBlocks {
		nb := b
		nb.Position, nb.EndPosition = zeroPos, zeroPos
		nb.TitleHTML, nb.HeadingHTML, nb.SubtitleHTML = "", "", ""
		nb.Elements = make([]ast.Element, len(b.Elements))
		for j, el := range b.Elements {
			nb.Elements[j] = normalizeElementForCompare(el)
		}
		cp.ContentBlocks[i] = nb
	}
	return &cp
}

func normalizeElementForCompare(el ast.Element) ast.Element {
	switch e := el.(type) {
	case *ast.TextElement:
		c := *e
		c.Position, c.EndPosition = zeroPos, zeroPos
		c.ContentHTML = ""
		return &c
	case *ast.PointsElement:
		c := *e
		c.Position, c.EndPosition = zeroPos, zeroPos
		c.Items = make([]ast.PointItem, len(e.Items))
		for i, it := range e.Items {
			ni := it
			ni.Position, ni.EndPosition = zeroPos, zeroPos
			ni.ContentHTML = ""
			c.Items[i] = ni
		}
		return &c
	case *ast.CodeElement:
		c := *e
		c.Position, c.EndPosition = zeroPos, zeroPos
		return &c
	case *ast.TableElement:
		c := *e
		c.Position, c.EndPosition = zeroPos, zeroPos
		return &c
	case *ast.MermaidElement:
		c := *e
		c.Position, c.EndPosition = zeroPos, zeroPos
		return &c
	case *ast.ChartElement:
		c := *e
		c.Position, c.EndPosition = zeroPos, zeroPos
		return &c
	case *ast.QuoteElement:
		c := *e
		c.Position, c.EndPosition = zeroPos, zeroPos
		return &c
	default:
		return el
	}
}

// astEqualIgnoringStrippedKeys compara a y b ignorando
// TableElement.RowPositions — metadata de posición que core agrega ANTES
// de que este módulo la consuma vía bump, y que por lo tanto puede no
// sobrevivir un round-trip de texto simplemente porque el core resuelto en
// este build (GOWORK=off, la versión PUBLICADA) todavía no la tiene.
//
// Trabaja sobre COPIAS PROFUNDAS de a/b (round-trip por JSON hacia un
// *ast.AST fresco, no hacia un mapa genérico) para no mutar los AST reales
// del llamador — TestInit_TemplatesRoundTripThroughFmt reusa `reparsed`
// para el chequeo de idempotencia después de esta comparación.
// clearRowPositionsForComparison limpia el campo en esas copias antes de
// comparar con reflect.DeepEqual sobre los structs TIPADOS (no JSON
// genérico): dos *ast.AST con el mismo contenido comparan iguales sin
// importar el orden de iteración de sus mapas internos, igual que antes.
//
// Hallazgo de tercera ronda de revisión: una versión anterior de este
// mecanismo (jsonKeyStripApplies) operaba sobre el JSON genérico ya
// decodificado, buscando claves por nombre + una firma estructural
// ("type":"table" + "headers"/"rows" como hermanas) — reducía muchísimo el
// riesgo de falso positivo, pero un objeto de usuario que por coincidencia
// tuviera esa forma exacta (mismos cuatro nombres de clave) seguía
// pudiendo colar un borrado indebido, porque el mecanismo nunca conocía el
// TIPO real del objeto, sólo su forma. Hallazgo de cuarta ronda de
// revisión: reemplazado por completo por esta versión, que recorre el AST
// TIPADO (ast.Walk) y sólo puede tocar un *ast.TableElement real — un
// mapa/objeto arbitrario del documento (una variable de
// FrontMatter.Variables, un parámetro de directiva, una opción de
// chart/map) nunca puede ser confundido con uno, sin importar qué forma
// tenga. RowPositions se limpia por reflect.FieldByName, no por
// `tbl.RowPositions = nil` directo: FieldByName simplemente no encuentra
// el campo (no-op seguro) si el core resuelto todavía no lo tiene, en vez
// de un error de compilación — mismo motivo que motivó el mecanismo
// original, ahora aplicado a la firma TIPADA en vez de al JSON genérico.
func astEqualIgnoringStrippedKeys(t *testing.T, a, b *ast.AST) bool {
	t.Helper()
	aCopy := deepCopyAST(t, a)
	bCopy := deepCopyAST(t, b)
	clearRowPositionsForComparison(aCopy)
	clearRowPositionsForComparison(bCopy)
	return reflect.DeepEqual(aCopy, bCopy)
}

// deepCopyAST produce una copia de doc totalmente independiente (round-trip
// por JSON hacia un *ast.AST fresco — ContentBlock.UnmarshalJSON,
// ast/decode.go, reconstruye el árbol polimórfico de Elements igual que
// cualquier otro consumidor de --format json) para que
// clearRowPositionsForComparison pueda mutarla sin tocar el AST original.
func deepCopyAST(t *testing.T, doc *ast.AST) *ast.AST {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var cp ast.AST
	if err := json.Unmarshal(data, &cp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return &cp
}

// clearRowPositionsForComparison pone en cero TableElement.RowPositions en
// TODO *ast.TableElement de doc, a cualquier profundidad de anidado
// (incluida una tabla dentro de un ::: bloque o una columna de grid,
// ast.Walk desciende a Elements anidados) — ver el comentario de
// astEqualIgnoringStrippedKeys para el porqué de la reflexión en vez de
// una referencia directa al campo.
func clearRowPositionsForComparison(doc *ast.AST) {
	_ = ast.Walk(doc, func(node ast.Node) error {
		tbl, ok := node.(*ast.TableElement)
		if !ok {
			return nil
		}
		f := reflect.ValueOf(tbl).Elem().FieldByName("RowPositions")
		if f.IsValid() && f.CanSet() {
			f.Set(reflect.Zero(f.Type()))
		}
		return nil
	})
}

func stripFrontMatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	if idx := strings.Index(content[4:], "\n---\n"); idx != -1 {
		return content[4+idx+5:]
	}
	return content
}

var subsectionHeadingHTMLRe = regexp.MustCompile(`^<h[2-6][^>]*>(.*?)</h[2-6]>$`)

func headingInnerText(html string) string {
	m := subsectionHeadingHTMLRe.FindStringSubmatch(strings.TrimSpace(html))
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// buildRowPositionsFixture arma un *ast.AST con: una variable de usuario en
// FrontMatter.Variables llamada literalmente "rowPositions" (un dato real
// del documento, no metadata de core), una directiva con un parámetro del
// mismo nombre, y dos tablas — una top-level y otra ANIDADA dentro de un
// ":::bloque" (SpecialBlockElement.Elements, issue #9) — cada una con
// RowPositions poblado si tablePos != nil. rowPositions distinto entre dos
// llamadas (o nil vs. poblado) es la única diferencia que
// astEqualIgnoringStrippedKeys debe ignorar; todo lo demás debe seguir
// siendo sensible.
func buildRowPositionsFixture(t *testing.T, tablePos []diagnostics.Position) *ast.AST {
	t.Helper()

	table := ast.NewTableElement(diagnostics.NewPosition(3, 1))
	table.Headers = []string{"A", "B"}
	table.Rows = [][]string{{"x", "y"}}
	setFieldIfExists(t, table, "RowPositions", tablePos)

	nestedTable := ast.NewTableElement(diagnostics.NewPosition(5, 1))
	nestedTable.Headers = []string{"C", "D"}
	nestedTable.Rows = [][]string{{"1", "2"}}
	setFieldIfExists(t, nestedTable, "RowPositions", tablePos)

	block := ast.NewSpecialBlockElement(diagnostics.NewPosition(4, 1), "info", "")
	setFieldIfExists(t, block, "Elements", []ast.Element{nestedTable})

	directive := ast.NewDirectiveNode(diagnostics.NewPosition(6, 1), "toc")
	directive.Parameters["rowPositions"] = "parámetro arbitrario de una directiva, no relacionado con TableElement"

	return &ast.AST{
		FrontMatter: &ast.FrontMatterNode{
			Mode: "flex",
			Variables: map[string]interface{}{
				"rowPositions": "no soy metadata de core, soy contenido del usuario",
			},
		},
		ContentBlocks: []ast.ContentBlock{
			{Elements: []ast.Element{table, block, directive}},
		},
	}
}

// setFieldIfExists pone obj.<name> = value por reflexión — nunca por
// `obj.RowPositions = ...`/`obj.Elements = ...` directo, que no compilaría
// si el core resuelto en este build (GOWORK=off, la versión PUBLICADA)
// todavía no declara ese campo (RowPositions/#323 y
// SpecialBlockElement.Elements/#324-325 son ambos de esta misma cadena de
// PRs, sin publicar todavía). Si el campo no existe, es un no-op seguro:
// la parte de esta fixture que depende de él simplemente no se ejercita en
// ese build aislado, en vez de romper la compilación — mismo mecanismo y
// mismo motivo que clearRowPositionsForComparison.
func setFieldIfExists(t *testing.T, obj interface{}, name string, value interface{}) {
	t.Helper()
	f := reflect.ValueOf(obj).Elem().FieldByName(name)
	if !f.IsValid() || !f.CanSet() {
		return
	}
	f.Set(reflect.ValueOf(value))
}

// TestAstEqualIgnoringStrippedKeys_OnlyIgnoresRealTableRowPositions cubre
// un hallazgo de cuarta ronda de revisión: el mecanismo anterior
// (jsonKeyStripApplies) borraba "rowPositions" de cualquier objeto JSON
// que tuviera la forma type:"table"+headers+rows, así que un dato de
// usuario que coincidiera exactamente con esa firma —construible, aunque
// improbable— también se habría perdido en la comparación. La versión
// actual recorre el AST TIPADO (ast.Walk) y sólo puede tocar un
// *ast.TableElement real, a cualquier profundidad de anidado (incluida una
// tabla dentro de un ::: bloque).
func TestAstEqualIgnoringStrippedKeys_OnlyIgnoresRealTableRowPositions(t *testing.T) {
	posA := []diagnostics.Position{diagnostics.NewPosition(3, 1)}
	posB := []diagnostics.Position{diagnostics.NewPosition(99, 1)}

	a := buildRowPositionsFixture(t, posA)
	b := buildRowPositionsFixture(t, posB)
	if !astEqualIgnoringStrippedKeys(t, a, b) {
		t.Errorf("dos AST que sólo difieren en RowPositions (tabla top-level Y anidada en un ::: bloque) deberían compararse iguales")
	}

	// Confirmar que la variable de FrontMatter y el parámetro de la
	// directiva, ambos llamados "rowPositions", siguen intactos en las
	// copias (nunca se tocan) y que la comparación sigue siendo SENSIBLE a
	// una diferencia real en cualquiera de los dos.
	c := buildRowPositionsFixture(t, posA)
	c.FrontMatter.Variables["rowPositions"] = "un valor distinto"
	if astEqualIgnoringStrippedKeys(t, a, c) {
		t.Errorf("una diferencia real en FrontMatter.Variables[\"rowPositions\"] no debería ignorarse")
	}

	d := buildRowPositionsFixture(t, posA)
	d.ContentBlocks[0].Elements[2].(*ast.DirectiveNode).Parameters["rowPositions"] = "otro valor"
	if astEqualIgnoringStrippedKeys(t, a, d) {
		t.Errorf("una diferencia real en el parámetro de directiva \"rowPositions\" no debería ignorarse")
	}

	// Sanity check: una diferencia real de CONTENIDO de tabla (no
	// RowPositions) sigue detectándose — si esto también diera "igual",
	// el guard de round-trip habría dejado de servir para algo.
	e := buildRowPositionsFixture(t, posA)
	e.ContentBlocks[0].Elements[0].(*ast.TableElement).Headers = []string{"X", "Y"}
	if astEqualIgnoringStrippedKeys(t, a, e) {
		t.Errorf("una diferencia real en TableElement.Headers no debería ignorarse")
	}
}
