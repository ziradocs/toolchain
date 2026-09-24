// Command reader_probe is an isolated compatibility probe for the current
// core/v2 reader and formatter. Run it on Ubuntu from the core module with:
// go run ../experiments/table-identity/reader_probe.go BASE_AST SCHEMA OUT_DIR
// `99.0.0` is a test sentinel, not a proposed AST release number.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/formatter"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func write(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path, append(data, '\n'), 0644))
}

func table(doc map[string]any) map[string]any {
	blocks := doc["contentBlocks"].([]any)
	elements := blocks[0].(map[string]any)["elements"].([]any)
	for _, element := range elements {
		item := element.(map[string]any)
		if item["type"] == "table" {
			return item
		}
	}
	panic("fixture contains no table")
}

func main() {
	if len(os.Args) != 4 {
		panic("usage: reader_probe BASE_AST SCHEMA OUT_DIR")
	}
	base, err := os.ReadFile(os.Args[1])
	must(err)
	must(os.MkdirAll(os.Args[3], 0755))
	compiler := jsonschema.NewCompiler()
	must(compiler.AddResource("schema.json", mustOpen(os.Args[2])))
	schema, err := compiler.Compile("schema.json")
	must(err)
	result := map[string]any{}
	for _, tc := range []struct {
		name, version string
		rows          bool
	}{
		{"baseline", "2.14.0", false},
		{"version_only", "99.0.0", false},
		{"field_only", "2.14.0", true},
		{"version_and_field", "99.0.0", true},
	} {
		var doc map[string]any
		must(json.Unmarshal(base, &doc))
		doc["schemaVersion"] = tc.version
		if tc.rows {
			table(doc)["tableRows"] = []any{
			map[string]any{"nodeId": "HeaderRow", "section": "header", "cells": []any{
				map[string]any{"nodeId": "KeyHeader", "content": "Key", "header": true},
				map[string]any{"nodeId": "ValueHeader", "content": "Value", "header": true},
			}},
			map[string]any{"nodeId": "DataRowA", "section": "body", "cells": []any{
				map[string]any{"nodeId": "KeyA", "content": "A"},
				map[string]any{"nodeId": "ValueA", "content": "1"},
			}},
		}
		}
		inputPath := filepath.Join(os.Args[3], tc.name+".json")
		write(inputPath, doc)
		input, err := os.ReadFile(inputPath)
		must(err)
		var generic any
		must(json.Unmarshal(input, &generic))
		schemaErr := schema.Validate(generic)
		item := map[string]any{"schemaAccepted": schemaErr == nil}
		if schemaErr != nil {
			item["schemaError"] = schemaErr.Error()
		}
		decoded, decodeErr := ast.DecodeAST(input)
		item["decodeAccepted"] = decodeErr == nil
		if decodeErr != nil {
			item["decodeError"] = decodeErr.Error()
		} else {
			encoded, err := json.Marshal(decoded)
			must(err)
			var round map[string]any
			must(json.Unmarshal(encoded, &round))
			item["decodedVersion"] = round["schemaVersion"]
			_, item["tableRowsSurvivedDecode"] = table(round)["tableRows"]
			write(filepath.Join(os.Args[3], tc.name+".decoded.json"), round)
			formatted, formatErr := formatter.FormatStrict(decoded)
			item["formatterAccepted"] = formatErr == nil
			if formatErr != nil {
				item["formatterError"] = formatErr.Error()
			} else {
				must(os.WriteFile(filepath.Join(os.Args[3], tc.name+".formatted.slidelang"), []byte(formatted), 0644))
			}
		}
		result[tc.name] = item
	}
	write(filepath.Join(os.Args[3], "reader-results.json"), result)
	fmt.Println("wrote", filepath.Join(os.Args[3], "reader-results.json"))
}

func mustOpen(path string) *os.File {
	f, err := os.Open(path)
	must(err)
	return f
}
