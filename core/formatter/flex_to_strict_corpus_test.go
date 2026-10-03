// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/transform"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
)

// TestFormatStrict_FlexToStrict_Corpus is the guard behind the contract that
// `slidelang fmt` documents for a flex source: the transpiled strict text
// must build to the SAME AST as the original, or fmt must fail with an error
// that names the element. Silently changing the document is the one outcome
// that is never acceptable.
//
// The comparison is deliberately the same one an operator makes with
// `slidelang build --format json` on both files: the full serialized AST,
// including every rendered *HTML field, minus the keys that only describe
// where something sat in the source text (position, endPosition, *Positions)
// and the front matter (fmt rewrites `mode:` on purpose). It is much stricter
// than normalizeForComparison, which blanks the *HTML fields and so cannot see
// a heading that lost its <strong>.
//
// The pipeline mirrors build: include expansion, parse, the built-in
// transforms, language-run derivation, then the linter (whose rules may
// rewrite the AST).
func TestFormatStrict_FlexToStrict_Corpus(t *testing.T) {
	files := flexCorpusFiles(t)

	seen := map[string]bool{}
	for _, f := range files {
		rel := corpusRel(f)
		seen[rel] = true

		t.Run(rel, func(t *testing.T) {
			content, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			want, ok := buildASTForComparison(t, string(content), f)
			if !ok {
				return
			}

			// Same entry point as `slidelang fmt`: parse (with the
			// normalizer), then FormatStrict.
			doc, diags := parser.New(util.NewNoop()).Parse(string(content), f)
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("flex source does not parse: %s", d.String())
				}
			}
			out, ferr := FormatStrict(doc)

			problem := ""
			if ferr != nil {
				var uerr *UnsupportedElementError
				switch {
				case !errors.As(ferr, &uerr) || uerr.NodeType == "":
					problem = fmt.Sprintf("fmt failed without naming an element: %v", ferr)
				case acceptedFmtRefusals[rel] != "":
					if node := acceptedFmtRefusals[rel]; uerr.NodeType != node {
						t.Fatalf("fmt refuses %s naming %q, want %q: %v", rel, uerr.NodeType, node, ferr)
					}
					return
				default:
					// Naming the element satisfies fmt's contract, but a
					// refusal on a corpus source is still a gap unless
					// acceptedFmtRefusals says strict cannot say it.
					problem = fmt.Sprintf("fmt refuses a source strict should represent: %v", ferr)
				}
			} else {
				if node := acceptedFmtRefusals[rel]; node != "" {
					t.Fatalf("%s is listed in acceptedFmtRefusals (%s) but fmt no longer refuses it: remove it from the list", rel, node)
				}
				got, ok := buildASTForComparison(t, out, f)
				if !ok {
					return
				}
				if diffs := diffASTs(want, got); len(diffs) > 0 {
					problem = "fmt changed the AST:\n  " + strings.Join(firstN(diffs, diffLimit()), "\n  ")
				}
			}

			reason, expected := expectedFmtGaps[rel]
			switch {
			case problem == "" && expected:
				t.Fatalf("%s is listed in expectedFmtGaps (%s) but now round-trips: remove it from the list", rel, reason)
			case problem != "" && expected:
				t.Skipf("known gap (%s): %s", reason, firstLine(problem))
			case problem != "":
				t.Fatalf("%s", problem)
			}
		})
	}

	for _, list := range []map[string]string{expectedFmtGaps, acceptedFmtRefusals} {
		for rel := range list {
			if !seen[rel] {
				t.Errorf("%s is listed but is not a flex source under examples/", rel)
			}
		}
	}
}

// acceptedFmtRefusals lists the flex sources fmt is allowed to refuse for good,
// because strict has no way to say what they contain, keyed by path under
// examples/ with the node type the error must name. Unlike expectedFmtGaps it
// is not meant to reach empty.
//
// special_block: a flex ::: block keeps its nested headings, fenced code,
// tables and images as Elements next to the raw Content. Strict recognizes those
// only in its own syntax, never inside the raw lines of a block, so there is no
// strict text for the block that builds the same; see checkNestedBlockElements.
var acceptedFmtRefusals = map[string]string{
	"01_title_and_content/01.2_text_formatting_flex.slidelang":        "special_block",
	"01_title_and_content/01.4_special_blocks_flex.slidelang":         "special_block",
	"01_title_and_content/01.6_ui_elements_flex.slidelang":            "special_block",
	"01_title_and_content/01.7_advanced_inline_syntax_flex.slidelang": "special_block",
	"01_title_and_content/01_title_and_content_flex.slidelang":        "special_block",
}

