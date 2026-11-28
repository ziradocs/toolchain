// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"

	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
)

// titleNoFrontMatter y titleFrontMatter son las dos formas de fuente que
// --title tiene que cubrir: Markdown sin frontmatter (el título sale del
// primer #) y un frontmatter con su propio `title:`.
const (
	titleNoFrontMatter = "# Setup\n\nIntro paragraph.\n\n## Install\n\nRun it.\n\n# Usage\n\nUse it.\n"
	titleFrontMatter   = "---\ntitle: \"Fuente\"\nauthor: Equipo\n---\n\n# Setup\n\nIntro paragraph.\n"
)

// runTitleBuild corre `doclang build` sobre fixture con --format format y
// los argumentos extra, y devuelve el archivo generado y el error.
func runTitleBuild(t *testing.T, fixture, format string, extraArgs ...string) ([]byte, error) {
	t.Helper()
	dir := t.TempDir()
	docPath := filepath.Join(dir, "doc.doclang")
	if err := os.WriteFile(docPath, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "out")
	cmd := NewBuildCommand(nil, nil, nil, nil, nil)
	cmd.SetArgs(append([]string{docPath, "--format", format, "--output", outDir, "--log-level", "error"}, extraArgs...))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		return nil, err
	}
	ext := format
	if format == "markdown" {
		ext = "md"
	}
	data, err := os.ReadFile(filepath.Join(outDir, "doc."+ext))
	if err != nil {
		t.Fatalf("reading the output: %v", err)
	}
	return data, nil
}

func mustTitleBuild(t *testing.T, fixture, format string, extraArgs ...string) []byte {
	t.Helper()
	data, err := runTitleBuild(t, fixture, format, extraArgs...)
	if err != nil {
		t.Fatalf("build %s %v: %v", format, extraArgs, err)
	}
	return data
}

func docxPart(t *testing.T, data []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		part, err := io.ReadAll(rc)
		if closeErr := rc.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			t.Fatal(err)
		}
		// Cada parte tiene que seguir siendo XML bien formado.
		dec := xml.NewDecoder(bytes.NewReader(part))
		for {
			if _, err := dec.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("%s is not well-formed XML: %v", name, err)
			}
		}
		return string(part)
	}
	t.Fatalf("%s not in the docx", name)
	return ""
}

var (
	dcTitleRe = regexp.MustCompile(`<dc:title>([^<]*)</dc:title>`)
	wTextRe   = regexp.MustCompile(`<w:t(?: [^>]*)?>([^<]*)</w:t>`)
)

func dcTitle(t *testing.T, data []byte) string {
	t.Helper()
	m := dcTitleRe.FindStringSubmatch(docxPart(t, data, "docProps/core.xml"))
	if m == nil {
		return ""
	}
	return m[1]
}

// docxTexts son los textos de los runs de word/document.xml, en orden.
func docxTexts(t *testing.T, data []byte) []string {
	t.Helper()
	var texts []string
	for _, m := range wTextRe.FindAllStringSubmatch(docxPart(t, data, "word/document.xml"), -1) {
		texts = append(texts, m[1])
	}
	return texts
}

var wParagraphRe = regexp.MustCompile(`(?s)<w:p(?: [^>]*)?>.*?</w:p>|<w:p(?: [^>]*)?/>`)

// docxParagraphs es el texto de cada párrafo del cuerpo de word/document.xml,
// en orden ("" para un párrafo sin texto).
func docxParagraphs(t *testing.T, data []byte) []string {
	t.Helper()
	var paragraphs []string
	for _, p := range wParagraphRe.FindAllString(docxPart(t, data, "word/document.xml"), -1) {
		var text strings.Builder
		for _, m := range wTextRe.FindAllStringSubmatch(p, -1) {
			text.WriteString(m[1])
		}
		paragraphs = append(paragraphs, text.String())
	}
	return paragraphs
}

// docxTitlePartsText concatena el texto de los runs de las partes cuyo nombre empieza
// con prefix ("word/header", "word/footer").
func docxTitlePartsText(t *testing.T, data []byte, prefix string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, prefix) && strings.HasSuffix(f.Name, ".xml") {
			for _, m := range wTextRe.FindAllStringSubmatch(docxPart(t, data, f.Name), -1) {
				text.WriteString(m[1])
				text.WriteString("|")
			}
		}
	}
	return text.String()
}

