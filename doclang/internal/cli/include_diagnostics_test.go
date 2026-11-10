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
	"sort"
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

// Con la entrada por un directorio enlazado y un archivo incluido fuera del
// directorio del documento, el `file` del reporte y la uri de SARIF unidos
// al directorio de document.path (path.Join, léxico, como lo une core)
// nombran el archivo incluido, con --base-dir y con --include-root, y con la
// entrada absoluta o relativa al directorio de trabajo.
func TestBuild_IncludeDiagnosticsThroughSymlinkedDirOutsideDocDir(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a", "b", "real", "main.doclang"), []byte("---\ntitle: D\nmode: flex\n---\n\n# A\n\nTexto.\n\n@include inc/x.doclang\n"))
	writeFile(t, filepath.Join(root, "inc", "x.doclang"), []byte("## X\n\nUno.\n\n```\n```\n\n```go\n@include no/existe.doclang\n```\n"))
	if err := os.Symlink(filepath.Join("a", "b", "real"), filepath.Join(root, "lnk")); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	wantFile, err := os.Stat(filepath.Join(root, "inc", "x.doclang"))
	if err != nil {
		t.Fatal(err)
	}
	// names dice si p (relativo al directorio de trabajo o absoluto) es el
	// archivo incluido.
	names := func(p string) bool {
		info, err := os.Stat(filepath.FromSlash(p))
		return err == nil && os.SameFile(info, wantFile)
	}
	t.Chdir(root)
	for _, input := range []string{filepath.Join(root, "lnk", "main.doclang"), filepath.Join("lnk", "main.doclang")} {
		for _, flag := range []string{"--base-dir", "--include-root"} {
			t.Run(flag+" "+input, func(t *testing.T) {
				out := filepath.Join(t.TempDir(), "r.json")
				runBuild(t, input, flag, root, "--lint-only", "--report", "json", "--report-out", out)
				data, err := os.ReadFile(out)
				if err != nil {
					t.Fatal(err)
				}
				var report struct {
					Document struct {
						Path string `json:"path"`
					} `json:"document"`
					Findings []finding `json:"findings"`
				}
				if err := json.Unmarshal(data, &report); err != nil {
					t.Fatal(err)
				}
				rules := map[string]bool{}
				for _, f := range report.Findings {
					rules[f.RuleID] = true
					if p := path.Join(path.Dir(report.Document.Path), f.File); !names(p) {
						t.Errorf("JSON %s: document.path %q + file %q = %q, which is not inc/x.doclang", f.RuleID, report.Document.Path, f.File, p)
					}
				}
				if !rules["CODE001"] || !rules["INCLUDE001"] {
					t.Fatalf("want CODE001 and INCLUDE001, got %v", report.Findings)
				}
				for rule, uri := range sarifURIs(t, input, flag, root) {
					if !names(uri) {
						t.Errorf("SARIF %s uri %q is not inc/x.doclang", rule, uri)
					}
				}
			})
		}
	}
}

// Un archivo incluido dos veces deja cada hallazgo una sola vez en el
// reporte y en SARIF: tras llevarlos al archivo de origen, las dos copias
// son el mismo hallazgo en la misma línea.
func TestBuild_IncludedTwiceReportsEachFindingOnce(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "main.doclang")
	writeFile(t, input, []byte("---\ntitle: D\nmode: flex\n---\n\n# A\n\n@include partes/x.doclang\n\n# B\n\n@include partes/x.doclang\n\n```\n```\n"))
	writeFile(t, filepath.Join(dir, "partes", "x.doclang"), []byte("Uno.\n\n```\n```\n\n```go\n@include no/existe.doclang\n```\n"))
	want := []string{
		"CODE001 :14",
		"CODE001 partes/x.doclang:3",
		"INCLUDE001 partes/x.doclang:7",
	}
	if got := lintReport(t, input); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	out := filepath.Join(t.TempDir(), "r.sarif")
	runBuild(t, input, "--lint-only", "--report", "sarif", "--report-out", out)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var sarif struct {
		Runs []struct {
			Results []json.RawMessage `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(data, &sarif); err != nil {
		t.Fatal(err)
	}
	if n := len(sarif.Runs[0].Results); n != len(want) {
		t.Errorf("SARIF has %d results, want %d", n, len(want))
	}
}

// sameLineFindingsBody tiene hallazgos que comparten regla, línea, columna y
// mensaje dentro de una sola copia del texto: dos enlaces javascript: en una
// línea, en una celda, en una lista, y dos imágenes iguales.
const sameLineFindingsBody = "See [a](javascript:x) and [b](javascript:x) here.\n\n" +
	"| c | d |\n|---|---|\n| [e](javascript:z) | [f](javascript:z) |\n\n" +
	"- uno [g](javascript:y)\n- dos [h](javascript:y)\n\n" +
	"Img ![](a b.png) and ![](a b.png).\n"

// Un archivo incluido, una o dos veces, deja los mismos hallazgos que su
// texto escrito en el documento, con la misma cantidad por línea: solo se
// quitan las copias que vienen de otra expansión del mismo archivo.
func TestBuild_IncludedKeepsSameLineFindings(t *testing.T) {
	head := "---\ntitle: D\nmode: flex\n---\n\n# A\n\n"
	headLines := strings.Count(head, "\n")
	// multiset es "regla línea:columna mensaje" de cada hallazgo, con la
	// línea contada desde el comienzo del cuerpo.
	multiset := func(input string, file string, offset int) []string {
		t.Helper()
		out := filepath.Join(t.TempDir(), "r.json")
		runBuild(t, input, "--lint-only", "--report", "json", "--report-out", out)
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Findings []struct {
				RuleID   string `json:"ruleId"`
				Message  string `json:"message"`
				File     string `json:"file"`
				Position struct {
					Line   int `json:"line"`
					Column int `json:"column"`
				} `json:"position"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range report.Findings {
			if f.File != file {
				t.Errorf("finding in %q, want %q: %+v", f.File, file, f)
				continue
			}
			got = append(got, f.RuleID+" "+strconv.Itoa(f.Position.Line-offset)+":"+strconv.Itoa(f.Position.Column)+" "+f.Message)
		}
		sort.Strings(got)
		return got
	}
	// El archivo incluido va junto al documento: los LINK001 de imagen de un
	// archivo incluido desde un subdirectorio no salen, ni en main (la fuente
	// se reescribe al directorio del archivo antes del lint).
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x.doclang"), []byte(sameLineFindingsBody))
	root := filepath.Join(dir, "root.doclang")
	writeFile(t, root, []byte(head+sameLineFindingsBody))
	want := multiset(root, "", headLines)
	if len(want) != 8 {
		t.Fatalf("the body gives %d findings in the document, want 8: %v", len(want), want)
	}
	for name, body := range map[string]string{
		"once":  head + "@include x.doclang\n",
		"twice": head + "@include x.doclang\n\n# B\n\n@include x.doclang\n",
	} {
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(dir, name+".doclang")
			writeFile(t, input, []byte(body))
			got := multiset(input, "x.doclang", 0)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("findings:\n%s\nwant (as in the document):\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}
