// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestRenderTableElement_SimpleTable_UnchangedHTML covers issue #20: a table
// with no merged cells (Cells auto-derived by ast.DeriveCellsFromFlat) must
// keep emitting exactly the same byte-for-byte HTML as before this change —
// tableUsesCellStructure must return false for it, so it falls into the
// pre-existing Headers/Rows path, not renderTableCells.
func TestRenderTableElement_SimpleTable_UnchangedHTML(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Headers = []string{"A", "B"}
	table.Rows = [][]string{{"1", "2"}}
	table.Cells = ast.DeriveCellsFromFlat(table.Headers, table.Rows)

	got := renderTableElement(table, nil, nil)
	want := `<table><thead><tr><th scope="col">A</th><th scope="col">B</th></tr></thead><tbody><tr><td>1</td><td>2</td></tr></tbody></table>`
	if got != want {
		t.Errorf("simple table HTML changed:\ngot:  %s\nwant: %s", got, want)
	}
}

// TestRenderTableElement_DoubleBacktickCodeSpan_RendersAsRealCode cubre un
// hallazgo de tercera ronda de revisión: inlineCodePattern (el regex de un
// solo backtick que ProcessInlineMarkdownFormatsSecure usaba para proteger
// code spans) no reconocía un span delimitado por una CORRIDA de 2+
// backticks (la forma CommonMark para meter un "|" o un backtick literal
// adentro, ya soportada del lado del parser/formatter de tablas desde F10)
// — el HTML final dejaba un backtick literal visible a cada lado del
// <code>, en vez de un <code> real. El test anterior de esta misma celda
// (TestTableParser_ParseMarkdownTable_DoubleBacktickSpanWithPipe /
// TestFormatPipeTable_DoubleBacktickSpanPipe_NotEscaped) sólo cubría
// parse/format/reparse, nunca el HTML final.
func TestRenderTableElement_DoubleBacktickCodeSpan_RendersAsRealCode(t *testing.T) {
	table := ast.NewTableElement(diagnostics.NewPosition(1, 1))
	table.Headers = []string{"A", "B"}
	table.Rows = [][]string{{"x", "``a|b``"}}

	got := renderTableElement(table, nil)
	if !strings.Contains(got, "<code>a|b</code>") {
		t.Errorf("renderTableElement con celda \"``a|b``\" = %q, quiere un <code>a|b</code> real, no backticks literales alrededor", got)
	}
	if strings.Contains(got, "`<code>") || strings.Contains(got, "</code>`") {
		t.Errorf("renderTableElement dejó un backtick literal pegado al <code>: %q", got)
	}
}

// TestRenderTableElement_MergedCells_EmitsColspanAndScope covers issue #20:
// a table with Cells declaring colspan/scope must render via
// renderTableCells, emitting the real colspan/scope attributes — something
// the Headers/Rows path can't express.
func TestRenderTableElement_MergedCells_EmitsColspanAndScope(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Cells = [][]ast.TableCell{
		{
			{Content: "A", IsHeader: true, Scope: "col", ColSpan: 2},
			{Content: "B", IsHeader: true, Scope: "col"},
		},
		{
			{Content: "1"}, {Content: "2"}, {Content: "3"},
		},
	}
	table.Headers, table.Rows = ast.FlattenCellsToRows(table.Cells)

	got := renderTableElement(table, nil, nil)

	if !strings.Contains(got, `<th scope="col" colspan="2">A</th>`) {
		t.Errorf("expected merged header cell with colspan=2, got: %s", got)
	}
	if !strings.Contains(got, `<th scope="col">B</th>`) {
		t.Errorf("expected non-merged header cell without colspan, got: %s", got)
	}
	if !strings.Contains(got, "<thead>") || !strings.Contains(got, "<tbody>") {
		t.Errorf("expected <thead>/<tbody> split for a header-led cell grid, got: %s", got)
	}
	if !strings.Contains(got, "<td>1</td><td>2</td><td>3</td>") {
		t.Errorf("expected 3 plain body cells, got: %s", got)
	}
}

