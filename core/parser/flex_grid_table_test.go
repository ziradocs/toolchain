// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// parseFlexSlides parsea content como .slidelang flex. Con normalizar=false
// el cuerpo igual pasa por applyBasicNormalization (las reglas de
// enhancement, entre ellas TablesRule, corren en ambos caminos): apagar la
// normalización no aísla al parser de esas reglas.
func parseFlexSlides(t *testing.T, content string, normalizar bool) []ast.Element {
	t.Helper()
	p := New(util.NewNoop())
	p.SetNormalization(normalizar)
	doc, diags := p.Parse(content, "grid_table.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("error de parseo: %v", d.Message)
		}
	}
	if len(doc.ContentBlocks) != 1 {
		t.Fatalf("ContentBlocks = %d, quería 1", len(doc.ContentBlocks))
	}
	return doc.ContentBlocks[0].Elements
}

// TestFlexParser_GridColumnWithTableRightAfterMarker cubre una tabla markdown
// pegada a la línea "::: column" (sin línea en blanco de por medio). Antes
// de la corrección el parser no producía un GridElement sino una única tabla
// de nivel slide cuyas filas incluían las líneas "::: grid", "::: column" y
// ":::" como celdas.
func TestFlexParser_GridColumnWithTableRightAfterMarker(t *testing.T) {
	const tabla = "| A | B |\n|---|---|\n| 1 | 2 |"

	casos := []struct {
		nombre string
		fuente string
	}{
		{
			nombre: "sin líneas en blanco",
			fuente: "---\nmode: flex\n---\n# Slide\n\n::: grid\n::: column\n" + tabla + "\n:::\n:::\n",
		},
		{
			nombre: "con líneas en blanco (control)",
			fuente: "---\nmode: flex\n---\n# Slide\n\n::: grid\n::: column\n\n" + tabla + "\n\n:::\n:::\n",
		},
	}

	for _, c := range casos {
		for _, normalizar := range []bool{false, true} {
			nombre := c.nombre + "/normalización apagada"
			if normalizar {
				nombre = c.nombre + "/normalización activa"
			}
			t.Run(nombre, func(t *testing.T) {
				els := parseFlexSlides(t, c.fuente, normalizar)
				if len(els) != 1 {
					t.Fatalf("elementos = %d (%v), quería 1 GridElement", len(els), els)
				}
				grid, ok := els[0].(*ast.GridElement)
				if !ok {
					t.Fatalf("elemento = %T, quería *ast.GridElement", els[0])
				}
				if len(grid.Columns) != 1 {
					t.Fatalf("columnas = %d, quería 1", len(grid.Columns))
				}
				if got := strings.TrimSpace(grid.Columns[0].Content); got != tabla {
					t.Errorf("Content de la columna = %q, quería la tabla markdown %q", got, tabla)
				}
			})
		}
	}
}

// TestParseDocument_GridColumnWithTableRightAfterMarker es el mismo caso por
// el camino documental (ParseDocument, dialecto flex de doclang), que
// comparte las reglas de normalización de cuerpo.
func TestParseDocument_GridColumnWithTableRightAfterMarker(t *testing.T) {
	const tabla = "| A | B |\n|---|---|\n| 1 | 2 |"
	fuente := "---\nmode: flex\n---\n# Sección\n\n::: grid\n::: column\n" + tabla + "\n:::\n:::\n"

	for _, normalizar := range []bool{false, true} {
		nombre := "normalización apagada"
		if normalizar {
			nombre = "normalización activa"
		}
		t.Run(nombre, func(t *testing.T) {
			p := New(util.NewNoop())
			p.SetNormalization(normalizar)
			doc, diags := p.ParseDocument(fuente, "grid_table.doclang")
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("error de parseo: %v", d.Message)
				}
			}
			if len(doc.ContentBlocks) != 1 {
				t.Fatalf("ContentBlocks = %d, quería 1", len(doc.ContentBlocks))
			}
			var grid *ast.GridElement
			for _, el := range doc.ContentBlocks[0].Elements {
				if g, ok := el.(*ast.GridElement); ok {
					grid = g
				}
				if _, ok := el.(*ast.TableElement); ok {
					t.Fatalf("la tabla salió como elemento de la sección en vez de quedar dentro de la columna del grid")
				}
			}
			if grid == nil || len(grid.Columns) != 1 {
				t.Fatalf("quería un GridElement con 1 columna, elementos = %v", doc.ContentBlocks[0].Elements)
			}
			if got := strings.TrimSpace(grid.Columns[0].Content); got != tabla {
				t.Errorf("Content de la columna = %q, quería %q", got, tabla)
			}
		})
	}
}
