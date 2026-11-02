// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"strings"

	"github.com/mmonterroca/docxgo/v2/domain"
	"go.ziradocs.com/core/v2/ast"
)

// Listas nativas de Word (numbering.xml).
//
// Antes las listas se escribían como párrafos con la viñeta o el número como
// texto ("• ", "1. ") y solo se recorría el primer nivel: los SubPoints de un
// POINTS y los SubItems de un CHECKLIST se perdían en silencio. Ahora cada
// ítem es un párrafo con w:numPr (numId + ilvl) y su sublista se escribe
// debajo, un nivel más adentro.
//
// Cada lista (el POINTS de primer nivel y cada sublista de un ítem) tiene su
// propio w:abstractNum y su propio w:num. Word sigue contando entre dos
// w:num que comparten abstractNum si ninguno lo reinicia con un
// startOverride; con una definición por lista no hay contador compartido, así
// que dos listas numeradas seguidas, o la sublista numerada del segundo ítem,
// empiezan cada una en su propio inicio. El costo es un abstractNum de 9
// niveles por lista, unos 2.4 KB sin comprimir (bastante menos dentro del
// zip, porque los niveles se repiten casi iguales).

// docxListKind es el formato de una lista de Word.
type docxListKind int

const (
	docxListBullet docxListKind = iota
	docxListOrdered
	// docxListChecklist no dibuja marcador: la casilla (☐/☑) sale como texto
	// del ítem porque cambia de un ítem a otro y w:lvlText es por nivel. El
	// nivel solo aporta la sangría y la estructura de lista.
	docxListChecklist
)

// docxMaxListLevels es el máximo de niveles que admite una definición de
// numeración de Word (ilvl 0 a 8).
const docxMaxListLevels = domain.NumberingLevelMax + 1

// docxBulletGlyphs alterna por nivel como disc/circle/square del HTML.
var docxBulletGlyphs = []string{"•", "◦", "▪"}

type docxListDef struct {
	kind  docxListKind
	level int
	start int64
	// paren escribe el número como "1)" en vez de "1.", cuando el autor usó
	// ese delimitador.
	paren bool
}

// docxNumbering junta las listas de un documento para escribir su
// numbering.xml al final de Generate.
type docxNumbering struct {
	lists []docxListDef
}

// add registra una lista que se escribe en el nivel level y devuelve su numId.
func (n *docxNumbering) add(kind docxListKind, level int, start int64, paren bool) int {
	if start < 0 {
		start = 1
	}
	n.lists = append(n.lists, docxListDef{kind: kind, level: level, start: start, paren: paren})
	return len(n.lists)
}

// docxListIndent es la sangría de un nivel: el primero queda donde quedaban
// las listas antes (ListIndent) y cada nivel suma otro tanto.
func docxListIndent(step, level int) int {
	return step * (level + 1)
}

func (n *docxNumbering) xml(fontFamily string, step int) []byte {
	if len(n.lists) == 0 {
		return nil
	}
	const hanging = 360
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	rPr := ""
	if fontFamily != "" {
		rPr = fmt.Sprintf(`<w:rPr><w:rFonts w:ascii="%[1]s" w:hAnsi="%[1]s" w:cs="%[1]s"/></w:rPr>`, xmlAttrEscape(fontFamily))
	}
	for i, list := range n.lists {
		fmt.Fprintf(&b, `<w:abstractNum w:abstractNumId="%d"><w:multiLevelType w:val="multilevel"/>`, i)
		for lvl := 0; lvl < docxMaxListLevels; lvl++ {
			start := int64(1)
			if lvl == list.level {
				start = list.start
			}
			fmt.Fprintf(&b, `<w:lvl w:ilvl="%d"><w:start w:val="%d"/>`, lvl, start)
			left := docxListIndent(step, lvl)
			switch list.kind {
			case docxListOrdered:
				// %N es el contador del nivel N (1-based): un "%1." en el
				// nivel 2 mostraría el número del padre.
				delim := "."
				if list.paren {
					delim = ")"
				}
				fmt.Fprintf(&b, `<w:numFmt w:val="decimal"/><w:lvlText w:val="%%%d%s"/><w:lvlJc w:val="left"/>`, lvl+1, delim)
				fmt.Fprintf(&b, `<w:pPr><w:ind w:left="%d" w:hanging="%d"/></w:pPr>%s`, left, hanging, rPr)
			case docxListBullet:
				glyph := docxBulletGlyphs[lvl%len(docxBulletGlyphs)]
				fmt.Fprintf(&b, `<w:numFmt w:val="bullet"/><w:lvlText w:val="%s"/><w:lvlJc w:val="left"/>`, glyph)
				fmt.Fprintf(&b, `<w:pPr><w:ind w:left="%d" w:hanging="%d"/></w:pPr>%s`, left, hanging, rPr)
			case docxListChecklist:
				// Sin marcador ni tabulador: la casilla es el primer run del
				// párrafo, alineada con la sangría del nivel.
				fmt.Fprintf(&b, `<w:numFmt w:val="none"/><w:suff w:val="nothing"/><w:lvlText w:val=""/><w:lvlJc w:val="left"/>`)
				fmt.Fprintf(&b, `<w:pPr><w:ind w:left="%d" w:hanging="0"/></w:pPr>`, docxListIndent(step/2, lvl))
			}
			b.WriteString(`</w:lvl>`)
		}
		b.WriteString(`</w:abstractNum>`)
	}
	for i := range n.lists {
		fmt.Fprintf(&b, `<w:num w:numId="%d"><w:abstractNumId w:val="%d"/></w:num>`, i+1, i)
	}
	b.WriteString(`</w:numbering>`)
	return []byte(b.String())
}

