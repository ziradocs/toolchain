// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/a11y"
)

// EscapeHTML escapes special HTML characters to prevent XSS attacks
// Replaces: & < > " '
func EscapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	text = strings.ReplaceAll(text, "\"", "&quot;")
	text = strings.ReplaceAll(text, "'", "&#39;")
	return text
}

// UnescapeHTML is the precise inverse of EscapeHTML: decodes exactly the
// five entities EscapeHTML produces (&amp; &lt; &gt; &quot; &#39;), nothing
// more — not a general-purpose HTML entity decoder (numeric refs like &#8217;
// or named ones like &nbsp; pass through unchanged). None of the five target
// strings is a substring of another, so the replacement order below cannot
// cause double-decoding.
func UnescapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	return text
}

// EscapeHTMLAttribute escapes text for use in HTML attributes
// More restrictive than EscapeHTML to prevent attribute injection
func EscapeHTMLAttribute(text string) string {
	text = EscapeHTML(text)
	return NormalizeAttributeWhitespace(text)
}

// NormalizeAttributeWhitespace colapsa saltos de línea/tabs para que un
// valor quede en una sola línea al interpolarse en un atributo HTML
// (data-label="...", data-details="..."), SIN volver a escapar entidades
// HTML. Separado de EscapeHTMLAttribute para poder aplicarse después de un
// escape que ya se hizo una vez (p.ej. ProcessVariablesSecure, que ya
// llama a EscapeHTML internamente) sin re-escapar el resultado — llamar
// EscapeHTMLAttribute de nuevo sobre texto ya escapado producía doble
// escape ("&" → "&amp;" → "&amp;amp;"), un bug cosmético en el popup de
// marcadores de mapa (#68). Perder solo la normalización de espacio en
// blanco (sin el segundo escape) habría sido una regresión aparte: un
// salto de línea literal dentro de un valor de marcador quedaría intacto
// dentro del atributo HTML generado.
func NormalizeAttributeWhitespace(text string) string {
	text = strings.ReplaceAll(text, "\n", "")
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "\t", " ")
	return text
}

// ValidateURLScheme valida el esquema de una URL contra la misma allowlist
// que SanitizeURL, pero retorna la URL validada SIN escapar. Pensado para
// consumidores que la interpolarán vía html/template (que ya aplica su
// propio escape contextual de atributo/URL) — escaparla aquí también
// produciría doble-escape (p. ej. "&" en query strings). Retorna "" si el
// esquema es peligroso o la URL es inválida.
func ValidateURLScheme(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	// Trim whitespace
	rawURL = strings.TrimSpace(rawURL)

	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// If URL is invalid, return empty string for safety
		return ""
	}

	// Get scheme in lowercase
	scheme := strings.ToLower(parsedURL.Scheme)

	// Block dangerous schemes
	dangerousSchemes := []string{"javascript", "data", "vbscript", "file"}
	for _, dangerous := range dangerousSchemes {
		if scheme == dangerous {
			return ""
		}
	}

	// Allow http, https, mailto, tel, ftp, and relative URLs
	safeSchemes := []string{"http", "https", "mailto", "tel", "ftp", ""}
	isSafe := false
	for _, safe := range safeSchemes {
		if scheme == safe {
			isSafe = true
			break
		}
	}

	if !isSafe {
		return ""
	}

	return rawURL
}

// SanitizeURL validates and sanitizes URLs to prevent javascript: and data: URI attacks
// Returns empty string if URL is potentially dangerous
func SanitizeURL(rawURL string) string {
	validated := ValidateURLScheme(rawURL)
	if validated == "" {
		return ""
	}

	// Return escaped URL
	return EscapeHTMLAttribute(validated)
}

// hexColorPattern valida colores hex de 3, 4, 6 u 8 dígitos (con o sin alfa).
var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)

