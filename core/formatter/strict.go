// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"encoding/json"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/internal/elements"
	"go.ziradocs.com/core/v2/layouts"
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
// serializa igual que el resto, con `<<column typed>>` para una columna de
// Elements (issue #373). Una columna que mezcla Content y Elements, o que anida
// un grid, devuelve UnsupportedElementError en vez de emitir texto que no
// re-parsearía.
func FormatStrict(doc *ast.AST) (string, error) {
	out, err := formatStrictWithoutIDs(doc)
	if err != nil {
		return "", err
	}
	return formatNodeIDs(doc, out, false)
}

func formatStrictWithoutIDs(doc *ast.AST) (string, error) {
	var b strings.Builder

	var fm string
	if fmNode := extensionFrontMatter(doc, "strict"); fmNode != nil {
		var err error
		fm, err = formatFrontMatter(fmNode, frontMatterOverrides(fmNode, "strict"), frontMatterFallbacks(fmNode))
		if err != nil {
			return "", err
		}
	}
	b.WriteString(fm)

	// Una sola secuencia de anchors para todo el deck, igual que el parser.
	anchors := &elements.HeadingAnchors{}
	prevAbsorbs := false
	for i, block := range doc.ContentBlocks {
		// La línea en blanco que separa dos slides la leería como parte del
		// código un CODE que cierre el slide anterior; ver
		// absorbsTrailingBlankLines.
		if (i > 0 || fm != "") && !prevAbsorbs {
			b.WriteString("\n")
		}
		prevAbsorbs = len(block.Elements) > 0 && absorbsTrailingBlankLines(block.Elements[len(block.Elements)-1])
		blockText, err := formatStrictContentBlock(&block, anchors, i == len(doc.ContentBlocks)-1)
		if err != nil {
			return "", err
		}
		b.WriteString(blockText)
	}

	out := b.String()
	if prevAbsorbs {
		// El final del archivo cuenta como una línea en blanco más para el
		// CODE con que termina el deck: el salto de línea que cierra la
		// última línea de texto ya es el "\n" que el contenido declara.
		out = strings.TrimSuffix(out, "\n")
	}
	return out, nil
}

func formatStrictContentBlock(block *ast.ContentBlock, anchors *elements.HeadingAnchors, last bool) (string, error) {
	var b strings.Builder
	blockType := block.BlockType
	if last && block.InfersClosingLayout() {
		// Un `SLIDE content` literal declararía el layout y la regla del
		// linter dejaría de reclasificar el último slide: el texto
		// formateado se construiría como "content" donde el original se
		// construía como "closing". Se escribe el layout que el build le
		// asigna.
		blockType = "closing"
	}
	fmt.Fprintf(&b, "SLIDE %s\n", blockType)

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
	if block.Kicker != "" {
		if err := checkQuotable("content_block", "kicker", block.Kicker); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "  kicker: %s\n", quote(block.Kicker))
	}
	if block.Logo != "" {
		if err := checkQuotable("content_block", "logo", block.Logo); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "  logo: %s\n", quote(block.Logo))
	}
	// Opciones de layout (issue #255). Van sin comillas: son un entero acotado
	// y un enum de un conjunto cerrado, así que no hay texto libre que citar.
	//
	// Se emiten solo si el layout las acepta. Un AST con `hero` y `Columns: 3`
	// es alcanzable —el linter solo advierte (LAYOUT_OPTION_NOT_APPLICABLE) y
	// ast/decode.go lo acepta desde JSON—, y emitirlo producía un archivo que
	// el parser strict rechaza con un ERROR duro ("Unknown content block
	// property: columns"). Que lo que sale de aquí vuelva a entrar es el
	// contrato del formatter; fabricar un archivo que no compila lo rompe más
	// que perder una opción que ese layout nunca iba a leer.
	if block.LayoutConfig != nil {
		if block.LayoutConfig.Align != "" && layouts.Accepts(blockType, "align") {
			fmt.Fprintf(&b, "  align: %s\n", block.LayoutConfig.Align)
		}
		if block.LayoutConfig.Columns != 0 && layouts.Accepts(blockType, "columns") {
			fmt.Fprintf(&b, "  columns: %d\n", block.LayoutConfig.Columns)
		}
	}

	for _, el := range block.Elements {
		if heading, ok, err := strictSlideHeading(el, anchors); ok || err != nil {
			if err != nil {
				return "", err
			}
			b.WriteString(heading)
			continue
		}
		elText, err := formatStrictElement(el)
		if err != nil {
			return "", err
		}
		b.WriteString(elText)
	}

	return b.String(), nil
}

