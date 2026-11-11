// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"strings"

	"github.com/mmonterroca/docxgo/v2/domain"
)

// docxUnavailableKind es el tipo de contenido que no se pudo generar en el
// DOCX.
type docxUnavailableKind int

const (
	docxUnavailableImage docxUnavailableKind = iota
	docxUnavailableMap
	docxUnavailableChart
	docxUnavailableDiagram
	docxUnavailableEquation
)

// docxIsEnglish aplica la regla de idioma de los marcadores de HTML/PDF
// (core/renderer/unavailable.go, la misma de xref.Labels): "en" con cualquier
// subetiqueta es inglés y todo lo demás, incluido sin lang, español.
func docxIsEnglish(lang string) bool {
	primary, _, _ := strings.Cut(strings.TrimSpace(lang), "-")
	primary, _, _ = strings.Cut(primary, "_")
	return strings.EqualFold(primary, "en")
}

// docxUnavailableNote es el texto del marcador que reemplaza en el DOCX a un
// contenido que no se pudo generar. Antes eran textos fijos en inglés con
// detalle técnico ("Map: osm (render failed)", "Mermaid Diagram: flowchart
// (not rendered: Chromium not available)", "[Image not found: a.png]"),
// también en un documento en español. El motivo y cómo arreglarlo siguen en
// el aviso del log; el documento dice solo qué falta, igual que el HTML.
func docxUnavailableNote(kind docxUnavailableKind, lang string) string {
	en := docxIsEnglish(lang)
	pick := func(es, eng string) string {
		if en {
			return eng
		}
		return es
	}
	switch kind {
	case docxUnavailableMap:
		return pick("Mapa no disponible", "Map not available")
	case docxUnavailableChart:
		return pick("Gráfica no disponible", "Chart not available")
	case docxUnavailableDiagram:
		return pick("Diagrama no disponible", "Diagram not available")
	case docxUnavailableEquation:
		return pick("Ecuación no disponible", "Equation not available")
	default:
		return pick("Imagen no disponible", "Image not available")
	}
}

// docxImageUnavailableText es el marcador de una imagen que no se insertó:
// la nota y el texto alternativo (o, sin alt, la fuente tal como la escribió
// el autor, nunca la ruta absoluta de la máquina que compiló). El alt lleva
// sus {{variables}} sustituidas fuera de sus code spans (fieldVariables), como
// el alt de una imagen insertada y como el alt del HTML; antes el marcador
// mostraba "{{v}}" tal cual. La fuente queda como se escribió.
func (g *DOCXGenerator) docxImageUnavailableText(alt, source string) string {
	g.recordUnavailable(docxUnavailableImage, source)
	label := strings.TrimSpace(fieldVariables(alt, g.variables))
	if label == "" {
		label = source
	}
	note := docxUnavailableNote(docxUnavailableImage, g.lang)
	if label == "" {
		return "[" + note + "]"
	}
	return fmt.Sprintf("[%s: %s]", note, label)
}

// renderUnavailable deja el marcador de un chart, mapa, diagrama o ecuación
// que no se pudo generar.
func (g *DOCXGenerator) renderUnavailable(doc domain.Document, kind docxUnavailableKind) error {
	g.recordUnavailable(kind, "")
	return g.renderPlaceholder(doc, docxUnavailableNote(kind, g.lang))
}

// String es el nombre del tipo en un UnavailableContent: el mismo que usa
// core para HTML/PDF.
func (k docxUnavailableKind) String() string {
	switch k {
	case docxUnavailableMap:
		return "map"
	case docxUnavailableChart:
		return "chart"
	case docxUnavailableDiagram:
		return "diagram"
	case docxUnavailableEquation:
		return "equation"
	default:
		return "image"
	}
}

// recordUnavailable anota un contenido que quedó como marcador (o, una
// imagen remota, como su texto alternativo), con la línea del elemento en
// curso, para GeneratorOptions.FailOnUnavailable.
func (g *DOCXGenerator) recordUnavailable(kind docxUnavailableKind, detail string) {
	g.unavailable.add(kind.String(), g.renderLine, detail)
}

// renderUnavailableCaptioned deja el marcador y, debajo, el título o la
// leyenda que el autor le dio al elemento: es texto suyo y antes se perdía
// junto con el diagrama, la ecuación, el chart o el mapa.
func (g *DOCXGenerator) renderUnavailableCaptioned(doc domain.Document, kind docxUnavailableKind, caption string) error {
	if err := g.renderUnavailable(doc, kind); err != nil {
		return err
	}
	if caption == "" {
		return nil
	}
	return g.renderFigureCaption(doc, caption)
}

// docxMediaText son los marcadores de un video o audio sin fuente, con un
// esquema bloqueado o que no se pudo enlazar.
func (g *DOCXGenerator) docxMediaText(label, reason, source string) string {
	en := docxIsEnglish(g.lang)
	switch reason {
	case "empty":
		if en {
			return label + " without a source"
		}
		return label + " sin fuente"
	case "blocked":
		if en {
			return label + " blocked for security"
		}
		return label + " bloqueado por seguridad"
	default:
		if en {
			return fmt.Sprintf("%s not available: %s", label, source)
		}
		return fmt.Sprintf("%s no disponible: %s", label, source)
	}
}