// TestRenderTableElement_HeaderCellInBodyRow_UsesCellPath covers the
// tableUsesCellStructure gate fix: a table whose Cells has an IsHeader cell
// inside a body row (no colspan/rowspan, no scope="row") used to fall
// through the narrower gate (which only checked span and scope=="row") into
// the Headers/Rows path, rendering that cell as a plain <td> even though
// elem.Cells (and thus --format json) says isHeader:true — HTML and JSON
// disagreeing, and the accessible <th> silently lost. It must now route
// through renderTableCells and emit <th>.
func TestRenderTableElement_HeaderCellInBodyRow_UsesCellPath(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Cells = [][]ast.TableCell{
		{
			{Content: "Region", IsHeader: true, Scope: "col"},
			{Content: "Sales", IsHeader: true, Scope: "col"},
		},
		{
			{Content: "Total", IsHeader: true},
			{Content: "100"},
		},
	}
	table.Headers, table.Rows = ast.FlattenCellsToRows(table.Cells)

	got := renderTableElement(table, nil, nil)

	if !strings.Contains(got, "<th>Total</th>") {
		t.Errorf("expected the body-row header cell to render as <th>, got: %s", got)
	}
	if strings.Contains(got, "<td>Total</td>") {
		t.Errorf("body-row header cell must not render as <td>: %s", got)
	}
}

// TestRenderTableElement_LeadRowSpanningRowsNoThead covers issue #51: a row 0
// that is entirely IsHeader used to be enough for renderTableCells to wrap it
// in <thead>, even when one of those cells declares RowSpan > 1 — a rowspan
// that reaches into row 1, which this function puts inside <tbody>. That
// crosses the <thead>/<tbody> boundary, which has no consistent
// browser/accessibility-tree interpretation. Same fix and same fixture as
// slidelang's cellsLeadIsHeader (PR #50,
// TestPrepareTemplateData_TableCells_LeadRowSpanningRowsIsFalse), applied to
// core's HTML renderer, which #50 deliberately left out of scope.
func TestRenderTableElement_LeadRowSpanningRowsNoThead(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Cells = [][]ast.TableCell{
		{
			{Content: "Grupo", IsHeader: true, Scope: "col", RowSpan: 2},
			{Content: "Otro", IsHeader: true, Scope: "col"},
		},
		{
			{Content: "a"},
		},
		{
			{Content: "b"}, {Content: "c"},
		},
	}
	table.Headers, table.Rows = ast.FlattenCellsToRows(table.Cells)

	got := renderTableElement(table, nil, nil)

	if strings.Contains(got, "<thead>") {
		t.Errorf("expected no <thead>: row 0's RowSpan=2 header cell reaches into row 1, which sits in <tbody> — a rowspan cannot cross the <thead>/<tbody> boundary, got: %s", got)
	}
	if !strings.Contains(got, `<th scope="col" rowspan="2">Grupo</th>`) {
		t.Errorf("expected row 0's header cell to still render as <th> with its rowspan preserved (falling back to a single <tbody> must not demote it to <td> or drop the rowspan), got: %s", got)
	}
	if !strings.Contains(got, `<th scope="col">Otro</th>`) {
		t.Errorf("expected row 0's non-spanning header cell to still render as <th>, got: %s", got)
	}
}

// TestRenderMediaElement_EmitsAttributes covers issue #21: video/audio must
// emit the right tag and the 4 boolean attributes only when true (no
// attribute by default, no "true"/"false" value — native HTML boolean
// attribute syntax).
func TestRenderMediaElement_EmitsAttributes(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	video := ast.NewMediaElement(pos, "video", "demo.mp4")
	video.Controls = true
	video.Loop = true

	got := renderMediaElement(video, nil, nil)
	want := `<video src="demo.mp4" controls loop></video>`
	if got != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}

	audio := ast.NewMediaElement(pos, "audio", "clip.mp3")
	got = renderMediaElement(audio, nil, nil)
	want = `<audio src="clip.mp3"></audio>`
	if got != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