// Patrones de ProcessInlineMarkdownFormatsSecure, compilados una sola vez a
// nivel de paquete en vez de en cada llamada — esta función corre por cada
// línea de párrafo/lista de todo documento (issue #39, BA-9; se hoistearon
// los demás casos del normalizer pero se pasó por alto este, el más
// caliente de todos por frecuencia de llamada).
var (
	inlineHighlightPattern     = regexp.MustCompile(`==(.*?)==`)
	inlineStrikethroughPattern = regexp.MustCompile(`~~(.*?)~~`)
	inlineBoldItalicPattern    = regexp.MustCompile(`(^|[^*])\*\*\*([^*]|[^*].*?[^*])\*\*\*($|[^*])`)
	// inlineNestedItalicInBoldPattern cubre **texto *anidado*** (negrita que
	// termina con una cursiva pegada a su cierre, fusionando 1+2 asteriscos
	// en un run de 3) — un caso DISTINTO de ***texto*** (issue #101, arriba):
	// aquí la apertura es "**" (2), no "***". Sin este patrón, el "**" no
	//-greedy de abajo consume solo 2 de los 3 asteriscos finales como su
	// cierre, dejando un "*" suelto que el patrón de cursiva luego reclama
	// cruzando el "</strong>" ya emitido, produciendo anidado inválido
	// <strong>...<em>...</strong></em> (issue #173: 68 errores
	// element-permitted-content + 6 close-order en cascada, un único
	// dangling <strong> arrastra el resto del documento). Debe correr ANTES
	// que el patrón de negrita genérico, por la misma razón que
	// inlineBoldItalicPattern corre antes: si negrita corre primero, ya
	// habrá consumido el "**" de apertura ambiguamente. Igual que ese
	// patrón, exige un carácter de contexto (o inicio/fin) sin "*" a cada
	// lado: sin este guard, un run de 4+ asteriscos (p.ej. "****texto****")
	// puede matchear PARCIALMENTE empezando a mitad del run, dejando un "*"
	// suelto a cada lado que la pasada de cursiva luego re-envuelve,
	// produciendo un <em><strong><em> doblado — una regresión real que
	// rompió TestProcessInlineMarkdownFormatsSecure_BoldItalicDelimiterRun
	// en desarrollo. Con el guard, 4+ asteriscos no matchean aquí y caen al
	// mismo comportamiento pre-existente (ya en main, no empeorado) que ya
	// tenían negrita/cursiva para ese caso límite. Limitación conocida y
	// aceptada (mismo espíritu que #101): solo cubre una única cursiva
	// anidada inmediatamente antes del cierre — con más de una cursiva
	// anidada dentro de la misma negrita (p.ej. "**a *b* c *d***") el
	// prefijo "[^*]*" no puede saltar por encima de los asteriscos
	// intermedios y el patrón simplemente no matchea, dejando el
	// comportamiento previo (ya roto, no empeorado) para ese caso más raro.
	inlineNestedItalicInBoldPattern = regexp.MustCompile(`(^|[^*])\*\*([^*]*)\*([^*\n]+)\*\*\*($|[^*])`)
	inlineBoldPattern               = regexp.MustCompile(`\*\*(.*?)\*\*`)
	inlineItalicPattern             = regexp.MustCompile(`\*([^*\n]+)\*`)
	inlineCodePattern               = regexp.MustCompile("`([^`]+)`")
	inlineLinkPattern               = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	// inlineSpanPattern reconoce spans con clase estilo pandoc
	// [contenido]{.token}: corchete + LLAVE, un delimitador que NO colisiona
	// con el enlace [texto](url) (corchete + PARÉNTESIS) ni con
	// ==/~~/**/*/`. El token se restringe a [a-zA-Z0-9-]+ (los mismos
	// caracteres que usan las claves de inlineSpanTokens, p.ej.
	// "highlight-warning"): un token con espacios/símbolos/comillas
	// (p.ej. "danger x onmouseover=...") NI SIQUIERA matchea, así que el
	// texto se deja literal. Esta es la primera de dos barreras de defensa;
	// la segunda es la validación contra el mapa fijo (ver más abajo).
	//
	// El contenido es [^\[\]]+ (ni "[" ni "]"), NO [^\]]+: prohibir el "["
	// impide que un span STRADDLE (se monte a caballo sobre) los corchetes de
	// un enlace. Con [^\]]+, en `[See [important]{.danger}](url)` el "["
	// exterior arrancaba el span y su contenido se tragaba hasta el "]" del
	// span interior ("See [important"), y la pasada de enlace posterior
	// consumía el corchete interior sobrante emitiendo HTML CRUZADO
	// <span>See <a>important</span></a> (P2 reportado en PR #260). Al excluir
	// el "[", el "[" exterior falla (su contenido se detiene en el "[" interno
	// sin encontrar "]{."), el span interior [important]{.danger} matchea
	// solo, y el enlace luego envuelve todo `[See <span>…</span>](url)` en un
	// enlace BIEN FORMADO cuyo texto contiene un span bien formado. Un enlace
	// DENTRO del contenido de un span (`[See [here](url)]{.danger}`) no matchea
	// como span (degrada a corchetes literales inertes), nunca a HTML cruzado.
	inlineSpanPattern = regexp.MustCompile(`\[([^\[\]]+)\]\{\.([a-zA-Z0-9-]+)\}`)
	// InlineLangSpanPattern reconoce spans de idioma [contenido]{lang=xx}
	// (issue #63): mismo delimitador corchete+llave que inlineSpanPattern,
	// pero SIN el punto — "lang=" es un atributo, no una clase de estilo, y
	// esa diferencia sintáctica es intencional: separa el namespace de
	// tokens de estilo (inlineSpanTokens, fijo) del de idioma (BCP 47,
	// abierto). El mismo guardián [^\[\]]+ evita que un span de idioma se
	// monte a caballo sobre los corchetes de un enlace, igual que el span de
	// clase (ver su comentario). El charset del tag [a-zA-Z0-9-]+ es
	// deliberadamente más ancho que a11y.bcp47Pattern — la primera barrera
	// es sintáctica (que el regex matchee), la segunda es semántica
	// (a11y.IsValidLangTag, el caller debe llamarlo sobre el grupo 2); un tag
	// que pasa el regex pero no el validador debe degradar a texto literal.
	//
	// El contenido excluye "\n" además de "[" y "]" — en la práctica esta
	// función nunca ve un salto de línea real (los tres entry points de
	// ProcessInlineMarkdownFormatsSecure parten por líneas antes de llamarla;
	// el único que no, ProcessInlineMarkdownSecureLine, solo lo usa
	// parseSubsectionHeader, que es single-line por construcción), pero
	// excluirlo explícitamente alinea el regex con esa invariante en vez de
	// depender de que ningún caller futuro la rompa.
	//
	// Exportado (issue #63 code review, hallazgo de regex triplicado): antes
	// vivía como var no exportada y doclang/slidelang mantenían cada uno su
	// propia copia textual del mismo patrón — las tres podían divergir
	// silenciosamente. Ahora es la única fuente de verdad; sanitizer.go,
	// populate_lang_runs.go, doclang's docx.go y slidelang's pptx.go la
	// referencian directamente.
	InlineLangSpanPattern = regexp.MustCompile(`\[([^\[\]\n]+)\]\{lang=([a-zA-Z0-9-]+)\}`)
)

