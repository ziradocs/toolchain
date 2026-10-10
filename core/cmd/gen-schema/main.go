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

	"go.ziradocs.com/core/v2/ast"
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
	{"MediaElement", ast.NodeTypeMedia, &ast.MediaElement{}},
	{"QuizElement", ast.NodeTypeQuiz, &ast.QuizElement{}},
	{"PollElement", ast.NodeTypePoll, &ast.PollElement{}},
	{"MetricElement", ast.NodeTypeMetric, &ast.MetricElement{}},
	{"HeadingElement", ast.NodeTypeHeading, &ast.HeadingElement{}},
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
	root.Description = "Contrato JSON/AST emitido por --format json. Ver https://ziradocs.com/docs/architecture/json-ast-contract/. Política de compatibilidad: cambio breaking ⇒ major (ver ast.SchemaVersion)."

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
	// Keep the public JSON validator aligned with ast.ValidNodeID, including
	// nested node definitions. A filter can introduce nodeId without going
	// through the source parser.
	for _, def := range root.Definitions {
		if def.Properties == nil {
			continue
		}
		if prop, ok := def.Properties.Get("nodeId"); ok {
			prop.Pattern = `^[A-Za-z][A-Za-z0-9._-]{0,127}$`
		}
	}
	if err := overrideProperty(root.Definitions, "AST", "schemaVersion", &jsonschema.Schema{Type: "string", Enum: []any{ast.PreviousSchemaVersion, ast.LegacySchemaVersion, ast.TableSchemaVersion, ast.NestedListSchemaVersion, ast.TypedHeadingsSchemaVersion, ast.MediaFigureSchemaVersion, ast.CodeFilenameSchemaVersion, ast.QuizPollResultsSchemaVersion, ast.ListStartSchemaVersion, ast.MathSourceSchemaVersion}}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := overrideProperty(root.Definitions, "PointItem", "subListType", &jsonschema.Schema{Type: "string", Enum: []any{"ordered", "unordered"}}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := overrideProperty(root.Definitions, "PointsElement", "listType", &jsonschema.Schema{Type: "string", Enum: []any{"ordered", "unordered"}}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := overrideProperty(root.Definitions, "TableRow", "section", &jsonschema.Schema{Type: "string", Enum: []any{"header", "body", "footer"}}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Mismas reglas que ast.ValidateHeading: nivel 1-6, texto de una línea no
	// vacío y anchor dentro de la lista blanca de renderer.SanitizeAnchor.
	for prop, raw := range map[string]string{
		"level":  `{"type":"integer","minimum":1,"maximum":6}`,
		"text":   `{"type":"string","pattern":"^[^\\r\\n]*\\S[^\\r\\n]*$"}`,
		"anchor": `{"type":"string","pattern":"^[a-z0-9_-]*$"}`,
	} {
		var constraint jsonschema.Schema
		if err := json.Unmarshal([]byte(raw), &constraint); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := overrideProperty(root.Definitions, "HeadingElement", prop, &constraint); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	// This recursive schema follows ContentBlock.elements, nested special
	// blocks, and grid columns. The root gate must agree with DecodeAST even
	// when the only authored table sits inside another element.
	presenceJSON := `{"anyOf":[{"required":["tableRows"]},{"required":["contentBlocks"],"properties":{"contentBlocks":{"contains":{"$ref":"#/$defs/TableRowsPresent"}}}},{"required":["elements"],"properties":{"elements":{"contains":{"$ref":"#/$defs/TableRowsPresent"}}}},{"required":["columns"],"properties":{"columns":{"contains":{"$ref":"#/$defs/TableRowsPresent"}}}}]}`
	var presence jsonschema.Schema
	if err := json.Unmarshal([]byte(presenceJSON), &presence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root.Definitions["TableRowsPresent"] = &presence
	listPresenceJSON := `{"anyOf":[{"required":["subListType"]},{"required":["contentBlocks"],"properties":{"contentBlocks":{"contains":{"$ref":"#/$defs/NestedListPresent"}}}},{"required":["elements"],"properties":{"elements":{"contains":{"$ref":"#/$defs/NestedListPresent"}}}},{"required":["columns"],"properties":{"columns":{"contains":{"$ref":"#/$defs/NestedListPresent"}}}},{"required":["items"],"properties":{"items":{"contains":{"$ref":"#/$defs/NestedListPresent"}}}},{"required":["subPoints"],"properties":{"subPoints":{"contains":{"$ref":"#/$defs/NestedListPresent"}}}}]}`
	var listPresence jsonschema.Schema
	if err := json.Unmarshal([]byte(listPresenceJSON), &listPresence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root.Definitions["NestedListPresent"] = &listPresence
	typedTreeJSON := `{"type":"object","allOf":[{"if":{"properties":{"type":{"const":"point_item"},"subPoints":{"type":"array","minItems":1}},"required":["type","subPoints"]},"then":{"required":["subListType"]}},{"if":{"properties":{"type":{"const":"point_item"}},"required":["type","subListType"]},"then":{"required":["subPoints"],"properties":{"subPoints":{"type":"array","minItems":1}}}}],"properties":{"contentBlocks":{"items":{"$ref":"#/$defs/TypedPointTree"}},"elements":{"items":{"$ref":"#/$defs/TypedPointTree"}},"columns":{"items":{"$ref":"#/$defs/TypedPointTree"}},"items":{"items":{"$ref":"#/$defs/TypedPointTree"}},"subPoints":{"items":{"$ref":"#/$defs/TypedPointTree"}}}}`
	var typedTree jsonschema.Schema
	if err := json.Unmarshal([]byte(typedTreeJSON), &typedTree); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root.Definitions["TypedPointTree"] = &typedTree
	headingPresenceJSON := `{"anyOf":[{"required":["type"],"properties":{"type":{"const":"heading"}}},{"required":["contentBlocks"],"properties":{"contentBlocks":{"contains":{"$ref":"#/$defs/TypedHeadingPresent"}}}},{"required":["elements"],"properties":{"elements":{"contains":{"$ref":"#/$defs/TypedHeadingPresent"}}}},{"required":["columns"],"properties":{"columns":{"contains":{"$ref":"#/$defs/TypedHeadingPresent"}}}}]}`
	var headingPresence jsonschema.Schema
	if err := json.Unmarshal([]byte(headingPresenceJSON), &headingPresence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root.Definitions["TypedHeadingPresent"] = &headingPresence
	mediaPresenceJSON := `{"anyOf":[{"required":["type"],"properties":{"type":{"const":"media"}},"anyOf":[{"required":["poster"]},{"required":["caption"]}]},{"required":["contentBlocks"],"properties":{"contentBlocks":{"contains":{"$ref":"#/$defs/MediaFigurePresent"}}}},{"required":["elements"],"properties":{"elements":{"contains":{"$ref":"#/$defs/MediaFigurePresent"}}}},{"required":["columns"],"properties":{"columns":{"contains":{"$ref":"#/$defs/MediaFigurePresent"}}}}]}`
	var mediaPresence jsonschema.Schema
	if err := json.Unmarshal([]byte(mediaPresenceJSON), &mediaPresence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root.Definitions["MediaFigurePresent"] = &mediaPresence
	codePresenceJSON := `{"anyOf":[{"required":["type","filename"],"properties":{"type":{"const":"code"}}},{"required":["contentBlocks"],"properties":{"contentBlocks":{"contains":{"$ref":"#/$defs/CodeFilenamePresent"}}}},{"required":["elements"],"properties":{"elements":{"contains":{"$ref":"#/$defs/CodeFilenamePresent"}}}},{"required":["columns"],"properties":{"columns":{"contains":{"$ref":"#/$defs/CodeFilenamePresent"}}}}]}`
	var codePresence jsonschema.Schema
	if err := json.Unmarshal([]byte(codePresenceJSON), &codePresence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root.Definitions["CodeFilenamePresent"] = &codePresence
	resultsPresenceJSON := `{"anyOf":[{"required":["type"],"properties":{"type":{"enum":["quiz","poll"]}},"anyOf":[{"required":["results"]},{"required":["responses"]}]},{"required":["contentBlocks"],"properties":{"contentBlocks":{"contains":{"$ref":"#/$defs/QuizPollResultsPresent"}}}},{"required":["elements"],"properties":{"elements":{"contains":{"$ref":"#/$defs/QuizPollResultsPresent"}}}},{"required":["columns"],"properties":{"columns":{"contains":{"$ref":"#/$defs/QuizPollResultsPresent"}}}}]}`
	var resultsPresence jsonschema.Schema
	if err := json.Unmarshal([]byte(resultsPresenceJSON), &resultsPresence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root.Definitions["QuizPollResultsPresent"] = &resultsPresence
	for _, def := range []string{"QuizElement", "PollElement"} {
		for prop, raw := range map[string]string{
			"results":   `{"type":"array","items":{"type":"number","minimum":0,"maximum":100}}`,
			"responses": `{"type":"integer","minimum":0}`,
		} {
			var constraint jsonschema.Schema
			if err := json.Unmarshal([]byte(raw), &constraint); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if err := overrideProperty(root.Definitions, def, prop, &constraint); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
	}
	var codeFilename jsonschema.Schema
	if err := json.Unmarshal([]byte(`{"type":"string","pattern":"^[^\\s\\[{][^\\s]*$"}`), &codeFilename); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := overrideProperty(root.Definitions, "CodeElement", "filename", &codeFilename); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var ListStartPresent jsonschema.Schema
	if err := json.Unmarshal([]byte(`{"anyOf":[{"anyOf":[{"required":["start"]},{"required":["subListStart"]}]},{"required":["contentBlocks"],"properties":{"contentBlocks":{"contains":{"$ref":"#/$defs/ListStartPresent"}}}},{"required":["elements"],"properties":{"elements":{"contains":{"$ref":"#/$defs/ListStartPresent"}}}},{"required":["columns"],"properties":{"columns":{"contains":{"$ref":"#/$defs/ListStartPresent"}}}},{"required":["items"],"properties":{"items":{"contains":{"$ref":"#/$defs/ListStartPresent"}}}},{"required":["subPoints"],"properties":{"subPoints":{"contains":{"$ref":"#/$defs/ListStartPresent"}}}}]}`), &ListStartPresent); err != nil { panic(err) }
	root.Definitions["ListStartPresent"] = &ListStartPresent
	var MathPresent jsonschema.Schema
	if err := json.Unmarshal([]byte(`{"anyOf":[{"required":["type"],"properties":{"type":{"const":"math"}}},{"required":["contentBlocks"],"properties":{"contentBlocks":{"contains":{"$ref":"#/$defs/MathPresent"}}}},{"required":["elements"],"properties":{"elements":{"contains":{"$ref":"#/$defs/MathPresent"}}}},{"required":["columns"],"properties":{"columns":{"contains":{"$ref":"#/$defs/MathPresent"}}}},{"required":["items"],"properties":{"items":{"contains":{"$ref":"#/$defs/MathPresent"}}}},{"required":["subPoints"],"properties":{"subPoints":{"contains":{"$ref":"#/$defs/MathPresent"}}}}]}`), &MathPresent); err != nil { panic(err) }
	root.Definitions["MathPresent"] = &MathPresent
	var ListStartTree jsonschema.Schema
	if err := json.Unmarshal([]byte(`{"type":"object","allOf":[{"if":{"required":["start"]},"then":{"required":["type","listType"],"properties":{"type":{"const":"points"},"listType":{"const":"ordered"}}}},{"if":{"required":["subListStart"]},"then":{"required":["type","subListType","subPoints"],"properties":{"type":{"const":"point_item"},"subListType":{"const":"ordered"},"subPoints":{"type":"array","minItems":1}}}}],"properties":{"contentBlocks":{"items":{"$ref":"#/$defs/ListStartTree"}},"elements":{"items":{"$ref":"#/$defs/ListStartTree"}},"columns":{"items":{"$ref":"#/$defs/ListStartTree"}},"items":{"items":{"$ref":"#/$defs/ListStartTree"}},"subPoints":{"items":{"$ref":"#/$defs/ListStartTree"}}}}`), &ListStartTree); err != nil { panic(err) }
	root.Definitions["ListStartTree"] = &ListStartTree

 for def, prop := range map[string]string{"PointsElement":"start", "PointItem":"subListStart"} {
  var bound jsonschema.Schema
  if err := json.Unmarshal([]byte(`{"type":"integer","minimum":1,"maximum":9007199254740991}`), &bound); err != nil { panic(err) }
  if err := overrideProperty(root.Definitions, def, prop, &bound); err != nil { panic(err) }
 }
	gateJSON, err := contractGateJSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var gate jsonschema.Schema
	if err := json.Unmarshal([]byte(gateJSON), &gate); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root.Definitions["AST"].AllOf = append(root.Definitions["AST"].AllOf, &gate)

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
	if err := setElementsProperty(root.Definitions, "SpecialBlockElement", elementsUnion); err != nil {
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
	// ThemeMode is an open Go string so the parser can preserve an invalid
	// authored value long enough for fmt to round-trip it with FRONT008, but
	// the public JSON contract only accepts the two usable visual schemes.
	if err := overrideProperty(root.Definitions, "FrontMatterNode", "theme_mode", themeModeSchema()); err != nil {
		fmt.Fprintf(os.Stderr, "error tipando theme_mode: %v\n", err)
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

func themeModeSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "string", Enum: []any{"light", "dark"}}
}

// contractGateJSON arma el gate de versión/capabilities del AST a partir de la
// misma regla que ast/extensions.go:
//
//   - por cada capability, declararla equivale a que su contenido esté
//     presente (en cualquier profundidad de elementos);
//   - cada versión extendida exige capabilities únicas, solo de extensiones de
//     su versión o anteriores, y la de su propia versión;
//   - las versiones legadas no declaran capabilities;
//   - nested-list-types-v1 además exige el árbol de puntos tipado.
func contractGateJSON() ([]byte, error) {
	type ext struct{ capability, version, present string }
	exts := []ext{
		{ast.TableRowsCapability, ast.TableSchemaVersion, "TableRowsPresent"},
		{ast.NestedListTypesCapability, ast.NestedListSchemaVersion, "NestedListPresent"},
		{ast.TypedHeadingsCapability, ast.TypedHeadingsSchemaVersion, "TypedHeadingPresent"},
		{ast.MediaFigureCapability, ast.MediaFigureSchemaVersion, "MediaFigurePresent"},
		{ast.CodeFilenameCapability, ast.CodeFilenameSchemaVersion, "CodeFilenamePresent"},
		{ast.QuizPollResultsCapability, ast.QuizPollResultsSchemaVersion, "QuizPollResultsPresent"},
		{ast.ListStartCapability, ast.ListStartSchemaVersion, "ListStartPresent"},
		{ast.MathSourceCapability, ast.MathSourceSchemaVersion, "MathPresent"},
	}
	ref := func(name string) map[string]any { return map[string]any{"$ref": "#/$defs/" + name} }
	declares := func(capability string) map[string]any {
		return map[string]any{"required": []any{"capabilities"}, "properties": map[string]any{"capabilities": map[string]any{"contains": map[string]any{"const": capability}}}}
	}
	versionIs := func(v string) map[string]any {
		return map[string]any{"required": []any{"schemaVersion"}, "properties": map[string]any{"schemaVersion": map[string]any{"const": v}}}
	}
	var all []any
	for _, e := range exts {
		if e.capability == ast.MathSourceCapability {
			all = append(all, map[string]any{"if": declares(e.capability), "then": ref(e.present)})
		} else {
			all = append(all, map[string]any{"if": declares(e.capability), "then": ref(e.present), "else": map[string]any{"not": ref(e.present)}})
		}
	}
	all = append(all, map[string]any{"if": declares(ast.NestedListTypesCapability), "then": ref("TypedPointTree")})
	all = append(all, ref("ListStartTree"))
	var allowed []any
	for _, e := range exts {
		allowed = append(allowed, e.capability)
		all = append(all, map[string]any{"if": versionIs(e.version), "then": map[string]any{
			"required": []any{"capabilities"},
			"properties": map[string]any{"capabilities": map[string]any{
				"type": "array", "uniqueItems": true,
				"items":    map[string]any{"enum": append([]any(nil), allowed...)},
				"contains": map[string]any{"const": e.capability},
			}},
		}})
	}
	all = append(all, map[string]any{"if": map[string]any{"required": []any{"schemaVersion"}, "properties": map[string]any{"schemaVersion": map[string]any{"enum": []any{ast.PreviousSchemaVersion, ast.LegacySchemaVersion}}}}, "then": map[string]any{"not": map[string]any{"required": []any{"capabilities"}}}})
	return json.Marshal(map[string]any{"allOf": all})
}
