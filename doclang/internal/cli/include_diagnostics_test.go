// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"html"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// includeDiagTree arma docs/main.doclang, que incluye partes/x.doclang, que
// a su vez incluye partes/sub/y.doclang. x trae un bloque de código vacío
// (CODE001, del linter) en la línea 5, un @include dentro de un bloque de
// código con una ruta que no existe (INCLUDE001) en la 9, una tabla con YAML
// inválido (TABLE004, del parser) en la 12 y una imagen que falta en la 16;
// y incluye a y, que trae otro bloque vacío en la 7. El reporte ordena por
// archivo y después por línea, así que y:7 va antes que x:5.
func includeDiagTree(t *testing.T) (root, docs string) {
	t.Helper()
	root = t.TempDir()
	docs = filepath.Join(root, "docs")
	writeFile(t, filepath.Join(docs, "main.doclang"), []byte("---\ntitle: D\nmode: flex\n---\n\n# A\n\nTexto.\n\n@include partes/x.doclang\n"))
	writeFile(t, filepath.Join(docs, "partes", "x.doclang"), []byte("## X\n\nUno.\n\n```\n```\n\n```go\n@include no/existe.doclang\n```\n\n"+
		"TABLE\n  cells:\n    - [ : ]\n\n![falta](falta.png)\n\n@include partes/sub/y.doclang\n"))
	writeFile(t, filepath.Join(docs, "partes", "sub", "y.doclang"), []byte("### Y\n\nA.\n\nB.\n\n```\n```\n"))
	return root, docs
}

type finding struct {
	RuleID   string `json:"ruleId"`
	File     string `json:"file"`
	Position struct {
		Line int `json:"line"`
	} `json:"position"`
}

// lintReport corre un build --lint-only con --report json y devuelve
// "regla archivo:línea" de cada hallazgo, en el orden del reporte.
func lintReport(t *testing.T, input string, extra ...string) []string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "r.json")
	runBuild(t, append([]string{input, "--lint-only", "--report", "json", "--report-out", out}, extra...)...)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Findings []finding `json:"findings"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range report.Findings {
		got = append(got, f.RuleID+" "+f.File+":"+strconv.Itoa(f.Position.Line))
	}
	return got
}

// sarifURIs corre un build --lint-only con --report sarif y devuelve la ruta
// en disco de cada resultado: el uri del artifactLocation (file:// si la
// ruta es absoluta), que core arma con el directorio del documento y
// Diagnostic.File.
func sarifURIs(t *testing.T, input string, extra ...string) map[string]string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "r.sarif")
	runBuild(t, append([]string{input, "--lint-only", "--report", "sarif", "--report-out", out}, extra...)...)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var sarif struct {
		Runs []struct {
			Results []struct {
				RuleID    string `json:"ruleId"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(data, &sarif); err != nil {
		t.Fatal(err)
	}
	uris := map[string]string{}
	for _, r := range sarif.Runs[0].Results {
		u, err := url.Parse(r.Locations[0].PhysicalLocation.ArtifactLocation.URI)
		if err != nil {
			t.Fatal(err)
		}
		uris[r.RuleID] = path.Clean(u.Path)
	}
	return uris
}

// Los diagnósticos de un archivo incluido llevan su archivo, relativo al
// documento, y su línea en ese archivo, en el reporte y en SARIF, con la
// entrada por su ruta real, por un directorio enlazado (symlink) y con
// --base-dir real o enlazado.
func TestBuild_IncludeDiagnosticsPointAtSource(t *testing.T) {
	root, docs := includeDiagTree(t)
	link := filepath.Join(root, "enlace")
	if err := os.Symlink(docs, link); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	want := []string{
		"CODE001 partes/sub/y.doclang:7",
		"CODE001 partes/x.doclang:5",
		"INCLUDE001 partes/x.doclang:9",
		"TABLE004 partes/x.doclang:12",
		"TABLE001 partes/x.doclang:12",
		"TABLE002 partes/x.doclang:12",
	}
	for _, c := range []struct {
		name  string
		input string
		extra []string
	}{
		{"real path", filepath.Join(docs, "main.doclang"), nil},
		{"symlinked dir", filepath.Join(link, "main.doclang"), nil},
		{"base-dir", filepath.Join(docs, "main.doclang"), []string{"--base-dir", docs}},
		{"symlinked base-dir", filepath.Join(link, "main.doclang"), []string{"--base-dir", link}},
		{"include-root", filepath.Join(link, "main.doclang"), []string{"--include-root", link}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := lintReport(t, c.input, c.extra...)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			uris := sarifURIs(t, c.input, c.extra...)
			for rule, rel := range map[string]string{"INCLUDE001": "partes/x.doclang", "TABLE004": "partes/x.doclang"} {
				wantURI := path.Join(filepath.ToSlash(filepath.Dir(c.input)), rel)
				if uris[rule] != wantURI {
					t.Errorf("SARIF %s uri = %q, want %q", rule, uris[rule], wantURI)
				}
			}
		})
	}
}