// inlineSpanTokens es la ALLOWLIST FIJA de tokens de clase para spans
// [contenido]{.token} → su par de tags de salida (apertura, cierre), ambos
// HARD-CODED. Es el modelo de SanitizeColor/cssNamedColors aplicado a markup:
// el token capturado NUNCA se interpola en el HTML — solo se usa como CLAVE de
// este mapa. En un miss (token válido en charset pero fuera de la allowlist,
// p.ej. "foo") el texto se deja tal cual (ya escapado por EscapeHTML), sin
// inyectar ninguna clase desconocida. Interpolar el token crudo en
// `class="slidelang-text-" + token` sería explotable: EscapeHTML neutraliza
// comillas pero no espacios, así que un token con espacios podría inyectar
// clases/atributos extra. Con este mapa, la salida solo puede ser una de las
// cadenas literales de aquí abajo. Preserva la invariante de sanitización
// (escapar todo el contenido de usuario, inyectar solo tags de un conjunto
// fijo authored por el renderer — ver docs/architecture/sanitization.md).
var inlineSpanTokens = map[string][2]string{
	"danger":            {`<span class="slidelang-text-danger">`, `</span>`},
	"info":              {`<span class="slidelang-text-info">`, `</span>`},
	"success":           {`<span class="slidelang-text-success">`, `</span>`},
	"warning":           {`<span class="slidelang-text-warning">`, `</span>`},
	"accent":            {`<span class="slidelang-text-accent">`, `</span>`},
	"highlight-warning": {`<mark class="slidelang-highlight-warning">`, `</mark>`},
	"highlight-info":    {`<mark class="slidelang-highlight-info">`, `</mark>`},
	"highlight-success": {`<mark class="slidelang-highlight-success">`, `</mark>`},
	"underline":         {`<u>`, `</u>`},
	"small":             {`<small class="slidelang-text-small">`, `</small>`},
	"large":             {`<span class="slidelang-text-large">`, `</span>`},
}

// cssNamedColors es la allowlist de nombres de color CSS aceptados para
// markers de mapa (evita inyección vía style="background-color: ...").
var cssNamedColors = map[string]bool{
	"black": true, "silver": true, "gray": true, "white": true, "maroon": true,
	"red": true, "purple": true, "fuchsia": true, "green": true, "lime": true,
	"olive": true, "yellow": true, "navy": true, "blue": true, "teal": true,
	"aqua": true, "orange": true, "pink": true, "brown": true, "cyan": true,
	"magenta": true, "gold": true, "indigo": true, "violet": true, "coral": true,
	"salmon": true, "khaki": true, "crimson": true, "turquoise": true, "orchid": true,
	"tomato": true, "chocolate": true, "darkgreen": true, "darkblue": true,
	"darkred": true, "lightblue": true, "lightgreen": true, "lightgray": true,
	"lightgrey": true, "darkgray": true, "darkgrey": true, "transparent": true,
}

// SanitizeColor valida un valor de color destinado a interpolarse en un
// atributo style/dataset (p. ej. markers de mapa). Solo permite hex (#rgb,
// #rgba, #rrggbb, #rrggbbaa) o un nombre de color CSS conocido; cualquier
// otro valor NO VACÍO se descarta y se reemplaza por el fallback (por
// defecto, azul neutro). Un input vacío se retorna vacío sin tocar: no es
// un vector de ataque (un "background-color: ;" vacío es inerte) y así los
// llamadores conservan su propio color por defecto en vez de heredar el de
// esta función. Evita duplicar esta validación en cada CLI (ver
// docs/SECURITY_AUDIT_2026-07.md, AL-7, y issue #55 sobre divergencia
// slidelang/doclang).
func SanitizeColor(color string) string {
	const fallback = "#2196F3"
	color = strings.TrimSpace(color)
	if color == "" {
		return ""
	}
	if hexColorPattern.MatchString(color) {
		return color
	}
	if cssNamedColors[strings.ToLower(color)] {
		return strings.ToLower(color)
	}
	return fallback
}

// cssCustomPropertyNamePattern valida el nombre de una custom property CSS
// (--foo-bar): debe empezar con "--" seguido solo de letras/dígitos/guion/
// guion bajo, sin ningún carácter que permita cerrar la declaración o el
// bloque que la contiene.
var cssCustomPropertyNamePattern = regexp.MustCompile(`^--[a-zA-Z0-9_-]+$`)

