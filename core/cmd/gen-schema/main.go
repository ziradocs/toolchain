// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Command gen-schema genera schema/ast.schema.json a partir de los structs Go
// de core/ast. Es la fuente de verdad para el JSON Schema versionado
// del contrato --format json (issue #8): en CI, este generador se vuelve a
// correr y el diff contra el archivo committeado debe ser vacío, o falla el job.
//
// Uso: go run ./cmd/gen-schema [-out ../schema/ast.schema.json]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"

	"go.ziradocs.com/core/ast"
)

// elementTypes enumera todos los tipos concretos que implementan ast.Element
// (identificados por su método marcador element()), junto con el valor
// literal de su discriminador BaseNode.Type. Debe mantenerse en sync con los
// "func (X) element() {}" en core/ast/*.go.
var elementTypes = []struct {
	name     string
	nodeType ast.NodeType
	sample   interface{}
}{
	{"TextElement", ast.NodeTypeText, &ast.TextElement{}},
	{"PointsElement", ast.NodeTypePoints, &ast.PointsElement{}},
	{"CodeElement", ast.NodeTypeCode, &ast.CodeElement{}},
	{"ImageElement", ast.NodeTypeImage, &ast.ImageElement{}},
	{"TableElement", ast.NodeTypeTable, &ast.TableElement{}},
	{"SpecialBlockElement", ast.NodeTypeSpecialBlock, &ast.SpecialBlockElement{}},
	{"CodeGroupElement", ast.NodeTypeCodeGroup, &ast.CodeGroupElement{}},
	{"MermaidElement", ast.NodeTypeMermaid, &ast.MermaidElement{}},
	{"PlantUMLElement", ast.NodeTypePlantUML, &ast.PlantUMLElement{}},
	{"ChartElement", ast.NodeTypeChart, &ast.ChartElement{}},
	{"MapElement", ast.NodeTypeMap, &ast.MapElement{}},
	{"QuoteElement", ast.NodeTypeQuote, &ast.QuoteElement{}},
	{"ChecklistElement", ast.NodeTypeChecklist, &ast.ChecklistElement{}},
	{"GridElement", ast.NodeTypeGrid, &ast.GridElement{}},
	{"ColumnElement", ast.NodeTypeColumn, &ast.ColumnElement{}},
	{"DirectiveNode", ast.NodeTypeDirective, &ast.DirectiveNode{}},
	{"MathElement", ast.NodeTypeMath, &ast.MathElement{}},
}

// nodeTypeConsts fija el valor literal del discriminador "type" para defs que
// no son parte de la unión Element pero también tienen un NodeType fijo.
var nodeTypeConsts = map[string]ast.NodeType{
	"AST":             ast.NodeTypePresentation,
	"FrontMatterNode": ast.NodeTypeFrontMatter,
	"ContentBlock":    ast.NodeTypeContentBlock,
	"PointItem":       ast.NodeTypePointItem,
	"ChecklistItem":   ast.NodeTypeChecklistItem, // issue #60: discriminador propio desde SchemaVersion 2.0.0
}

func newReflector() *jsonschema.Reflector {
	return &jsonschema.Reflector{
		DoNotReference: false,
	}
}

