// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

func flexDoc(t *testing.T, src string) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	return New(util.NewNoop()).ParseDocument(src, "d.md")
}

// rawDoc es flexDoc sin el normalizador: su regla de mermaid todavía lee
// ````mermaid con la regla anterior (lo cambia una pieza siguiente).
func rawDoc(t *testing.T, src string) (*ast.AST, []diagnostics.Diagnostic) {
	t.Helper()
	p := New(util.NewNoop())
	p.SetNormalization(false)
	return p.ParseDocument(src, "d.md")
}

func codesOf(doc *ast.AST) []*ast.CodeElement {
	var out []*ast.CodeElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if c, ok := n.(*ast.CodeElement); ok {
			out = append(out, c)
		}
		return nil
	})
	return out
}

func groupsOf(doc *ast.AST) []*ast.CodeGroupElement {
	var out []*ast.CodeGroupElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if g, ok := n.(*ast.CodeGroupElement); ok {
			out = append(out, g)
		}
		return nil
	})
	return out
}

func hasHeading(doc *ast.AST, text string) bool {
	found := false
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch h := n.(type) {
		case *ast.TextElement:
			found = found || h.IsRawHTML && strings.Contains(h.Content, ">"+text+"<")
		case *ast.HeadingElement:
			found = found || h.Text == text
		}
		return nil
	})
	for _, b := range doc.ContentBlocks {
		title, _ := b.SectionTitle()
		found = found || b.Title == text || title == text
	}
	return found
}

// rulesAt son los RuleID CODE* de diags en la línea line.
func rulesAt(diags []diagnostics.Diagnostic, line int) []string {
	var out []string
	for _, d := range diags {
		if d.Position.Line == line && strings.HasPrefix(d.RuleID, "CODE") {
			out = append(out, d.RuleID)
		}
	}
	return out
}

func noErrors(t *testing.T, diags []diagnostics.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error: %s", d.String())
		}
	}
}

const fourBacktickBody = "````md\n```go\nx := 1\n```\n````"

// Una cerca de cuatro backticks guarda un ``` como contenido y cierra en su
// ````, con el lenguaje que sigue a los cuatro backticks, y el ## de después
// sigue siendo un encabezado. Antes cerraba en el ``` de adentro, el lenguaje
// era "`md" y la ```` de cierre abría otra cerca hasta el final.
func TestCodeFence_FourBackticksCloseInFlex(t *testing.T) {
	doc, diags := flexDoc(t, "# A\n\n"+fourBacktickBody+"\n\n## B\n\nfin\n")
	noErrors(t, diags)
	codes := codesOf(doc)
	if len(codes) != 1 || codes[0].Content != "```go\nx := 1\n```" || codes[0].Language != "md" {
		t.Fatalf("codes %#v", codes)
	}
	if !hasHeading(doc, "B") {
		t.Error("the ## after the fence must be a heading")
	}
	for _, d := range diags {
		if strings.HasPrefix(d.RuleID, "CODE") {
			t.Errorf("a well-formed four-backtick fence needs no warning: %v", d)
		}
	}
}

// Lo mismo en una presentación flex.
func TestCodeFence_FourBackticksCloseInSlides(t *testing.T) {
	doc, diags := New(util.NewNoop()).Parse("---\ntitle: \"T\"\nmode: flex\n---\n\n# S\n\n"+fourBacktickBody+"\n\nfin\n", "s.slidelang")
	noErrors(t, diags)
	if codes := codesOf(doc); len(codes) != 1 || codes[0].Content != "```go\nx := 1\n```" {
		t.Fatalf("codes %#v", codes)
	}
}

// Dentro de un callout la cerca de cuatro cierra en su ```` y el callout en
// su ":::".
func TestCodeFence_FourBackticksCloseInACallout(t *testing.T) {
	doc, diags := flexDoc(t, "# A\n\n:::note\n"+fourBacktickBody+"\n:::\n\nfin\n")
	noErrors(t, diags)
	for _, d := range diags {
		if d.RuleID == "SPECIAL002" {
			t.Fatalf("the note must close: %v", d)
		}
	}
	if codes := codesOf(doc); len(codes) != 1 || codes[0].Content != "```go\nx := 1\n```" {
		t.Fatalf("codes %#v", codes)
	}
}

// En un code-group la pestaña de cuatro guarda el ``` y la siguiente
// pestaña sigue siendo otra.
func TestCodeFence_FourBackticksCloseInACodeGroup(t *testing.T) {
	doc, diags := flexDoc(t, "# A\n\n:::code-group\n````md [doc]\n```go\nx\n```\n````\n```sh [sh]\necho\n```\n:::\n")
	noErrors(t, diags)
	g := groupsOf(doc)
	if len(g) != 1 || len(g[0].CodeBlocks) != 2 || g[0].CodeBlocks[0].Content != "```go\nx\n```" ||
		g[0].CodeBlocks[0].Language != "md" || g[0].CodeBlocks[0].Label != "doc" || g[0].CodeBlocks[1].Content != "echo" {
		t.Fatalf("code group %#v", g)
	}
}