func xmlAttrEscape(s string) string {
	r := strings.NewReplacer(`&`, "&amp;", `<`, "&lt;", `>`, "&gt;", `"`, "&quot;", `'`, "&apos;")
	return r.Replace(s)
}

// attachNumbering escribe el numbering.xml en el paquete. docxgo no expone
// SetNumberingPart en domain.Document; es el mismo método que usa su lector
// para conservar la numeración de un .docx abierto.
func (g *DOCXGenerator) attachNumbering(doc domain.Document) error {
	if g.numbering == nil || len(g.numbering.lists) == 0 {
		return nil
	}
	setter, ok := doc.(interface{ SetNumberingPart([]byte, string) })
	if !ok {
		return fmt.Errorf("DOCX: el documento no admite numbering.xml (%T); las listas no tendrían numeración", doc)
	}
	setter.SetNumberingPart(g.numbering.xml(g.style.FontFamily, g.listStep()), "numbering.xml")
	return nil
}

func (g *DOCXGenerator) listStep() int {
	if step := g.parseTwips(g.style.ListIndent); step > 0 {
		return step
	}
	return 720
}

func (g *DOCXGenerator) addList(kind docxListKind, level int, start int64) int {
	return g.addListDelim(kind, level, start, false)
}

func (g *DOCXGenerator) addListDelim(kind docxListKind, level int, start int64, paren bool) int {
	if g.numbering == nil {
		g.numbering = &docxNumbering{}
	}
	return g.numbering.add(kind, level, start, paren)
}

// docxParenDelimiter dice si el autor numeró la lista con "1)". El
// delimitador vive solo en memoria en core (PointItem.WrittenDelimiter) y se
// consulta con una aserción de interfaz, como WrittenOrdinal.
func docxParenDelimiter(items []ast.PointItem) bool {
	if len(items) == 0 {
		return false
	}
	w, ok := any(&items[0]).(interface{ WrittenDelimiter() string })
	return ok && w.WrittenDelimiter() == ")"
}

func docxListKindOf(listType string) docxListKind {
	if listType == "ordered" {
		return docxListOrdered
	}
	return docxListBullet
}

// docxSubListType es el tipo de la sublista de item: SubListType (opt-in
// nested-list-types-v1), si no el marcador que escribió el autor
// (SubListMarker, solo en memoria) y, si el AST vino de JSON y no lo trae, el
// tipo de la lista padre.
func docxSubListType(item *ast.PointItem, parentType string) string {
	if item.SubListType != "" {
		return item.SubListType
	}
	if item.SubListMarker != "" {
		return item.SubListMarker
	}
	return parentType
}

func docxStart(start *int64) int64 {
	if start == nil {
		return 1
	}
	return *start
}

// docxListStart es el inicio de una lista: el serializado (list-start-v1) o,
// si no hay, el número que el autor escribió en su primer item. Ese número
// vive solo en memoria en core (PointItem.WrittenOrdinal); se consulta con una
// aserción de interfaz para que doclang siga compilando contra un core que
// todavía no lo tiene, donde la lista empieza en 1 como antes.
func docxListStart(start *int64, items []ast.PointItem) int64 {
	if start != nil || len(items) == 0 {
		return docxStart(start)
	}
	if w, ok := any(&items[0]).(interface{ WrittenOrdinal() (int64, bool) }); ok {
		if n, has := w.WrittenOrdinal(); has {
			return n
		}
	}
	return 1
}

