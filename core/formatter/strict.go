// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"encoding/json"
	"fmt"
	"strings"

	"go.ziradocs.com/core/ast"
)

// FormatStrict serializa doc a la forma canónica del dialecto strict de
// SlideLang (marcadores SLIDE, ver parser.StrictParser). Cubre exactamente
// los elementos que parser.StrictParser despacha hoy — TEXT, POINTS, CODE,
// IMAGE, TABLE (forma YAML y pipe), QUOTE, CHECKLIST, :::code-group, bloques
// especiales :::, <<mermaid>>, <<plantuml>>, <<chart:...>>, <<map>>, <<grid>>,
// directivas @ — porque ese es el conjunto de node types que un parse-strict
// real puede producir; canonicalizar más que eso sería transpilar, no
// formatear (ver docs/plan/mvp-estandar-oss.md, feature fmt --strict).
// GRID/COLUMN ahora tiene sintaxis strict propia (<<grid>>/<<column>>/<<end>>,
// ver formatStrictGrid y elements.GridParser.parseStrictGrid), así que se
// serializa igual que el resto; solo un GridElement con columnas de Elements
// tipados anidados (forma que ningún parser produce hoy) sigue devolviendo
// UnsupportedElementError en vez de emitir texto que no re-parsearía.
func FormatStrict(doc *ast.AST) (string, error) {
	var b strings.Builder

	var fm string
	if doc.FrontMatter != nil {
		var err error
		fm, err = formatFrontMatter(doc.FrontMatter, frontMatterOverrides(doc.FrontMatter, "strict"))
		if err != nil {
			return "", err
		}
	}
	b.WriteString(fm)

	for i, block := range doc.ContentBlocks {
		if i > 0 || fm != "" {
			b.WriteString("\n")
		}
		blockText, err := formatStrictContentBlock(&block)
		if err != nil {
			return "", err
		}
		b.WriteString(blockText)
	}

	return b.String(), nil
}

func formatStrictContentBlock(block *ast.ContentBlock) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "SLIDE %s\n", block.BlockType)

	if block.Heading != "" {
		if err := checkQuotable("content_block", "heading", block.Heading); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "  heading: %s\n", quote(block.Heading))
	}
	if block.Title != "" {
		if err := checkQuotable("content_block", "title", block.Title); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "  title: %s\n", quote(block.Title))
	}
	if block.Subtitle != "" {
		if err := checkQuotable("content_block", "subtitle", block.Subtitle); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "  subtitle: %s\n", quote(block.Subtitle))
	}
	if block.Logo != "" {
		if err := checkQuotable("content_block", "logo", block.Logo); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "  logo: %s\n", quote(block.Logo))
	}

	for _, el := range block.Elements {
		elText, err := formatStrictElement(el)
		if err != nil {
			return "", err
		}
		b.WriteString(elText)
	}

	return b.String(), nil
}

// formatStrictElement despacha por NodeType, indentando el resultado 2
// espacios (nivel de un elemento dentro de un SLIDE).
func formatStrictElement(el ast.Element) (string, error) {
	var body string
	var err error

	switch e := el.(type) {
	case *ast.TextElement:
		body = formatStrictText(e)
	case *ast.PointsElement:
		body = formatStrictPoints(e)
	case *ast.CodeElement:
		body = formatStrictCode(e)
	case *ast.ImageElement:
		body, err = formatStrictImage(e)
	case *ast.TableElement:
		body, err = formatStrictTable(e)
	case *ast.SpecialBlockElement:
		body = formatSpecialBlock(e)
	case *ast.CodeGroupElement:
		body = formatCodeGroup(e)
	case *ast.MermaidElement:
		body = formatMermaid(e)
	case *ast.PlantUMLElement:
		body = formatPlantUML(e)
	case *ast.ChartElement:
		body, err = formatChart(e)
	case *ast.MapElement:
		body, err = formatMap(e)
	case *ast.DirectiveNode:
		body, err = formatDirective(e)
	case *ast.QuoteElement:
		body, err = formatStrictQuote(e)
	case *ast.ChecklistElement:
		body, err = formatStrictChecklist(e)
	case *ast.GridElement:
		body, err = formatStrictGrid(e)
	case *ast.MathElement:
		body, err = formatStrictMath(e)
	default:
		err = newUnsupported(string(el.GetType()), "tipo de elemento no reconocido por el formatter strict")
	}
	if err != nil {
		return "", err
	}
	if body == "" {
		return "", nil
	}
	// body puede ya terminar en "\n" cuando el contenido del elemento
	// (TEXT/CODE/MERMAID/PLANTUML) es multi-línea y su Content original
	// terminaba en newline — indent() preserva ese trailing "\n" tal cual.
	// TrimRight antes de agregar el separador de línea evita una línea en
	// blanco fantasma que no existía en el AST original.
	return strings.TrimRight(indent(body, 2), "\n") + "\n", nil
}