// En strict: un code-group de un SLIDE y un bloque {typed} de un SLIDE y de
// una SECTION, con su sangría.
func TestCodeFence_FourBackticksCloseInStrict(t *testing.T) {
	const head = "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n"
	doc, diags := New(util.NewNoop()).Parse(head+"  :::code-group\n  ````md [doc]\n  ```go\n  x\n  ```\n  ````\n  ```sh [sh]\n  echo\n  ```\n  :::\n", "s.slidelang")
	noErrors(t, diags)
	g := groupsOf(doc)
	if len(g) != 1 || len(g[0].CodeBlocks) != 2 || g[0].CodeBlocks[0].Content != "```go\nx\n```" {
		t.Fatalf("strict code group %#v", g)
	}

	doc, diags = New(util.NewNoop()).Parse(head+"  :::note{typed}\n  ````md\n  ```go\n  x\n  ```\n  ````\n  :::\n", "s.slidelang")
	noErrors(t, diags)
	if codes := codesOf(doc); len(codes) != 1 || codes[0].Content != "```go\nx\n```" {
		t.Fatalf("strict typed block %#v", codes)
	}

	doc, diags = New(util.NewNoop()).ParseDocument("---\nmode: strict\ntitle: \"T\"\n---\n\nSECTION \"A\"\n\n  :::card{typed}\n  ````md\n  ```go\n  x\n  ```\n  ````\n  :::\n\n  TEXT\n    fin\n", "d.doclang")
	noErrors(t, diags)
	if codes := codesOf(doc); len(codes) != 1 || codes[0].Content != "```go\nx\n```" {
		t.Fatalf("strict document typed block %#v", codes)
	}
}

// Una cerca de tres cierra con una línea de solo backticks, al menos tres
// (CODE002 en esa línea: antes era código).
func TestCodeFence_ThreeClosedByFour(t *testing.T) {
	doc, diags := flexDoc(t, "# A\n\n```\nuno\n````\n\ndespués\n")
	if codes := codesOf(doc); len(codes) != 1 || codes[0].Content != "uno" {
		t.Fatalf("``` must close at ````: %#v", codes)
	}
	if got := strings.Join(rulesAt(diags, 5), ","); got != "CODE002" {
		t.Errorf("want CODE002 at the closing line, got %q (%v)", got, diags)
	}
}

// Una línea que empieza con ``` pero lleva un backtick en el info string no
// abre una cerca: es prosa, salvo que como prosa el resto deje una cerca sin
// cerrar.
func TestCodeFence_BacktickInInfoIsProse(t *testing.T) {
	doc, _ := flexDoc(t, "# A\n\n```js` es un code span suelto\n\ntexto\n")
	if len(codesOf(doc)) != 0 {
		t.Fatalf("no code block expected: %#v", codesOf(doc))
	}
	found := false
	_ = ast.Walk(doc, func(n ast.Node) error {
		if tx, ok := n.(*ast.TextElement); ok && strings.Contains(tx.Content, "es un code span suelto") {
			found = true
		}
		return nil
	})
	if !found {
		t.Error("the line must stay as prose")
	}
}

// Un elemento ya consumido con una cantidad impar de líneas que empiezan con
// ``` no deja al parser "dentro de código": el ## siguiente sigue siendo un
// encabezado.
func TestCodeFence_TrackingDoesNotDriftAfterConsumedElements(t *testing.T) {
	for name, body := range map[string]string{
		"callout with a backtick-info line": ":::info\n```js` uno\n:::\n",
		"prose with backtick info":          "```js` uno\n```js` dos\n```js` tres\n",
		"four-backtick fence":               fourBacktickBody + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			doc, _ := flexDoc(t, "# A\n\n"+body+"\n## B\n\nfin\n")
			if !hasHeading(doc, "B") {
				t.Errorf("the ## after the element must be a heading")
			}
		})
	}
}

// Los diagramas en cerca siguen la misma regla: ````mermaid guarda un ```.
func TestCodeFence_DiagramFencesUseTheSameRule(t *testing.T) {
	var m []*ast.MermaidElement
	doc, _ := rawDoc(t, "# A\n\n````mermaid\ngraph TD\n```\nA-->B\n````\n")
	_ = ast.Walk(doc, func(n ast.Node) error {
		if e, ok := n.(*ast.MermaidElement); ok {
			m = append(m, e)
		}
		return nil
	})
	if len(m) != 1 || !strings.Contains(m[0].Content, "A-->B") {
		t.Errorf("````mermaid must not close at ```: %#v", m)
	}
}