// TestRenderMediaElement_BlocksDangerousSource covers issue #21: a Source
// with a dangerous scheme (javascript:) must be blocked by SanitizeURL, same
// as renderImageElement — it must never reach the src attribute interpolated.
func TestRenderMediaElement_BlocksDangerousSource(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	media := ast.NewMediaElement(pos, "video", `javascript:alert(document.domain)`)

	got := renderMediaElement(media, nil, nil)
	if strings.Contains(got, "javascript:") {
		t.Errorf("dangerous source was not blocked: %s", got)
	}
	if !strings.Contains(got, "media-error") {
		t.Errorf("expected a media-error fallback for a blocked source, got: %s", got)
	}
	if !strings.Contains(got, "blocked for security") {
		t.Errorf("expected the security-block message for a dangerous scheme, got: %s", got)
	}
}

// TestRenderMediaElement_EmptySourceGetsDistinctMessage covers a fix: an
// empty Source (author never set src) used to fall into the same
// "blocked for security reasons" branch as a scheme SanitizeURL actually
// rejected, misleadingly implying SanitizeURL blocked something when
// nothing was ever provided. It must get its own, non-security message.
func TestRenderMediaElement_EmptySourceGetsDistinctMessage(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	media := ast.NewMediaElement(pos, "video", "")

	got := renderMediaElement(media, nil, nil)
	if !strings.Contains(got, "media-error") {
		t.Errorf("expected a media-error fallback for an empty source, got: %s", got)
	}
	if strings.Contains(got, "blocked for security") {
		t.Errorf("empty source must not be reported as a security block: %s", got)
	}
}

// TestRenderMediaElement_UnknownMediaTypeFallsBackToVideo covers issue #21:
// a MediaType outside the "video"/"audio" allowlist (possible via the JSON
// --filter pipeline, issue #240, not just the parser itself) must fall back
// to the "video" default instead of being interpolated raw as an HTML tag
// name.
func TestRenderMediaElement_UnknownMediaTypeFallsBackToVideo(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	media := ast.NewMediaElement(pos, `script onload=alert(1) x`, "demo.mp4")

	got := renderMediaElement(media, nil, nil)
	if strings.Contains(got, "onload") {
		t.Fatalf("MediaType was interpolated raw as a tag name: %s", got)
	}
	if !strings.HasPrefix(got, "<video ") {
		t.Errorf("expected fallback to <video>, got: %s", got)
	}
}

// TestRenderChartElement_JSONMode_EscapesScriptBreakout cubre issue #19 (CR-1
// del audit de seguridad 2026-07): un chart en modo JSON directo cuyo RawJSON
// contiene un literal "</script>" no debe poder cerrar el <script
// type="application/json"> en el que renderChartBrowser lo embebe e inyectar
// HTML/JS ejecutable.
func TestRenderChartElement_JSONMode_EscapesScriptBreakout(t *testing.T) {
	ctx := &RenderContext{ChartMode: "browser"}

	pos := diagnostics.NewPosition(1, 1)
	chart := ast.NewChartElement(pos, "bar")
	chart.IsJSONMode = true
	chart.RawJSON = json.RawMessage(`{"type":"bar","data":{"labels":["</script><img src=x onerror=alert(document.domain)>"]}}`)

	html := renderChartElement(chart, nil, ctx)

	if strings.Contains(html, "</script><img") {
		t.Fatalf("chart HTML contains an unescaped script-breakout payload:\n%s", html)
	}

	// El JSON embebido debe seguir siendo válido y, al decodificarse, producir
	// el string original intacto (Chart.js debe recibir el dato tal cual).
	start := strings.Index(html, `class="chart-config">`) + len(`class="chart-config">`)
	end := strings.Index(html[start:], "</script>")
	if start < len(`class="chart-config">`) || end < 0 {
		t.Fatalf("could not locate the chart-config <script> block in: %s", html)
	}
	embedded := html[start : start+end]

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(embedded), &decoded); err != nil {
		t.Fatalf("embedded chart config is not valid JSON: %v\n%s", err, embedded)
	}
	labels := decoded["data"].(map[string]interface{})["labels"].([]interface{})
	want := `</script><img src=x onerror=alert(document.domain)>`
	if labels[0] != want {
		t.Errorf("decoded label = %q, want %q (round-trip must preserve the original string)", labels[0], want)
	}
}

