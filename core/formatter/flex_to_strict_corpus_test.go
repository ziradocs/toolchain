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
// is not meant to reach empty. It is empty today: every refusal on the corpus
// is still a gap.
var acceptedFmtRefusals = map[string]string{}

// expectedFmtGaps lists the flex sources whose fmt output is known not to
// round-trip yet, keyed by path under examples/. Each entry carries the defect
// it belongs to. The list may only shrink: a source that starts round-tripping
// while still listed fails the test above, so nobody forgets to delete it.
// Defects, by the letter each entry carries:
//
//	B  the nested elements of :::card, :::columns, :::tabs and :::accordion
//	   are lost
//	C  a closing slide that the linter infers (last slide, no layout, no
//	   title) comes back as an explicit "SLIDE content"
//	D  a heading in a section loses its <strong>/<em>/<code>
//	E  the content of a code block gains a trailing newline
//	F  a combo chart loses seriesAxes
//	G  a quote containing an empty ">" line cannot be written in strict, so
//	   fmt refuses (correctly naming the element) instead of transpiling
//	H  the body of a @notes directive swallows the elements that follow it,
//	   so the slide ends up with fewer elements
//
// A refusal stops the comparison, so a source listed only as G may be hiding
// other defects that show up once it gets past the refusal.
var expectedFmtGaps = map[string]string{
	"01_title_and_content/01.2_text_formatting_flex.slidelang":               "G",
	"01_title_and_content/01.3_lists_and_tables_flex.slidelang":              "G",
	"01_title_and_content/01.4_special_blocks_flex.slidelang":                "G",
	"01_title_and_content/01.5_layouts_and_images_flex.slidelang":            "G",
	"01_title_and_content/01.6_ui_elements_flex.slidelang":                   "B",
	"01_title_and_content/01.7_advanced_inline_syntax_flex.slidelang":        "G",
	"01_title_and_content/01_title_and_content_flex.slidelang":               "B",
	"01_title_and_content/content_platform_launch_flex.slidelang":            "H",
	"02_diagrams_and_charts/02.2_technical_diagrams_flex.slidelang":          "G",
	"02_diagrams_and_charts/analytics_dashboard_presentation_flex.slidelang": "G",
	"gallery/02_flex_mode_essentials.slidelang":                              "G",
	"gallery/07_special_blocks_and_checklists.slidelang":                     "G",
	"gallery/10_startup_pitch_deck.slidelang":                                "G",
}

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