func main() {
	outPath := flag.String("out", "../schema/ast.schema.json", "ruta de salida del schema generado")
	flag.Parse()

	r := newReflector()
	root := r.Reflect(&ast.AST{})
	root.ID = "https://go.ziradocs.com/core/schema/ast.schema.json"
	root.Title = "SlideLang/DocLang AST"
	root.Description = "Contrato JSON/AST emitido por --format json. Ver docs/architecture/json-ast-contract.md. Política de compatibilidad: cambio breaking ⇒ major (ver ast.SchemaVersion)."

	// Fusionar los $defs de cada tipo concreto de Element (y otros nodos con
	// NodeType fijo) en el conjunto principal de definiciones.
	for _, et := range elementTypes {
		sub := r.Reflect(et.sample)
		mergeDefs(root.Definitions, sub.Definitions)
	}

	// Fijar el discriminador "type" como const en cada def que representa un
	// NodeType concreto (permite validación estricta y unions discriminadas
	// en los tipos TS generados a partir de este schema).
	for _, et := range elementTypes {
		if err := setTypeConst(root.Definitions, et.name, et.nodeType); err != nil {
			fmt.Fprintf(os.Stderr, "error fijando discriminador: %v\n", err)
			os.Exit(1)
		}
	}
	for name, nt := range nodeTypeConsts {
		if err := setTypeConst(root.Definitions, name, nt); err != nil {
			fmt.Fprintf(os.Stderr, "error fijando discriminador: %v\n", err)
			os.Exit(1)
		}
	}

	// Reemplazar "elements: any[]" (lo único que la reflexión pura no puede
	// resolver, por ser una interfaz Go) con una unión discriminada real.
	elementsUnion := &jsonschema.Schema{}
	for _, et := range elementTypes {
		elementsUnion.OneOf = append(elementsUnion.OneOf, &jsonschema.Schema{
			Ref: "#/$defs/" + et.name,
		})
	}
	if err := setElementsProperty(root.Definitions, "ContentBlock", elementsUnion); err != nil {
		fmt.Fprintf(os.Stderr, "error fijando unión de elements: %v\n", err)
		os.Exit(1)
	}
	if err := setElementsProperty(root.Definitions, "ColumnElement", elementsUnion); err != nil {
		fmt.Fprintf(os.Stderr, "error fijando unión de elements: %v\n", err)
		os.Exit(1)
	}

	// ChartElement.RawJSON es json.RawMessage, que la reflexión mapea al schema
	// booleano permisivo `true` (acepta cualquier valor JSON). En modo JSON
	// directo rawJSON siempre es un objeto de config Chart.js (issue #11/#49),
	// así que lo restringimos a {"type":"object"} para que el schema SÍ pueda
	// detectar una regresión de doble-encoding (rawJSON como string re-escapado).
	if err := overrideProperty(root.Definitions, "ChartElement", "rawJSON", &jsonschema.Schema{Type: "object"}); err != nil {
		fmt.Fprintf(os.Stderr, "error tipando rawJSON: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling schema: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')

	if err := os.WriteFile(*outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing schema to %s: %v\n", *outPath, err)
		os.Exit(1)
	}
	fmt.Printf("schema escrito en %s\n", *outPath)
}

func mergeDefs(dst, src jsonschema.Definitions) {
	for name, def := range src {
		dst[name] = def
	}
}

// setTypeConst fija el discriminador "type" como const en la $def `defName`.
// Retorna error en vez de no-opear en silencio: un defName con typo o un
// rename del struct Go correspondiente dejaría el schema sin ese discriminador
// sin ningún aviso, y el CI de schema-drift no lo detectaría (regenera con el
// mismo typo y no encuentra diff contra lo committeado).
func setTypeConst(defs jsonschema.Definitions, defName string, nodeType ast.NodeType) error {
	def, ok := defs[defName]
	if !ok {
		return fmt.Errorf("setTypeConst: no se encontró la definición %q (¿typo en elementTypes/nodeTypeConsts, o el struct fue renombrado?)", defName)
	}
	if def.Properties == nil {
		return fmt.Errorf("setTypeConst: la definición %q no tiene Properties", defName)
	}
	typeProp, ok := def.Properties.Get("type")
	if !ok {
		return fmt.Errorf("setTypeConst: la definición %q no tiene una propiedad \"type\" (¿el struct no embebe BaseNode?)", defName)
	}
	typeProp.Const = string(nodeType)
	return nil
}

// setElementsProperty ver setTypeConst: retorna error en vez de no-opear en
// silencio ante un defName inválido.
func setElementsProperty(defs jsonschema.Definitions, defName string, elementsSchema *jsonschema.Schema) error {
	def, ok := defs[defName]
	if !ok {
		return fmt.Errorf("setElementsProperty: no se encontró la definición %q", defName)
	}
	if def.Properties == nil {
		return fmt.Errorf("setElementsProperty: la definición %q no tiene Properties", defName)
	}
	elementsProp, ok := def.Properties.Get("elements")
	if !ok {
		return fmt.Errorf("setElementsProperty: la definición %q no tiene una propiedad \"elements\"", defName)
	}
	elementsProp.Items = elementsSchema
	// Ya no es un array de "any"; limpiar el tipo/items huérfanos de la reflexión previa.
	elementsProp.Type = "array"
	return nil
}

// overrideProperty reemplaza por completo el schema de la propiedad `propName`
// en la $def `defName`. Se reemplaza (no se muta) porque la reflexión puede
// producir un schema booleano (`true`/`false`), cuyo estado interno no es
// mutable desde fuera del paquete jsonschema. Retorna error en vez de no-opear
// en silencio ante un defName/propName inválido (mismo motivo que setTypeConst).
func overrideProperty(defs jsonschema.Definitions, defName, propName string, schema *jsonschema.Schema) error {
	def, ok := defs[defName]
	if !ok {
		return fmt.Errorf("overrideProperty: no se encontró la definición %q", defName)
	}
	if def.Properties == nil {
		return fmt.Errorf("overrideProperty: la definición %q no tiene Properties", defName)
	}
	if _, ok := def.Properties.Get(propName); !ok {
		return fmt.Errorf("overrideProperty: la definición %q no tiene una propiedad %q (¿renombrada?)", defName, propName)
	}
	def.Properties.Set(propName, schema)
	return nil
}
