// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
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
	processedText := renderer.ProcessInlineMarkdownSecureLine(text)

	anchorSource := text
	if explicitID != "" {
		anchorSource = explicitID
	}
	anchor := DeriveAnchor(anchorSource)

	htmlContent := fmt.Sprintf("<h%d id=\"%s\">%s</h%d>", level, anchor, processedText, level)

	// Expose the level as a semantic field alongside the rendered `<hN>`, so
	// a linter rule doesn't have to re-parse the HTML (issue #22).
	el := ast.NewRawHTMLTextElement(pos, htmlContent)
	el.Level = level
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
