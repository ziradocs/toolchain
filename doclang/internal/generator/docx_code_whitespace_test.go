// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/xml"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestCodeLineSegments(t *testing.T) {
	cases := map[string][]string{
		"":                {""},
		"x := 1":          {"x := 1"},
		"\tfmt.Println()": {"    fmt.Println()"},
		"  x  = 1":        {"  x", "  =", " 1"},
		"a\tb":            {"a", "   b"},
		"trail  ":         {"trail", "  "},
	}
	for in, want := range cases {
		got := codeLineSegments(in)
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("codeLineSegments(%q) = %q, want %q", in, got, want)
		}
		for _, seg := range got {
			// Un segmento con espacios repetidos tiene que empezar en espacio,
			// que es cuando docxgo le pone xml:space="preserve".
			if strings.Contains(seg, "  ") && !strings.HasPrefix(seg, " ") {
				t.Errorf("segmento %q de %q tiene espacios repetidos sin empezar en espacio", seg, in)
			}
		}
	}
}

var (
	paragraphRe = regexp.MustCompile(`(?s)<w:p\b.*?</w:p>`)
	textRe      = regexp.MustCompile(`(?s)<w:t( xml:space="preserve")?>(.*?)</w:t>`)
)

// TestDOCX_CodeKeepsTabsAndSpaces: un bloque con tabs y espacios mezclados
// conserva su sangría y sus espacios repetidos en document.xml. Antes el tab
// llegaba crudo a <w:t> sin preserve y Word/LibreOffice lo descartaban, así
// que `fmt.Println` quedaba pegado al margen.
func TestDOCX_CodeKeepsTabsAndSpaces(t *testing.T) {
	code := "func main() {\n\tfmt.Println(\"a  b\")\n\t\tif  x {\n  y := 2\t// fin\n}"
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "Code"
	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "Section"
	block.Elements = append(block.Elements, ast.NewCodeElement(diagnostics.NewPosition(3, 1), "go", code))
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out := filepath.Join(t.TempDir(), "code.docx")
	if err := New(newTestLogger()).Generate(doc, out, GeneratorOptions{Format: "docx", AssetRoot: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	docXML := docxDocumentXML(t, out)

	var lines []string
	for _, p := range paragraphRe.FindAllString(docXML, -1) {
		var b strings.Builder
		for _, m := range textRe.FindAllStringSubmatch(p, -1) {
			text := m[2]
			if strings.HasPrefix(text, " ") && m[1] == "" {
				t.Errorf("run %q empieza en espacio sin xml:space=\"preserve\"", text)
			}
			if strings.Contains(text, "  ") && m[1] == "" {
				t.Errorf("run %q tiene espacios repetidos sin xml:space=\"preserve\"", text)
			}
			var unescaped string
			if err := xml.Unmarshal([]byte("<x>"+text+"</x>"), &unescaped); err != nil {
				t.Fatal(err)
			}
			b.WriteString(unescaped)
		}
		lines = append(lines, b.String())
	}
	joined := "\n" + strings.Join(lines, "\n") + "\n"
	for _, want := range []string{
		"func main() {",
		"    fmt.Println(\"a  b\")",
		"        if  x {",
		"  y := 2    // fin", // "  y := 2" mide 8 columnas: el tab llega a la 12
		"}",
	} {
		if !strings.Contains(joined, "\n"+want+"\n") {
			t.Errorf("falta la línea %q en document.xml; párrafos: %q", want, lines)
		}
	}
	if strings.Contains(docXML, "\t") {
		t.Error("document.xml todavía tiene un tab crudo")
	}
}