// TestGenerateChartConfig_PieDoughnutMoreThanEightSeries cubre issue #244:
// GenerateChartConfigWithMode paniqueaba ("slice bounds out of range") en la
// rama pie/doughnut con más de 8 filas de datos — colors[:len(values)] sobre
// una paleta de 8 colores, con input perfectamente válido. El fix cicla la
// paleta con el módulo (colors[i%len(colors)]), igual que las ramas
// combo/bar/line — este test construye 9 filas (una más que la paleta) para
// que un regreso al slicing directo vuelva a paniquear.
func TestGenerateChartConfig_PieDoughnutMoreThanEightSeries(t *testing.T) {
	for _, chartType := range []string{"pie", "doughnut"} {
		t.Run(chartType, func(t *testing.T) {
			pos := diagnostics.NewPosition(1, 1)
			chart := ast.NewChartElement(pos, chartType)
			chart.Data = make([][]interface{}, 9)
			for i := range chart.Data {
				chart.Data[i] = []interface{}{"Label", float64(i + 1)}
			}

			var config string
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("GenerateChartConfig panicked with %d data rows: %v", len(chart.Data), r)
					}
				}()
				config = GenerateChartConfig(chart)
			}()

			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(config), &decoded); err != nil {
				t.Fatalf("chart config is not valid JSON: %v\n%s", err, config)
			}
			datasets := decoded["data"].(map[string]interface{})["datasets"].([]interface{})
			dataset := datasets[0].(map[string]interface{})
			backgroundColor := dataset["backgroundColor"].([]interface{})
			if len(backgroundColor) != len(chart.Data) {
				t.Errorf("backgroundColor has %d entries, want %d (one per data row, palette cycled)", len(backgroundColor), len(chart.Data))
			}
		})
	}
}

// TestResolveChartJSONMode cubre issue #55: la resolución de un ChartElement
// en modo JSON directo es ahora una única fuente de verdad compartida por
// slidelang (converter.go) y este mismo paquete (renderChartElement,
// usado por doclang) — antes cada uno la reimplementaba de forma
// independiente, y solo una de las dos respetaba RawJSON/IsJSONMode
// correctamente (issue histórico #11).
func TestResolveChartJSONMode(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	t.Run("not JSON mode returns nil without error", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "bar")
		config, chartType, err := ResolveChartJSONMode(chart)
		if err != nil || config != nil || chartType != "" {
			t.Fatalf("got (%v, %q, %v), want (nil, \"\", nil)", config, chartType, err)
		}
	})

	t.Run("invalid RawJSON returns an error", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "bar")
		chart.IsJSONMode = true
		chart.RawJSON = json.RawMessage(`{not valid json`)
		config, _, err := ResolveChartJSONMode(chart)
		if err == nil || config != nil {
			t.Fatalf("got (%v, err=%v), want (nil, non-nil error)", config, err)
		}
	})

	t.Run("type present in JSON is preserved as-is", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "bar")
		chart.IsJSONMode = true
		chart.RawJSON = json.RawMessage(`{"type":"pie","data":{}}`)
		config, chartType, err := ResolveChartJSONMode(chart)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chartType != "pie" || config["type"] != "pie" {
			t.Fatalf("got type=%q config[type]=%v, want \"pie\"", chartType, config["type"])
		}
	})

	t.Run("missing type in JSON falls back to the <<chart: TYPE>> tag", func(t *testing.T) {
		chart := ast.NewChartElement(pos, "bar")
		chart.IsJSONMode = true
		chart.RawJSON = json.RawMessage(`{"data":{}}`)
		config, chartType, err := ResolveChartJSONMode(chart)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chartType != "bar" || config["type"] != "bar" {
			t.Fatalf("got type=%q config[type]=%v, want \"bar\" (from the tag)", chartType, config["type"])
		}
	})
}