func formatStrictText(e *ast.TextElement) string {
	return "TEXT\n" + indent(e.Content, 2)
}

func formatStrictPoints(e *ast.PointsElement) string {
	var b strings.Builder
	b.WriteString("POINTS\n")
	b.WriteString(indent(formatPointItems(e.Items, e.ListType), 2))
	return strings.TrimRight(b.String(), "\n")
}

// formatPointItems emite el marcador según listType: PointsParser.
// detectListType decide "ordered" vs "unordered" leyendo el marcador del
// PRIMER item de nivel base (ver internal/elements/points.go) — así que
// para round-trip-ear un ListType "ordered" hay que reemitir marcadores
// numerados, no "- " genérico. Los sub-points siempre van con "-" (el
// detector de tipo de lista solo mira el nivel base, y el parser strict
// mismo no distingue tipo de lista por nivel de anidamiento).
func formatPointItems(items []ast.PointItem, listType string) string {
	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteString("\n")
		}
		if listType == "ordered" {
			fmt.Fprintf(&b, "%d. %s", i+1, item.Content)
		} else {
			fmt.Fprintf(&b, "- %s", item.Content)
		}
		if len(item.SubPoints) > 0 {
			b.WriteString("\n")
			b.WriteString(indent(formatPointItems(item.SubPoints, "unordered"), 2))
		}
	}
	return b.String()
}

func formatStrictCode(e *ast.CodeElement) string {
	header := "CODE"
	if e.Language != "" {
		header += " " + e.Language
	}
	return header + "\n" + indent(e.Content, 2)
}

func formatStrictImage(e *ast.ImageElement) (string, error) {
	if err := checkQuotable("image", "source", e.Source); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "IMAGE %s", quote(e.Source))
	if e.Alt != "" {
		if err := checkQuotable("image", "alt", e.Alt); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, " %s", quote(e.Alt))
	}
	if e.Caption != "" {
		if err := checkQuotable("image", "caption", e.Caption); err != nil {
			return "", err
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "  caption: %s", quote(e.Caption))
	}
	if e.Label != "" {
		// issue #239: identificador de referencia cruzada — mismo patrón que
		// caption arriba, para que sobreviva un round-trip fmt→build.
		if err := checkQuotable("image", "label", e.Label); err != nil {
			return "", err
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "  label: %s", quote(e.Label))
	}
	return b.String(), nil
}