// Una cerca que con la regla de CommonMark quedaría sin cerrar se lee como en
// las versiones anteriores, con un aviso CODE003 en su apertura; el resto del
// documento no cambia de significado.
func TestCodeFence_FallsBackToTheEarlierRule(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		check     func(t *testing.T, doc *ast.AST)
		line      int
		rules     string
	}{
		{"four-backtick code closed by three", "# A\n\n````js\nq\n```\n\n## T\n", func(t *testing.T, doc *ast.AST) {
			if c := codesOf(doc); len(c) != 1 || c[0].Content != "q" {
				t.Errorf("codes %#v", c)
			}
		}, 3, "CODE003"},
		{"four-backtick mermaid closed by three", "# A\n\n````mermaid\ngraph TD\n```\n\n## T\n", func(t *testing.T, doc *ast.AST) {
			var m []*ast.MermaidElement
			_ = ast.Walk(doc, func(n ast.Node) error {
				if e, ok := n.(*ast.MermaidElement); ok {
					m = append(m, e)
				}
				return nil
			})
			if len(m) != 1 || strings.TrimSpace(m[0].Content) != "graph TD" {
				t.Errorf("mermaid %#v", m)
			}
		}, 3, "CODE003"},
		{"four-backtick plantuml closed by three", "# A\n\n````plantuml\nA -> B\n```\n\n## T\n", nil, 3, "CODE003"},
		{"four-backtick chart closed by three", "# A\n\n````chart\n{\"type\":\"bar\"}\n```\n\n## T\n", nil, 3, "CODE003"},
		{"four-backtick map closed by three", "# A\n\n````map\n{\"type\":\"world\"}\n```\n\n## T\n", nil, 3, "CODE003"},
		{"code-group tab closed by three", "# A\n\n:::code-group\n````js [a]\nx\n```\n:::\n\n## T\n", func(t *testing.T, doc *ast.AST) {
			if g := groupsOf(doc); len(g) != 1 || len(g[0].CodeBlocks) != 1 || g[0].CodeBlocks[0].Content != "x" {
				t.Errorf("code-group %#v", g)
			}
		}, 4, "CODE003"},
		{"backtick in the info string", "# A\n\nIntro.\n```ts title=`a.ts`\ncode\n```\n\n## T\n", func(t *testing.T, doc *ast.AST) {
			if c := codesOf(doc); len(c) != 1 || c[0].Content != "code" {
				t.Errorf("codes %#v", c)
			}
		}, 4, "CODE003"},
		{"unclosed in the earlier rule too", "# A\n\n````js\nq\n", nil, 3, "CODE003"},
		// ```md cierra en ```` (CODE002); el ```` siguiente no tiene cierre
		// de cuatro y cierra en ``` (CODE003).
		{"three-backtick fence around four", "# A\n\n```md\n````\ninner\n````\n```\n\n## T\n", nil, 6, "CODE003"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parse := flexDoc
			if strings.Contains(tc.src, "mermaid") {
				parse = rawDoc
			}
			doc, diags := parse(t, tc.src)
			if tc.check != nil {
				tc.check(t, doc)
			}
			if got := strings.Join(rulesAt(diags, tc.line), ","); got != tc.rules {
				t.Errorf("rules at line %d: %q, want %q (all: %v)", tc.line, got, tc.rules, diags)
			}
			if strings.Contains(tc.src, "## T") && !hasHeading(doc, "T") {
				t.Errorf("the ## after the fence must be a heading")
			}
		})
	}
}

// Una cerca de CommonMark que termina en otra línea que en las versiones
// anteriores lleva CODE004 si su línea de cierre era, para esas versiones,
// contenido de una cerca que cerraban, o si esas versiones leían el resto sin
// dejar ninguna cerca abierta; y una línea que antes era una cerca que cerraba
// y ahora es texto también. Si no (una cerca de cuatro que guarda un ```, cuya
// línea de cierre les abría otra cerca hasta el final), la leían mal y no hay
// aviso.
func TestCodeFence_ExtentChangeIsReported(t *testing.T) {
	for name, src := range map[string]string{
		"closing line inside the same earlier fence": "# A\n\n````js\na\n````\nb\n```\n",
		"earlier reading well formed":                "# A\n\n````md\n```\nx\n````\nb\n```\n",
		"backtick-info line that closed":             "# A\n\n```ts title=`a.ts`\ncode\n```\n## Siguiente\n```js\nb\n```\n",
	} {
		_, diags := flexDoc(t, src)
		if got := strings.Join(rulesAt(diags, 3), ","); got != "CODE004" {
			t.Errorf("%s: want CODE004 at line 3, got %q (all %v)", name, got, diags)
		}
	}
	for name, src := range map[string]string{
		"four-backtick fence holding ```":               "# A\n\n" + fourBacktickBody + "\n",
		"closing line opened a fence that never closed": "# A\n\n````js\na\n```\n## T\n````md\nb\n````\n",
		"backtick-info line that never closed":          "# A\n\n```js` uno\n\ntexto\n",
	} {
		_, diags := flexDoc(t, src)
		for _, d := range diags {
			if d.RuleID == "CODE004" {
				t.Errorf("%s: no CODE004 expected: %v", name, d)
			}
		}
	}
}

