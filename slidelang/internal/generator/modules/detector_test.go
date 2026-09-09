// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package modules

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// Qué módulos JS se empaquetan tiene que decidirse por el TIPO de nodo, que es
// como decide el renderer, y no por el lenguaje de un fence ni por el nombre de
// un bloque especial.
//
// Este test existe porque los que había no medían esto: verificaban el JSON de
// features/libraries y el markup, y con eso una regresión del detector de
// módulos pasaba entera. La forma de comprobarlo es mutar el detector —hacer
// que un CodeElement con `Language: "mermaid"` vuelva a pedir el módulo— y ver
// que ESTE test se pone rojo mientras aquellos siguen verdes.
//
// Cada fila es un par: la forma que NO tiene a qué engancharse y la que sí,
// para el mismo módulo. Sin el positivo, "sacar el módulo" se arregla borrando
// la detección entera.
func TestDetectRequiredModulesWithConfig_DecidesByNodeTypeNotByName(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	codeGroup := ast.NewCodeGroupElement(pos)
	codeGroup.CodeBlocks = []ast.CodeBlock{
		{Label: "Go", Language: "go", Content: "fmt.Println()"},
		{Label: "Python", Language: "python", Content: "print()"},
	}

	for _, tc := range []struct {
		name   string
		elem   ast.Element
		module string
		want   bool
		razon  string
	}{
		// mermaid
		{
			name:   "CodeElement con lenguaje mermaid",
			elem:   ast.NewCodeElement(pos, "mermaid", "graph TD\nA-->B"),
			module: "mermaid", want: false,
			razon: "renderiza .slidelang-code, no el .slidelang-mermaid anidado que busca mermaid.js",
		},
		{
			name:   "MermaidElement",
			elem:   ast.NewMermaidElement(pos, "graph", "graph TD\nA-->B"),
			module: "mermaid", want: true,
			razon: "es el nodo que sí emite .slidelang-mermaid",
		},
		{
			name:   "bloque especial llamado mermaid",
			elem:   ast.NewSpecialBlockElement(pos, "mermaid", "graph TD\nA-->B"),
			module: "mermaid", want: false,
			razon: "un ::: mermaid emite un contenedor de prosa",
		},
		// charts
		{
			name:   "bloque especial llamado chart",
			elem:   ast.NewSpecialBlockElement(pos, "chart", "type: bar"),
			module: "charts", want: false,
			razon: "no emite ningún <canvas> que Chart.js pueda tomar",
		},
		{
			name:   "ChartElement",
			elem:   ast.NewChartElement(pos, "bar"),
			module: "charts", want: true,
		},
		// maps
		{
			name:   "bloque especial llamado map",
			elem:   ast.NewSpecialBlockElement(pos, "map", "lat: 19.4"),
			module: "maps", want: false,
			razon: "no emite .slidelang-map-container",
		},
		{
			name:   "MapElement",
			elem:   ast.NewMapElement(pos, "osm"),
			module: "maps", want: true,
		},
		// quizpoll, que sí decide por tipo desde siempre: es el control del test.
		{
			name:   "QuizElement",
			elem:   ast.NewQuizElement(pos),
			module: "quizpoll", want: true,
		},
		{
			name:   "bloque especial llamado quiz",
			elem:   ast.NewSpecialBlockElement(pos, "quiz", "question: Q"),
			module: "quizpoll", want: false,
		},
		// code-group: hoy ninguno de los dos pide módulo propio, porque las
		// tabs las engancha utilities.js, que entra siempre (ver el gate en
		// detector.go). La fila fija que el nombre tampoco lo cambia.
		{
			name:   "CodeGroupElement",
			elem:   codeGroup,
			module: "mermaid", want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			block := ast.NewContentBlock(pos, "content")
			block.Elements = []ast.Element{tc.elem}
			doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*block}}

			got := DetectRequiredModulesWithConfig(doc, DefaultModuleConfig())
			if contains(got, tc.module) != tc.want {
				verbo := "no pidió"
				if !tc.want {
					verbo = "pidió"
				}
				msg := "%s %s el módulo %q; módulos: %v"
				if tc.razon != "" {
					msg += "\n    " + tc.razon
				}
				t.Errorf(msg, tc.name, verbo, tc.module, got)
			}
		})
	}
}

// El gate de `utilities` es la opción y nada más. Las dos ramas que había
// —`hasCodeGroups || hasCollapsibles` y su `else`— agregaban el mismo módulo,
// así que la condición no decidía nada; el test fija que hoy se lee como lo que
// hace, en las dos direcciones.
func TestDetectRequiredModulesWithConfig_UtilitiesSoloDependeDeLaOpcion(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)

	// Con bloques de verdad: un code-group vacío no renderiza una sola
	// `.slidelang-tab` y el linter lo rechaza con CODEGROUP001 (error), así
	// que como fixture de "el deck tiene tabs" sería falso.
	cg := ast.NewCodeGroupElement(pos)
	cg.CodeBlocks = []ast.CodeBlock{{Label: "Go", Language: "go", Content: "fmt.Println()"}}
	conTabs := ast.NewContentBlock(pos, "content")
	conTabs.Elements = []ast.Element{cg}

	soloTexto := ast.NewContentBlock(pos, "content")
	soloTexto.Elements = []ast.Element{ast.NewTextElement(pos, "prosa y nada más")}

	for _, tc := range []struct {
		name   string
		block  *ast.ContentBlock
		config ModuleConfig
		want   bool
	}{
		{name: "con code-group, utilities prendido", block: conTabs, config: DefaultModuleConfig(), want: true},
		{name: "sin nada que enganchar, utilities prendido", block: soloTexto, config: DefaultModuleConfig(), want: true},
		{
			name:  "con code-group, --no-utilities",
			block: conTabs,
			config: ModuleConfig{
				EnableNavigation: true,
				EnableUtilities:  false,
				ExcludeModules:   []string{"utilities"},
			},
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*tc.block}}
			got := DetectRequiredModulesWithConfig(doc, tc.config)
			if contains(got, "utilities") != tc.want {
				t.Errorf("utilities presente = %v, se esperaba %v; módulos: %v",
					contains(got, "utilities"), tc.want, got)
			}
		})
	}
}

// `core` va siempre, y ninguna de las filas de arriba puede pasar por accidente
// porque el módulo que busca esté en la lista base.
func TestDetectRequiredModulesWithConfig_BaseNoTraeModulosDeContenido(t *testing.T) {
	doc := &ast.AST{ContentBlocks: []ast.ContentBlock{*ast.NewContentBlock(diagnostics.NewPosition(1, 1), "content")}}
	got := DetectRequiredModulesWithConfig(doc, DefaultModuleConfig())
	if !contains(got, "core") {
		t.Errorf("falta el módulo core; módulos: %v", got)
	}
	for _, m := range []string{"mermaid", "charts", "maps", "quizpoll"} {
		if contains(got, m) {
			t.Errorf("un deck vacío trae %q; los pares del test de arriba medirían la base: %v",
				m, got)
		}
	}
}
