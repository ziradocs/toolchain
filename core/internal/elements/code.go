// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// CodeParser handles parsing of code blocks
type CodeParser struct{}

// CanParse determines if the current line starts a code block
func (p *CodeParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// Strict mode: CODE keyword
	if mode == "strict" {
		return strings.HasPrefix(trimmed, "CODE")
	}

	// Flex mode: ```language fenced code blocks
	if mode == "flex" {
		return strings.HasPrefix(trimmed, "```")
	}

	return false
}

// Parse extracts a code block element
func (p *CodeParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := ctx.Position(startIndex)
	line := strings.TrimSpace(ctx.Lines[startIndex])

	if ctx.Mode == "strict" {
		element, consumed, err := p.parseStrictCode(ctx, startIndex, pos, line)
		return &ParseResult{
			Element:       element,
			ConsumedLines: consumed,
			Error:         err,
		}
	} else {
		element, consumed, err := p.parseFlexCode(ctx, startIndex, pos, line)
		return &ParseResult{
			Element:       element,
			ConsumedLines: consumed,
			Error:         err,
		}
	}
}

// CodeVerbatimMarker is the attribute that follows the CODE keyword to say that
// the body keeps all of its indentation past the structural one (the header's
// plus two spaces), instead of having the whitespace its lines share removed.
const CodeVerbatimMarker = "{verbatim}"

// parseStrictCode handles strict mode code parsing: CODE language
func (p *CodeParser) parseStrictCode(ctx *ParseContext, startIndex int, pos diagnostics.Position, line string) (ast.Element, int, error) {
	// `CODE{verbatim} ...` es la misma cabecera con la base de sangría fija. Se
	// reconoce sólo con la palabra completa seguida de un espacio o del fin de
	// la línea; la cabecera se lee después como si dijera `CODE ...`.
	verbatim := false
	if rest, ok := strings.CutPrefix(line, "CODE"+CodeVerbatimMarker); ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
		verbatim = true
		line = "CODE" + rest
	}
	parts := strings.Fields(line)
	consumedLines := 1 // skip CODE line

	// `CODE <lenguaje> <archivo>`: el tercer token es el nombre de archivo
	// (code-filename-v1). Antes se descartaba en silencio, igual que
	// cualquier token de más; ahora un cuarto token es un error.
	language, filename := "", ""
	var headerErr error
	if len(parts) > 1 {
		language = parts[1]
	}
	if len(parts) > 2 {
		filename = parts[2]
	}
	if len(parts) > 3 {
		headerErr = fmt.Errorf("CODE takes at most a language and a filename; unexpected %q", strings.Join(parts[3:], " "))
	}
	// Una segunda palabra que empieza con `{` o `[` no es un nombre de archivo:
	// es lo que una cerca flex guarda como parte del lenguaje (líneas resaltadas,
	// ```python {1,3-5}, o una etiqueta de code-group), y así la guarda
	// parseFenceInfo. La cabecera strict la lee igual, para poder reproducir lo
	// que flex ya guarda; el resaltado no tiene semántica propia en strict. Antes
	// era un error (el contrato de code-filename rechaza ese nombre de archivo).
	// Solo con la palabra CODE completa: una línea que empieza con otra palabra
	// (CODEX python {1}) no pasa a leerse como cabecera de código por esta regla.
	if parts[0] == "CODE" && len(parts) > 2 && (strings.HasPrefix(parts[2], "[") || strings.HasPrefix(parts[2], "{")) {
		language = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "CODE"))
		filename = ""
		headerErr = nil
	}
	// El cuerpo son las líneas con más sangría que la propia cabecera `CODE`
	// (las vacías van incluidas), igual que el cuerpo de un SLIDE o de una
	// columna tipada. Antes terminaba en la primera línea con MENOS sangría que
	// la primera del cuerpo: un cuerpo cuya primera línea estaba más hundida que
	// las siguientes se cortaba ahí, y el resto desaparecía sin diagnóstico
	// (issue #400).
	headerIndent := CalculateIndentLevel(ctx.Lines[startIndex])
	body := make([]string, 0, 8)
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		raw := ctx.Lines[i]
		if strings.TrimSpace(raw) != "" && CalculateIndentLevel(raw) <= headerIndent {
			break
		}
		body = append(body, raw)
	}
	consumedLines += len(body)

	// La sangría estructural es, por omisión, el prefijo de espacios/tabs que
	// comparten TODAS las líneas con texto, comparado como cadena y no por
	// columnas: así un tab vale un tab y no cuatro espacios, y una línea con
	// espacios y tab ya no conserva toda su sangría dentro del contenido. Lo que
	// sobra de ese prefijo en una línea es sangría del propio código y se
	// conserva. Ese prefijo no distingue la sangría que el cuerpo comparte por
	// ser código de la que pone la estructura, así que `CODE{verbatim}` la fija:
	// la de la cabecera más dos espacios, que es lo que escribe fmt. Una línea
	// escrita a mano con menos sangría que esa base pierde sólo la que tiene.
	var base string
	hasText := true
	if verbatim {
		header := ctx.Lines[startIndex]
		base = header[:len(header)-len(strings.TrimLeft(header, " \t"))] + "  "
	} else {
		base, hasText = commonIndentPrefix(body)
	}
	lines := make([]string, len(body))
	for i, raw := range body {
		switch {
		case strings.TrimSpace(raw) == "":
			// Una línea con sólo espacios conserva lo que pase de la base
			// (fmt la escribe con la base por delante); una más corta es vacía.
			if rest, ok := strings.CutPrefix(raw, base); ok && hasText {
				lines[i] = rest
			}
		default:
			lead := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
			n := 0
			for n < len(lead) && n < len(base) && lead[n] == base[n] {
				n++
			}
			lines[i] = raw[n:]
		}
	}
	content := strings.Join(lines, "\n")

	codeElement := ast.NewCodeElement(pos, language, content)
	codeElement.Filename = filename
	return codeElement, consumedLines, headerErr
}

