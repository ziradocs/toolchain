// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package xref

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// refPattern reconoce \ref{label} en cualquier campo de texto.
var refPattern = regexp.MustCompile(`\\ref\{([^}]+)\}`)

// ResolveRefs es la Fase B: recorre doc (ast.Walk) y reescribe cada
// \ref{label} encontrado en TODOS los campos con prosa a un link markdown —
// `[Figura N](#ancla)` / `[Tabla N](#ancla)` — reutilizando el
// inlineLinkPattern ya existente del pipeline inline
// (renderer/sanitizer.go), así que fluye sin cambios por HTML/PDF/DOCX/JSON.
//
// Cubre explícitamente: ContentBlock (Title/Heading/Subtitle), TextElement,
// PointItem, ChecklistItem, ImageElement/TableElement (Caption),
// SpecialBlockElement (Title/Content), QuoteElement (Content/Author/Source),
// GridElement/ColumnElement (Content), y el campo Title (vars-only) de
// Mermaid/PlantUML/Chart/Map. Deliberadamente EXCLUIDO: contenido de código
// (CodeElement.Content, CodeGroupElement.CodeBlocks[].Content) — literal, no
// prosa, resolver ahí corrompería el código; celdas/headers de TableElement
// (datos tabulares, no prosa suelta — seguimiento posible si hay demanda
// real); DirectiveNode (sin campo de prosa).
//
// Un \ref a un label que no existe es un ERROR de build (no un no-op
// silencioso) — mismo principio que un \ref roto en LaTeX.
//
// El idioma del texto generado ("Figura N" vs "Figure N") sale de
// doc.FrontMatter.Lang vía Labels(lang) — ver su doc comment para el
// fallback a "es" cuando no hay front matter o el tag no tiene labels
// propios. Se deriva del propio doc en vez de recibirse como parámetro
// aparte: core/doc.go no da garantías de SemVer sobre el Go API de core/xref
// (solo cli.Options y el schema del AST las tienen), pero un tercer
// parámetro cuyo único valor posible en todo el árbol de llamadas YA está
// en doc era, de cualquier forma, ruido — Transform (el único caller real)
// lo derivaba así mismo antes de pasarlo (hallazgo de code review: ver
// git blame de este comentario).
func ResolveRefs(doc *ast.AST, table Table) error {
	lang := ""
	if doc.FrontMatter != nil {
		lang = doc.FrontMatter.Lang
	}
	labels := Labels(lang)
	unresolvedSet := map[string]bool{}

	err := ast.Walk(doc, func(n ast.Node) error {
		switch v := n.(type) {
		case *ast.ContentBlock:
			v.Title = rewriteRefs(v.Title, table, labels, unresolvedSet)
			v.Heading = rewriteRefs(v.Heading, table, labels, unresolvedSet)
			v.Subtitle = rewriteRefs(v.Subtitle, table, labels, unresolvedSet)
		case *ast.TextElement:
			v.Content = rewriteRefs(v.Content, table, labels, unresolvedSet)
		case *ast.PointItem:
			v.Content = rewriteRefs(v.Content, table, labels, unresolvedSet)
		case *ast.ChecklistItem:
			v.Content = rewriteRefs(v.Content, table, labels, unresolvedSet)
		case *ast.ImageElement:
			v.Caption = rewriteRefs(v.Caption, table, labels, unresolvedSet)
		case *ast.TableElement:
			v.Caption = rewriteRefs(v.Caption, table, labels, unresolvedSet)
		case *ast.MathElement:
			v.Caption = rewriteRefs(v.Caption, table, labels, unresolvedSet)
		case *ast.SpecialBlockElement:
			v.Title = rewriteRefs(v.Title, table, labels, unresolvedSet)
			v.Content = rewriteRefs(v.Content, table, labels, unresolvedSet)
		case *ast.QuoteElement:
			v.Content = rewriteRefs(v.Content, table, labels, unresolvedSet)
			v.Author = rewriteRefs(v.Author, table, labels, unresolvedSet)
			v.Source = rewriteRefs(v.Source, table, labels, unresolvedSet)
		case *ast.GridElement:
			v.Content = rewriteRefs(v.Content, table, labels, unresolvedSet)
		case *ast.ColumnElement:
			v.Content = rewriteRefs(v.Content, table, labels, unresolvedSet)
		case *ast.MermaidElement:
			v.Title = rewriteRefs(v.Title, table, labels, unresolvedSet)
		case *ast.PlantUMLElement:
			v.Title = rewriteRefs(v.Title, table, labels, unresolvedSet)
		case *ast.ChartElement:
			v.Title = rewriteRefs(v.Title, table, labels, unresolvedSet)
		case *ast.MapElement:
			v.Title = rewriteRefs(v.Title, table, labels, unresolvedSet)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(unresolvedSet) > 0 {
		unresolvedLabels := make([]string, 0, len(unresolvedSet))
		for l := range unresolvedSet {
			unresolvedLabels = append(unresolvedLabels, l)
		}
		sort.Strings(unresolvedLabels)
		return fmt.Errorf("referencia(s) \\ref sin resolver (sin figura/tabla con ese label): %s", strings.Join(unresolvedLabels, ", "))
	}
	return nil
}

// rewriteRefs reemplaza cada \ref{label} de text por su link resuelto. Un
// label no encontrado se deja literal en el texto (para que el mensaje de
// error final sea legible) y se registra en unresolved. labels traduce
// entry.Kind al texto de display en el idioma del documento (ver
// Labels(lang) en labels.go).
func rewriteRefs(text string, table Table, labels map[Kind]string, unresolved map[string]bool) string {
	if text == "" || !strings.Contains(text, `\ref{`) {
		return text
	}
	return refPattern.ReplaceAllStringFunc(text, func(match string) string {
		sub := refPattern.FindStringSubmatch(match)
		label := sub[1]
		entry, ok := table[label]
		if !ok {
			unresolved[label] = true
			return match
		}
		return fmt.Sprintf("[%s %d](#%s)", labels[entry.Kind], entry.Number, entry.AnchorID)
	})
}
