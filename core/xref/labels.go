// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package xref

import "strings"

// Labels devuelve el texto de display por Kind para lang (tag BCP 47 tal
// como llega en FrontMatter.Lang — ver ast/nodes.go). Un lang vacío o no
// soportado cae a "es": el comportamiento histórico, de cuando Kind era la
// única fuente de este texto (issue del audit 2026-09-11 — un documento con
// `lang: en` seguía mostrando "Tabla 1"/"Figura 1" porque ni
// ResolveRefs ni los renderers de tabla/imagen tenían forma de leer el
// idioma declarado).
//
// Solo el SUBTAG PRIMARIO decide (todo lo que precede al primer "-"),
// comparado sin distinguir mayúsculas — "en-US", "en-GB", "EN" y "en" son
// el mismo idioma para este propósito, igual que un documento en "es-MX" no
// deja de ser español. Antes de este fix el switch comparaba lang ENTERO
// contra el literal "en", así que cualquier variante regional (el propio
// ejemplo de FrontMatter.Lang en su doc comment, "en-US") caía al default
// en español — contradiciendo esa misma documentación, que promete "tag BCP
// 47 tal como llega". Un subtag primario de forma inválida (ver
// a11y.IsValidLangTag, que este paquete no necesita importar para esto)
// simplemente no matchea "en" y cae al mismo default de siempre.
//
// Kind sigue siendo la IDENTIDAD del tipo de entidad (KindFigure/KindTable/
// KindEquation) — el valor string que lleva (coincidentemente el label en
// español) es un detalle de implementación de esa identidad, no algo que el
// código de display deba leer directamente. Este mapa es el único lugar que
// traduce esa identidad a texto para el lector.
func Labels(lang string) map[Kind]string {
	primary, _, _ := strings.Cut(lang, "-")
	switch strings.ToLower(primary) {
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
