// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/transform"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
)

// TestFormatDocumentStrict_FlexToStrict_Corpus is the document counterpart of
// TestFormatStrict_FlexToStrict_Corpus: every flex .doclang under examples/ must
// transpile to strict text that builds to the same AST (the serialized AST minus
// position, endPosition, *Positions and the front matter), or fmt must fail with
// an error that names the element. The pipeline mirrors `doclang build`: include
// expansion, ParseDocument, the built-in transforms, language runs, the linter.
func TestFormatDocumentStrict_FlexToStrict_Corpus(t *testing.T) {
	files := flexDocCorpusFiles(t)
	seen := map[string]bool{}
	for _, f := range files {
		rel := corpusRel(f)
		seen[rel] = true

		t.Run(rel, func(t *testing.T) {
			content, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			want, ok := buildDocASTForComparison(t, string(content), f)
			if !ok {
				return
			}
			doc, diags := parser.New(util.NewNoop()).ParseDocument(string(content), f)
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("flex source does not parse: %s", d.String())
				}
			}
			out, ferr := FormatDocumentStrict(doc)

			problem := ""
			if ferr != nil {
				var uerr *UnsupportedElementError
				switch {
				case !errors.As(ferr, &uerr) || uerr.NodeType == "":
					problem = fmt.Sprintf("fmt failed without naming an element: %v", ferr)
				case acceptedDocFmtRefusals[rel] != "":
					if node := acceptedDocFmtRefusals[rel]; uerr.NodeType != node {
						t.Fatalf("fmt refuses %s naming %q, want %q: %v", rel, uerr.NodeType, node, ferr)
					}
					return
				default:
					problem = fmt.Sprintf("fmt refuses a source strict should represent: %v", ferr)
				}
			} else {
				if node := acceptedDocFmtRefusals[rel]; node != "" {
					t.Fatalf("%s is listed in acceptedDocFmtRefusals (%s) but fmt no longer refuses it: remove it from the list", rel, node)
				}
				got, ok := buildDocASTForComparison(t, out, f)
				if !ok {
					return
				}
				if diffs := diffASTs(want, got); len(diffs) > 0 {
					problem = "fmt changed the AST:\n  " + strings.Join(firstN(diffs, diffLimit()), "\n  ")
				}
			}

			reason, expected := expectedDocFmtGaps[rel]
			switch {
			case problem == "" && expected:
				t.Fatalf("%s is listed in expectedDocFmtGaps (%s) but now round-trips: remove it from the list", rel, reason)
			case problem != "" && expected:
				t.Skipf("known gap (%s): %s", reason, firstLine(problem))
			case problem != "":
				t.Fatalf("%s", problem)
			}
		})
	}
	for _, list := range []map[string]string{expectedDocFmtGaps, acceptedDocFmtRefusals} {
		for rel := range list {
			if !seen[rel] {
				t.Errorf("%s is listed but is not a flex source under examples/", rel)
			}
		}
	}
}

// acceptedDocFmtRefusals is the permanent list: flex documents that strict
// cannot say, with the node type the error must name.
//
// grid: a flex column that holds an embedded element with its own <<end>> keeps
// that line in its raw Content, and strict reads it as the end of the grid.
var acceptedDocFmtRefusals = map[string]string{
	"advanced_elements_test.doclang": "grid",
}

// expectedDocFmtGaps lists the flex documents that do not round-trip yet, with a
// short reason. It may only shrink.
var expectedDocFmtGaps = map[string]string{}

func flexDocCorpusFiles(t *testing.T) []string {
	t.Helper()
	root := "../../examples"
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".doclang" {
			return err
		}
		content, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		doc, diags := parser.New(util.NewNoop()).ParseDocument(string(content), path)
		for _, d := range diags {
			if d.IsError() {
				return nil
			}
		}
		if doc == nil || doc.FrontMatter == nil || doc.FrontMatter.Mode == "strict" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no flex .doclang fixtures found under examples/")
	}
	return files
}

func buildDocASTForComparison(t *testing.T, content, path string) (interface{}, bool) {
	t.Helper()
	abs, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	expanded, err := include.Expand(content, abs, os.ReadFile)
	if err != nil {
		t.Fatalf("include.Expand: %v", err)
	}
	doc, perr := parser.New(util.NewNoop()).ParseDocument(expanded, path)
	for _, d := range perr {
		if d.IsError() {
			t.Errorf("does not parse: %s", d.String())
			return nil, false
		}
	}
	doc, err = transform.RunBuiltins(doc, []transform.Transform{xref.Transform})
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	renderer.PopulateLangRuns(doc, doc.FrontMatter.BuildVariables())
	linter.New().LintUnfiltered(doc)
	return stripForComparison(astToTree(t, doc)), true
}