// Una pestaña busca su cierre solo dentro de su code-group, y un bloque :::
// acota igual las cercas de adentro.
func TestCodeFence_ContainersBoundTheirFences(t *testing.T) {
	doc, diags := flexDoc(t, "# A\n\n:::code-group\n````js [a]\nx\n```\n:::\n\ntexto\n\n:::code-group\n````md [b]\ny\n````\n:::\n")
	if got := strings.Join(rulesAt(diags, 4), ","); got != "CODE003" {
		t.Errorf("want CODE003 at the first tab, got %q (all %v)", got, diags)
	}
	if g := groupsOf(doc); len(g) != 2 || g[0].CodeBlocks[0].Content != "x" || g[1].CodeBlocks[0].Content != "y" {
		t.Errorf("want two groups with their own tabs: %#v", g)
	}

	doc, diags = flexDoc(t, "# A\n\n:::note\n````js\nx\n```\n:::\n\ntexto\n\n````md\ny\n````\n")
	if got := strings.Join(rulesAt(diags, 4), ","); got != "CODE003" {
		t.Errorf("want CODE003 at the fence inside the note, got %q (all %v)", got, diags)
	}
	if c := codesOf(doc); len(c) != 2 || c[0].Content != "x" || c[1].Content != "y" {
		t.Errorf("codes %#v", c)
	}
}

// Un documento que con CommonMark tiene un error que con la regla anterior no
// tiene se lee con la regla anterior, con un aviso CODE005 en la línea del
// error: ningún documento que compilaba deja de compilar.
func TestCodeFence_ErrorsFallBackToTheEarlierRule(t *testing.T) {
	// ```js cierra en ```` con CommonMark y "```x y z" abre una cerca con un
	// info string que es un error; antes las tres líneas eran un solo bloque.
	src := "# A\n\n```js\n````\n```x y z\n```\n"
	doc, diags := flexDoc(t, src)
	noErrors(t, diags)
	if c := codesOf(doc); len(c) != 1 || c[0].Content != "````\n```x y z" {
		t.Fatalf("want the earlier reading: %#v", c)
	}
	if got := strings.Join(rulesAt(diags, 5), ","); got != "CODE005" {
		t.Errorf("want CODE005 at the line of the error, got %q (all %v)", got, diags)
	}
	for _, d := range diags {
		if d.RuleID == "CODE002" {
			t.Errorf("the earlier reading has no CODE002: %v", d)
		}
	}

	// Si las dos lecturas tienen errores, queda la de CommonMark.
	_, diags = flexDoc(t, src+"\n```a b c\n```\n")
	for _, d := range diags {
		if d.RuleID == "CODE005" {
			t.Errorf("no fallback when the earlier reading also has errors: %v", d)
		}
	}
}

// El pre-pase de node-id cierra una cerca de cuatro donde la cierra el parser,
// así que la anotación de después nombra al párrafo siguiente.
func TestCodeFence_NodeIDAfterAFourBacktickFence(t *testing.T) {
	doc, diags := flexDoc(t, "# A\n\n"+fourBacktickBody+"\n\n<!-- node-id: Para -->\nTexto.\n")
	noErrors(t, diags)
	named := false
	_ = ast.Walk(doc, func(n ast.Node) error {
		if tx, ok := n.(*ast.TextElement); ok && tx.Content == "Texto." {
			named = tx.NodeID == "Para"
		}
		return nil
	})
	if !named {
		t.Error("the node-id after the fence must name the paragraph")
	}
}

// El fragmento de contenido acepta una cerca de cuatro cerrada por su ```` y
// una cerrada con la regla anterior (fence.ClosedByMain): están cerradas.
func TestParseMarkdownContent_FenceRule(t *testing.T) {
	for _, src := range []string{fourBacktickBody + "\n\nfin\n", "````js\nq\n```\n\nfin\n"} {
		els, diags := ParseMarkdownContent(src, ContentOptions{AnchorNamespace: "t"})
		for _, d := range diags {
			if d.IsError() {
				t.Fatalf("%q: unexpected error: %v", src, d)
			}
		}
		if len(els) != 2 {
			t.Fatalf("%q: want the code block and the text: %#v", src, els)
		}
	}
}
