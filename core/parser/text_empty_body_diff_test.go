// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/elements"
	"go.ziradocs.com/core/v2/util"
)

// Propiedad diferencial del TEXT strict vacío: con un TEXT sin cuerpo al
// frente de un elemento (o de una secuencia de elementos, prosa suelta y otros
// TEXT vacíos), escrito a su misma sangría, el AST y los diagnósticos son los
// de leer esas mismas líneas como el cuerpo sangrado del TEXT, que es como
// main las lee. Lo único que se agrega es un TEXT002 en la línea TEXT, y solo
// cuando la primera línea del cuerpo abre un elemento en la forma que acepta.
//
// El generador es el de strict_no_silent_drop_test.go (29 plantillas, 8
// contextos y líneas sueltas), más tres aperturas propias, cabeceras con
// texto de más o una letra pegada, elementos sin su cierre, secuencias de 2 a
// 4 elementos con prosa intercalada y TEXT vacíos encadenados.
//
// Con TEXT_DIFF_DUMP=<archivo> escribe, por caso, el AST y los diagnósticos
// sin TEXT001 ni TEXT002: corriendo la misma generación sobre origin/main, los
// dos archivos tienen que ser idénticos (así se comparó este cambio).

var textDiffExtraTemplates = []noDropTemplate{
	{name: "CODE_ARGS", keyword: "CODE", lines: []noDropLine{
		{"CODE python main.py extra30", 0, tok("python", "main.py", "extra30"), false}, {"print(alfa30)", 2, tok("print(alfa30)"), false}}},
	{name: "IMAGE_SRC", keyword: "IMAGE", lines: []noDropLine{
		{"IMAGE foto31.png", 0, tok("foto31.png"), false}, {`caption: "beta31"`, 2, tok("beta31"), false}}},
	{name: "SPECIAL_BARE", closer: true, lines: []noDropLine{
		{":::warning", 0, nil, false}, {"alfa32 beta32", 0, tok("alfa32", "beta32"), false}, {":::", 0, nil, false}}},
}

func textDiffTemplates(slide bool) []noDropTemplate {
	var out []noDropTemplate
	for _, tpl := range append(append([]noDropTemplate(nil), noDropTemplates...), textDiffExtraTemplates...) {
		if !tpl.slideOnly || slide {
			out = append(out, tpl)
		}
	}
	return out
}

// textDiffTemplateLines son las líneas de tpl con las sueltas y la cabecera
// alterada, como en buildNoDropCase; dropCloser quita el cierre.
func textDiffTemplateLines(tpl noDropTemplate, strays []noDropStray, header noDropHeader, dropCloser bool) []noDropLine {
	lines := append([]noDropLine(nil), tpl.lines...)
	if header.suffix != "" {
		lines[0].text += header.sep + " " + header.suffix
	}
	if header.glued {
		lines[0].text = tpl.keyword + "Z" + strings.TrimPrefix(lines[0].text, tpl.keyword)
	}
	end := len(lines)
	if tpl.closer {
		end--
		if dropCloser {
			lines = lines[:end]
		}
	}
	sort.SliceStable(strays, func(a, b int) bool { return strays[a].at > strays[b].at })
	for _, s := range strays {
		at := min(1+s.at%end, end)
		l := s.line
		l.indent = s.indent
		lines = append(lines[:at], append([]noDropLine{l}, lines[at:]...)...)
		end++
	}
	return lines
}

func textDiffStrays(rng *rand.Rand, most int) []noDropStray {
	var strays []noDropStray
	for n := rng.Intn(most + 1); n > 0; n-- {
		strays = append(strays, noDropStray{
			at:     rng.Intn(16),
			indent: []int{2, 2, 4, 6, 0, -2}[rng.Intn(6)],
			line:   noDropStrayLine(rng),
		})
	}
	return strays
}