// TestMergeChartOptions_AuthorNestedOverrideKeepsSiblingDefaults cubre el bug
// de merge de options descrito junto a issue #11/#55: antes de este fix,
// GenerateChartConfigWithMode mezclaba el `options:` del autor con un merge
// SUPERFICIAL por clave de primer nivel (`options[k] = v`), así que un
// `options: plugins: title: ...` del autor reemplazaba el mapa `plugins`
// entero, borrando cualquier default que el renderer ya hubiera puesto ahí.
// El único default anidado que existe en el camino no-JSON de core es
// plugins.legend, puesto por applyExportOptimizations solo cuando forExport
// es true (GenerateChartConfig sin export nunca puebla `plugins`, así que un
// test sobre ese camino pasaría igual antes y después del fix, sin probar
// nada) — por eso este test usa GenerateChartConfigForExport.
func TestMergeChartOptions_AuthorNestedOverrideKeepsSiblingDefaults(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	chart := ast.NewChartElement(pos, "bar")
	chart.Data = [][]interface{}{{"A", float64(1)}, {"B", float64(2)}}
	chart.Options = map[string]interface{}{
		"plugins": map[string]interface{}{
			"title": map[string]interface{}{
				"display": true,
				"text":    "Custom title",
			},
		},
	}

	config := GenerateChartConfigForExport(chart)

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(config), &decoded); err != nil {
		t.Fatalf("chart config is not valid JSON: %v\n%s", err, config)
	}

	plugins, ok := decoded["options"].(map[string]interface{})["plugins"].(map[string]interface{})
	if !ok {
		t.Fatalf("options.plugins missing or not an object: %v", decoded["options"])
	}

	title, ok := plugins["title"].(map[string]interface{})
	if !ok || title["text"] != "Custom title" {
		t.Fatalf("options.plugins.title = %v, want the author's override to survive", plugins["title"])
	}

	legend, ok := plugins["legend"].(map[string]interface{})
	if !ok || legend["position"] != "top" {
		t.Fatalf("options.plugins.legend = %v, want the applyExportOptimizations default (position: top) to survive the author's plugins.title override", plugins["legend"])
	}
}

// TestRenderTextElement_RawHTMLEscapesVariableValues es una regresión
// encontrada en code-review de la PR de XSS (docs/SECURITY_AUDIT_2026-07.md,
// CR-2): un TextElement crudo (p. ej. un heading de subsección) sustituía
// {{variable}} vía ProcessVariables (sin escapar), así que un valor de
// variable con HTML se inyectaba directo en el <h2>/<h3> del cuerpo del
// documento. renderTextElement ahora usa ProcessVariablesEscapeValues.
func TestRenderTextElement_RawHTMLEscapesVariableValues(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewRawHTMLTextElement(pos, `<h2 id="config">Config {{evil}}</h2>`)

	variables := map[string]interface{}{"evil": "<script>alert(1)</script>"}
	html := renderTextElement(elem, variables)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("raw HTML text element leaked an unescaped variable value:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("expected the variable's value to be HTML-escaped, got:\n%s", html)
	}
	if !strings.HasPrefix(strings.TrimSpace(html), "<h2") {
		t.Errorf("expected the surrounding <h2> HTML to be preserved untouched, got:\n%s", html)
	}
}