// formatStrictTable elige la forma pipe (sin caption ni label, la usada en
// los ejemplos de la vitrina) o la forma TABLE/YAML (única capaz de portar
// Caption/Label — parser.StrictParser.parseMarkdownTableElement no los
// parsea, solo elements.TableParser.parseYAMLTable lo hace). issue #239:
// Label sin Caption también obliga a la forma YAML — si solo se chequeara
// Caption=="", una tabla CON label pero SIN caption caería a formatPipeTable,
// que no puede portar label, y lo perdería en silencio en un round-trip
// fmt→build (mismo bug que @include tuvo con la forma genérica de directiva).
func formatStrictTable(e *ast.TableElement) (string, error) {
	if e.Caption == "" && e.Label == "" {
		// formatPipeTable delimita con "|", no con comillas — no pasa por
		// quote()/checkQuotable, así que no hereda esta limitación (una
		// comilla literal en un header/celda es representable en la forma
		// pipe sin cambios; un "|" literal sí sería un problema distinto,
		// pre-existente, fuera de scope acá).
		return formatPipeTable(e.Headers, e.Rows), nil
	}

	if e.Caption != "" {
		if err := checkQuotable("table", "caption", e.Caption); err != nil {
			return "", err
		}
	}
	if e.Label != "" {
		if err := checkQuotable("table", "label", e.Label); err != nil {
			return "", err
		}
	}
	headers, err := formatInlineArray("table", "headers", e.Headers)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("TABLE\n")
	fmt.Fprintf(&b, "  headers: %s\n", headers)
	if e.Caption != "" {
		fmt.Fprintf(&b, "  caption: %s\n", quote(e.Caption))
	}
	if e.Label != "" {
		fmt.Fprintf(&b, "  label: %s\n", quote(e.Label))
	}
	b.WriteString("  rows:\n")
	for _, row := range e.Rows {
		rowText, err := formatStringRow(row)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "      %s\n", rowText)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func formatStringRow(row []string) (string, error) {
	return formatInlineArray("table", "row value", row)
}

func formatPipeTable(headers []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	b.WriteString("|" + strings.Join(seps, "|") + "|\n")
	for i, row := range rows {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("| " + strings.Join(row, " | ") + " |")
	}
	return b.String()
}

// strictNewElementKeywords espeja la lista de internal/elements/common.go
// IsNewElement (branch "strict") — duplicada acá en vez de importada porque
// el formatter es deliberadamente un espejo autocontenido del parser, no un
// reusador de su código (ver el resto de este archivo: formatChart/formatMap
// tampoco llaman a internal/elements). Usada solo para detectar contenido de
// QUOTE que el parser interpretaría como el inicio de OTRO elemento en vez de
// como texto de la cita (ver validateStrictQuoteContent).
var strictNewElementKeywords = []string{
	"TEXT", "POINTS", "CODE", "IMAGE", "TABLE",
	"QUOTE", "CHECKLIST", "MERMAID", "CHART", "MAP",
	"DIRECTIVE", "SPECIAL_BLOCK", "CODE_GROUP",
}

// startsWithStrictSymbolicMarker espeja la mitad simbólica de
// internal/elements/common.go IsNewElement (branch "strict") — @ directiva,
// ::: special block/grid/code-group, << diagrama/chart/map/math, | tabla
// Markdown. Antes del fix de esa función (issue: parseStrictChecklist/
// parseStrict de QUOTE se tragaban un elemento hermano simbólico), esta
// validación tampoco los cubría — consistente con el bug, pero igual de
// incompleta: ahora que el parser SÍ corta el loop en estas líneas, un
// QuoteElement.Content que empiece con una de ellas rompería el round-trip en
// silencio si esta guarda no lo rechazara también.
func startsWithStrictSymbolicMarker(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '@', '|':
		return true
	case ':':
		return strings.HasPrefix(trimmed, ":::")
	case '<':
		return strings.HasPrefix(trimmed, "<<")
	}
	return false
}

// formatStrictQuote serializa QuoteElement. elements.QuoteParser.parseStrict
// termina la cita en la primera línea vacía, "---", o que empiece con uno de
// los keywords de elemento strict — y trata cualquier línea "AUTHOR:"/
// "SOURCE:" como metadata, no contenido. Un Content que contenga alguna de
// esas formas (posible si el QuoteElement vino de un parse flex, donde el
// markdown ">" no tiene esas restricciones) no es representable sin pérdida
// en modo strict — se reporta en vez de emitir texto que reparsearía distinto
// (mismo principio que chart.Options en formatChart).
func formatStrictQuote(e *ast.QuoteElement) (string, error) {
	if err := validateStrictQuoteContent(e.Content); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("QUOTE\n")
	b.WriteString(indent(e.Content, 2))
	if e.Author != "" {
		b.WriteString("\n")
		fmt.Fprintf(&b, "  AUTHOR: %s", e.Author)
	}
	if e.Source != "" {
		b.WriteString("\n")
		fmt.Fprintf(&b, "  SOURCE: %s", e.Source)
	}
	return b.String(), nil
}