// strictSlideHeading serializa un encabezado de subsección —el tipado
// (HeadingElement) o el legado (TextElement RawHTML `<hN id>`)— con la
// sintaxis strict `SECTION "Texto"` + `level:` (issue #259). Antes el legado
// salía como la línea Markdown `### Texto` dentro de un TEXT y, al reparsear,
// el encabezado se volvía prosa.
//
// `id:` se emite solo cuando el anchor real no es el que el parser derivaría
// en esa posición del deck (anchors reproduce su secuencia), así que un
// encabezado sin id declarado vuelve igual y uno cuyo anchor dependía de otro
// contexto no cambia al reordenar o reparsear. El texto del tipado es la
// fuente autoral exacta; el del legado se des-renderiza con la misma pérdida
// de énfasis que documenta formatSubsectionHeading. ok=false significa que el
// elemento no es un encabezado.
func strictSlideHeading(el ast.Element, anchors *elements.HeadingAnchors) (string, bool, error) {
	var level int
	var text, anchor string
	switch e := el.(type) {
	case *ast.HeadingElement:
		level, text, anchor = e.Level, e.Text, e.Anchor
	case *ast.TextElement:
		h, ok := asDocumentHeading(e)
		if !ok {
			return "", false, nil
		}
		level, text, anchor = h.level, h.text, h.id
	default:
		return "", false, nil
	}
	if level < 3 || level > 6 {
		return "", true, newUnsupported("heading", fmt.Sprintf("un encabezado de nivel %d no es representable dentro de un SLIDE (strict acepta 3-6)", level))
	}
	if strings.TrimSpace(text) == "" {
		return "", true, newUnsupported("heading", "un encabezado sin texto no es representable como SECTION")
	}
	if err := checkSectionTitle("heading", text); err != nil {
		return "", true, err
	}
	var b strings.Builder
	// Sin escapar: parseSectionHeader toma el título hasta la ÚLTIMA comilla,
	// así que las comillas internas sobreviven tal cual.
	fmt.Fprintf(&b, "  SECTION \"%s\"\n", text)
	fmt.Fprintf(&b, "    level: %d\n", level)
	if anchor != anchors.Derive(text) {
		if anchor == "" || elements.DeriveAnchor(anchor) != anchor {
			return "", true, newUnsupported("heading", fmt.Sprintf("el anchor %q no es representable con `id:`", anchor))
		}
		if !anchors.Reserve(anchor) {
			return "", true, newUnsupported("heading", fmt.Sprintf("anchor de encabezado duplicado %q", anchor))
		}
		fmt.Fprintf(&b, "    id: %s\n", anchor)
	} else {
		anchors.Unique(text)
	}
	return b.String(), true, nil
}

