// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// knownFieldsRows es como se leía tableRows: en origin/main: KnownFields(true)
// sobre las mismas estructuras. Es la referencia del diferencial.
func knownFieldsRows(text string) ([]yamlTableRowEntry, error) {
	var parsed []yamlTableRowEntry
	d := yaml.NewDecoder(strings.NewReader(text))
	d.KnownFields(true)
	err := d.Decode(&parsed)
	return parsed, err
}

// compareRowDecoders falla si decodeTableRows y la referencia no aceptan o
// rechazan lo mismo, o si aceptan con valores distintos.
func compareRowDecoders(t *testing.T, name, text string) {
	t.Helper()
	want, wantErr := knownFieldsRows(text)
	got, _, gotErr := decodeTableRows(strings.Split(text, "\n"))
	if (wantErr == nil) != (gotErr == nil) {
		t.Errorf("%s: KnownFields error %v, decodeTableRows error %v\n%s", name, wantErr, gotErr, text)
		return
	}
	if wantErr != nil {
		return // con error el bloque no se usa: los valores a medias no cuentan
	}
	w, _ := json.Marshal(want)
	g, _ := json.Marshal(got)
	if string(w) != string(g) {
		t.Errorf("%s: values differ\nKnownFields     %s\ndecodeTableRows %s\n%s", name, w, g, text)
	}
}

var tableRowsDiffCases = map[string]string{
	"plain":                    "- cells: [{content: a, header: true}]\n- cells: [{content: b}]",
	"unknown row key":          "- cells: [{content: a}]\n  bogus: 1",
	"unknown cell key":         "- cells: [{content: a, bogus: 1}]",
	"scalar row":               "- just text",
	"scalar cell":              "- cells: [plain]",
	"number content":           "- cells: [{content: 42}]",
	"string header":            "- cells: [{content: a, header: \"yes\"}]",
	"colspan string":           "- cells: [{content: a, colspan: two}]",
	"null content":             "- cells: [{content: ~}]",
	"tagged content":           "- cells: [{content: !!str 12}]",
	"block literal":            "- cells:\n    - content: |\n        line1\n        line2",
	"alias row":                "- &r {cells: [{content: a}]}\n- *r",
	"alias of a cell as a row": "- cells: [&c {content: a}]\n- *c",
	"alias cell":               "- cells: [&c {content: a}]\n  section: body\n- cells: [*c]",
	"alias cells sequence":     "- cells: &cs [{content: a}]\n- cells: *cs",
	"undefined alias":          "- nodeId: x\n  cells: [{content: a}]\n- cells: *bad\n",
	"merge row":                "- &base {section: body, cells: [{content: a}]}\n- <<: *base\n  nodeId: r2",
	"merge cell":               "- cells: [&c {content: a}, {<<: *c, header: true}]",
	"merge cell block":         "- cells:\n    - &cell {content: Ana}\n    - <<: *cell\n      header: true",
	"merge with unknown key":   "- &base {zz: 1}\n- <<: *base\n  cells: [{content: a}]",
	"merge sequence":           "- &a {section: body}\n- &b {nodeId: x}\n- <<: [*a, *b]\n  cells: [{content: c}]",
	"merge inline mapping":     "- <<: {section: footer}\n  cells: [{content: a}]",
	"merge scalar":             "- <<: 3\n  cells: [{content: a}]",
	"merge cell into row":      "- cells: [&c {content: a}]\n- <<: *c\n  cells: [{content: b}]",
	"duplicate row key":        "- cells: [{content: a}]\n  cells: [{content: b}]",
	"duplicate cell key":       "- cells: [{content: a, content: b}]",
	"invalid":                  "- cells: [{content: a}\n- x",
	"empty":                    "",
	"only comment":             "# nothing",
	"two documents":            "- cells: [{content: a}]\n---\n- cells: [{content: b, bogus: 1}]",
	"mapping root":             "cells: [{content: a}]",
	"nested mapping value":     "- cells: [{content: a, scope: {x: 1}}]",
	"numeric key":              "- cells: [{content: a}]\n  1: x",
	"tagged key":               "- !!str cells: [{content: a}]",
	"quoted key":               "- \"cells\": [{content: a}]",
	"null cells":               "- cells:",
	"cells mapping":            "- cells: {content: a}",
	"section number":           "- section: 3\n  cells: [{content: a}]",
	"null row":                 "- ~\n- cells: [{content: a}]",
	"null cell":                "- cells: [~, {content: a}]",
	"flow row unknown":         "- {cells: [{content: a}], extra: 2}",
}

