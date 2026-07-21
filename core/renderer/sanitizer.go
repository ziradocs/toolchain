// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
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
func ProcessInlineMarkdownFormatsSecure(text string) string {
	// El texto ya está escapado, ahora aplicamos formatos markdown

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

	// Procesar código `código` -> <code>código</code>
	// El contenido ya está escapado por EscapeHTML
	text = inlineCodePattern.ReplaceAllString(text, `<code>$1</code>`)

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
		tags, ok := inlineSpanTokens[token]
		if !ok {
			// Token fuera de la allowlist: dejar el texto literal, sin inyectar.
			return match
		}
		return tags[0] + content + tags[1]
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

		return fmt.Sprintf(`<a href="%s">%s</a>`, sanitizedURL, linkText)
	})

	return text
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