func validateStrictQuoteContent(content string) error {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// elements.QuoteParser.parseStrict hace TrimSpace de cada línea al
		// reparsear (internal/elements/quote.go) — cualquier espacio en
		// blanco al inicio o final de una línea de contenido (p.ej. código
		// indentado citado dentro de un QUOTE) se perdería en silencio.
		// Chequeado ANTES que las condiciones sobre `trimmed` porque es una
		// verificación más básica: da igual qué diga la línea si su forma
		// exacta no sobrevive el reparse.
		if line != trimmed {
			return newUnsupported("quote", fmt.Sprintf("el contenido de la cita tiene una línea con espacio en blanco al inicio o final (%q) — el parser strict hace TrimSpace de cada línea al reparsear, perdiendo ese espaciado en silencio", line))
		}
		if trimmed == "" || trimmed == "---" ||
			strings.HasPrefix(trimmed, "AUTHOR:") || strings.HasPrefix(trimmed, "SOURCE:") {
			return newUnsupported("quote", fmt.Sprintf("el contenido de la cita contiene una línea (%q) que el parser strict interpretaría como fin de bloque o metadata, no como texto de la cita — no representable sin pérdida", trimmed))
		}
		if startsWithStrictSymbolicMarker(trimmed) {
			return newUnsupported("quote", fmt.Sprintf("el contenido de la cita contiene una línea (%q) que el parser strict interpretaría como el inicio de otro elemento (marcador simbólico @/:::/<</|), no como texto de la cita — no representable sin pérdida", trimmed))
		}
		for _, kw := range strictNewElementKeywords {
			if strings.HasPrefix(trimmed, kw) {
				return newUnsupported("quote", fmt.Sprintf("el contenido de la cita contiene una línea (%q) que el parser strict interpretaría como el inicio de otro elemento (%q), no como texto de la cita — no representable sin pérdida", trimmed, kw))
			}
		}
	}
	return nil
}

// formatStrictChecklist serializa ChecklistElement. Espeja
// elements.ChecklistParser.parseStrictChecklist: un item principal es "  [x]
// contenido" (2 espacios); sus SubItems van indentados 2 espacios más (4
// total) — el parser auto-detecta el nivel base del primer item no-vacío y
// trata cualquier indentación mayor como sub-item del item principal activo,
// sin importar la profundidad relativa entre sub-items, así que un único
// nivel extra de indentación es suficiente y es lo único que ambos parsers
// (strict y flex/markdown) de este codebase producen en la práctica —
// ninguno anida SubItems dentro de SubItems.
func formatStrictChecklist(e *ast.ChecklistElement) (string, error) {
	if err := validateStrictChecklistItems(e.Items, false); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("CHECKLIST\n")
	b.WriteString(formatStrictChecklistItems(e.Items, 2))
	return strings.TrimRight(b.String(), "\n"), nil
}

