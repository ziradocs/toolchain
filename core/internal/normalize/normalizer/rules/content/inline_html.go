// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"regexp"
	"strconv"
	"strings"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// inlineHTMLRewrites mapea cada tag inline a su forma canónica del DSL. El
// contenido de código ya viene protegido cuando estas corren (ver
// rewriteInlineHTML), así que acá no hay orden significativo entre ellas.
var inlineHTMLRewrites = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)<u\b[^>]*>([^<>\[\]]*)</u>`), "[$1]{.underline}"},
	{regexp.MustCompile(`(?i)<sub\b[^>]*>([^<>\[\]]*)</sub>`), "[$1]{.sub}"},
	{regexp.MustCompile(`(?i)<sup\b[^>]*>([^<>\[\]]*)</sup>`), "[$1]{.sup}"},
	{regexp.MustCompile(`(?i)<kbd\b[^>]*>([^<>\[\]]*)</kbd>`), "[$1]{.kbd}"},
	{regexp.MustCompile(`(?i)<mark\b[^>]*>([^<>\[\]]*)</mark>`), "==$1=="},
	{regexp.MustCompile(`(?i)<small\b[^>]*>([^<>\[\]]*)</small>`), "[$1]{.small}"},
}

// codeSpanPattern captura los tramos que son CÓDIGO y cuyo interior, por lo
// tanto, es literal: un `<code>…</code>` (con cualquier contenido, incluidas
// otras tags) y un tramo entre backticks. Se protegen antes de reescribir nada.
var codeSpanPattern = regexp.MustCompile("(?is)<code\\b[^>]*>.*?</code>|`[^`]*`")

// cleanCodeContent reconoce el interior de un `<code>` que SÍ se puede pasar a
// backticks: sin tags anidadas ni corchetes, que son la sintaxis de destino.
// El interior excluye TAMBIÉN el backtick: pasar un `<code>a`b</code>` a
// backticks produciría "`a`b`", que se lee como código "a" seguido de "b`"
// suelto. Un <code> así se deja literal, igual que uno con tags adentro.
var cleanCodeContent = regexp.MustCompile("(?is)^<code\\b[^>]*>([^<>\\[\\]`]*)</code>$")

// InlineHTMLTagsRule reescribe las tags HTML inline que un autor (o un modelo)
// escribe por costumbre a la sintaxis que el DSL sí entiende (issue #243).
//
// El problema que resuelve: el sanitizer escapa TODO el HTML del usuario — es
// su modelo de seguridad y no cambia —, así que un `<u>texto</u>` escrito a
// mano no sale subrayado: sale con las tags visibles en la diapositiva.
// Markdown no tiene forma de escribir subrayado, subíndice, superíndice ni
// teclas, así que la tag es lo que la gente alcanza. Esta regla la traduce
// ANTES de que el sanitizer la escape.
//
// Tres límites deliberados, cada uno para no romper contenido que hoy funciona:
//
//   - Solo pares que abren y cierran en la MISMA línea. Un `<u>` suelto puede
//     ser prosa sobre HTML, o una tag mal cerrada cuyo alcance no se puede
//     adivinar; en los dos casos dejarla intacta es mejor que inventar.
//   - El interior no puede traer `<`, `>`, `[` ni `]`: los corchetes son la
//     sintaxis del token de destino, y reescribir `<u>[a](b)</u>` produciría un
//     `[[a](b)]{.underline}` que el sanitizer no sabe leer.
//   - Nada dentro de un fence de código ni de un bloque `<<…>>`: ahí el HTML
//     literal es el contenido, no formato. No hay helper compartido para esto
//     en base/, así que el rastreo va acá (mismo criterio que
//     CodeGroupFormatterRule y MermaidSyntaxFixerRule).
//
// Los atributos se descartan: el conjunto de tags que el sanitizer emite es
// cerrado y sin atributos, así que conservarlos no tendría a dónde ir.
type InlineHTMLTagsRule struct {
	analyzer *base.DocumentAnalyzer
}

func NewInlineHTMLTagsRule() *InlineHTMLTagsRule {
	return &InlineHTMLTagsRule{analyzer: base.NewDocumentAnalyzer()}
}

func (r *InlineHTMLTagsRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	modified := false
	inFence := false
	inEmbeddedBlock := false

	for i := r.analyzer.SkipFrontmatter(lines); i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		// Bloques `<<chart>>`/`<<quiz>>`/`<<mermaid>>`…: el cuerpo es datos
		// (JSON, YAML, fuente de diagrama), no prosa.
		if trimmed == "<<end>>" || (strings.HasPrefix(trimmed, "<</") && strings.HasSuffix(trimmed, ">>")) {
			inEmbeddedBlock = false
			continue
		}
		// Una frontera de documento también termina un bloque embebido: los
		// parsers los cierran así cuando falta el `<<end>>`. Sin esto, UN
		// bloque terminado por `---` —el separador de slides, o sea la forma
		// más común— dejaba la regla apagada para todo el resto del archivo.
		if trimmed == "---" || strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") {
			inEmbeddedBlock = false
			continue
		}
		if strings.HasPrefix(trimmed, "<<") {
			inEmbeddedBlock = true
			continue
		}
		if inEmbeddedBlock {
			continue
		}

		if rewritten := rewriteInlineHTML(lines[i]); rewritten != lines[i] {
			lines[i] = rewritten
			modified = true
		}
	}

	if !modified {
		return content, nil
	}
	return strings.Join(lines, "\n"), nil
}

