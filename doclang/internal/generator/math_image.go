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

// HasUnrenderedMath es renderer.HasMathElements contando solo las fórmulas
// sin render previo, que son las que el HTML offline compone con Chromium.
// Mira el mismo alcance que esa función (el primer nivel de cada sección),
// así que para un documento sin math-image-v1 da lo mismo.
func HasUnrenderedMath(doc *ast.AST) bool {
	if doc == nil {
		return false
	}
	for _, block := range doc.ContentBlocks {
		for _, el := range block.Elements {
			if m, ok := el.(*ast.MathElement); ok && MathImage(m) == "" {
				return true
			}
		}
	}
	return false
}
