// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// fmt es idempotente sobre todo examples/: formatear lo ya formateado no lo
// cambia. Un archivo que fmt no puede escribir (UnsupportedElementError) se
// salta: eso lo cubren las pruebas de corpus de cada dialecto.
func TestFormat_IdempotentOverExamples(t *testing.T) {
	root := "../../examples"
	n := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := filepath.Ext(path)
		if ext != ".slidelang" && ext != ".doclang" && ext != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(data)
		slides := ext == ".slidelang"
		parse := func(s string) (*ast.AST, bool) {
			p := parser.New(util.NewNoop())
			var doc *ast.AST
			var diags []interface{ IsError() bool }
			if slides {
				d, ds := p.Parse(s, path)
				doc = d
				for _, x := range ds {
					diags = append(diags, x)
				}
			} else {
				d, ds := p.ParseDocument(s, path)
				doc = d
				for _, x := range ds {
					diags = append(diags, x)
				}
			}
			for _, x := range diags {
				if x.IsError() {
					return doc, false
				}
			}
			return doc, doc != nil
		}
		format := func(doc *ast.AST) (string, error) {
			if slides {
				return FormatStrict(doc)
			}
			if doc.FrontMatter != nil && doc.FrontMatter.Mode == "strict" {
				return FormatDocumentStrict(doc)
			}
			return FormatDocument(doc)
		}
		doc, ok := parse(src)
		if !ok {
			return nil
		}
		once, err := format(doc)
		if err != nil {
			return nil
		}
		doc2, ok := parse(once)
		if !ok {
			return nil
		}
		twice, err := format(doc2)
		if err != nil {
			t.Errorf("%s: the formatted source can no longer be formatted: %v", path, err)
			return nil
		}
		n++
		if once != twice {
			t.Errorf("%s: fmt is not idempotent", path)
			a, b := strings.Split(once, "\n"), strings.Split(twice, "\n")
			for i := 0; i < len(a) && i < len(b); i++ {
				if a[i] != b[i] {
					t.Logf("first difference at line %d:\n once:  %q\n twice: %q", i+1, a[i], b[i])
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no example could be formatted")
	}
	t.Logf("%d examples formatted twice", n)
}