// TestRenderMermaidBrowser_EscapesContent cubre issue #73: a diferencia de
// los raster builders de chromium_renderer.go (que ya escapaban el source vía
// EscapeHTML desde PR #67), renderMermaidBrowser emitía el diagrama sin
// escapar en el HTML por defecto de doclang (modo "browser"), un XSS
// zero-interaction en la salida estándar de `doclang build`.
func TestRenderMermaidBrowser_EscapesContent(t *testing.T) {
	payload := `</div><img src=x onerror=alert(document.domain)><script>alert(1)</script>`
	html := renderMermaidBrowser(payload)

	if strings.Contains(html, "<img src=x onerror") || strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("mermaid browser content was not escaped, breakout survived:\n%s", html)
	}
	if !strings.Contains(html, "&lt;img") || !strings.Contains(html, "&lt;/div&gt;") {
		t.Errorf("expected escaped payload to appear as HTML entities, got:\n%s", html)
	}
}

// TestRenderMermaidElement_DefaultModeEscapesContent prueba el sink completo
// alcanzado por el pipeline real de doclang (renderMermaidElement, modo
// "browser" por defecto), no solo la función interna.
func TestRenderMermaidElement_DefaultModeEscapesContent(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	elem := ast.NewMermaidElement(pos, "flowchart", `A["</div><img src=x onerror=alert(document.domain)>"] --> B`)

	// ctx nil -> resolveRenderContext usa el default (MermaidMode "browser")
	html := renderMermaidElement(elem, nil, nil)

	if strings.Contains(html, "<img src=x onerror") {
		t.Fatalf("mermaid element (default/browser mode) leaked unescaped content:\n%s", html)
	}
	if !strings.Contains(html, "&lt;img") {
		t.Errorf("expected escaped payload to appear as HTML entities, got:\n%s", html)
	}
}

// TestRenderTableElement_CaptionLangDefaultsToSpanish y
// TestRenderTableElement_CaptionRespectsEnglishLang son el repro de F7 (audit
// 2026-09-11): un documento con `lang: en` seguía mostrando "Tabla 1" en el
// caption de una tabla numerada, porque el prefijo estaba hardcodeado en
// español sin importar ctx. Un ctx nil (como en el resto de este archivo)
// debe seguir cayendo al default histórico "es".
func TestRenderTableElement_CaptionLangDefaultsToSpanish(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Headers = []string{"A"}
	table.Rows = [][]string{{"1"}}
	table.Cells = ast.DeriveCellsFromFlat(table.Headers, table.Rows)
	table.Label = "tbl:x"
	table.Number = 1
	table.Caption = "Datos"

	got := renderTableElement(table, nil, nil)
	if !strings.Contains(got, `<p class="table-caption">Tabla 1: Datos</p>`) {
		t.Errorf("ctx nil debe caer al default \"es\" (\"Tabla 1: ...\"), got: %s", got)
	}
}

func TestRenderTableElement_CaptionRespectsEnglishLang(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Headers = []string{"A"}
	table.Rows = [][]string{{"1"}}
	table.Cells = ast.DeriveCellsFromFlat(table.Headers, table.Rows)
	table.Label = "tbl:x"
	table.Number = 1
	table.Caption = "Data"

	got := renderTableElement(table, nil, &RenderContext{Lang: "en"})
	if !strings.Contains(got, `<p class="table-caption">Table 1: Data</p>`) {
		t.Errorf("ctx.Lang=en debe producir \"Table 1: ...\", got: %s", got)
	}
}

// TestRenderImageElement_CaptionRespectsEnglishLang es el par de imagen del
// mismo hallazgo F7.
func TestRenderImageElement_CaptionRespectsEnglishLang(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	img := ast.NewImageElement(pos, "x.png", "alt")
	img.Label = "fig:x"
	img.Number = 1
	img.Caption = "Chart"

	got := renderImageElement(img, nil, &RenderContext{Lang: "en"})
	if !strings.Contains(got, "Figure 1: Chart") {
		t.Errorf("ctx.Lang=en debe producir \"Figure 1: ...\", got: %s", got)
	}
}

