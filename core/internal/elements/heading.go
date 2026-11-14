// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

// BuildHeadingElement construye el TextElement RawHTML `<hN id="...">` con
// que un encabezado de nivel 2-6 se representa en el AST, para los dos
// dialectos (`## Título` de flex, `SECTION "Título" / level: N` de strict) Y
// para un encabezado dentro de contenido anidado (":::bloque", columna de
// grid) vía ParseLines/HeadingParser — movida de core/parser a este paquete
// (PR-9 paso 3) precisamente para que el registry de elements pueda usarla
// sin que parser dependa hacia atrás de sí mismo. Sigue siendo una única
// función compartida y no varias porque el HTML y el anchor que produce son
// un CONTRATO, no un detalle: el generador de TOC (renderer.extractSubsections)
// los re-extrae con una expresión regular sobre este mismo `<hN id=...>`. Dos
// productores que derivaran aunque sea en el orden de los atributos
// romperían el TOC en silencio.
//
// pos es la posición YA resuelta (relativa al archivo, offset incluido) del
// encabezado. Se pasa explícita en vez de derivarse acá de un lineIndex
// crudo por dos motivos: el parser strict construye el encabezado DESPUÉS
// de consumir el cuerpo de la sección, cuando su línea actual ya no es la
// del `SECTION` (necesita capturar la posición antes); y el offset del
// frontmatter (#245) vive en cada parser/ParseContext, no en esta función
// compartida — cada caller la resuelve con su propio
// p.position(...)/ctx.Position(...).
//
// explicitID vacío deriva el anchor del texto (lo que hace flex, único
// camino posible ahí, y también ParseLines/HeadingParser). No vacío, lo usa
// como base — pero pasa por EXACTAMENTE el mismo saneado: el anchor se
// interpola crudo dentro de un atributo HTML, así que un id de autor sin
// sanear sería una inyección directa (`id: x"><script>`). Ver DeriveAnchor.
func BuildHeadingElement(text string, level int, pos diagnostics.Position, explicitID string) *ast.TextElement {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}

	// Procesar Markdown inline básico (**, *, `, etc.) de forma segura:
	// escapa el HTML del texto y aplica formatos con un procesador de un
	// solo paso (RE2, lineal) — evita el DoS de bucle infinito del antiguo
	// scanner hand-written y cierra el XSS de subsección en el mismo golpe
	// (ver docs/SECURITY_AUDIT_2026-07.md, AL-3 y CR-2).
	// Se usa la variante "Line" (no la ProcessInlineMarkdownSecure genérica):
	// un header es una sola línea y nunca debe interpretarse como lista
	// ("- Foo" no debe volverse <ul><li>Foo</li></ul> dentro de un <h3>).
	anchorSource := text
	if explicitID != "" {
		anchorSource = explicitID
	}
	anchor := DeriveAnchor(anchorSource)

	// renderer.HeadingHTML es el único productor del `<hN id>`: lo usan este
	// constructor y la bajada de HeadingElement a la forma legada, así que un
	// encabezado tipado y uno legado no pueden divergir en el HTML.
	htmlContent := renderer.HeadingHTML(level, text, anchor)

	// Expose the level as a semantic field alongside the rendered `<hN>`, so
	// a linter rule doesn't have to re-parse the HTML (issue #22).
	el := ast.NewRawHTMLTextElement(pos, htmlContent)
	el.Level = level
	// Fuente autoral en memoria para promover a HeadingElement cuando el
	// documento declara typed-headings-v1 (ver ast.PromoteTypedHeadings).
	el.HeadingSource = text
	el.HeadingAnchor = anchor
	el.HeadingContent = htmlContent
	return el
}

// DeriveAnchor es el alias local de renderer.DeriveAnchor — el algoritmo
// único de derivación de anchors. Que el `id` que embebe el parser y el href
// que genera el TOC coincidan no puede depender de dos implementaciones
// gemelas.
func DeriveAnchor(text string) string {
	return renderer.DeriveAnchor(text)
}