// rewriteInlineHTML reescribe una línea, protegiendo primero los tramos de
// código.
//
// La protección no es cosmética: sin ella, un `<code><u>x</u></code>` —la forma
// de MOSTRAR una tag como texto— salía con el `<u>` de adentro convertido a
// token, o sea que el ejemplo dejaba de decir lo que decía. Lo mismo con un
// tramo entre backticks. Es el mismo centinela que usa
// renderer.ProcessInlineMarkdownFormatsSecure para proteger `código` de las
// pasadas de negrita/cursiva, y por el mismo motivo.
func rewriteInlineHTML(line string) string {
	var protected []string
	masked := codeSpanPattern.ReplaceAllStringFunc(line, func(match string) string {
		protected = append(protected, match)
		return codeSentinel(len(protected) - 1)
	})

	for _, rw := range inlineHTMLRewrites {
		masked = rw.re.ReplaceAllString(masked, rw.repl)
	}

	for i, original := range protected {
		// Un `<code>` de interior limpio se pasa a backticks (la forma
		// canónica); uno con tags adentro vuelve tal cual, porque convertirlo
		// arrastraría su contenido a una reinterpretación. Un tramo que ya
		// venía entre backticks siempre vuelve igual.
		replacement := original
		if m := cleanCodeContent.FindStringSubmatch(original); m != nil {
			replacement = "`" + m[1] + "`"
		}
		masked = strings.Replace(masked, codeSentinel(i), replacement, 1)
	}

	return masked
}

// codeSentinel produce un marcador que no puede aparecer en prosa real ni ser
// tocado por las reescrituras de arriba (no tiene `<`, `[` ni backticks).
func codeSentinel(i int) string {
	return "\x00zdcode" + strconv.Itoa(i) + "\x00"
}

// Description nombra la regla. El string importa: applyBasicFormatting
// (normalizer.go) decide por SUBSTRING qué reglas corren cuando el detector no
// puntúa el documento como generado por IA, y esta regla tiene que estar en esa
// lista — ver el comentario de Priority.
func (r *InlineHTMLTagsRule) Description() string {
	return "InlineHTMLTags: reescribe tags HTML inline (<u>, <sub>, <sup>, <kbd>, <mark>, <small>, <code>) a la sintaxis del DSL"
}

// Priority 3: después de HeadersRule, que reescribe líneas enteras. Esta solo
// toca el interior de una línea, así que no compite con ninguna otra.
//
// Corre SIEMPRE, no solo cuando el detector marca el documento (ver
// applyBasicFormatting): un `<u>` escrito a mano es justo lo que aparece en un
// documento humano, que es el que el detector NO puntúa. Es el mismo
// razonamiento que el comentario de ElementClosingTagsRule en registry.go —
// una cuestión de sintaxis no puede depender de una heurística de estilo.
func (r *InlineHTMLTagsRule) Priority() int {
	return 3
}

func (r *InlineHTMLTagsRule) Category() base.RuleCategory {
	return base.CategoryContent
}
