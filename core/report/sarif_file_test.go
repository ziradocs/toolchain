// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"go.ziradocs.com/core/v2/diagnostics"
)

// Un diagnóstico con File (un archivo traído con @include, relativo al
// directorio del documento) va en SARIF con ese archivo unido al directorio
// del documento, para que las dos uris resuelvan contra la misma base; el
// resto, con el documento.
func TestSARIF_ArtifactLocationUsesDiagnosticFile(t *testing.T) {
	at := func(file string, line int) diagnostics.Diagnostic {
		d := diagnostics.NewError("x", diagnostics.NewPosition(line, 1), "parser")
		d.File = file
		return d
	}
	cases := []struct {
		name, docPath string
		diags         []diagnostics.Diagnostic
		want          []string
	}{
		{"same directory", "main.doclang", []diagnostics.Diagnostic{at("", 3), at("partes/a.doclang", 2)}, []string{"main.doclang", "partes/a.doclang"}},
		{"document in a subdirectory", "docs/main.doclang", []diagnostics.Diagnostic{at("partes/a.doclang", 2), at("../comun/pie.doclang", 1)}, []string{"docs/partes/a.doclang", "comun/pie.doclang"}},
		{"absolute file", "docs/main.doclang", []diagnostics.Diagnostic{at("/srv/comun/pie.doclang", 1)}, []string{"file:///srv/comun/pie.doclang"}},
		{"no document path", "", []diagnostics.Diagnostic{at("partes/a.doclang", 1)}, []string{"partes/a.doclang"}},
	}
	for _, c := range cases {
		data, err := generateSARIF(c.diags, nil, c.docPath, nil)
		if err != nil {
			t.Fatal(err)
		}
		var out SARIF
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, r := range out.Runs[0].Results {
			got = append(got, r.Locations[0].PhysicalLocation.ArtifactLocation.Uri)
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: uris %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: uris %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

// artifactURI no depende del sistema: "\\" y "/" valen igual, una ruta con
// letra de unidad o absoluta va como URI file:, y cada segmento va escapado
// según RFC 3986.
func TestArtifactURI(t *testing.T) {
	cases := []struct{ docPath, file, want string }{
		{"main.doclang", "", "main.doclang"},
		{"docs/main.doclang", "partes/a.doclang", "docs/partes/a.doclang"},
		{"docs\\main.doclang", "partes\\a.doclang", "docs/partes/a.doclang"},
		{"docs/main.doclang", "../comun/pie.doclang", "comun/pie.doclang"},
		{"main.doclang", "../comun/pie.doclang", "../comun/pie.doclang"},
		{"mis docs/informe final.doclang", "", "mis%20docs/informe%20final.doclang"},
		{"docs/main.doclang", "partes/año 1/sección #2.doclang", "docs/partes/a%C3%B1o%201/secci%C3%B3n%20%232.doclang"},
		{"D:\\docs\\main.doclang", "", "file:///D:/docs/main.doclang"},
		{"D:\\docs\\main.doclang", "partes\\a.doclang", "file:///D:/docs/partes/a.doclang"},
		{"D:\\docs\\main.doclang", "E:/comun/pie.doclang", "file:///E:/comun/pie.doclang"},
		{"docs/main.doclang", "C:/Users/Ana María/pie.doclang", "file:///C:/Users/Ana%20Mar%C3%ADa/pie.doclang"},
		{"/srv/docs/main.doclang", "", "file:///srv/docs/main.doclang"},
		{"/srv/docs/main.doclang", "partes/a b.doclang", "file:///srv/docs/partes/a%20b.doclang"},
		{"\\\\servidor\\r\\main.doclang", "", "file://servidor/r/main.doclang"},
		{"\\\\servidor\\r\\main.doclang", "partes\\a.doclang", "file://servidor/r/partes/a.doclang"},
		{"", "partes/a.doclang", "partes/a.doclang"},
		{"a:b.doclang", "", "./a:b.doclang"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := artifactURI(c.docPath, c.file); got != c.want {
			t.Errorf("artifactURI(%q, %q) = %q, want %q", c.docPath, c.file, got, c.want)
		}
	}
}

// El SARIF con uris de documento y de archivos incluidos (espacios, Unicode,
// letra de unidad, UNC, absolutas) cumple el esquema 2.1.0 con sus "format"
// (artifactLocation.uri es uri-reference).
func TestSARIF_URIsMatchSchemaFormats(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat = true
	sch, err := compiler.Compile(filepath.Join("testdata", "sarif-schema-2.1.0.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, docPath := range []string{"mis docs/informe ñ.doclang", "D:\\docs\\main.doclang", "\\\\servidor\\r\\main.doclang", "/srv/a b/main.doclang", "a:b.doclang"} {
		var diags []diagnostics.Diagnostic
		for _, file := range []string{"", "partes/año 1/sección #2.doclang", "../comun/pie.doclang", "C:/Users/Ana María/pie.doclang", "/srv/x.doclang"} {
			d := diagnostics.NewError("x", diagnostics.NewPosition(1, 1), "parser")
			d.File = file
			diags = append(diags, d)
		}
		data, err := generateSARIF(diags, nil, docPath, nil)
		if err != nil {
			t.Fatal(err)
		}
		var v interface{}
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatal(err)
		}
		if err := sch.Validate(v); err != nil {
			t.Errorf("docPath %q: SARIF does not match the schema: %v", docPath, err)
		}
		// El format uri-reference del validador acepta espacios y Unicode
		// sin escapar; RFC 3986 no.
		var out SARIF
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		for _, r := range out.Runs[0].Results {
			uri := r.Locations[0].PhysicalLocation.ArtifactLocation.Uri
			for _, c := range []byte(uri) {
				if c <= ' ' || c > '~' || c == '\\' {
					t.Errorf("docPath %q: uri %q has an unescaped %q", docPath, uri, c)
					break
				}
			}
		}
	}
}
