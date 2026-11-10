// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

var codeContentPattern = regexp.MustCompile(`(?s)<code[^>]*>(.*?)</code>`)

func codeContents(html string) []string {
	var out []string
	for _, m := range codeContentPattern.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out
}

// outsideCode es html sin el contenido de sus <code>…</code>.
func outsideCode(html string) string {
	return codeContentPattern.ReplaceAllString(html, "<code>")
}

// oracleFenceLines es la regla de las cercas dentro de la prosa, escrita aparte
// de fenceLines: en un texto de varias líneas, una línea recortada de tres o
// más backticks cuyo info string no tiene backticks abre una cerca; la cierra
// una línea de solo backticks, al menos tantos; sin cierre llega al final.
func oracleFenceLines(text string) []bool {
	lines := strings.Split(text, "\n")
	marks := make([]bool, len(lines))
	if len(lines) < 2 {
		return marks
	}
	open := 0
	for i, line := range lines {
		t := strings.TrimSpace(line)
		run := 0
		for run < len(t) && t[run] == '`' {
			run++
		}
		switch {
		case open > 0:
			marks[i] = true
			if run == len(t) && run >= open {
				open = 0
			}
		case run >= 3 && !strings.ContainsRune(t[run:], '`'):
			marks[i] = true
			open = run
		}
	}
	return marks
}

// literalProsePaths son los caminos públicos que pintan prosa: el render de
// cada elemento y su *HTML en el AST. multiline dice si el camino admite
// cercas (un encabezado es una sola línea).
var literalProsePaths = []struct {
	name      string
	multiline bool
	render    func(text string, vars map[string]interface{}, literal bool) string
}{
	{"text", true, func(text string, vars map[string]interface{}, literal bool) string {
		return RenderElementToHTML(ast.NewTextElement(diagnostics.Position{}, text), vars, &RenderContext{LiteralCode: literal})
	}},
	{"quote", true, func(text string, vars map[string]interface{}, literal bool) string {
		return RenderElementToHTML(ast.NewQuoteElement(diagnostics.Position{}, text), vars, &RenderContext{LiteralCode: literal})
	}},
	{"block content", true, func(text string, vars map[string]interface{}, literal bool) string {
		return RenderElementToHTML(&ast.SpecialBlockElement{BlockType: "note", Content: text}, vars, &RenderContext{LiteralCode: literal})
	}},
	{"block nested text", true, func(text string, vars map[string]interface{}, literal bool) string {
		block := &ast.SpecialBlockElement{BlockType: "note", Content: text, Elements: []ast.Element{ast.NewTextElement(diagnostics.Position{}, text)}}
		return RenderElementToHTML(block, vars, &RenderContext{LiteralCode: literal})
	}},
	{"heading", false, func(text string, vars map[string]interface{}, literal bool) string {
		return RenderElementToHTML(&ast.HeadingElement{Level: 2, Text: text, Anchor: "a"}, vars, &RenderContext{LiteralCode: literal})
	}},
	{"populate", true, func(text string, vars map[string]interface{}, literal bool) string {
		pos := diagnostics.Position{}
		textEl := ast.NewTextElement(pos, text)
		quote := ast.NewQuoteElement(pos, text)
		block := &ast.SpecialBlockElement{BlockType: "note", Content: text}
		section := ast.NewContentBlock(pos, "content")
		section.Elements = []ast.Element{textEl, quote, block}
		PopulateInlineHTMLWithOptions(&ast.AST{ContentBlocks: []ast.ContentBlock{*section}}, vars, InlineHTMLOptions{LiteralCode: literal})
		return textEl.ContentHTML + "\x00" + quote.ContentHTML + "\x00" + block.ContentHTML
	}},
	{"populate heading", false, func(text string, vars map[string]interface{}, literal bool) string {
		heading := &ast.HeadingElement{Level: 2, Text: text, Anchor: "a"}
		PopulateInlineHTMLWithOptions(newTestDoc(heading), vars, InlineHTMLOptions{LiteralCode: literal})
		return heading.TextHTML
	}},
}