func TestBuildTitle_DOCX(t *testing.T) {
	t.Run("without front matter", func(t *testing.T) {
		plain := mustTitleBuild(t, titleNoFrontMatter, "docx")
		titled := mustTitleBuild(t, titleNoFrontMatter, "docx", "--title", "Guía Q3")
		if got := dcTitle(t, plain); got != "Setup" {
			t.Fatalf("fixture premise: dc:title = %q, want Setup", got)
		}
		if got := dcTitle(t, titled); got != "Guía Q3" {
			t.Errorf("dc:title = %q, want Guía Q3", got)
		}
		// Antes del cuerpo entran dos párrafos, los mismos que agrega un
		// `title:` de frontmatter: el de título y uno vacío que lo separa. El
		// cuerpo queda igual, sin TOC ni numeración.
		want := append([]string{"Guía Q3", ""}, docxParagraphs(t, plain)...)
		if got := docxParagraphs(t, titled); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("document.xml paragraphs:\n%q\nwant\n%q", got, want)
		}
	})
	t.Run("over a front matter title", func(t *testing.T) {
		plain := mustTitleBuild(t, titleFrontMatter, "docx")
		titled := mustTitleBuild(t, titleFrontMatter, "docx", "--title", "Guía Q3")
		if got := dcTitle(t, titled); got != "Guía Q3" {
			t.Errorf("dc:title = %q, want Guía Q3", got)
		}
		want := docxTexts(t, plain)
		if len(want) == 0 || want[0] != "Fuente" {
			t.Fatalf("fixture premise: the title paragraph says Fuente: %q", want)
		}
		want[0] = "Guía Q3"
		if got := docxTexts(t, titled); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("document.xml texts:\n%q\nwant\n%q", got, want)
		}
	})
	t.Run("same text as the first heading is not repeated", func(t *testing.T) {
		titled := mustTitleBuild(t, titleNoFrontMatter, "docx", "--title", "setup")
		if n := strings.Count(strings.Join(docxTexts(t, titled), "|"), "Setup"); n != 1 {
			t.Errorf("the first heading appears %d times, want 1 (it takes the title's place)", n)
		}
	})
	t.Run("{{title}} in the header and footer", func(t *testing.T) {
		const fixture = "---\ntitle: \"Fuente\"\nheader:\n  enabled: true\n  text: \"H {{title}}\"\nfooter:\n  enabled: true\n  text: \"F {{title}}\"\n---\n\n# Setup\n\nCuerpo {{title}}.\n"
		plain := mustTitleBuild(t, fixture, "docx")
		titled := mustTitleBuild(t, fixture, "docx", "--title", "Guía Q3")
		if got := docxTitlePartsText(t, plain, "word/header") + docxTitlePartsText(t, plain, "word/footer"); got != "H Fuente|F Fuente|" {
			t.Fatalf("fixture premise: header and footer = %q", got)
		}
		if got := docxTitlePartsText(t, titled, "word/header") + docxTitlePartsText(t, titled, "word/footer"); got != "H Guía Q3|F Guía Q3|" {
			t.Errorf("header and footer = %q, want them with --title", got)
		}
		// DOCX no sustituye {{title}} en los párrafos del cuerpo, con o sin la
		// bandera: --title no cambia eso.
		if body := strings.Join(docxParagraphs(t, titled), "|"); !strings.Contains(body, "Cuerpo {{title}}.") {
			t.Errorf("body paragraphs: %q", body)
		}
	})
	t.Run("markup stays text", func(t *testing.T) {
		titled := mustTitleBuild(t, titleNoFrontMatter, "docx", "--title", `</dc:title><x/> "q" & 'a'`)
		if got := dcTitle(t, titled); got != `&lt;/dc:title&gt;&lt;x/&gt; &#34;q&#34; &amp; &#39;a&#39;` {
			t.Errorf("dc:title = %q", got)
		}
	})
}

func TestBuildTitle_HTML(t *testing.T) {
	titled := string(mustTitleBuild(t, titleFrontMatter, "html", "--title", "Uno\n---\nmode: strict"))
	if !strings.Contains(titled, "<title>Uno --- mode: strict</title>") {
		t.Errorf("a multi-line --title is one line in <title>: %s", regexp.MustCompile(`<title>[^<]*</title>`).FindString(titled))
	}
}