// SanitizeCSSCustomProperty valida un par nombre/valor de custom property
// (--var: value;) antes de interpolarlo SIN escapar dentro de un bloque
// `:root { ... }` en un <style> generado por el servidor (ver
// generateThemeVariables en document_html.go y GenerateThemeCSS en
// doclang/themes/document/loader.go). Los valores hoy vienen de un
// registro de temas hardcodeado, pero también de temas EXTERNOS
// (theme.json cargado por ruta, sin sanitizar) — sin este chequeo, un tema
// externo con un valor como `red; } </style><script>...` rompe la regla
// CSS y el elemento <style> que la envuelve (ver
// docs/SECURITY_AUDIT_2026-07.md, BA-11). Retorna ok=false si el nombre no
// matchea cssCustomPropertyNamePattern o el valor contiene un carácter que
// permitiría escapar la declaración/bloque; el llamador debe omitir esa
// entrada en vez de escribirla.
func SanitizeCSSCustomProperty(name, value string) (safeValue string, ok bool) {
	if !cssCustomPropertyNamePattern.MatchString(name) {
		return "", false
	}
	if strings.ContainsAny(value, "{}<>;\n\r") {
		return "", false
	}
	return value, true
}

// ProcessInlineMarkdownSecure procesa formato Markdown inline con sanitización
// Esta versión escapa HTML antes de procesar markdown para prevenir XSS
func ProcessInlineMarkdownSecure(text string) string {
	// Primero escapar HTML para prevenir inyección
	text = EscapeHTML(text)

	// Ahora procesar formatos Markdown (que insertarán tags HTML seguros)
	// Procesar listas con viñetas (- item)
	lines := strings.Split(text, "\n")
	var processedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Procesar listas con viñetas
		if strings.HasPrefix(trimmed, "- ") {
			item := strings.TrimSpace(trimmed[2:]) // Remover "- "
			item = ProcessInlineMarkdownFormatsSecure(item)
			processedLines = append(processedLines, "<li>"+item+"</li>")
		} else if trimmed != "" {
			// Procesar líneas normales
			processed := ProcessInlineMarkdownFormatsSecure(trimmed)
			processedLines = append(processedLines, processed)
		} else {
			// Línea vacía
			processedLines = append(processedLines, "")
		}
	}

	// Unir líneas con <br> pero envolver listas en <ul>
	var result strings.Builder
	var inList bool

	for i, line := range processedLines {
		if strings.HasPrefix(line, "<li>") {
			if !inList {
				result.WriteString("<ul>")
				inList = true
			}
			result.WriteString(line)
		} else {
			if inList {
				result.WriteString("</ul>")
				inList = false
			}
			if line != "" {
				result.WriteString(line)
			}
			// Agregar <br> excepto para la última línea
			if i < len(processedLines)-1 && line != "" {
				nextLine := ""
				if i+1 < len(processedLines) {
					nextLine = processedLines[i+1]
				}
				// No agregar <br> si la siguiente línea es una lista o está vacía
				if nextLine != "" && !strings.HasPrefix(nextLine, "<li>") {
					result.WriteString("<br>")
				}
			}
		}
	}

	if inList {
		result.WriteString("</ul>")
	}

	return result.String()
}

// ProcessInlineMarkdownSecureLine procesa formato Markdown inline con
// sanitización para texto de una sola línea (títulos, headers, subsecciones)
// que NUNCA debe tratarse como lista ni partirse en <br>: a diferencia de
// ProcessInlineMarkdownSecure, no interpreta un "- " inicial como viñeta ni
// une múltiples líneas — solo escapa HTML y aplica los formatos inline
// (bold/italic/code/highlight/strikethrough/links).
func ProcessInlineMarkdownSecureLine(text string) string {
	return ProcessInlineMarkdownFormatsSecure(EscapeHTML(text))
}

// ProcessInlineMarkdownSecureMultiline aplica formatos inline línea por línea
// y une el resultado con <br>, sin nunca emitir HTML de bloque (ni <ul>/<li>
// para un "- " inicial, ni headings) — para usar dentro de un <p> ya
// existente, donde HTML de bloque anidado sería inválido, pero donde
// preservar saltos de línea (p.ej. una quote multilínea) sigue importando.
// A diferencia de ProcessInlineMarkdownSecureLine (pensada para texto de una
// sola línea), esta SÍ preserva "\n" como "<br>".
func ProcessInlineMarkdownSecureMultiline(text string) string {
	text = EscapeHTML(text)
	lines := strings.Split(text, "\n")

	var result strings.Builder
	for i, line := range lines {
		result.WriteString(ProcessInlineMarkdownFormatsSecure(strings.TrimSpace(line)))
		if i < len(lines)-1 {
			result.WriteString("<br>")
		}
	}

	return result.String()
}