// formatStrictElement despacha por NodeType, indentando el resultado 2
// espacios (nivel de un elemento dentro de un SLIDE).
func formatStrictElement(el ast.Element) (string, error) {
	var body string
	var err error

	switch e := el.(type) {
	case *ast.TextElement:
		body, err = formatStrictText(e)
	case *ast.PointsElement:
		body = formatStrictPoints(e)
	case *ast.CodeElement:
		body, err = formatStrictCode(e)
	case *ast.ImageElement:
		body, err = formatStrictImage(e)
	case *ast.TableElement:
		body, err = formatTableElement(e)
	case *ast.SpecialBlockElement:
		// El cuerpo de un bloque especial se re-emite como texto crudo y el
		// parser strict no reconoce `###` dentro de él (HeadingParser solo
		// corre en flex): un encabezado tipado anidado se volvería prosa al
		// reparsear. Se rechaza en vez de perderlo en silencio.
		if nestedTypedHeading(e.Elements) {
			err = newUnsupported("heading", fmt.Sprintf("un encabezado tipado dentro de un bloque :::%s no es representable en el dialecto strict", e.BlockType))
			break
		}
		body = formatSpecialBlock(e)
	case *ast.CodeGroupElement:
		body = formatCodeGroup(e)
	case *ast.MermaidElement:
		body, err = formatMermaid(e)
	case *ast.PlantUMLElement:
		body, err = formatPlantUML(e)
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
	case *ast.QuizElement:
		body, err = formatQuiz(e)
	case *ast.PollElement:
		body, err = formatPoll(e)
	case *ast.MetricElement:
		body, err = formatMetric(e)
	case *ast.GridElement:
		body, err = formatStrictGrid(e)
	case *ast.MathElement:
		body, err = formatStrictMath(e)
	case *ast.MediaElement:
		body, err = formatMedia(e)
	case *ast.HeadingElement:
		// Un encabezado hijo directo de un SLIDE o de una sección lo
		// serializan strictSlideHeading y formatDocumentStrict como SECTION;
		// llegar acá significa que está anidado (bloque especial, columna),
		// donde strict no tiene sintaxis de encabezado.
		err = newUnsupported("heading", "un encabezado tipado solo es representable como hijo directo de un SLIDE o de una sección")
	default:
		err = newUnsupported(string(el.GetType()), "tipo de elemento no reconocido por el formatter strict")
	}
	if err != nil {
		return "", err
	}
	if body == "" {
		return "", nil
	}
	if absorbsTrailingBlankLines(el) {
		// Los saltos de línea del final del contenido son líneas en blanco
		// que el parser strict sí cuenta como parte del código, así que no se
		// recortan.
		return indent(body, 2) + "\n", nil
	}
	// body puede ya terminar en "\n" cuando el contenido del elemento
	// (TEXT/CODE/MERMAID/PLANTUML) es multi-línea y su Content original
	// terminaba en newline — indent() preserva ese trailing "\n" tal cual.
	// TrimRight antes de agregar el separador de línea evita una línea en
	// blanco fantasma que no existía en el AST original.
	return strings.TrimRight(indent(body, 2), "\n") + "\n", nil
}

// absorbsTrailingBlankLines dice si el parser strict lee como parte del
// elemento las líneas en blanco que le siguen. Es el caso de CODE: su cuerpo
// sigue hasta la primera línea con menos sangría, las líneas vacías cuentan
// como contenido, y solo se descarta un salto de línea final. El contenido
// `x` se escribe entonces SIN línea en blanco detrás, y `x\n` con una.
// Recortarlas, o agregar la línea en blanco que separa un SLIDE del
// siguiente, le añade un "\n" al código.
func absorbsTrailingBlankLines(el ast.Element) bool {
	c, ok := el.(*ast.CodeElement)
	return ok && c.Content != ""
}

// formatStrictText serializa un TextElement como bloque TEXT.
//
// El caso IsRawHTML es el encabezado de subsección que el dialecto flex de
// SlideLang produce desde `###`..`######` (issue #194). El dialecto strict
// no tiene forma de escribir un encabezado dentro de un slide (issue #259),
// así que se re-emite la línea Markdown de origen —"### Foo"— dentro del
// TEXT, reusando el mismo formatSubsectionHeading que DocLang.
//
// Dos consecuencias, las dos verificadas contra los 74 ejemplos del repo:
//
//   - El texto no se pierde, pero deja de ir PEGADO al párrafo vecino. Antes
//     de #194 la línea "### Foo" era un TextElement común, y TextParser la
//     fusionaba con las líneas de alrededor: en 8 ejemplos el fmt emitía un
//     solo TEXT con el encabezado y el párrafo siguiente en la MISMA línea
//     (header_test.slidelang llegaba a juntar los niveles 3, 4, 5 y 6 y su
//     prosa en una). Ahora cada encabezado es su propio elemento. La salida
//     cambia; el contenido no.
//   - El énfasis inline SÍ se pierde: "### **Foo**" vuelve como "### Foo".
//     buildHeadingElement ya convirtió "**Foo**" en <strong>Foo</strong> y
//     formatSubsectionHeading corre stripTags. Es la misma pérdida que
//     DocLang documenta como la única de su formatter (ver
//     formatSubsectionHeading), y afecta a 21 encabezados en 5 ejemplos.
//     Es reversible en principio —ProcessInlineMarkdownSecureLine escapa el
//     HTML ANTES de aplicar Markdown, así que un <strong> en la salida solo
//     puede venir de "**"— pero escribir esa inversa es trabajo aparte:
//     issue #260.
func formatStrictText(e *ast.TextElement) (string, error) {
	content := e.Content
	if e.IsRawHTML {
		md, err := formatSubsectionHeading(e)
		if err != nil {
			return "", err
		}
		content = md
	}
	return "TEXT\n" + indent(content, 2), nil
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
			childType := item.SubListType
			if childType == "" {
				childType = "unordered"
			}
			b.WriteString(indent(formatPointItems(item.SubPoints, childType), 2))
		}
	}
	return b.String()
}