// textDiffSource escribe lines en ctx. reindent suma dos espacios a cada línea
// del cuerpo que main le da al primer TEXT (las que siguen con sangría, desde
// la primera con texto, hasta la primera con menos), para leerlas como su
// cuerpo sangrado.
func textDiffSource(ctx noDropContext, lines []noDropLine, reindent bool) string {
	var out []string
	for _, l := range lines {
		out = append(out, strings.Repeat(" ", max(ctx.indent+l.indent, 0))+l.text)
	}
	if reindent {
		expected := -1
		for i := 1; i < len(out); i++ {
			trimmed := strings.TrimSpace(out[i])
			if trimmed == "" {
				continue
			}
			indent := elements.CalculateIndentLevel(out[i])
			if expected == -1 {
				if indent == 0 {
					break
				}
				expected = indent
			}
			if indent < expected {
				break
			}
			out[i] = "  " + out[i]
		}
	}
	return ctx.prefix + strings.Join(out, "\n") + "\n" + ctx.suffix
}

type textDiffResult struct {
	AST   json.RawMessage
	Diags []string
}

func textDiffParse(t *testing.T, ctx noDropContext, src string) (textDiffResult, []diagnostics.Diagnostic) {
	t.Helper()
	p := New(util.NewNoop())
	var tree *ast.AST
	var diags []diagnostics.Diagnostic
	if ctx.slide {
		tree, diags = p.Parse(src, "diff.slidelang")
	} else {
		tree, diags = p.ParseDocument(src, "diff.doclang")
	}
	raw, err := json.Marshal(tree.ContentBlocks)
	if err != nil {
		t.Fatal(err)
	}
	r := textDiffResult{AST: raw}
	for _, d := range diags {
		if d.RuleID == "TEXT001" || d.RuleID == "TEXT002" {
			continue
		}
		end := 0
		if d.EndPosition != nil {
			end = d.EndPosition.Line
		}
		r.Diags = append(r.Diags, fmt.Sprintf("%d-%d %s %s %s", d.Position.Line, end, d.Severity, d.RuleID, d.Message))
	}
	return r, diags
}

type textDiffCase struct {
	name  string
	ctx   noDropContext
	lines []noDropLine
}

// textDiffCases genera los casos: un elemento solo tras el TEXT vacío,
// secuencias, y cada plantilla seguida de una línea suelta a su sangría.
func textDiffCases() []textDiffCase {
	empty := noDropLine{text: "TEXT"}
	var cases []textDiffCase
	for _, ctx := range noDropContexts {
		tpls := textDiffTemplates(ctx.slide)
		for _, tpl := range tpls {
			for seed := int64(0); seed <= noDropSeeds; seed++ {
				var strays []noDropStray
				var header noDropHeader
				dropCloser := false
				if seed > 0 {
					rng := rand.New(rand.NewSource(seed))
					strays = textDiffStrays(rng, 3)
					header = noDropHeaderFor(rng, tpl)
					dropCloser = tpl.closer && rng.Intn(5) == 0
				}
				cases = append(cases, textDiffCase{fmt.Sprintf("%s/%s/%d", ctx.name, tpl.name, seed), ctx,
					append([]noDropLine{empty}, textDiffTemplateLines(tpl, strays, header, dropCloser)...)})
			}
			for _, after := range []string{"nota suelta", "Importante: leer", "---"} {
				cases = append(cases, textDiffCase{fmt.Sprintf("%s/%s/then %q", ctx.name, tpl.name, after), ctx,
					append(append([]noDropLine{empty}, textDiffTemplateLines(tpl, nil, noDropHeader{}, false)...), noDropLine{text: after})})
			}
		}
		for seed := int64(1); seed <= 400; seed++ {
			rng := rand.New(rand.NewSource(seed * 7919))
			lines := []noDropLine{empty}
			if rng.Intn(4) == 0 {
				lines = append(lines, empty)
			}
			for k := 2 + rng.Intn(3); k > 0; k-- {
				tpl := tpls[rng.Intn(len(tpls))]
				header := noDropHeader{}
				if rng.Intn(3) == 0 {
					header = noDropHeaderFor(rng, tpl)
				}
				lines = append(lines, textDiffTemplateLines(tpl, textDiffStrays(rng, 1), header, tpl.closer && rng.Intn(8) == 0)...)
				switch rng.Intn(6) {
				case 0, 1, 2:
					lines = append(lines, noDropLine{text: noDropStrays[rng.Intn(len(noDropStrays))]})
				case 3:
					lines = append(lines, empty)
				}
			}
			cases = append(cases, textDiffCase{fmt.Sprintf("%s/sequence/%d", ctx.name, seed), ctx, lines})
		}
	}
	return cases
}