// validateStrictChecklistItems recorre items validando que sean
// representables sin pérdida en modo strict — mismo principio que
// validateStrictQuoteContent, aplicado a las 3 formas en que un
// ChecklistItem puede venir de un origen no-strict (p.ej. un import
// JSON/AST, o el futuro transpiler flex→strict de issue #206) y romper el
// round-trip en silencio en vez de fallar:
//   - Content vacío: elements.ChecklistParser.parseStrictChecklistContent
//     exige content != "" para reconocer un item (internal/elements/checklist.go)
//     — un item con Content vacío simplemente desaparece al reparsear.
//   - Content con salto de línea: parseStrictChecklist procesa línea por
//     línea; una segunda línea de "contenido" se interpretaría como
//     continuation del item o como un item nuevo, nunca como texto del
//     mismo Content.
//   - SubItems anidados dentro de SubItems (profundidad > 2 total):
//     parseStrictChecklist solo trackea UN currentItem de nivel base —
//     cualquier indentación más profunda que eso se aplana directamente
//     sobre ese item, perdiendo el nivel intermedio.
func validateStrictChecklistItems(items []ast.ChecklistItem, isSubLevel bool) error {
	for _, item := range items {
		if item.Content == "" {
			return newUnsupported("checklist", "un item sin contenido (Content vacío) no es representable: el parser strict exige contenido no vacío para reconocer un item — al reparsear, el item desaparece en silencio")
		}
		if strings.Contains(item.Content, "\n") {
			return newUnsupported("checklist", fmt.Sprintf("el contenido del item %q contiene un salto de línea — el parser strict procesa cada línea de un item por separado, así que la línea siguiente se interpretaría como continuación o como un item nuevo, no como parte del mismo contenido", item.Content))
		}
		if isSubLevel && len(item.SubItems) > 0 {
			return newUnsupported("checklist", fmt.Sprintf("el item %q tiene sub-items anidados dentro de otro sub-item — el parser strict solo soporta UN nivel de anidamiento (todo lo indentado más allá del nivel base se adjunta al item principal activo, aplanando cualquier nivel más profundo)", item.Content))
		}
		if err := validateStrictChecklistItems(item.SubItems, true); err != nil {
			return err
		}
	}
	return nil
}

func formatStrictChecklistItems(items []ast.ChecklistItem, indentLevel int) string {
	var b strings.Builder
	prefix := strings.Repeat(" ", indentLevel)
	for _, item := range items {
		mark := " "
		if item.Checked {
			mark = "x"
		}
		fmt.Fprintf(&b, "%s[%s] %s\n", prefix, mark, item.Content)
		if len(item.SubItems) > 0 {
			b.WriteString(formatStrictChecklistItems(item.SubItems, indentLevel+2))
		}
	}
	return b.String()
}

func formatSpecialBlock(e *ast.SpecialBlockElement) string {
	var b strings.Builder
	b.WriteString(":::" + e.BlockType)
	if e.Title != "" {
		b.WriteString(" " + e.Title)
	}
	b.WriteString("\n")
	b.WriteString(e.Content)
	b.WriteString("\n:::")
	return b.String()
}

func formatCodeGroup(e *ast.CodeGroupElement) string {
	var b strings.Builder
	b.WriteString(":::code-group\n")
	for _, cb := range e.CodeBlocks {
		fmt.Fprintf(&b, "```%s", cb.Language)
		if cb.Label != "" {
			fmt.Fprintf(&b, " [%s]", cb.Label)
		}
		b.WriteString("\n")
		b.WriteString(cb.Content)
		b.WriteString("\n```\n")
	}
	b.WriteString(":::")
	return b.String()
}

// formatStrictGrid serializa GridElement a la forma delimitada
// <<grid>>/<<column>>/<<end>> (ver elements.GridParser.parseStrictGrid, que es
// su inverso exacto). El Content suelto del grid va justo tras <<grid>>; cada
// columna emite <<column>> seguido de su Content en bruto. formatStrictElement
// luego indenta todo 2 espacios; el parser quita esa sangría base al reparsear,
// así que el round-trip es idempotente.
//
// Un GridElement cuyas columnas traen Elements tipados anidados (en vez de
// Content en bruto) no es representable en esta forma de texto — ningún parser
// produce esa forma hoy (el pipeline flex/strict usa Content por columna), pero
// un AST de otro origen (p. ej. un futuro transpiler) podría; se reporta en vez
// de perder esos Elements en silencio, mismo principio que chart.Options o el
// contenido no representable de QUOTE/CHECKLIST.
func formatStrictGrid(e *ast.GridElement) (string, error) {
	if err := validateStrictGridContent(e); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("<<grid>>")
	if e.Content != "" {
		b.WriteString("\n")
		b.WriteString(e.Content)
	}
	for _, col := range e.Columns {
		b.WriteString("\n<<column>>")
		if col.Content != "" {
			b.WriteString("\n")
			b.WriteString(col.Content)
		}
	}
	b.WriteString("\n<<end>>")
	return b.String(), nil
}