func formatStrictCode(e *ast.CodeElement) (string, error) {
	header := "CODE"
	if e.Language != "" {
		header += " " + e.Language
	}
	if e.Filename != "" {
		if err := checkCodeFilename(e); err != nil {
			return "", err
		}
		header += " " + e.Filename
	}
	return header + "\n" + indent(e.Content, 2), nil
}

// checkCodeFilename valida que el nombre de archivo de un CODE tenga forma
// que re-parsee: es un token (sin espacios) y necesita un lenguaje delante,
// porque `CODE archivo.ts` se leería como lenguaje.
func checkCodeFilename(e *ast.CodeElement) error {
	if e.Language == "" || strings.ContainsAny(e.Language, " \t") {
		return newUnsupported("code", fmt.Sprintf("el archivo %q necesita un lenguaje de un solo token delante para re-parsear", e.Filename))
	}
	if strings.ContainsAny(e.Filename, " \t\r\n") || strings.HasPrefix(e.Filename, "[") || strings.HasPrefix(e.Filename, "{") {
		return newUnsupported("code", fmt.Sprintf("el nombre de archivo %q no es representable como un token", e.Filename))
	}
	return nil
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
	if e.Fit != "" {
		b.WriteString("\n")
		fmt.Fprintf(&b, "  fit: %s", quote(e.Fit))
	}
	if e.Focus != "" {
		if err := checkQuotable("image", "focus", e.Focus); err != nil {
			return "", err
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "  focus: %s", quote(e.Focus))
	}
	if e.Bleed {
		b.WriteString("\n  bleed: true")
	}
	return b.String(), nil
}

func formatPipeTable(headers []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString("| " + strings.Join(escapeTableCells(headers), " | ") + " |\n")
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	b.WriteString("|" + strings.Join(seps, "|") + "|\n")
	for i, row := range rows {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("| " + strings.Join(escapeTableCells(row), " | ") + " |")
	}
	return b.String()
}

// escapeTableCells aplica escapeTableCellPipe a cada celda de una fila.
func escapeTableCells(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = escapeTableCellPipe(c)
	}
	return out
}