// ProcessInlineMarkdownFormatsSecure procesa los formatos inline de markdown de forma segura
// NOTA: Asume que el texto ya fue escapado con EscapeHTML
//
// Issue #63 code review finding #7: código “ `así` “ se protege ANTES de
// cualquier otra pasada, sustituyendo cada `código` por un centinela
// (<zdc0>, <zdc1>, ...) que ninguna pasada posterior reconoce como sintaxis
// propia, y restaurándolo a <code>...</code> al final. Antes de este fix,
// código corría en la posición 7 de 10 (después de highlight/strikethrough/
// bold-italic/nested-italic/bold/italic), así que “ `**a**` “ producía
// <code><strong>a</strong></code> — el contenido de un code span se
// interpretaba como markdown en vez de mostrarse literal, para las 6
// pasadas que corrían antes. El centinela es seguro de forjar: el texto
// llega pre-escapado (ver la nota de la función), así que un "<" real
// jamás puede existir en texto de usuario en este punto — solo lo emite
// este archivo — y ninguna de las pasadas restantes (marcado, span,
// enlace) reconoce "<zdcN>" como sintaxis propia.
func ProcessInlineMarkdownFormatsSecure(text string) string {
	// El texto ya está escapado, ahora aplicamos formatos markdown

	var codeSpans []string
	text = inlineCodePattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := inlineCodePattern.FindStringSubmatch(match)
		codeSpans = append(codeSpans, submatches[1])
		return fmt.Sprintf("<zdc%d/>", len(codeSpans)-1)
	})

	// Procesar resaltado ==texto== -> <mark>texto</mark>
	text = inlineHighlightPattern.ReplaceAllString(text, `<mark>$1</mark>`)

	// Procesar tachado ~~texto~~ -> <del>texto</del>
	text = inlineStrikethroughPattern.ReplaceAllString(text, `<del>$1</del>`)

	// Procesar negrita+cursiva ***texto*** -> <strong><em>texto</em></strong>
	// Debe correr ANTES de negrita/cursiva por separado: de lo contrario
	// negrita (no-greedy **texto**) consume "**texto**" de "***texto***"
	// dejando un "*" suelto, y luego cursiva envuelve ese "*" sobrante junto
	// con el "</strong>" ya emitido, produciendo un anidado cruzado
	// <strong><em>texto</strong></em> en vez de anidado válido (issue #101).
	// El patrón exige un carácter de contexto (o inicio/fin de texto) sin
	// "*" a cada lado del delimitador: sin esto, una racha de 4+ asteriscos
	// (p.ej. "****texto****") consumía solo 3 de cada lado, dejando un "*"
	// suelto que las pasadas de negrita/cursiva de más abajo re-envolvían
	// alrededor del HTML ya emitido, produciendo un anidado roto. Con el
	// contexto exigido, una racha de 4+ no matchea aquí y cae exactamente al
	// mismo comportamiento pre-existente (ya en main antes de este fix) que
	// ya tenían negrita/cursiva para ese caso límite.
	text = inlineBoldItalicPattern.ReplaceAllString(text, `$1<strong><em>$2</em></strong>$3`)

	// Procesar negrita-con-cursiva-anidada-al-final **texto *anidado*** ->
	// <strong>texto <em>anidado</em></strong> (issue #173; ver comentario en
	// la declaración de inlineNestedItalicInBoldPattern). Debe correr ANTES
	// que negrita genérica, por la misma razón que el patrón de arriba.
	text = inlineNestedItalicInBoldPattern.ReplaceAllString(text, `$1<strong>$2<em>$3</em></strong>$4`)

	// Procesar negritas **texto** -> <strong>texto</strong>
	text = inlineBoldPattern.ReplaceAllString(text, `<strong>$1</strong>`)

	// Procesar cursivas *texto* -> <em>texto</em>
	// La captura exige contenido no vacío y sin '*': un "**" residual (p.ej.
	// una negrita sin cerrar que el regex de arriba no consumió) no debe
	// producir un <em></em> vacío en el HTML generado (issue #12e1).
	text = inlineItalicPattern.ReplaceAllString(text, `<em>$1</em>`)

	// Procesar spans con clase [contenido]{.token} -> tag fijo de la allowlist.
	// Corre DESPUÉS de negrita/cursiva/código (para que el contenido interno ya
	// lleve esos formatos aplicados: [**bold** text]{.danger} conserva la
	// negrita) y ANTES del enlace: si un enlace queda DENTRO del contenido de un
	// span ([ver [aquí](url)]{.info}), el "[^\]]+" del span se detiene en el
	// primer "]", el span no matchea, y el enlace se procesa normal dejando los
	// corchetes literales — una degradación inerte, preferible al anidado roto
	// <a>…<span>…</a></span> que produciría correr el span DESPUÉS del enlace.
	// El token se valida contra la allowlist FIJA inlineSpanTokens; en un miss
	// se devuelve el match intacto (ya escapado), nunca una clase arbitraria.
	text = inlineSpanPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := inlineSpanPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		content := submatches[1]
		token := submatches[2]
		// issue #63 code review finding #8: content puede cruzar el tag de
		// una pasada anterior (p.ej. "**[a**]{.danger}" — el "**" de cierre
		// cae DENTRO de los corchetes) — ver bracketContentTagsBalanced.
		if !bracketContentTagsBalanced(content) {
			return match
		}
		tags, ok := inlineSpanTokens[token]
		if !ok {
			// Token fuera de la allowlist: dejar el texto literal, sin inyectar.
			return match
		}
		return tags[0] + content + tags[1]
	})

	// Procesar spans de idioma [contenido]{lang=fr} -> <span lang="fr">contenido</span>
	// (issue #63). Corre DESPUÉS del span de clase (mismo razonamiento: el
	// contenido interno ya lleva bold/italic/code aplicados) y ANTES del
	// enlace (mismo razonamiento de degradación inerte que el span de clase).
	//
	// A diferencia de inlineSpanTokens (un mapa fijo cuyo propósito explícito
	// es que el token capturado NUNCA llegue al HTML, solo se use como clave),
	// acá el valor SÍ tiene que llegar a la salida — no hay forma de marcar
	// "este pasaje está en francés" sin escribir "fr" en algún lado. Eso
	// invierte el invariante que el resto de este archivo sostiene, así que
	// la defensa se mueve a la validación en vez de a la indirección: el tag
	// capturado se exige contra a11y.IsValidLangTag (BCP 47 sintáctico —
	// ^[a-zA-Z]{2,8}(-[a-zA-Z0-9]{2,8})*$, ASCII puro por clases explícitas
	// para bloquear homoglifos Unicode, ver su doc comment) ANTES de
	// interpolarlo, más EscapeHTMLAttribute como defensa en profundidad. Un
	// tag que no pasa el validador (p.ej. "es_MX", o el mismo "eſ" con el
	// homoglifo de "s" larga) degrada a texto literal — mismo comportamiento
	// que un miss de la allowlist del span de clase.
	text = InlineLangSpanPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := InlineLangSpanPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		content := submatches[1]
		tag := submatches[2]
		// issue #63 code review finding #8: mismo chequeo que la pasada de
		// span de clase — ver bracketContentTagsBalanced.
		if !bracketContentTagsBalanced(content) {
			return match
		}
		if !a11y.IsValidLangTag(tag) {
			// Tag fuera de forma BCP 47: dejar el texto literal, sin inyectar.
			return match
		}
		return `<span lang="` + EscapeHTMLAttribute(tag) + `">` + content + `</span>`
	})

	// Procesar enlaces [texto](url) -> <a href="url">texto</a>
	// IMPORTANTE: Sanitizar URLs para prevenir javascript: y data: URIs
	text = inlineLinkPattern.ReplaceAllStringFunc(text, func(match string) string {
		// Extraer texto y URL
		submatches := inlineLinkPattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		linkText := submatches[1]
		linkURL := submatches[2]

		// issue #63 code review finding #8: mismo chequeo que las pasadas de
		// span — ver bracketContentTagsBalanced. linkText, no linkURL: la URL
		// no lleva tags HTML, es el texto visible el que puede cruzar un tag
		// de una pasada anterior (p.ej. "**[a**](url)").
		if !bracketContentTagsBalanced(linkText) {
			return match
		}

		// Decode HTML entities that were escaped (for URLs in variables)
		linkURL = strings.ReplaceAll(linkURL, "&lt;", "<")
		linkURL = strings.ReplaceAll(linkURL, "&gt;", ">")
		linkURL = strings.ReplaceAll(linkURL, "&quot;", "\"")
		linkURL = strings.ReplaceAll(linkURL, "&#39;", "'")
		linkURL = strings.ReplaceAll(linkURL, "&amp;", "&")

		// Sanitizar URL
		sanitizedURL := SanitizeURL(linkURL)
		if sanitizedURL == "" {
			// URL peligrosa, mostrar solo el texto sin enlace
			return linkText
		}

		// Restaurar un centinela de código que cae dentro del grupo URL
		// (p.ej. "[a](`url`)", donde el code span ES el destino del
		// enlace) — ver el comentario de restoreCodeSentinelsInURL sobre
		// por qué esto se hace ACÁ, localizado a sanitizedURL, y no en el
		// pase global de restauración de más abajo.
		sanitizedURL = restoreCodeSentinelsInURL(sanitizedURL, codeSpans)

		return fmt.Sprintf(`<a href="%s">%s</a>`, sanitizedURL, linkText)
	})

	// Restaurar los centinelas de código al final, ya con <code>...</code> —
	// ver el comentario de esta función sobre por qué código se protege
	// primero. SOLO la forma cruda "<zdcN/>" (advisor follow-up + code-review
	// de esta misma PR, hallazgo confirmado): el texto llega pre-escapado
	// con EscapeHTML ANTES de que esta función corra (ver la nota al inicio
	// de la función), así que un "<zdcN/>" crudo en `text` a esta altura
	// SOLO puede venir de la inserción de centinela de esta misma función —
	// ningún texto de usuario puede sobrevivir con un "<" literal hasta
	// acá. La forma ESCAPADA "&lt;zdcN/&gt;" NO tiene esa garantía: EscapeHTML
	// convierte cualquier "<" de usuario en "&lt;" (sin tocar el "/"), así
	// que un usuario que escriba literalmente "<zdc0/>" en texto plano —
	// fuera de cualquier code span, sin pasar por SanitizeURL — produce
	// exactamente esa cadena tras el escape de entrada. Restaurarla acá de
	// forma global (como hacía una versión anterior de este fix) mangla
	// texto de usuario arbitrario que nunca tuvo nada que ver con el
	// esquema interno de centinelas. La única fuente REAL de la forma
	// escapada es SanitizeURL → EscapeHTMLAttribute sobre una URL que
	// contenía un centinela crudo, y esa restauración ya se hizo arriba,
	// LOCALIZADA a sanitizedURL, en el momento en que se conoce con certeza
	// que la forma escapada vino de ahí.
	//
	// UN SOLO pase con ReplaceAllStringFunc, no N pases de strings.ReplaceAll
	// (code-review de esta misma PR, hallazgo confirmado): un loop de
	// strings.ReplaceAll por índice vuelve a escanear la cadena COMPLETA en
	// cada iteración, incluyendo el texto que una iteración anterior del
	// mismo loop acaba de insertar — si el contenido literal de un code span
	// contiene la subcadena "<zdcK/>" para cualquier K válido (p.ej. un
	// usuario documentando el propio formato del centinela, o el contenido
	// real de un code span posterior con índice K), ese texto recién
	// restaurado se vuelve a matchear y reescribe, produciendo un <code>
	// anidado espurio o, peor, sustituyendo el contenido de un span por el
	// de OTRO. ReplaceAllStringFunc hace un solo barrido lineal sobre el
	// texto ORIGINAL (antes de esta pasada) y nunca vuelve a escanear lo
	// que la función de reemplazo devuelve, así que ninguna sustitución
	// puede alimentar a otra — y de paso baja el costo de
	// O(N * len(texto)) a O(len(texto)).
	if len(codeSpans) > 0 {
		text = zdcRawSentinelPattern.ReplaceAllStringFunc(text, func(match string) string {
			sub := zdcRawSentinelPattern.FindStringSubmatch(match)
			idx, err := strconv.Atoi(sub[1])
			// Índice fuera de rango: no puede pasar (el patrón solo matchea
			// dígitos y este archivo solo emite índices válidos), pero deja
			// el texto intacto en vez de indexar fuera de rango si algún
			// día deja de ser cierto.
			if err != nil || idx < 0 || idx >= len(codeSpans) {
				return match
			}
			return "<code>" + codeSpans[idx] + "</code>"
		})
	}

	return text
}