// validateStrictGridContent rechaza contenido que el parser strict de grid
// reinterpretaría como marcador (rompiendo el re-parse), mismo patrón que
// validateStrictQuoteContent/validateStrictChecklistItems: una línea de
// Content cuyo texto trimeado sea <<column>>, <<end>> o <<grid>> no
// round-trip-earía (el parser matchea esos marcadores sobre la línea trimeada,
// sin importar la sangría); y una columna con Elements tipados anidados no
// tiene representación en la forma de texto Content-based.
//
// "SLIDE …" a propósito NO se rechaza: el parser solo lo trata como límite de
// slide cuando está SIN sangría (isSlideBoundary mira la línea cruda), y
// formatStrictElement siempre indenta el cuerpo del grid, así que un
// "SLIDE overview" de contenido se re-emite indentado y se reparsea como texto,
// no como límite — round-trip estable.
func validateStrictGridContent(e *ast.GridElement) error {
	checkContent := func(where, content string) error {
		for _, line := range strings.Split(content, "\n") {
			t := strings.TrimSpace(line)
			if t == "<<grid>>" || t == "<<column>>" || t == "<<end>>" {
				return newUnsupported("grid", fmt.Sprintf("%s contiene una línea (%q) que el parser strict interpretaría como un marcador de grid, no como texto — no representable sin pérdida", where, t))
			}
		}
		return nil
	}

	if err := checkContent("la prosa suelta del grid", e.Content); err != nil {
		return err
	}
	for i := range e.Columns {
		if len(e.Columns[i].Elements) > 0 {
			return newUnsupported("grid", "una columna con Elements tipados anidados no es representable en el dialecto strict: la forma <<grid>>/<<column>> guarda el cuerpo de cada columna como Content en bruto (lo que produce el parser y consume el renderer), no como sub-elementos tipados")
		}
		if err := checkContent(fmt.Sprintf("la columna %d", i+1), e.Columns[i].Content); err != nil {
			return err
		}
	}
	return nil
}

func formatMermaid(e *ast.MermaidElement) string {
	return "<<mermaid>>\n" + indent(e.Content, 2)
}

// formatStrictMath emite <<math>> (issue #239-B) con una línea label:
// opcional, mismo patrón exacto que el label de TABLE/IMAGE
// (formatStrictImage/formatStrictTable): quote() + checkQuotable() —
// consistencia de round-trip, no una forma alterna sin comillas.
//
// A DIFERENCIA de formatMermaid (que no emite <<end>> — su contenido nunca
// coincide en indentación con la etiqueta label: opcional), acá SIEMPRE se
// emite <<end>> explícito: el contenido LaTeX + una línea label: quedan a
// la MISMA indentación de 2 espacios que el elemento hermano siguiente, así
// que sin <<end>> el re-parse no tiene forma de distinguir "fin del bloque
// math" de "más contenido del bloque math" por dedent solo — encontrado y
// corregido vía TestFormatStrict_RoundTrip_Corpus (formatter/strict_roundtrip_test.go).
func formatStrictMath(e *ast.MathElement) (string, error) {
	body := "<<math>>\n" + indent(e.Content, 2)
	if e.Label != "" {
		if err := checkQuotable("math", "label", e.Label); err != nil {
			return "", err
		}
		body += "\n" + indent("label: "+quote(e.Label), 2)
	}
	body += "\n<<end>>"
	return body, nil
}

func formatPlantUML(e *ast.PlantUMLElement) string {
	return "<<plantuml>>\n" + indent(e.Content, 2)
}

