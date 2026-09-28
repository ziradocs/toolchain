// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// slideTexts reúne el texto autoral de todos los elementos de un slide, no
// solo de sus TextElement: encabezados, puntos y checklists (con sus hijos),
// métricas, citas, tablas, bloques especiales, columnas de grid y pies de
// figura. Las reglas de layout que buscan una señal en el contenido (un
// "antes/después", un precio, una llamada a la acción) daban falsos positivos
// cuando el autor la expresaba con un elemento tipado, por ejemplo una métrica.
func slideTexts(slide *ast.ContentBlock) []string {
	var out []string
	add := func(values ...string) {
		for _, v := range values {
			if strings.TrimSpace(v) != "" {
				out = append(out, v)
			}
		}
	}
	var visit func(ast.Element)
	var visitPoints func([]ast.PointItem)
	visitPoints = func(items []ast.PointItem) {
		for _, it := range items {
			add(it.Content)
			visitPoints(it.SubPoints)
		}
	}
	visit = func(el ast.Element) {
		switch e := el.(type) {
		case *ast.TextElement:
			add(e.Content)
		case *ast.HeadingElement:
			add(e.Text)
		case *ast.PointsElement:
			visitPoints(e.Items)
		case *ast.ChecklistElement:
			for _, it := range e.Items {
				add(it.Content)
			}
		case *ast.MetricElement:
			add(e.Label, e.Value, e.Delta, e.Caption)
		case *ast.QuoteElement:
			add(e.Content, e.Author, e.Source)
		case *ast.TableElement:
			add(e.Caption)
			add(e.Headers...)
			for _, row := range e.Rows {
				add(row...)
			}
		case *ast.SpecialBlockElement:
			add(e.Title, e.Content)
			for _, nested := range e.Elements {
				visit(nested)
			}
		case *ast.GridElement:
			add(e.Content)
			for _, col := range e.Columns {
				add(col.Content)
				for _, nested := range col.Elements {
					visit(nested)
				}
			}
		case *ast.ImageElement:
			add(e.Alt, e.Caption)
		case *ast.MediaElement:
			add(e.Caption)
		case *ast.QuizElement:
			add(e.Question)
		case *ast.PollElement:
			add(e.Question)
		}
	}
	for _, el := range slide.Elements {
		visit(el)
	}
	return out
}

// slideTextContains reporta si algún texto autoral del slide contiene alguno
// de los términos (comparación sin distinguir mayúsculas).
func slideTextContains(slide *ast.ContentBlock, terms ...string) bool {
	for _, text := range slideTexts(slide) {
		lower := strings.ToLower(text)
		for _, term := range terms {
			if strings.Contains(lower, strings.ToLower(term)) {
				return true
			}
		}
	}
	return false
}

// hasElement reporta si el slide tiene algún elemento del tipo dado en su
// primer nivel.
func hasElement(slide *ast.ContentBlock, nodeType ast.NodeType) bool {
	for _, el := range slide.Elements {
		if el.GetType() == nodeType {
			return true
		}
	}
	return false
}