// restoreCodeSentinelsInURL restaura, DENTRO de una URL ya sanitizada
// (post-SanitizeURL), un centinela de código que cayó en el destino de un
// enlace — p.ej. "[a](`url`)", donde el code span ES la URL. SanitizeURL →
// EscapeHTMLAttribute escapa el centinela crudo una vez ("<zdc0/>" →
// "&lt;zdc0/&gt;") antes de que este helper corra, así que acá solo hace
// falta la forma escapada.
//
// Deliberadamente LOCALIZADO a `url` (advisor + code-review de esta misma
// PR, hallazgo confirmado): una versión anterior de este fix hacía esta
// misma sustitución de forma GLOBAL sobre el texto completo al final de
// ProcessInlineMarkdownFormatsSecure. Eso era inseguro — a diferencia del
// centinela crudo ("<zdcN/>", que solo esta función puede producir porque
// el texto llega pre-escapado), la forma ESCAPADA sí puede originarse en
// texto de usuario ORDINARIO: EscapeHTML (aplicado a TODO el texto de
// entrada antes de que esta función corra) convierte cualquier "<" de
// usuario en "&lt;" sin tocar el "/", así que un usuario que escribe
// literalmente "<zdc0/>" en prosa normal —sin código, sin URL de por
// medio— produce exactamente "&lt;zdc0/&gt;". Un reemplazo global habría
// mangleado ese texto de usuario, reemplazándolo por <code>...</code> sin
// relación alguna. Restaurar solo dentro de `url`, en el único punto del
// pipeline donde se sabe con certeza que la forma escapada vino
// legítimamente de SanitizeURL, evita ese falso positivo por completo.
func restoreCodeSentinelsInURL(url string, codeSpans []string) string {
	if len(codeSpans) == 0 {
		return url
	}
	return zdcEscapedSentinelPattern.ReplaceAllStringFunc(url, func(match string) string {
		sub := zdcEscapedSentinelPattern.FindStringSubmatch(match)
		idx, err := strconv.Atoi(sub[1])
		if err != nil || idx < 0 || idx >= len(codeSpans) {
			return match
		}
		return "&lt;code&gt;" + codeSpans[idx] + "&lt;/code&gt;"
	})
}