// tableRowsFragments son filas sueltas para el generador. %d es un número
// único; las que definen o usan un ancla lo hacen sobre anclas ya definidas
// (r0, c0 se definen siempre al principio).
var tableRowsFragments = []string{
	"- cells: [{content: x%d}]",
	"- &r%d {section: body, cells: [{content: x%d}]}",
	"- *r0",
	"- <<: *r0\n  nodeId: id%d",
	"- <<: [*r0, *r1]\n  nodeId: id%d",
	"- <<: {section: footer}\n  cells: [{content: x%d}]",
	"- <<: *c0\n  cells: [{content: x%d}]",
	"- cells: [&c%d {content: x%d}]",
	"- cells: [*c0, {<<: *c0, header: true}]",
	"- cells: [{<<: [*c0, *c1], scope: col}]",
	"- cells:\n    - <<: *c0\n      header: true\n    - content: x%d",
	"- bogus: %d",
	"- cells: [{content: x%d, zz: 1}]",
	"- cells: [{content: x%d, colspan: two}]",
	"- cells: [{content: x%d, header: maybe}]",
	"- ~",
	"- cells: [~]",
	"- cells:",
	"- <<: 3\n  cells: [{content: x%d}]",
	"- *c0",
	"- section: [a]\n  cells: [{content: x%d}]",
	"- cells: [{content: [x%d]}]",
}

// El camino nuevo de tableRows: acepta y rechaza exactamente lo mismo que
// KnownFields(true), con los mismos valores: llaves de merge (a nivel de fila y
// de celda, con una llave desconocida, con una secuencia de alias), alias de
// una fila y de una celda, llaves desconocidas, tipos equivocados y nulos.
func TestDecodeTableRowsMatchesKnownFields(t *testing.T) {
	for name, text := range tableRowsDiffCases {
		compareRowDecoders(t, name, text)
	}
	const prelude = "- &r0 {section: header, cells: [&c0 {content: h0, header: true}]}\n- &r1 {nodeId: n1, cells: [&c1 {content: h1}]}\n"
	for seed := int64(1); seed <= 500; seed++ {
		rng := rand.New(rand.NewSource(seed))
		var b strings.Builder
		b.WriteString(prelude)
		for n := 0; n < 1+rng.Intn(4); n++ {
			f := tableRowsFragments[rng.Intn(len(tableRowsFragments))]
			args := make([]any, strings.Count(f, "%d"))
			for k := range args {
				args[k] = 100*int(seed) + n
			}
			b.WriteString(fmt.Sprintf(f, args...) + "\n")
		}
		compareRowDecoders(t, fmt.Sprintf("seed %d", seed), b.String())
	}
}

// parseCellsYAML lee el bloque a un yaml.Node y de ahí a las celdas; es lo
// mismo que yaml.Unmarshal directo (lo que hacía origin/main), también con
// alias, merges, nulos y tipos.
func TestCellsNodeDecodeMatchesUnmarshal(t *testing.T) {
	cases := []string{
		"- [{content: a, header: true}, {content: b}]",
		"- [{content: ~}]",
		"- [{content: !!str 12, colspan: 2}]",
		"- - content: |\n      x\n      y\n    header: true",
		"- &r [{content: a}]\n- *r",
		"- [&c {content: a}, {<<: *c, header: true}]",
		"- [{<<: [{content: a}, {scope: col}]}]",
		"- [{content: a, content: b}]",
		"- [{content: a, zz: 1}]",
		"- [{content: a, colspan: x}]",
		"- [{content: a}",
		"- [{content: a}]\n---\n- [{content: b}]",
		"- [{<<: 3}]",
		"- ~",
		"",
	}
	for _, text := range cases {
		var old [][]yamlTableCellEntry
		oldErr := yaml.Unmarshal([]byte(text), &old)
		var doc yaml.Node
		var got [][]yamlTableCellEntry
		gotErr := yaml.Unmarshal([]byte(text), &doc)
		if gotErr == nil && len(doc.Content) > 0 {
			gotErr = doc.Decode(&got)
		}
		if (oldErr == nil) != (gotErr == nil) {
			t.Errorf("%q: Unmarshal error %v, node path error %v", text, oldErr, gotErr)
			continue
		}
		o, _ := json.Marshal(old)
		g, _ := json.Marshal(got)
		if oldErr == nil && string(o) != string(g) {
			t.Errorf("%q: Unmarshal %s, node path %s", text, o, g)
		}
	}
}

// El error de YAML que cita CHART007 no lleva números de línea del bloque ni
// saltos de línea, une varios errores con "; " y dice en palabras que
// options: tiene que ser un mapeo.
func TestYAMLErrorTextForChartOptions(t *testing.T) {
	cases := map[string]string{
		"plugins: true\nplugins: false": `mapping key "plugins" already defined`,
		"- a\n- b":                      "options: must be a mapping",
		"solo texto":                    "options: must be a mapping",
		"a: [1\nb: 2":                   "did not find expected ',' or ']'",
		"a: 1\nb:\n  - x\nb: 2\na: 3":   `mapping key "a" already defined; mapping key "b" already defined`,
	}
	for src, want := range cases {
		var opts map[string]interface{}
		err := yaml.Unmarshal([]byte(src), &opts)
		if err == nil {
			t.Fatalf("%q: want a YAML error", src)
		}
		got := yamlErrorText(err)
		if got != want {
			t.Errorf("%q: yamlErrorText = %q, want %q (raw %q)", src, got, want, err.Error())
		}
	}
}