// HeadingParser reconoce un encabezado de nivel 3-6 ("### "… "###### ")
// DENTRO de contenido anidado — el cuerpo de un ":::bloque" delegado por
// tryParseNestedContent (PR-9 paso 3). No es lo mismo que el heading de
// nivel superior de FlexParser/DocumentFlexParser: ese sigue viviendo
// inline en cada parser (flexSubsectionLevel/isSubsectionHeader), fuera del
// registry, precisamente porque tiene que decidir fronteras de slide/
// subtítulo que este paquete no conoce. Este parser solo se alcanza a
// llamar desde un contexto que YA decidió que la línea es cuerpo de un
// bloque anidado, así que no compite con esa lógica de nivel superior.
//
// El piso es 3, no 2, en los dos dialectos por igual: un HeadingParser
// compartido no puede distinguir "estoy dentro de un deck de slidelang" de
// "estoy dentro de un documento de doclang" solo con ctx.Mode (los dos usan
// "flex"), y 3-6 es el conjunto más chico — igual al que FlexParser ya
// usaba a nivel top — que no le quita a "##" ningún significado que no
// tuviera ya dentro de un bloque anidado (ahí nunca fue una frontera de
// slide/subtítulo: eso solo existe a nivel top).
type HeadingParser struct{}

// nestedHeadingLevel devuelve el nivel (3 a 6) si trimmed es un encabezado
// de esa forma, o 0 si no. strings.TrimSpace ya corrido por el caller
// (mismo patrón que el resto de los ElementParser de este paquete).
func nestedHeadingLevel(trimmed string) int {
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level < 3 || level > 6 || level >= len(trimmed) {
		return 0
	}
	if trimmed[level] != ' ' && trimmed[level] != '\t' {
		return 0
	}
	if strings.TrimSpace(trimmed[level:]) == "" {
		return 0
	}
	return level
}

func (p *HeadingParser) CanParse(line string, mode string) bool {
	if mode != "flex" {
		return false
	}
	return nestedHeadingLevel(strings.TrimSpace(line)) > 0
}

func (p *HeadingParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{}
	}
	trimmed := strings.TrimSpace(ctx.Lines[startIndex])
	level := nestedHeadingLevel(trimmed)
	if level == 0 {
		return &ParseResult{}
	}
	text := strings.TrimSpace(trimmed[level:])

	explicitID := ""
	if ctx.HeadingAnchor != nil {
		explicitID = ctx.HeadingAnchor(text)
	}

	return &ParseResult{
		Element:       BuildHeadingElement(text, level, ctx.Position(startIndex), explicitID),
		ConsumedLines: 1,
	}
}

// HeadingAnchors reparte los anchors de los encabezados de un deck de
// SlideLang: el prefijo fijo "heading-" más un sufijo numérico cuando el
// anchor ya se usó (ver el comentario de FlexParser.uniqueHeadingAnchor para
// por qué el prefijo existe). Vive acá, y no en el parser, porque el parser
// flex, el strict y el formatter tienen que reproducir exactamente la misma
// secuencia: el formatter decide si emitir `id:` comparando el anchor real
// contra el que el parser derivaría en esa misma posición.
type HeadingAnchors struct {
	used map[string]int
	// next es, por prefijo, el sufijo desde el que buscar el próximo libre.
	// Un anchor usado no se libera salvo por Rollback, que también devuelve
	// next al valor que tenía, así que todo sufijo por debajo del último
	// asignado sigue ocupado: empezar ahí da el mismo resultado que empezar
	// en 2, sin volver a probar uno por uno los ya tomados. Sin esto, k
	// encabezados con el mismo texto costaban O(k²).
	next map[string]int
	// base, si no es nil, es la capa de abajo (ver Overlay): sus anchors
	// cuentan como usados, pero lo que se registra va solo en esta.
	base *HeadingAnchors
	// journal, en una capa, anota cada anchor registrado, en orden, para
	// poder deshacer los últimos (Rollback). Es un puntero para que las
	// copias de la capa compartan la bitácora, igual que comparten used.
	journal *[]anchorEntry
}