// zdcRawSentinelPattern/zdcEscapedSentinelPattern reconocen, por separado,
// las dos formas en que un centinela de código interno puede aparecer —
// ver los comentarios sobre dónde se usa cada uno en
// ProcessInlineMarkdownFormatsSecure y restoreCodeSentinelsInURL. Separados
// en dos patrones (no uno con dos grupos de alternancia) a propósito: cada
// uno se usa en un contexto distinto donde solo esa forma es segura de
// restaurar — mantenerlos separados hace ese alcance explícito en el tipo.
var (
	zdcRawSentinelPattern     = regexp.MustCompile(`<zdc(\d+)/>`)
	zdcEscapedSentinelPattern = regexp.MustCompile(`&lt;zdc(\d+)/&gt;`)
)

// bracketTagRe reconoce un tag HTML de apertura o cierre — usado por
// htmlTagsWellNested. Grupo 3 (atributos + posible "/" final) se usa para
// detectar tags self-closing (<br>, y el centinela interno <zdcN/> de esta
// misma función — ver ProcessInlineMarkdownFormatsSecure). Solo nombres
// alfanuméricos (sin ":"/"-" con namespace, que este archivo nunca emite),
// consistente con el resto del paquete.
var bracketTagRe = regexp.MustCompile(`<(/?)([a-zA-Z0-9]+)([^>]*)>`)