// TestGenerateDocumentHTML_DerivesLangFromFrontMatter confirma el cableado
// end-to-end: un caller que arma ctx sin Lang explícito (el caso común, ver
// los ~9 sitios de construcción de RenderContext en doclang/slidelang) igual
// obtiene el idioma correcto porque GenerateDocumentHTML lo deriva de
// doc.FrontMatter.Lang.
func TestGenerateDocumentHTML_DerivesLangFromFrontMatter(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	table := ast.NewTableElement(pos)
	table.Headers = []string{"A"}
	table.Rows = [][]string{{"1"}}
	table.Cells = ast.DeriveCellsFromFlat(table.Headers, table.Rows)
	table.Label = "tbl:x"
	table.Number = 1
	table.Caption = "Data"

	block := ast.NewContentBlock(pos, "content")
	block.Elements = append(block.Elements, table)
	doc := ast.NewAST(pos)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	doc.FrontMatter = &ast.FrontMatterNode{Lang: "en"}

	// ctx no fija Lang explícito — igual que casi todos los callers reales.
	got := GenerateDocumentHTML(doc, DocumentHTMLOptions{}, &RenderContext{})
	if !strings.Contains(got, "Table 1: Data") {
		t.Errorf("GenerateDocumentHTML no derivó Lang=\"en\" de FrontMatter.Lang; caption \"Table 1: Data\" no encontrado en el HTML")
	}
}

// TestGenerateDocumentHTML_DerivedLangDoesNotLeakAcrossReusedContext cubre
// un hallazgo de revisión independiente: la derivación de Lang escribía
// directo en el *RenderContext que el caller pasó — y resolveRenderContext
// devuelve el MISMO puntero, no un clon. Un caller que reutiliza un único
// RenderContext para varios documentos (un batch, un servidor de larga
// vida) dejaba el Lang del primer documento pegado en los siguientes: el
// guard "ctx.Lang == \"\"" nunca se volvía a cumplir, así que un segundo
// documento con su propio `lang: es` seguía mostrando "Table" en vez de
// "Tabla". Este test reutiliza el MISMO puntero *RenderContext para dos
// documentos (inglés, luego español) — el segundo debe ver su propio
// idioma, no el heredado del primero.
func TestGenerateDocumentHTML_DerivedLangDoesNotLeakAcrossReusedContext(t *testing.T) {
	makeDoc := func(lang string) *ast.AST {
		pos := diagnostics.NewPosition(1, 1)
		table := ast.NewTableElement(pos)
		table.Headers = []string{"A"}
		table.Rows = [][]string{{"1"}}
		table.Cells = ast.DeriveCellsFromFlat(table.Headers, table.Rows)
		table.Label = "tbl:x"
		table.Number = 1
		table.Caption = "Data"

		block := ast.NewContentBlock(pos, "content")
		block.Elements = append(block.Elements, table)
		doc := ast.NewAST(pos)
		doc.ContentBlocks = append(doc.ContentBlocks, *block)
		doc.FrontMatter = &ast.FrontMatterNode{Lang: lang}
		return doc
	}

	shared := &RenderContext{}

	firstHTML := GenerateDocumentHTML(makeDoc("en"), DocumentHTMLOptions{}, shared)
	if !strings.Contains(firstHTML, "Table 1: Data") {
		t.Fatalf("primer documento (en): esperaba \"Table 1: Data\", no lo encontró: %s", firstHTML)
	}
	if shared.Lang != "" {
		t.Fatalf("GenerateDocumentHTML mutó el RenderContext del caller: shared.Lang = %q, want \"\" (sin tocar)", shared.Lang)
	}

	secondHTML := GenerateDocumentHTML(makeDoc("es"), DocumentHTMLOptions{}, shared)
	if !strings.Contains(secondHTML, "Tabla 1: Data") {
		t.Errorf("segundo documento (es) con el MISMO *RenderContext reutilizado: esperaba \"Tabla 1: Data\", got: %s", secondHTML)
	}
	if strings.Contains(secondHTML, "Table 1: Data") {
		t.Errorf("segundo documento heredó el Lang=\"en\" del primero: %s", secondHTML)
	}
}