// docxTypedPoints dice si la lista usa nested-list-types-v1 (algún item
// declara SubListType).
func docxTypedPoints(items []ast.PointItem) bool {
	for i := range items {
		if items[i].SubListType != "" || docxTypedPoints(items[i].SubPoints) {
			return true
		}
	}
	return false
}

func (g *DOCXGenerator) addListParagraph(doc domain.Document, numID, level int) (domain.Paragraph, error) {
	p, err := doc.AddParagraph()
	if err != nil {
		return nil, err
	}
	if err := p.SetNumbering(domain.NumberingReference{ID: numID, Level: level}); err != nil {
		return nil, fmt.Errorf("invalid list numbering: %w", err)
	}
	if err := p.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter) / 2); err != nil {
		return nil, fmt.Errorf("invalid spacing after: %w", err)
	}
	return p, nil
}

func docxListTooDeep(line int) error {
	return fmt.Errorf("DOCX: la lista de la línea %d anida más de %d niveles, el máximo de Word", line, docxMaxListLevels)
}

func (g *DOCXGenerator) renderPoints(doc domain.Document, elem *ast.PointsElement) error {
	listType := elem.ListType
	if listType == "" {
		listType = "unordered"
	}
	return g.renderPointItems(doc, elem.Items, listType, docxListStart(elem.Start, elem.Items), 0, docxTypedPoints(elem.Items))
}

// renderPointItems escribe una lista en level. Una lista tipada
// (nested-list-types-v1) más profunda que los nueve niveles de Word falla: su
// contrato pide conservar la jerarquía. Una sin tipar se dibuja en el último
// nivel, como el PPTX de slidelang, en vez de cortar el documento.
func (g *DOCXGenerator) renderPointItems(doc domain.Document, items []ast.PointItem, listType string, start int64, level int, typed bool) error {
	if len(items) == 0 {
		return nil
	}
	if level >= docxMaxListLevels {
		if typed {
			return docxListTooDeep(items[0].GetPosition().Line)
		}
		level = docxMaxListLevels - 1
	}
	numID := g.addListDelim(docxListKindOf(listType), level, start, listType == "ordered" && docxParenDelimiter(items))
	for i := range items {
		item := &items[i]
		p, err := g.addListParagraph(doc, numID, level)
		if err != nil {
			return err
		}
		if err := g.renderInlineMarkdown(p, item.Content); err != nil {
			return err
		}
		if len(item.SubPoints) > 0 {
			subType := docxSubListType(item, listType)
			if err := g.renderPointItems(doc, item.SubPoints, subType, docxListStart(item.SubListStart, item.SubPoints), level+1, typed); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *DOCXGenerator) renderChecklist(doc domain.Document, elem *ast.ChecklistElement) error {
	return g.renderChecklistItems(doc, elem.Items, 0)
}

func (g *DOCXGenerator) renderChecklistItems(doc domain.Document, items []ast.ChecklistItem, level int) error {
	if len(items) == 0 {
		return nil
	}
	if level >= docxMaxListLevels {
		// Las tareas no tienen versión tipada: pasado el último nivel de Word
		// siguen en ese nivel.
		level = docxMaxListLevels - 1
	}
	numID := g.addList(docxListChecklist, level, 1)
	for i := range items {
		item := &items[i]
		p, err := g.addListParagraph(doc, numID, level)
		if err != nil {
			return err
		}
		r, err := p.AddRun()
		if err != nil {
			return err
		}
		checkbox := "☐ "
		if item.Checked {
			checkbox = "☑ "
		}
		_ = r.SetText(checkbox)
		if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(g.style.TextColor))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
		if err := g.renderInlineMarkdown(p, item.Content); err != nil {
			return err
		}
		if err := g.renderChecklistItems(doc, item.SubItems, level+1); err != nil {
			return err
		}
	}
	return nil
}

// renderBodyListItems escribe una lista de la prosa de una cita o un bloque
// (body_lists.go) como lista nativa, con la misma numeración que POINTS.
func (g *DOCXGenerator) renderBodyListItems(doc domain.Document, items []*bodyItem, level int) error {
	if level >= docxMaxListLevels {
		level = docxMaxListLevels - 1
	}
	for _, run := range bodyRuns(items) {
		kind, start := docxListBullet, int64(1)
		if run[0].ordered {
			kind, start = docxListOrdered, run[0].ordinal
		}
		numID := g.addListDelim(kind, level, start, run[0].delim == ')')
		for _, item := range run {
			p, err := g.addListParagraph(doc, numID, level)
			if err != nil {
				return err
			}
			if err := g.renderInlineMarkdown(p, item.text); err != nil {
				return err
			}
			if err := g.renderBodyListItems(doc, item.children, level+1); err != nil {
				return err
			}
		}
	}
	return nil
}