// formatChart maneja los dos sub-dialectos que el parser strict de
// Chart realmente alcanza en la práctica:
//   - JSON mode (IsJSONMode/RawJSON): passthrough exacto, siempre lossless.
//   - forma plana type:/data:/series:/labels:/title: — la que usa TODO
//     chart strict hoy, incluyendo "combo": la rama YAML anidada de
//     internal/elements/chart.go (parseComboChartYAML, data organizada por
//     serie bajo "data: {labels:, series:}") requiere que el valor de
//     "data:" sea un mapping YAML; un chart combo autor con la forma común
//     "data: [[fila], [fila]]" (una secuencia) hace fallar el
//     yaml.Unmarshal esperado por esa rama, así que SIEMPRE cae al loop de
//     propiedades plano igual que un chart no-combo — confirmado
//     parseando examples/10_advanced_elements: un chart combo con
//     type/data/series terminó con esos 3 campos poblados por la vía
//     plana, no por parseComboChartYAML. Por eso el formatter no
//     distingue por ChartType: siempre emite la forma plana.
//
// El loop de propiedades plano NUNCA parsea "options:" (solo la vía
// combo-YAML late o JSON lo pueblan, y esta última no es alcanzable de
// forma determinista desde Data/Series ya poblados). Un chart con
// Options no vacío fuera de JSON mode no es representable en modo strict
// hoy (gap pre-existente del parser): se reporta en vez de perder datos
// en silencio.
func formatChart(e *ast.ChartElement) (string, error) {
	header := fmt.Sprintf("<<chart: %s width=%q height=%q>>", e.ChartType, fmtInt(e.Width), fmtInt(e.Height))

	if e.IsJSONMode {
		raw, err := canonicalJSON(e.RawJSON)
		if err != nil {
			return "", err
		}
		return header + "\n" + raw + "\n<</chart>>", nil
	}

	if len(e.Options) > 0 {
		return "", newUnsupported("chart", "chart.Options no vacío no es representable en modo strict fuera de JSON mode (internal/elements/chart.go no parsea \"options:\" en el loop de propiedades plano)")
	}

	var b strings.Builder
	b.WriteString(header + "\n")
	if len(e.SeriesTypes) > 0 {
		types, err := formatInlineArray("chart", "type", e.SeriesTypes)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "type: %s\n", types)
	}
	if len(e.Data) > 0 {
		b.WriteString("data: [\n")
		for _, row := range e.Data {
			rowText, err := formatInlineRow("chart", "data", row)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "  %s\n", rowText)
		}
		b.WriteString("]\n")
	}
	if len(e.Series) > 0 {
		series, err := formatInlineArray("chart", "series", e.Series)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "series: %s\n", series)
	}
	if len(e.Labels) > 0 {
		labels, err := formatInlineArray("chart", "labels", e.Labels)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "labels: %s\n", labels)
	}
	if e.Title != "" {
		if err := checkQuotable("chart", "title", e.Title); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "title: %s\n", quote(e.Title))
	}
	b.WriteString("<<end>>")
	return b.String(), nil
}

func fmtInt(n int) string {
	return fmt.Sprintf("%d", n)
}

func canonicalJSON(raw json.RawMessage) (string, error) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", fmt.Errorf("formatter: RawJSON de chart inválido: %w", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// formatMap serializa MapElement. El parser strict solo puebla
// Options con 3 claves conocidas (title/showValues/clustering, ver
// internal/elements/map.go) — no es un mapa arbitrario como el de Chart —
// así que es completamente representable.
// mapDefaultWidth/mapDefaultHeight espejan los defaults hardcoded en
// internal/elements/map.go (MapParser.Parse) — width/height solo se
// emiten como atributos inline en la línea de apertura "<<map ...>>"
// (único lugar donde el parser los lee, ver extractAttribute) cuando
// difieren del default, para no ensuciar la salida común y para que un
// mapa con dimensiones custom no pierda silenciosamente ese dato en el
// round-trip (MapElement.Width/Height solo se puede fijar por ahí; no
// existe una clave "width:"/"height:" en el cuerpo del bloque).
const (
	mapDefaultWidth  = 800
	mapDefaultHeight = 600
)

