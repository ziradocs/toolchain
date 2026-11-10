// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"strings"

	"github.com/mmonterroca/docxgo/v2/domain"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
)

// setDOCXImageDescription escribe el texto alternativo de una imagen ya
// insertada (descr de wp:docPr, lo que Word muestra como "Texto
// alternativo" y lo que lee un lector de pantalla). Antes ninguna imagen del
// DOCX lo llevaba, ni siquiera una con alt. El texto se compacta a una línea;
// uno vacío no escribe nada.
func setDOCXImageDescription(img domain.Image, text string) {
	text = strings.Join(strings.Fields(text), " ")
	if img == nil || text == "" {
		return
	}
	_ = img.SetDescription(text)
}

// firstNonBlank es el primer texto que no queda vacío sin espacios.
func firstNonBlank(texts ...string) string {
	for _, t := range texts {
		if strings.TrimSpace(t) != "" {
			return t
		}
	}
	return ""
}

// docxInlineImageTitle es el título de un destino Markdown inline
// (`![alt](a.png "título")`), o "" si no lleva.
func docxInlineImageTitle(dest string) string {
	if ld, ok := renderer.ParseLinkDestination(dest + ")"); ok {
		return ld.Title
	}
	return ""
}

// docxFigureAlt es el texto alternativo de una figura rasterizada (diagrama,
// gráfica o mapa) sin título: el tipo de figura en el idioma del documento y,
// si lo hay, su subtipo ("Diagrama: sequence", "Gráfica: bar").
func (g *DOCXGenerator) docxFigureAlt(kind docxUnavailableKind, subtype string) string {
	en := docxIsEnglish(g.lang)
	var label string
	switch kind {
	case docxUnavailableMap:
		label = map[bool]string{true: "Map", false: "Mapa"}[en]
	case docxUnavailableChart:
		label = map[bool]string{true: "Chart", false: "Gráfica"}[en]
	case docxUnavailableEquation:
		label = map[bool]string{true: "Equation", false: "Ecuación"}[en]
	default:
		label = map[bool]string{true: "Diagram", false: "Diagrama"}[en]
	}
	if subtype = strings.TrimSpace(subtype); subtype != "" {
		return label + ": " + subtype
	}
	return label
}

// docxMapAlt es el texto alternativo de un mapa: su título o, sin él,
// "Mapa" y las etiquetas de sus marcadores.
func (g *DOCXGenerator) docxMapAlt(elem *ast.MapElement) string {
	if elem.Title != "" {
		return fieldVariables(elem.Title, g.variables)
	}
	var labels []string
	for _, m := range elem.Markers {
		if l := strings.TrimSpace(renderer.ProcessVariables(m.Label, g.variables)); l != "" {
			labels = append(labels, l)
		}
	}
	return g.docxFigureAlt(docxUnavailableMap, strings.Join(labels, ", "))
}

// docxMathAlt es el texto alternativo de una ecuación: su leyenda o, sin
// ella, la fuente LaTeX, que un lector de pantalla puede leer.
func (g *DOCXGenerator) docxMathAlt(elem *ast.MathElement) string {
	return firstNonBlank(fieldVariables(elem.Caption, g.variables), elem.Content)
}

// chartOptionsTitle es options.plugins.title.text de una gráfica, el título
// que Chart.js dibuja cuando el autor no usó el campo title; "" si no hay.
func chartOptionsTitle(elem *ast.ChartElement) string {
	plugins, _ := elem.Options["plugins"].(map[string]interface{})
	title, _ := plugins["title"].(map[string]interface{})
	text, _ := title["text"].(string)
	return text
}