// FuzzLiteralCodeProse prueba, en cada camino público de prosa, que con la
// opción de código literal:
//
//   - el contenido de cada <code> es el mismo que sale sin sustituir nada (si
//     la sustitución emparejara los code spans con otra unidad que el
//     renderer, por ejemplo por línea donde el renderer junta un item con su
//     continuación, una {{x}} saldría sustituida dentro de un <code>);
//   - fuera de los <code> el HTML es el que sale sustituyendo todo salvo en
//     las líneas de cerca (oracleFenceLines). El HTML de la prosa no pinta la
//     cerca como <code>, así que esas líneas no se pueden reconocer en la
//     salida: se comparan contra un texto de referencia donde su {{x}} ya no
//     es una variable con valor.
func FuzzLiteralCodeProse(f *testing.F) {
	for _, seed := range []string{
		"a {{x}} `b {{x}}` c",
		"x ` y\n`{{x}}`",
		"`uno\n{{x}}` dos",
		"``a ` {{x}}`` {{x}}",
		"texto {{x}}\n```yaml\na: {{x}}\n```\n`{{x}}` y {{x}}",
		"- item `{{x}}`\n- otro {{x}}",
		"\\`{{x}}` `{{x}}\\`",
		"```\n{{x}}",
		"* `{{x}}\n `",
		"- corre `make {{x}}\n  --verbose` ahora",
		"1. a `{{x}}\n   2. b` {{x}}",
		"```code``` inline {{x}}\nluego {{x}}",
		"````md\n```\n{{x}}\n```\n````\n{{x}}",
	} {
		f.Add(seed)
	}
	// Valores con un backtick y con un salto de línea: con la opción no pueden
	// cambiar los code spans de la fuente (hallazgo 6 del review de #497).
	vars := map[string]interface{}{"x": "VAL", "t": "a`b", "n": "p\nq"}
	// La referencia sustituye los backticks y saltos de línea de los valores
	// por las marcas que elige proseMarkdownVariables cuando ni el texto ni
	// los valores las traen (U+E000, U+E001) y las deshace en el HTML: un valor
	// no empareja con la fuente ni parte sus líneas.
	refMarks := valueMarks{tick: "\ue000", newline: "\ue001"}
	refVars := map[string]interface{}{"x": "VAL", "t": "a" + refMarks.tick + "b", "n": "p" + refMarks.newline + "q"}
	f.Add("Pre {{t}} mid `{{x}}` post {{x}}")
	f.Add("- a {{n}} `{{x}}`\n  `{{t}}` {{t}}")
	f.Fuzz(func(t *testing.T, text string) {
		if strings.Contains(text, "zzfence") || strings.ContainsAny(text, refMarks.tick+refMarks.newline) {
			return
		}
		for _, path := range literalProsePaths {
			if !path.multiline && strings.Contains(text, "\n") {
				continue
			}
			literal := path.render(text, vars, true)
			if got, want := codeContents(literal), codeContents(path.render(text, nil, true)); strings.Join(got, "\x00") != strings.Join(want, "\x00") {
				t.Fatalf("%s: <code> contents differ for %q:\n literal %q\n    none %q", path.name, text, got, want)
			}
			reference := text
			if path.multiline {
				lines := strings.Split(text, "\n")
				for i, inFence := range oracleFenceLines(text) {
					if inFence {
						for _, name := range []string{"x", "t", "n"} {
							lines[i] = strings.ReplaceAll(lines[i], "{{"+name+"}}", "{{zzfence"+name+"}}")
						}
					}
				}
				reference = strings.Join(lines, "\n")
			}
			want := refMarks.html(outsideCode(path.render(reference, refVars, false)))
			for _, name := range []string{"x", "t", "n"} {
				want = strings.ReplaceAll(want, "{{zzfence"+name+"}}", "{{"+name+"}}")
			}
			if got := outsideCode(literal); got != want {
				t.Fatalf("%s: HTML outside <code> differs for %q:\n literal %q\n    want %q", path.name, text, got, want)
			}
		}
	})
}