// expectedFmtGaps lists the flex sources whose fmt output is known not to
// round-trip yet, keyed by path under examples/ with a short reason. It is empty
// now, and a gap that shows up has to be fixed or recorded here. The list may
// only shrink: a source that starts round-tripping while still listed fails the
// test above, so nobody forgets to delete it. A refusal stops the comparison, so
// a source listed for one may be hiding other defects that show up once it gets
// past the refusal.
var expectedFmtGaps = map[string]string{}

func flexCorpusFiles(t *testing.T) []string {
	t.Helper()
	root := "../../examples"
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".slidelang" {
			return err
		}
		content, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		doc, diags := parser.New(util.NewNoop()).Parse(string(content), path)
		for _, d := range diags {
			if d.IsError() {
				return nil // deliberately broken fixture, outside fmt's scope
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
		t.Fatal("no flex .slidelang fixtures found under examples/")
	}
	return files
}

func corpusRel(path string) string {
	rel, err := filepath.Rel("../../examples", path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// buildASTForComparison runs the build pipeline up to the point where
// `--format json` serializes, and returns the JSON tree without the
// positional keys and the front matter.
func buildASTForComparison(t *testing.T, content, path string) (interface{}, bool) {
	t.Helper()
	abs, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	expanded, err := include.Expand(content, abs, os.ReadFile)
	if err != nil {
		t.Fatalf("include.Expand: %v", err)
	}
	doc, diags := parser.New(util.NewNoop()).Parse(expanded, path)
	for _, d := range diags {
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
	// The linter is part of the comparison on purpose: some of its rules
	// rewrite the AST (LastSlideClosingRule reclassifies a final untitled
	// slide), and `build --format json` serializes the result.
	linter.New().LintUnfiltered(doc)
	return stripForComparison(astToTree(t, doc)), true
}

func astToTree(t *testing.T, doc *ast.AST) interface{} {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var tree interface{}
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return tree
}

func stripForComparison(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			if k == "position" || k == "endPosition" || k == "frontMatter" || strings.HasSuffix(k, "Positions") {
				continue
			}
			out[k] = stripForComparison(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, val := range x {
			out[i] = stripForComparison(val)
		}
		return out
	}
	return v
}

func diffASTs(a, b interface{}) []string {
	var out []string
	diffTree(a, b, "$", &out)
	return out
}

func diffTree(a, b interface{}, path string, out *[]string) {
	switch x := a.(type) {
	case map[string]interface{}:
		y, ok := b.(map[string]interface{})
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: type differs", path))
			return
		}
		keys := map[string]bool{}
		for k := range x {
			keys[k] = true
		}
		for k := range y {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			xv, inX := x[k]
			yv, inY := y[k]
			switch {
			case !inY:
				*out = append(*out, fmt.Sprintf("%s.%s only in flex: %s", path, k, short(xv)))
			case !inX:
				*out = append(*out, fmt.Sprintf("%s.%s only in strict: %s", path, k, short(yv)))
			default:
				diffTree(xv, yv, path+"."+k, out)
			}
		}
	case []interface{}:
		y, ok := b.([]interface{})
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: type differs", path))
			return
		}
		if len(x) != len(y) {
			*out = append(*out, fmt.Sprintf("%s length %d (flex) vs %d (strict)", path, len(x), len(y)))
		}
		for i := 0; i < len(x) && i < len(y); i++ {
			diffTree(x[i], y[i], fmt.Sprintf("%s[%d]", path, i), out)
		}
	default:
		if !reflect.DeepEqual(a, b) {
			*out = append(*out, fmt.Sprintf("%s: %s (flex) vs %s (strict)", path, short(a), short(b)))
		}
	}
}

func short(v interface{}) string {
	b, _ := json.Marshal(v)
	s := string(b)
	if len(s) > 160 {
		s = s[:160] + "..."
	}
	return s
}

// diffLimit caps how many differences a failure prints. FMT_DIFF_LIMIT raises
// it when triaging a whole group of failures at once.
func diffLimit() int {
	if v, err := strconv.Atoi(os.Getenv("FMT_DIFF_LIMIT")); err == nil && v > 0 {
		return v
	}
	return 8
}

func firstN(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return append(s[:n:n], fmt.Sprintf("... and %d more", len(s)-n))
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
