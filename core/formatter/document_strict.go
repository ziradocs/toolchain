// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"fmt"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
)

// FormatDocumentStrict serializa doc al dialecto strict de DocLang: bloques
// `SECTION` declarados (ver parser.DocumentStrictParser). Es el par de
// FormatDocument, que emite el dialecto flex, y comparte con FormatStrict
// —el serializador de presentaciones— todo el vocabulario de elementos: un
// TEXT, una TABLE o un <<mermaid>> se escriben igual dentro de un SECTION
// que dentro de un SLIDE.
//
// Usarlo sobre un documento flex es una TRANSPILACIÓN, no un formateo: la
// salida es un documento distinto en un dialecto distinto. Es una operación
// legítima y deseada (es como se promueve un borrador a artefacto
// auditable), pero el caller tiene que saber que lo es.
func FormatDocumentStrict(doc *ast.AST) (string, error) {
	var b strings.Builder

	var fm string
	if doc.FrontMatter != nil {
		var err error
		fm, err = formatFrontMatter(doc.FrontMatter, frontMatterOverrides(doc.FrontMatter, "strict"), frontMatterFallbacks(doc.FrontMatter))
		if err != nil {
			return "", err
		}
	}
	b.WriteString(fm)

	needsBlankLine := fm != ""
	for i := range doc.ContentBlocks {
		block := &doc.ContentBlocks[i]

		// Misma regla posicional que FormatDocument: el bloque "title" lleva
		// su texto en Heading, los "content" en Title.
		title := block.Heading
		if title == "" {
			title = block.Title
		}
		if err := checkSectionTitle("content_block", title); err != nil {
			return "", err
		}

		if needsBlankLine {
			b.WriteString("\n")
		}
		needsBlankLine = true
		fmt.Fprintf(&b, "SECTION %s\n", quote(title))

		for _, el := range block.Elements {
			heading, isHeading := asDocumentHeading(el)
			if isHeading {
				if err := checkSectionTitle("text", heading.text); err != nil {
					return "", err
				}
				b.WriteString("\n")
				fmt.Fprintf(&b, "SECTION %s\n", quote(heading.text))
				fmt.Fprintf(&b, "  level: %d\n", heading.level)
				// El `id:` solo se re-emite cuando NO es derivable del
				// título. Emitirlo siempre sería ruido en cada subsección, y
				// omitirlo cuando sí importa cambiaría a dónde apunta una
				// referencia.
				if heading.id != "" && heading.id != renderer.DeriveAnchor(heading.text) {
					fmt.Fprintf(&b, "  id: %s\n", heading.id)
				}
				continue
			}

			elText, err := formatStrictElement(el)
			if err != nil {
				return "", err
			}
			b.WriteString(elText)
		}
	}

	return b.String(), nil
}

// checkSectionTitle valida el título de un SECTION, que NO usa la guarda
// genérica checkQuotable.
//
// checkQuotable prohíbe la comilla doble porque los lectores de campos
// entrecomillados del dialecto (las properties de un SLIDE, IMAGE, TABLE…)
// cierran el valor en la PRÓXIMA comilla y truncarían el resto. El título de
// un SECTION no se lee así: parseSectionHeader corta en la ÚLTIMA comilla de
// la línea, precisamente para que un título con comillas adentro sobreviva
// sin necesitar escapes. Aplicarle la guarda genérica rechazaba al formatear
// títulos que el parser acepta y conserva —`SECTION "El informe "final""`—,
// es decir, rompía el round-trip en un caso que el propio parser tiene
// testeado.
//
// Lo que sí es irrepresentable es un salto de línea: un SECTION vive en una
// sola línea, así que un título multilínea produciría texto que no
// re-parsea. Es inalcanzable desde cualquier parser (ambos dialectos leen el
// título de una línea), pero un AST construido a mano o modificado por un
// filtro externo sí puede traerlo.
func checkSectionTitle(nodeType, title string) error {
	if strings.ContainsAny(title, "\n\r") {
		return newUnsupported(nodeType, fmt.Sprintf(
			"el título de un %s no puede contener saltos de línea (%q): la sintaxis lo declara en una sola línea",
			"SECTION", title))
	}
	return nil
}

// documentHeadingRe extrae nivel, id y texto interno de un
// `<h2 id="...">…</h2>` — la forma exacta que produce
// parser.buildHeadingElement para cada subsección, en los dos dialectos.
//
// A diferencia de subsectionHeadingRe (el de FormatDocument, que emite
// `## texto` y por eso descarta el id), acá el id se CAPTURA: en strict es
// declarable con `id:` y perderlo cambiaría el anchor al que apunta una
// referencia.
var documentHeadingRe = regexp.MustCompile(`(?s)^<h([1-6])(?: id="([^"]*)")?>(.*)</h[1-6]>$`)

type documentHeading struct {
	level int
	id    string
	text  string
}

// asDocumentHeading reconoce un TextElement RawHTML que es un encabezado de
// subsección y lo des-renderiza.
//
// La reconstrucción del texto es best-effort por la misma razón documentada
// en formatSubsectionHeading: buildHeadingElement corrió el original por
// ProcessInlineMarkdownSecureLine, que convierte `**bold**` en <strong> —
// una transformación no invertible (un <strong> tecleado a mano produce el
// mismo elemento). Para títulos de texto plano, que son el caso normal, el
// round-trip es exacto; con formato inline se recupera el texto sin el
// énfasis. Un canonicalizador legítimo, no una pérdida silenciosa: el
// documento resultante re-parsea a la misma estructura.
func asDocumentHeading(el ast.Element) (documentHeading, bool) {
	t, ok := el.(*ast.TextElement)
	if !ok || !t.IsRawHTML {
		return documentHeading{}, false
	}
	m := documentHeadingRe.FindStringSubmatch(t.Content)
	if m == nil {
		return documentHeading{}, false
	}

	// LangSpanHTMLToSource antes de stripTags por el mismo motivo que en
	// formatSubsectionHeading (issue #63): un <span lang="xx"> marca un
	// pasaje en otro idioma y colapsarlo a texto liso es un defecto de
	// accesibilidad silencioso; a diferencia del énfasis, sí es invertible
	// sin ambigüedad.
	return documentHeading{
		level: int(m[1][0] - '0'),
		id:    m[2],
		text:  stripTags(renderer.LangSpanHTMLToSource(m[3])),
	}, true
}
