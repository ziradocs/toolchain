// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import "go.ziradocs.com/core/v2/ast"

// MathImage devuelve el render ya hecho de una fórmula (MathElement.Image,
// math-image-v1, AST 2.27.0), o "" si no lo tiene. Se lee por interfaz y no
// por el campo porque este módulo compila también contra el core publicado,
// que puede no tenerlo todavía; con ese core ninguna fórmula trae render.
func MathImage(m *ast.MathElement) string {
	if r, ok := any(m).(interface{ MathImage() string }); ok {
		return r.MathImage()
	}
	return ""
}

// MathNeedsChromiumDOCX dice si el DOCX necesita Chromium para esta fórmula:
// sin render previo la compone MathJax en Chromium (renderMath); con uno, solo
// si es un SVG, que se rasteriza igual que una imagen SVG local.
func MathNeedsChromiumDOCX(m *ast.MathElement) bool {
	image := MathImage(m)
	return image == "" || isSVGImageSource(docxBlockImagePath(image))
}

// HasUnrenderedMath dice si doc tiene alguna fórmula sin render previo, a
// cualquier profundidad (bloques especiales y columnas incluidos): son las
// que el HTML offline compone con Chromium. Antes esta compuerta usaba
// renderer.HasMathElements, que solo mira el primer nivel de cada sección,
// así que una fórmula dentro de un bloque especial no arrancaba Chromium y
// salía como "Ecuación no disponible".
func HasUnrenderedMath(doc *ast.AST) bool {
	found := false
	WalkElements(doc, func(el ast.Element) bool {
		if m, ok := el.(*ast.MathElement); ok && MathImage(m) == "" {
			found = true
		}
		return !found
	})
	return found
}
