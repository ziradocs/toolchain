// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// rebasedDoc expande main (con sub2/k.doclang = part) en un directorio
// temporal, lo parsea y reescribe las imágenes de los incluidos, como Prepare.
func rebasedDoc(t *testing.T, main, part string) (*ast.AST, destRewrites) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub2", "k.doclang"), []byte(part), 0o644); err != nil {
		t.Fatal(err)
	}
	exp, err := include.ExpandWithOptions(main, dir, os.ReadFile, include.ExpandOptions{KeepIncludesInCode: true})
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := parser.New(util.NewNoop()).ParseDocument(exp.Text, filepath.Join(dir, "main.doclang"))
	return doc, rebaseIncludedAssets(doc, strings.Split(exp.Text, "\n"), exp.Origins, dir)
}

// linkFindingsUnderRebase recupera los LINK001 que el rebase escondió, deja
// el AST exactamente como estaba (con las rutas reescritas que ven los
// generadores) y no corre nada si el rebase no reescribió ningún destino.
func TestLinkFindingsUnderRebase(t *testing.T) {
	head := "---\ntitle: D\nmode: flex\n---\n\n# A\n\n"
	doc, rewrites := rebasedDoc(t, head+"@include sub2/k.doclang\n", "Para ![](a b.png) fin.\n\n- uno ![](a b.png)\n\n> cita ![](a b.png) y ![](<c d.png>)\n")
	n := 0
	for _, q := range rewrites.queues {
		n += len(q)
	}
	if n != 4 || !rewrites.relevant {
		t.Fatalf("want the 4 rewritten occurrences recorded and marked relevant, got %+v", rewrites)
	}
	before, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	delta := linkFindingsUnderRebase(doc, rewrites)
	after, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("the AST changed")
	}
	if !strings.Contains(string(after), `\u003csub2/a b.png\u003e`) {
		t.Error("the AST lost the rewritten destinations")
	}
	if len(delta.hidden) != 3 || len(delta.added) != 0 {
		t.Errorf("hidden %v, added %v; want the 3 spaced-destination warnings and nothing added", delta.hidden, delta.added)
	}
	for _, d := range delta.hidden {
		if !strings.Contains(d.Message, `"a b.png"`) {
			t.Errorf("the warning does not name the path as written: %q", d.Message)
		}
	}

	same, none := rebasedDoc(t, head+"Para ![](a b.png) fin.\n", "")
	if len(none.queues) != 0 || none.relevant {
		t.Fatalf("no include, but rewrites %+v", none)
	}
	if d := linkFindingsUnderRebase(same, none); len(d.hidden)+len(d.added) != 0 {
		t.Errorf("without rewrites the delta is %+v", d)
	}
}

// Rutas sin espacios ni bloque {…} reescritas desde otro directorio se
// anotan, pero no hacen correr la regla.
func TestLinkFindingsUnderRebaseGateWithoutSpacedPaths(t *testing.T) {
	doc, rewrites := rebasedDoc(t, "---\ntitle: D\nmode: flex\n---\n\n# A\n\n@include sub2/k.doclang\n", "Uno ![](a.png) y ![](<b c.png>).\n")
	if len(rewrites.queues) != 2 || rewrites.relevant {
		t.Fatalf("want 2 recorded, not relevant: %+v", rewrites)
	}
	if d := linkFindingsUnderRebase(doc, rewrites); len(d.hidden)+len(d.added) != 0 {
		t.Errorf("delta %+v, want none", d)
	}
}

// Un destino nuevo con ocurrencias escritas de dos formas no se deshace si
// en el AST quedan menos ocurrencias que las reescritas (un filtro borró
// una): no se sabe cuál quedó.
func TestRewriteUndoerSkipsAmbiguousMissingOccurrence(t *testing.T) {
	doc, rewrites := rebasedDoc(t, "---\ntitle: D\nmode: flex\n---\n\n# A\n\n@include sub2/k.doclang\n", "Uno ![](a b.png) y ![](<a b.png>) fin.\n")
	var text *ast.TextElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if e, ok := n.(*ast.TextElement); ok && strings.Contains(e.Content, "Uno") {
			text = e
		}
		return nil
	})
	if text == nil {
		t.Fatal("no paragraph")
	}
	// Lo que haría un filtro que borra la primera imagen.
	text.Content = strings.Replace(text.Content, "![](<sub2/a b.png>) y ", "", 1)
	if d := linkFindingsUnderRebase(doc, rewrites); len(d.hidden)+len(d.added) != 0 {
		t.Errorf("delta %+v, want none: the remaining occurrence cannot be told apart", d)
	}
}