// htmlTagsWellNested reporta si html contiene solo tags bien anidados: cada
// tag de cierre tiene su apertura más temprana AÚN ABIERTA en html, y
// ningún tag queda abierto sin su cierre correspondiente también en html.
// Tags self-closing (que terminan en "/>", p.ej. <br> o el centinela
// interno <zdcN/>) no tienen cierre que exigir y se saltan.
//
// Compartida por dos usos relacionados pero distintos (issue #63 code
// review finding #8 + advisor follow-up, que señaló que las dos copias de
// este algoritmo en sanitizer_test.go habían divergido — una saltaba <br>,
// la otra no — precisamente porque no compartían una sola fuente):
//
//  1. bracketContentTagsBalanced (producción, abajo): gatea el CONTENIDO
//     capturado de un [content]{...}/[content](url) ANTES de envolverlo en
//     un tag nuevo — sin esto, un delimitador cuyo cierre cae DENTRO de los
//     corchetes (p.ej. "**[a**]{.danger}", donde el "**" de cierre queda
//     adentro) hace que la pasada de negrita —que corre ANTES que las de
//     span/lang/enlace— emita su apertura ANTES del "[" y su cierre DENTRO
//     del contenido capturado; la pasada de span envuelve ESE contenido con
//     su propio tag, produciendo HTML CRUZADO
//     (<strong><span class="...">a</strong></span>) en vez de anidado
//     válido. El caso simétrico (apertura sin cierre, p.ej.
//     "[a **b]{.danger} c**") produce el mismo cruce en la otra dirección —
//     el algoritmo LIFO detecta ambos: un cierre sin apertura en la pila
//     falla de inmediato, y una pila no vacía al final significa una
//     apertura sin cerrar. Contenido bien anidado DENTRO de content (p.ej.
//     "**bold** text", donde tanto la apertura como el cierre de <strong>
//     quedan adentro) sigue permitido — el caso legítimo que ya cubren los
//     tests existentes ([**bold** text]{.danger} →
//     <span>...<strong>bold</strong> text</span>).
//  2. Los tests de sanitizer_test.go la usan sobre la SALIDA completa de
//     ProcessInlineMarkdownFormatsSecure, como verificación end-to-end de
//     que ningún par de pasadas produjo HTML cruzado.
func htmlTagsWellNested(html string) bool {
	var stack []string
	for _, m := range bracketTagRe.FindAllStringSubmatch(html, -1) {
		closing := m[1] == "/"
		name := strings.ToLower(m[2])
		attrs := m[3]
		if name == "br" || strings.HasSuffix(strings.TrimSpace(attrs), "/") {
			continue // void o self-closing: sin cierre que exigir
		}
		if !closing {
			stack = append(stack, name)
			continue
		}
		if len(stack) == 0 || stack[len(stack)-1] != name {
			return false
		}
		stack = stack[:len(stack)-1]
	}
	return len(stack) == 0
}

// bracketContentTagsBalanced es htmlTagsWellNested aplicado al interior
// capturado de un [content]{.token}/[content]{lang=xx}/[content](url) —
// ver el punto 1 del doc comment de htmlTagsWellNested.
func bracketContentTagsBalanced(content string) bool {
	return htmlTagsWellNested(content)
}

// variablePlaceholderPattern encuentra placeholders {{variable_name}}.
// Compilado una sola vez y compartido por ProcessVariablesSecure y
// ProcessVariablesEscapeValues en vez de recompilarse en cada llamada.
var variablePlaceholderPattern = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

// ProcessVariablesSecure reemplaza variables y escapa el resultado
func ProcessVariablesSecure(text string, variables map[string]interface{}) string {
	if variables == nil {
		return EscapeHTML(text)
	}

	// Primero escapar todo el texto
	escapedText := EscapeHTML(text)

	result := variablePlaceholderPattern.ReplaceAllStringFunc(escapedText, func(match string) string {
		// Extraer el nombre de la variable (sin las llaves)
		varName := match[2 : len(match)-2]

		if value, exists := variables[varName]; exists {
			// Escapar el valor de la variable para prevenir XSS
			return EscapeHTML(fmt.Sprintf("%v", value))
		}

		// Si la variable no existe, devolver el match escapado
		return match
	})

	return result
}

// ProcessVariablesEscapeValues sustituye {{variable_name}} en un texto que YA
// es HTML de confianza (p. ej. un título de subsección ya procesado por
// ProcessInlineMarkdownSecureLine, que puede contener tags reales como
// <strong>/<em>/<code>) — a diferencia de ProcessVariablesSecure, NO escapa
// el texto circundante (lo destruiría), solo el valor sustituido de cada
// variable. Ver docs/SECURITY_AUDIT_2026-07.md, CR-2: sin esto, una variable
// de frontmatter referenciada en un heading (## Foo {{var}}) se inyectaba sin
// escapar en el TOC/sidebar del documento.
func ProcessVariablesEscapeValues(text string, variables map[string]interface{}) string {
	if variables == nil {
		return text
	}

	return variablePlaceholderPattern.ReplaceAllStringFunc(text, func(match string) string {
		varName := match[2 : len(match)-2]

		if value, exists := variables[varName]; exists {
			return EscapeHTML(fmt.Sprintf("%v", value))
		}

		return match
	})
}

// ProcessTextWithVariablesAndMarkdownSecure procesa variables y formato Markdown de forma segura
func ProcessTextWithVariablesAndMarkdownSecure(text string, variables map[string]interface{}) string {
	// Primero reemplazar variables
	processed := ProcessVariables(text, variables)
	// Luego procesar formato Markdown de forma segura
	return ProcessInlineMarkdownSecure(processed)
}