func TestStrictText_EmptyBodyDiffersOnlyByWarning(t *testing.T) {
	var dump *json.Encoder
	if path := os.Getenv("TEXT_DIFF_DUMP"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		}()
		dump = json.NewEncoder(f)
	}
	warned := 0
	cases := textDiffCases()
	for _, c := range cases {
		src := textDiffSource(c.ctx, c.lines, false)
		got, diags := textDiffParse(t, c.ctx, src)
		if dump != nil {
			if err := dump.Encode(map[string]any{"case": c.name, "result": got}); err != nil {
				t.Fatal(err)
			}
		}
		// Un callout tipado lee su cuerpo como flex: ahí no hay TEXT strict,
		// y sangrar el cuerpo cambiaría la lectura flex.
		if strings.HasPrefix(c.ctx.name, "typed callout") {
			continue
		}
		want, wantDiags := textDiffParse(t, c.ctx, textDiffSource(c.ctx, c.lines, true))
		if string(got.AST) != string(want.AST) || strings.Join(got.Diags, "\n") != strings.Join(want.Diags, "\n") {
			t.Errorf("%s: the empty TEXT does not read as main\n--- source ---\n%s--- got ---\n%s\n%s\n--- want ---\n%s\n%s",
				c.name, src, got.AST, strings.Join(got.Diags, "\n"), want.AST, strings.Join(want.Diags, "\n"))
			continue
		}
		textLine := strings.Count(c.ctx.prefix, "\n") + 1
		count := func(ds []diagnostics.Diagnostic, rule string) int {
			n := 0
			for _, d := range ds {
				if d.RuleID == rule && d.Position.Line == textLine {
					n++
				}
			}
			return n
		}
		first := ""
		srcLines := strings.Split(src, "\n")
		for _, l := range srcLines[textLine:] {
			if l = strings.TrimSpace(l); l != "" {
				first = l
				break
			}
		}
		// SECTION "…" es encabezado en el cuerpo de un SLIDE, no en una
		// columna tipada (ahí es un error).
		slideHeadings := c.ctx.slide && !strings.HasPrefix(c.ctx.name, "typed column")
		opens := first != "" && elements.CalculateIndentLevel(srcLines[textLine]) == c.ctx.indent &&
			elements.OpensElementAfterEmptyText(&elements.ParseContext{SlideHeadings: slideHeadings}, first)
		// Con el cuerpo sangrado el TEXT nunca avisa; al frente, avisa
		// exactamente cuando su primera línea abre un elemento.
		if count(wantDiags, "TEXT002") != 0 {
			t.Errorf("%s: the indented body warns: %v", c.name, wantDiags)
		}
		if n := count(diags, "TEXT002"); (n == 1) != opens || n > 1 {
			t.Errorf("%s: %d TEXT002 for first line %q (opens an element: %v)\n%s", c.name, n, first, opens, src)
		}
		if count(diags, "TEXT002") > 0 && count(diags, "TEXT001") > 0 {
			t.Errorf("%s: TEXT001 and TEXT002 on the same TEXT", c.name)
		}
		if opens {
			warned++
		}
	}
	if warned == 0 {
		t.Fatal("no case warned: the generator does not exercise TEXT002")
	}
	t.Logf("%d cases, %d with TEXT002", len(cases), warned)
}