func TestBuildTitle_UsageErrors(t *testing.T) {
	invisible := string([]rune{0xFEFF, 0x200B, 0x202E})
	for _, value := range []string{"", "   ", "\n\t", invisible, strings.Repeat("a", 501)} {
		if _, err := runTitleBuild(t, titleNoFrontMatter, "html", "--title", value); err == nil || !strings.Contains(err.Error(), "--title") {
			t.Errorf("--title %q: error = %v, want a --title usage error", value, err)
		}
	}
	if _, err := runTitleBuild(t, titleNoFrontMatter, "html", "--title", strings.Repeat("é", 500)); err != nil {
		t.Errorf("--title of 500 characters: %v", err)
	}
	for _, format := range []string{"markdown", "md"} {
		if _, err := runTitleBuild(t, titleNoFrontMatter, format, "--title", "T"); err == nil || !strings.Contains(err.Error(), "--format "+format) {
			t.Errorf("--title with %s: error = %v, want it refused", format, err)
		}
	}
	// Sin --title, Markdown sigue como siempre.
	if _, err := runTitleBuild(t, titleNoFrontMatter, "markdown"); err != nil {
		t.Errorf("markdown without --title: %v", err)
	}
}

// Un --title inválido es un error de uso también con --lint-only, y falla
// antes de leer la fuente: no escribe --report-out ni crea --output.
func TestBuildTitle_UsageErrorsBeforeLintOnly(t *testing.T) {
	for _, args := range [][]string{
		{"--format", "html", "--title", ""},
		{"--format", "markdown", "--title", "X"},
		{"--format", "md", "--title", "X"},
	} {
		dir := t.TempDir()
		docPath := filepath.Join(dir, "doc.doclang")
		if err := os.WriteFile(docPath, []byte(titleNoFrontMatter), 0o644); err != nil {
			t.Fatal(err)
		}
		outDir := filepath.Join(dir, "out")
		reportPath := filepath.Join(dir, "report.json")
		cmd := NewBuildCommand(nil, nil, nil, nil, nil)
		cmd.SetArgs(append([]string{docPath, "--lint-only", "--report", "json", "--report-out", reportPath, "--output", outDir, "--log-level", "error"}, args...))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		if err := cmd.Execute(); err == nil {
			t.Errorf("%v --lint-only: no error", args)
		}
		for _, path := range []string{reportPath, outDir} {
			if _, err := os.Stat(path); err == nil {
				t.Errorf("%v: %s was written before the usage error", args, filepath.Base(path))
			}
		}
	}
	// Sin --title, --lint-only con reporte sigue escribiéndolo.
	dir := t.TempDir()
	docPath := filepath.Join(dir, "doc.doclang")
	if err := os.WriteFile(docPath, []byte(titleNoFrontMatter), 0o644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "report.json")
	cmd := NewBuildCommand(nil, nil, nil, nil, nil)
	cmd.SetArgs([]string{docPath, "--lint-only", "--report", "json", "--report-out", reportPath, "--log-level", "error"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--lint-only without --title: %v", err)
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Errorf("fixture premise: --lint-only writes --report-out: %v", err)
	}
}

var pdfTitleRe = regexp.MustCompile(`/Title\s*(?:\(((?:[^\\)]|\\.)*)\)|<([0-9A-Fa-f]+)>)`)

// pdfTitle lee el /Title del diccionario Info del PDF: un string literal o,
// fuera de ASCII, UTF-16BE en hexadecimal con BOM (lo que escribe Chromium).
func pdfTitle(t *testing.T, data []byte) string {
	t.Helper()
	m := pdfTitleRe.FindSubmatch(data)
	if m == nil {
		t.Fatal("the PDF has no /Title")
	}
	if m[2] == nil {
		return string(m[1])
	}
	raw, err := hex.DecodeString(string(m[2]))
	if err != nil || len(raw) < 2 || raw[0] != 0xFE || raw[1] != 0xFF {
		t.Fatalf("unexpected /Title encoding: %s", m[2])
	}
	units := make([]uint16, 0, len(raw)/2)
	for i := 2; i+1 < len(raw); i += 2 {
		units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
	}
	return string(utf16.Decode(units))
}

func TestBuildTitle_PDF(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Chromium")
	}
	if _, err := chromium.NewChromiumManager("", false, &renderer.ChromiumLoggerAdapter{Logger: util.NewNoop()}).Detect(); err != nil {
		t.Skipf("Chromium not available: %v", err)
	}
	for _, fixture := range []string{titleNoFrontMatter, titleFrontMatter} {
		plain := mustTitleBuild(t, fixture, "pdf")
		titled := mustTitleBuild(t, fixture, "pdf", "--title", "Guía Q3")
		if got := pdfTitle(t, plain); got == "Guía Q3" {
			t.Fatalf("fixture premise: the PDF is already titled %q", got)
		}
		if got := pdfTitle(t, titled); got != "Guía Q3" {
			t.Errorf("/Title = %q, want Guía Q3", got)
		}
	}
}