// Con --base-dir y la entrada fuera de él (un manifiesto temporal), el
// archivo incluido se nombra relativo al documento, y SARIF lo encuentra.
func TestBuild_IncludeDiagnosticsFromManifestOutsideBaseDir(t *testing.T) {
	_, docs := includeDiagTree(t)
	manifest := filepath.Join(t.TempDir(), "seleccion.doclang")
	writeFile(t, manifest, []byte("---\ntitle: S\nmode: flex\n---\n\n# S\n\n@include partes/x.doclang\n"))
	got := lintReport(t, manifest, "--base-dir", docs)
	realDocs, err := filepath.EvalSymlinks(docs)
	if err != nil {
		t.Fatal(err)
	}
	realDir, err := filepath.EvalSymlinks(filepath.Dir(manifest))
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(realDir, filepath.Join(realDocs, "partes", "x.doclang"))
	if err != nil {
		t.Fatal(err)
	}
	wantInclude := "INCLUDE001 " + filepath.ToSlash(rel) + ":9"
	found := false
	for _, g := range got {
		found = found || g == wantInclude
	}
	if !found {
		t.Errorf("want %q in %v", wantInclude, got)
	}
	uri := sarifURIs(t, manifest, "--base-dir", docs)["INCLUDE001"]
	if _, err := os.Stat(filepath.FromSlash(uri)); err != nil {
		t.Errorf("SARIF uri %q does not name the included file: %v", uri, err)
	}
}

// --fail-on-unavailable nombra el archivo incluido y la línea de la imagen
// que falta, no la línea del contenido expandido.
func TestBuild_FailOnUnavailableNamesIncludedFile(t *testing.T) {
	_, docs := includeDiagTree(t)
	err := buildErr(filepath.Join(docs, "main.doclang"), "--format", "docx", "--fail-on-unavailable", "--output", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "partes/x.doclang:16: image") {
		t.Fatalf("want the included file and its line in the error, got %v", err)
	}
}

// Issue #487: un @include dentro de un bloque de código es código: el build
// no falla aunque la ruta no exista, y el HTML lo muestra tal cual.
func TestBuild_IncludeInCodeIsKeptAsWritten(t *testing.T) {
	_, docs := includeDiagTree(t)
	out := t.TempDir()
	runBuild(t, filepath.Join(docs, "main.doclang"), "--format", "html", "--output", out)
	data, err := os.ReadFile(filepath.Join(out, "main.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "@include no/existe.doclang") {
		t.Error("the @include inside the code block is not in the HTML as written")
	}
}

// Los tres casos del issue #487, con la sintaxis estricta: un @include en un
// CODE (también con U+00A0 o U+3000 delante) queda como código y deja un
// INCLUDE001 en su línea, y una {{variable}} en un CODE queda literal, en el
// HTML y en el DOCX.
func TestBuild_Issue487Repros(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "other.txt"), []byte("    INCLUDED-LINE-1\n    INCLUDED-LINE-2\n"))
	head := "---\nmode: strict\ntitle: \"Demo\"\n---\n\nSECTION \"A\"\n"
	for name, c := range map[string]struct {
		body, want string
		include    bool
	}{
		"include":           {"  CODE\n    @include other.txt\n", "@include other.txt", true},
		"include NBSP":      {"  CODE\n     @include other.txt\n", "@include other.txt", true},
		"include ideograph": {"  CODE\n    　@include other.txt\n", "@include other.txt", true},
		"variable":          {"  CODE yaml\n    a: {{title}}\n", "a: {{title}}", false},
	} {
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".doclang")
			writeFile(t, input, []byte(head+c.body))
			out := t.TempDir()
			runBuild(t, input, "--format", "html", "--render-mode", "offline-inline", "--asset-root", dir, "--output", out)
			runBuild(t, input, "--format", "docx", "--asset-root", dir, "--output", out)
			base := strings.TrimSuffix(filepath.Base(input), ".doclang")
			htmlData, err := os.ReadFile(filepath.Join(out, base+".html"))
			if err != nil {
				t.Fatal(err)
			}
			for format, text := range map[string]string{
				"html": html.UnescapeString(string(htmlData)),
				"docx": html.UnescapeString(docxDocumentXML(t, filepath.Join(out, base+".docx"))),
			} {
				if strings.Contains(text, "INCLUDED-LINE") || strings.Contains(text, "a: Demo") {
					t.Errorf("%s: code was expanded or substituted", format)
				}
				if !strings.Contains(text, c.want) {
					t.Errorf("%s: missing %q", format, c.want)
				}
			}
			got := lintReport(t, input)
			hasInclude := false
			for _, g := range got {
				hasInclude = hasInclude || g == "INCLUDE001 :8"
			}
			if hasInclude != c.include {
				t.Errorf("INCLUDE001 at line 8 = %v, want %v (findings %v)", hasInclude, c.include, got)
			}
		})
	}
}