// anchorEntry es un registro de la bitácora de una capa: el anchor y, si
// Unique movió next, el prefijo y el valor que next tenía antes en la capa.
type anchorEntry struct {
	anchor   string
	prefixed string
	prevNext int
}

// count es cuántas veces se registró anchor, en esta capa y las de abajo.
func (a *HeadingAnchors) count(anchor string) int {
	n := 0
	for ; a != nil; a = a.base {
		n += a.used[anchor]
	}
	return n
}

// hint es el sufijo desde el que buscar el próximo libre de prefixed: el
// mayor next de esta capa y las de abajo. Cada capa garantiza que los
// sufijos por debajo de su next están ocupados en ella o debajo.
func (a *HeadingAnchors) hint(prefixed string) int {
	n := 0
	for ; a != nil; a = a.base {
		n = max(n, a.next[prefixed])
	}
	return n
}

// Derive devuelve el anchor que Unique asignaría a text, sin registrarlo.
func (a *HeadingAnchors) Derive(text string) string {
	candidate, _, _ := a.derive(text)
	return candidate
}

func (a *HeadingAnchors) derive(text string) (candidate, prefixed string, n int) {
	base := DeriveAnchor(text)
	prefixed = "heading"
	if base != "" {
		prefixed = "heading-" + base
	}
	candidate = prefixed
	n = max(2, a.hint(prefixed))
	if a.count(candidate) == 0 {
		return candidate, prefixed, 0
	}
	for candidate = prefixed + "-" + strconv.Itoa(n); a.count(candidate) > 0; n++ {
		candidate = prefixed + "-" + strconv.Itoa(n+1)
	}
	return candidate, prefixed, n
}

// Overlay devuelve una capa sobre a: ve los anchors de a, pero lo que se
// registra en ella no cambia a a y se descarta con la capa. Las copias de la
// capa comparten lo registrado. La usa el parse de prueba del cuerpo de un
// TEXT vacío (ParseContext.EmptyTextCutKeepsText).
func (a *HeadingAnchors) Overlay() HeadingAnchors {
	return HeadingAnchors{used: map[string]int{}, next: map[string]int{}, base: a, journal: &[]anchorEntry{}}
}

// Mark es el punto de la bitácora de una capa al que Rollback vuelve.
func (a *HeadingAnchors) Mark() int {
	if a.journal == nil {
		return 0
	}
	return len(*a.journal)
}

// Rollback deshace lo que la capa registró después de mark, next incluido.
// Va del último al primero para que cada next vuelva al valor de antes.
func (a *HeadingAnchors) Rollback(mark int) {
	if a.journal == nil {
		return
	}
	entries := (*a.journal)[mark:]
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		a.used[e.anchor]--
		if e.prefixed == "" {
			continue
		}
		if e.prevNext == 0 {
			delete(a.next, e.prefixed)
		} else {
			a.next[e.prefixed] = e.prevNext
		}
	}
	*a.journal = (*a.journal)[:mark]
}

// Unique deriva el anchor de text y lo registra.
func (a *HeadingAnchors) Unique(text string) string {
	candidate, prefixed, n := a.derive(text)
	a.Reserve(candidate)
	if n > 0 {
		if a.next == nil {
			a.next = make(map[string]int)
		}
		if a.journal != nil {
			e := &(*a.journal)[len(*a.journal)-1]
			e.prefixed, e.prevNext = prefixed, a.next[prefixed]
		}
		a.next[prefixed] = n
	}
	return candidate
}

// Reserve registra un anchor declarado por el autor. Devuelve false si ya se
// había usado, en cuyo caso el caller reporta el duplicado.
func (a *HeadingAnchors) Reserve(anchor string) bool {
	if a.used == nil {
		a.used = make(map[string]int)
	}
	a.used[anchor]++
	if a.journal != nil {
		*a.journal = append(*a.journal, anchorEntry{anchor: anchor})
	}
	return a.count(anchor) == 1
}
