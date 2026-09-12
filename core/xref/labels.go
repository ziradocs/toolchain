// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package xref

// Labels devuelve el texto de display por Kind para lang (tag BCP 47 tal
// como llega en FrontMatter.Lang — ver ast/nodes.go). Un lang vacío o no
// soportado cae a "es": el comportamiento histórico, de cuando Kind era la
// única fuente de este texto (issue del audit 2026-09-11 — un documento con
// `lang: en` seguía mostrando "Tabla 1"/"Figura 1" porque ni
// ResolveRefs ni los renderers de tabla/imagen tenían forma de leer el
// idioma declarado).
//
// Kind sigue siendo la IDENTIDAD del tipo de entidad (KindFigure/KindTable/
// KindEquation) — el valor string que lleva (coincidentemente el label en
// español) es un detalle de implementación de esa identidad, no algo que el
// código de display deba leer directamente. Este mapa es el único lugar que
// traduce esa identidad a texto para el lector.
func Labels(lang string) map[Kind]string {
	switch lang {
	case "en":
		return map[Kind]string{
			KindFigure:   "Figure",
			KindTable:    "Table",
			KindEquation: "Equation",
		}
	default:
		return map[Kind]string{
			KindFigure:   "Figura",
			KindTable:    "Tabla",
			KindEquation: "Ecuación",
		}
	}
}