// escapeTableCellPipe es el espejo, del lado de escritura, de
// elements.splitMarkdownTableRow: un "|" LITERAL en el texto de una celda
// (fuera de un code span) tiene que volver a escaparse a "\|" al
// reserializar, o se convierte en un separador de columna de más al
// reparsear — exactamente el bug que este PR (F10, audit 2026-09-11) cierra
// del lado del parser, pero abierto de nuevo del lado de `fmt` si el
// escritor no participa: antes de este fix, ninguna celda parseada de una
// tabla markdown podía contener un "|" (el split ciego ya lo habría partido
// como columna), así que este caso nunca era alcanzable — ahora que el
// parser decodifica "\|" y protege code spans, si el formatter no lo
// reescapa, un round-trip fmt→reparse infla la fila con una columna de más
// en silencio.
//
// Un "|" DENTRO de un code span BIEN FORMADO no se toca: los backticks ya
// lo protegen estructuralmente en el reparse (SplitMarkdownTableRow no lo
// trata como separador mientras esté dentro de backticks), y escaparlo ahí
// metería un "\" visible dentro del <code> renderizado, que un code span no
// interpreta como escape (cambiaría el contenido, no solo la sintaxis).
//
// Un "`" suelto (una corrida de backticks sin otra corrida del MISMO largo
// más adelante en la celda que la cierre) no abre ningún code span real —
// codeSpanRunRanges no lo marca como tal, así que el "|" que sigue se
// escapa igual que cualquier otro "|" literal (hallazgo de revisión: "use `
// for code | see docs" tiene un backtick suelto; escaparlo es correcto en
// el reparse — SplitMarkdownTableRow decodifica "\|" a "|" sin importar si
// está o no entre backticks — aunque dejar de ser la representación más
// prolija de un code span que en el fondo nunca estuvo bien formado).
//
// codeSpanRunRanges empareja CORRIDAS de backticks, no backticks sueltos —
// espejo exacto de elements.codeSpanRanges (core/internal/elements/table.go),
// no reusado directo porque este archivo es deliberadamente un espejo
// autocontenido del parser (ver el comentario de strictNewElementKeywords
// más abajo). Un span delimitado por dos o más backticks seguidos (la
// forma CommonMark para meter un backtick literal adentro) se rompía antes
// de esto: alternar por CARÁCTER en vez de por corrida leía dos backticks
// consecutivos como "abre, cierra" en vez de "abre un delimitador de largo
// 2", así que el "|" que en realidad está adentro del
// span se escapaba igual que uno literal — metiendo un "\" visible dentro
// del <code> renderizado en el próximo build, que un code span no
// interpreta como escape (cambia el contenido, no solo la sintaxis;
// hallazgo de segunda ronda de revisión).
func escapeTableCellPipe(cell string) string {
	if !strings.Contains(cell, "|") {
		return cell
	}
	runes := []rune(cell)
	inSpan := codeSpanRunRanges(runes)

	var b strings.Builder
	for i, r := range runes {
		switch {
		case r == '|' && inSpan[i]:
			b.WriteRune(r)
		case r == '|':
			// Hallazgo de cuarta ronda de revisión: escribir siempre
			// exactamente UN backslash ("\|") ignora los que ya haya
			// justo antes en la celda (alcanzable desde una tabla YAML o
			// un AST armado por un filtro, no solo desde este parser) —
			// "a\|b" salía como "a\\|b", que el splitter (ahora que
			// empareja de a dos, CommonMark §2.4) lee como "un backslash
			// literal + pipe SEPARADOR", partiendo la fila de más. Si ya
			// hay k backslashes escritos, hacen falta k+1 MÁS (total
			// 2k+1, siempre impar) para que el splitter reconstruya
			// exactamente esos k backslashes y el pipe quede literal —
			// espejo exacto, del lado de escritura, del apareo que ahora
			// hace elements.SplitMarkdownTableRow.
			k := precedingBackslashRunLength(runes, i)
			for j := 0; j <= k; j++ {
				b.WriteRune('\\')
			}
			b.WriteRune('|')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// precedingBackslashRunLength cuenta los backslashes consecutivos
// inmediatamente antes de la posición i en runes (ya escritos a b por el
// caso default de escapeTableCellPipe en vueltas anteriores del loop).
func precedingBackslashRunLength(runes []rune, i int) int {
	count := 0
	for k := i - 1; k >= 0 && runes[k] == '\\'; k-- {
		count++
	}
	return count
}

// codeSpanRunRanges marca, por índice de rune, qué posiciones caen dentro
// de un code span delimitado por una corrida de backticks — ver el
// comentario de escapeTableCellPipe. Espejo exacto de
// elements.codeSpanRanges (core/internal/elements/table.go): el delimitador
// es una CORRIDA de N backticks consecutivos, y solo otra corrida de
// exactamente N backticks la cierra; una corrida sin cierre del mismo largo
// en el resto del texto no es un delimitador, son backticks literales.
//
// Un backtick escapado (precedido por una corrida IMPAR de backslashes) no
// abre span (hallazgo de tercera ronda de revisión, mismo fix que su
// espejo en elements.codeSpanRanges — ver el comentario de esa función
// para el precedente del spec de CommonMark).
func codeSpanRunRanges(runes []rune) []bool {
	n := len(runes)
	inSpan := make([]bool, n)
	escaped := backslashEscapedRunes(runes)
	i := 0
	for i < n {
		if runes[i] != '`' || escaped[i] {
			i++
			continue
		}
		start := i
		for i < n && runes[i] == '`' {
			i++
		}
		openLen := i - start

		j := i
		matched := false
		for j < n {
			if runes[j] != '`' {
				j++
				continue
			}
			closeStart := j
			for j < n && runes[j] == '`' {
				j++
			}
			if j-closeStart == openLen {
				for k := start; k < j; k++ {
					inSpan[k] = true
				}
				i = j
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
	}
	return inSpan
}

// backslashEscapedRunes espeja elements.backslashEscaped
// (core/internal/elements/table.go): escaped[i] es true cuando el número
// de backslashes consecutivos justo antes de la posición i es IMPAR — sólo
// el último backslash de una corrida impar escapa al carácter siguiente.
func backslashEscapedRunes(runes []rune) []bool {
	n := len(runes)
	escaped := make([]bool, n)
	i := 0
	for i < n {
		if runes[i] != '\\' {
			i++
			continue
		}
		start := i
		for i < n && runes[i] == '\\' {
			i++
		}
		runLen := i - start
		if runLen%2 == 1 && i < n {
			escaped[i] = true
		}
	}
	return escaped
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
	"DIRECTIVE", "SPECIAL_BLOCK", "CODE_GROUP", "MATH", "SECTION ",
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
// Una columna con Elements (la forma tipada, issue #373) se escribe como
// `<<column typed>>` seguida de cada elemento con formatStrictElement, a la
// sangría de dos espacios bajo el marcador que el parser exige. Una columna
// con Content se escribe cruda, como siempre, y las dos pueden mezclarse en un
// grid. Una columna con las dos cosas a la vez no tiene forma de texto que
// conserve ambas y se rechaza (mismo principio que chart.Options o el
// contenido no representable de QUOTE/CHECKLIST: se reporta en vez de perder
// algo en silencio), igual que un grid o un encabezado tipado anidados.
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
		if len(col.Elements) > 0 {
			b.WriteString("\n<<column typed>>")
			for _, nested := range col.Elements {
				text, err := formatStrictElement(nested)
				if err != nil {
					return "", err
				}
				if text == "" {
					continue
				}
				b.WriteString("\n")
				b.WriteString(strings.TrimRight(text, "\n"))
			}
			continue
		}
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
// Content cuyo texto trimeado sea <<column>>, <<column typed>>, <<end>> o
// <<grid>> no round-trip-earía (el parser matchea esos marcadores sobre la
// línea trimeada, sin importar la sangría); una columna que mezcla Content y
// Elements no tiene forma de texto que conserve las dos; y un grid anidado en
// una columna tipada tampoco (el parser lo rechaza).
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
			if t == "<<grid>>" || t == "<<column>>" || t == "<<column typed>>" || t == "<<end>>" {
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
			if e.Columns[i].Content != "" {
				return newUnsupported("grid", fmt.Sprintf("la columna %d trae Content y Elements a la vez; la forma strict guarda el cuerpo de una columna como texto crudo (<<column>>) o como elementos (<<column typed>>), no las dos cosas", i+1))
			}
			for _, nested := range e.Columns[i].Elements {
				if _, isGrid := nested.(*ast.GridElement); isGrid {
					return newUnsupported("grid", fmt.Sprintf("la columna %d anida un grid; el dialecto strict no lo admite dentro de una columna tipada", i+1))
				}
			}
			continue
		}
		if err := checkContent(fmt.Sprintf("la columna %d", i+1), e.Columns[i].Content); err != nil {
			return err
		}
	}
	return nil
}

func formatMermaid(e *ast.MermaidElement) (string, error) {
	open, err := diagramTagOpen("mermaid", e.Title)
	if err != nil {
		return "", err
	}
	return open + "\n" + indent(e.Content, 2), nil
}

// diagramTagOpen emite la apertura de un diagrama con su pie opcional
// (`<<mermaid title="Texto">>`), el inverso de elements.parseDiagramTag. Usa
// comillas simples si el pie trae dobles; un pie con saltos de línea, con
// `>>` o con los dos tipos de comillas no tiene forma que re-parsee y se
// rechaza en vez de perderse.
func diagramTagOpen(tag, title string) (string, error) {
	if title == "" {
		return "<<" + tag + ">>", nil
	}
	if strings.ContainsAny(title, "\r\n") || strings.Contains(title, ">>") {
		return "", newUnsupported(tag, fmt.Sprintf("el pie %q no es representable en la apertura <<%s>>", title, tag))
	}
	switch {
	case !strings.Contains(title, `"`):
		return fmt.Sprintf(`<<%s title="%s">>`, tag, title), nil
	case !strings.Contains(title, "'"):
		return fmt.Sprintf(`<<%s title='%s'>>`, tag, title), nil
	default:
		return "", newUnsupported(tag, fmt.Sprintf("el pie %q mezcla comillas simples y dobles", title))
	}
}

// formatStrictMath emite <<math>> (issue #239-B) con líneas caption:/label:
// opcionales, el mismo patrón que TABLE/IMAGE: quote() + checkQuotable() —
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
	if e.Caption != "" {
		if err := checkQuotable("math", "caption", e.Caption); err != nil {
			return "", err
		}
		body += "\n" + indent("caption: "+quote(e.Caption), 2)
	}
	if e.Label != "" {
		if err := checkQuotable("math", "label", e.Label); err != nil {
			return "", err
		}
		body += "\n" + indent("label: "+quote(e.Label), 2)
	}
	body += "\n<<end>>"
	return body, nil
}

func formatPlantUML(e *ast.PlantUMLElement) (string, error) {
	open, err := diagramTagOpen("plantuml", e.Title)
	if err != nil {
		return "", err
	}
	return open + "\n" + indent(e.Content, 2), nil
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
// "options:" se emite como bloque YAML anidado al final. Esto ANTES devolvía
// UnsupportedElementError, con el argumento de que el loop de propiedades
// plano nunca poblaba Options — cierto hasta que el issue #146 le agregó
// justamente ese case (ChartParser.parseNestedOptions). Desde entonces, y
// hasta este fix, `fmt` moría en cualquier chart con options: — o sea en casi
// todos los reales. El harness de round-trip no lo detectó porque SKIPeaba
// todo fixture que devolviera UnsupportedElementError; ese skip ahora está
// acotado a un allowlist (ver document_roundtrip_test.go).
func formatChart(e *ast.ChartElement) (string, error) {
	// width/height solo se emiten si el autor los declaró (Width/Height != 0).
	// Antes salían SIEMPRE, porque ChartParser horneaba su 800x600 en el AST:
	// `fmt` escribía `width="800" height="600"` en la apertura de charts que
	// nunca los habían pedido. Mismo criterio que formatMap acá abajo, que ya
	// omitía el default (aunque comparándolo contra una constante espejo, que
	// con el parser arreglado dejó de hacer falta).
	header := fmt.Sprintf("<<chart: %s%s>>", e.ChartType, formatDimensionAttrs(e.Width, e.Height))

	if e.IsJSONMode {
		raw, err := canonicalJSON(e.RawJSON)
		if err != nil {
			return "", err
		}
		return header + "\n" + raw + "\n<</chart>>", nil
	}

	var b strings.Builder
	if len(e.SeriesTypes) > 0 {
		types, err := formatInlineArray("chart", "type", e.SeriesTypes)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "type: %s\n", types)
	}
	// Un SeriesAxes sin ningún eje declarado pero con una entrada por serie
	// (así lo arma la forma YAML anidada de un combo) se escribe tal cual:
	// omitirlo reparsearía como nil, y el AST de un build deja de ser el
	// mismo aunque ningún renderer distinga una cosa de la otra. Un chart
	// que nunca tuvo la propiedad (nil) no la emite.
	if len(e.SeriesAxes) > 0 {
		axes, err := formatInlineArray("chart", "yAxisID", e.SeriesAxes)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "yAxisID: %s\n", axes)
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
	// options: va al final por legibilidad, no por necesidad: el parser corta
	// el bloque anidado en el primer dedent (ChartParser.parseNestedOptions),
	// así que una propiedad plana después tampoco lo rompería.
	if len(e.Options) > 0 {
		opts, err := marshalYAMLIndent2(e.Options)
		if err != nil {
			return "", fmt.Errorf("formatter: chart.Options no serializable a YAML: %w", err)
		}
		b.WriteString("options:\n")
		b.WriteString(indent(strings.TrimRight(opts, "\n"), 2) + "\n")
	}
	return header + "\n" + indent(strings.TrimRight(b.String(), "\n"), 2) + "\n<<end>>", nil
}

// marshalYAMLIndent2 serializa v con 2 espacios por nivel en vez de los 4 que
// usa yaml.Marshal para mappings anidados. No es cosmético: el bloque
// resultante convive con el resto del cuerpo del chart, que va a 2, y una
// mezcla de 2 y 4 hace que ChartFormatterRule (el re-indentador del
// normalizer) lo vea como "mal formateado". El orden de las claves lo fija
// yaml, que ordena los mapas alfabéticamente — sin eso la salida de fmt no
// sería determinista, porque el orden de un map de Go es aleatorio.
func marshalYAMLIndent2(v interface{}) (string, error) {
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
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
// formatDimensionAttrs emite los atributos width/height de la línea de
// apertura de un chart o un mapa, y solo los que el autor declaró: 0 es "no
// dijo nada" (ver el comentario de ChartParser.Parse en
// internal/elements/chart.go). Es el único lugar donde el parser los lee —
// no existe una clave "width:"/"height:" en el cuerpo del bloque— así que
// omitirlos acá cuando valen 0 es exactamente lo que hace falta para no
// perder unas dimensiones custom ni inventar unas que nadie pidió.
//
// Acá vivían mapDefaultWidth/mapDefaultHeight, dos constantes que espejaban
// los defaults del parser para poder omitirlos. Sobraban en cuanto el parser
// dejó de hornearlos: ahora la señal está en el propio AST y no hay que
// adivinarla comparando contra un valor mágico —que además tenía un falso
// positivo real: un autor que declaraba `width="800"` a mano perdía el
// atributo en el round-trip, porque coincidía con el default.
func formatDimensionAttrs(width, height int) string {
	var b strings.Builder
	if width != 0 {
		fmt.Fprintf(&b, " width=%q", fmtInt(width))
	}
	if height != 0 {
		fmt.Fprintf(&b, " height=%q", fmtInt(height))
	}
	return b.String()
}

func formatMap(e *ast.MapElement) (string, error) {
	var b strings.Builder
	b.WriteString("<<map")
	b.WriteString(formatDimensionAttrs(e.Width, e.Height))
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

// formatMedia serializes MediaElement (issue #21) — unlike formatChart/
// formatMap, it's a single-line element (elements.MediaParser consumes
// exactly 1 line, no property block or "<<end>>"), so every attribute goes
// inline in the opening marker. Shared between strict and flex (same reason
// as formatChart/formatMap: no branch of MediaParser.CanParse distinguishes
// by ctx.Mode).
func formatMedia(e *ast.MediaElement) (string, error) {
	if err := checkQuotable("media", "source", e.Source); err != nil {
		return "", err
	}
	// MediaType is validated against the fixed "video"/"audio" allowlist
	// before interpolation, defaulting to "video" otherwise — same
	// defensive pattern (and same reason) as renderMediaElement's tag
	// allowlist in renderer/html.go: MediaType can arrive from an
	// externally supplied AST via the JSON --filter pipeline (issue #240),
	// not just this package's own parser, so it must never be interpolated
	// raw into the marker.
	mediaType := "video"
	if e.MediaType == "audio" {
		mediaType = "audio"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<<%s src=%s", mediaType, quote(e.Source))
	for _, attr := range []struct{ name, value string }{{"poster", e.Poster}, {"caption", e.Caption}} {
		if attr.value == "" {
			continue
		}
		if err := checkQuotable("media", attr.name, attr.value); err != nil {
			return "", err
		}
		if strings.Contains(attr.value, ">>") {
			return "", newUnsupported("media", fmt.Sprintf("%s %q contiene '>>', que cerraría la apertura", attr.name, attr.value))
		}
		fmt.Fprintf(&b, " %s=%s", attr.name, quote(attr.value))
	}
	if e.Controls {
		b.WriteString(" controls")
	}
	if e.Autoplay {
		b.WriteString(" autoplay")
	}
	if e.Loop {
		b.WriteString(" loop")
	}
	if e.Muted {
		b.WriteString(" muted")
	}
	b.WriteString(">>")
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

// nestedTypedHeading reporta si elements (o un bloque especial anidado dentro
// de ellos) contiene un HeadingElement.
func nestedTypedHeading(elements []ast.Element) bool {
	for _, el := range elements {
		switch e := el.(type) {
		case *ast.HeadingElement:
			return true
		case *ast.SpecialBlockElement:
			if nestedTypedHeading(e.Elements) {
				return true
			}
		}
	}
	return false
}