// commonIndentPrefix devuelve el prefijo de espacios y tabs que comparten todas
// las líneas con texto de lines (las vacías o de sólo espacios no cuentan), y si
// había alguna línea con texto.
func commonIndentPrefix(lines []string) (string, bool) {
	prefix, seen := "", false
	for _, raw := range lines {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		lead := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
		if !seen {
			prefix, seen = lead, true
			continue
		}
		n := 0
		for n < len(prefix) && n < len(lead) && prefix[n] == lead[n] {
			n++
		}
		prefix = prefix[:n]
	}
	return prefix, seen
}

// parseFlexCode handles flex mode code parsing: ```language
func (p *CodeParser) parseFlexCode(ctx *ParseContext, startIndex int, pos diagnostics.Position, line string) (ast.Element, int, error) {
	// ```lenguaje [archivo] o ```lenguaje title="archivo"
	language, filename, headerErr := parseFenceInfo(strings.TrimSpace(line[3:]))
	consumedLines := 1 // skip opening ``` line

	var content strings.Builder
	lineCount := 0

	// Collect content until closing ```
	for i := startIndex + 1; i < len(ctx.Lines); i++ {
		line := ctx.Lines[i]

		if strings.TrimSpace(line) == "```" {
			consumedLines++ // count closing ```
			break
		}

		if lineCount > 0 {
			content.WriteString("\n")
		}
		content.WriteString(line)
		lineCount++
		consumedLines++
	}

	codeElement := ast.NewCodeElement(pos, language, content.String())
	codeElement.Filename = filename
	return codeElement, consumedLines, headerErr
}

// parseFenceInfo separa la línea de info de una fence (```ts renewals.ts o
// ```ts title="renewals.ts") en lenguaje y nombre de archivo. Antes todo el
// resto de la línea terminaba en Language ("ts renewals.ts"), así que el
// lenguaje quedaba contaminado y el nombre sin campo propio.
func parseFenceInfo(info string) (language, filename string, err error) {
	if info == "" {
		return "", "", nil
	}
	language, rest, _ := strings.Cut(info, " ")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return language, "", nil
	}
	// `[etiqueta]` (la sintaxis de code-group) y `{1,3-5}` (líneas
	// resaltadas) no son nombres de archivo: conservan el comportamiento
	// anterior, en el que la línea completa queda como Language.
	if strings.HasPrefix(rest, "[") || strings.HasPrefix(rest, "{") {
		return info, "", nil
	}
	if value, ok := strings.CutPrefix(rest, "title="); ok {
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, " \t") {
			return language, "", fmt.Errorf("code fence title %q must be a filename without spaces", value)
		}
		return language, value, nil
	}
	if strings.ContainsAny(rest, " \t") {
		return language, "", fmt.Errorf("code fence info %q takes at most a language and a filename", info)
	}
	return language, rest, nil
}