func formatMap(e *ast.MapElement) (string, error) {
	var b strings.Builder
	b.WriteString("<<map")
	if e.Width != 0 && e.Width != mapDefaultWidth {
		fmt.Fprintf(&b, " width=%q", fmtInt(e.Width))
	}
	if e.Height != 0 && e.Height != mapDefaultHeight {
		fmt.Fprintf(&b, " height=%q", fmtInt(e.Height))
	}
	b.WriteString(">>\n")
	if e.MapType != "" {
		fmt.Fprintf(&b, "type: %s\n", e.MapType)
	}
	if e.Center != nil {
		fmt.Fprintf(&b, "center: %s, %s\n", formatFloat(e.Center.Lat), formatFloat(e.Center.Lng))
	}
	if len(e.Markers) > 0 {
		b.WriteString("markers:\n")
		for _, m := range e.Markers {
			fmt.Fprintf(&b, "  - lat: %s\n", formatFloat(m.Lat))
			fmt.Fprintf(&b, "    lng: %s\n", formatFloat(m.Lng))
			if m.Label != "" {
				if err := checkQuotable("map", "marker.label", m.Label); err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "    label: %s\n", quote(m.Label))
			}
			if m.Value != 0 {
				fmt.Fprintf(&b, "    value: %s\n", formatFloat(m.Value))
			}
			if m.Color != "" {
				if err := checkQuotable("map", "marker.color", m.Color); err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "    color: %s\n", quote(m.Color))
			}
			if m.Size != "" {
				if err := checkQuotable("map", "marker.size", m.Size); err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "    size: %s\n", quote(m.Size))
			}
			if m.Details != "" {
				if err := checkQuotable("map", "marker.details", m.Details); err != nil {
					return "", err
				}
				fmt.Fprintf(&b, "    details: %s\n", quote(m.Details))
			}
		}
	}
	if e.Heatmap {
		b.WriteString("heatmap: true\n")
	}
	if e.Zoom != 0 {
		fmt.Fprintf(&b, "zoom: %d\n", e.Zoom)
	}
	for _, k := range []string{"title", "showValues", "clustering"} {
		if v, ok := e.Options[k]; ok {
			if s, ok := v.(string); ok {
				if err := checkQuotable("map", "options."+k, s); err != nil {
					return "", err
				}
			}
			fmt.Fprintf(&b, "%s: %s\n", k, formatScalar(v))
		}
	}
	b.WriteString("<<end>>")
	return b.String(), nil
}

// formatDirective serializa @nombre. "delay" es el único caso donde
// DirectiveParser.parseDirectiveNameAndParams asigna parameters["ms"] al
// paramString CRUDO sin importar si contiene "=" (a diferencia de
// timer/highlight/auto-play, que sí ramifican en "="): emitir una forma
// key="value" para delay produciría parameters["ms"] = `ms="valor"`
// literal en el re-parse, corrompiendo el round-trip — por eso "delay"
// siempre se emite en forma bare (valor plano), nunca key=value.
func formatDirective(e *ast.DirectiveNode) (string, error) {
	if e.Name == "notes" {
		content, _ := e.Parameters["content"].(string)
		if content == "" {
			return "@notes", nil
		}
		return "@notes\n" + indent(content, 2), nil
	}

	if len(e.Parameters) == 0 {
		return "@" + e.Name, nil
	}

	if e.Name == "delay" {
		if v, ok := e.Parameters["ms"]; ok {
			return fmt.Sprintf("@delay %v", v), nil
		}
	}

	if e.Name == "include" {
		// issue #238: emitir @include <ruta> verbatim (bare, nunca
		// path="ruta") — mismo motivo que "delay" arriba. Necesario para que
		// `fmt` no expanda ni reescriba la directiva a una forma que
		// core/include.Expand ya no reconozca en un build posterior.
		if v, ok := e.Parameters["path"]; ok {
			return fmt.Sprintf("@include %v", v), nil
		}
	}

	keys := sortedStringKeys(e.Parameters)
	parts := make([]string, len(keys))
	for i, k := range keys {
		value := fmt.Sprint(e.Parameters[k])
		if err := checkQuotable("directive", "parameters."+k, value); err != nil {
			return "", err
		}
		parts[i] = fmt.Sprintf("%s=%s", k, quote(value))
	}
	return "@" + e.Name + " " + strings.Join(parts, " "), nil
}
