// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"errors"
	"strings"
	"testing"
)

// codeBodies are bodies a flex fence can hold. The strict CODE block takes the
// whitespace that all of its non-blank lines have in common as structure, so a
// body that has some needs `CODE{verbatim}`; every other body is written as a
// plain CODE, exactly as before. Either way the code comes back unchanged.
var codeBodies = map[string]struct {
	body     string
	verbatim bool
}{
	"plain":                         {"func a() {\n\tb()\n}\n", false},
	"blank line inside":             {"a\n\nb\n", false},
	"leading blank lines":           {"\n\na\n  b\n", false},
	"later line more indented":      {"a\n    b\n      c\n", false},
	"spaces then a tab":             {"  a\n\tb\n", false},
	"whitespace-only line":          {"a\n  \nb\n", false},
	"whitespace-only tab line":      {"a\n\t\nb\n", false},
	"first line deeper than later":  {"    x\n  y\n", true},
	"every line indented":           {"  a\n  b\n", true},
	"every line starts with a tab":  {"\tfoo()\n\tbar()\n", true},
	"first line indented, one more": {"  a\nb\n", false},
	"common indent, deeper later":   {"  a\n    b\n", true},
	"only whitespace-only lines":    {"  \n", true},
}

// A fence inside a typed block does not go through the CODE parser: the block
// keeps its body as flex text and TestFlexToStrict_TypedBlockFenceShapes covers
// the indentation it restores.
func fenceDeck(body string) map[string]string {
	fence := "```go\n" + body + "```\n"
	return map[string]string{
		"slide":        "---\nmode: flex\n---\n\n## One\n\n" + fence,
		"between":      "---\nmode: flex\n---\n\n## One\n\ntext\n\n" + fence + "\nafter\n",
		"typed column": "---\nmode: flex\n---\n\n## One\n\n::: grid\n::: column typed\n" + fence + "::: column\nright\n:::\n",
	}
}

func TestFlexToStrict_CodeBodyIndentRoundTrips(t *testing.T) {
	for name, c := range codeBodies {
		for where, src := range fenceDeck(c.body) {
			t.Run(name+"/"+where, func(t *testing.T) {
				want := codeContents(parseSlides(t, src))
				if len(want) != 1 {
					t.Fatalf("source has %d code blocks, want 1", len(want))
				}
				out, err := FormatStrict(parseSlides(t, src))
				if name == "only whitespace-only lines" && where == "typed column" {
					// The body of a typed column ends at its last non-blank line,
					// so a code whose last line is whitespace cannot end one.
					var uerr *UnsupportedElementError
					if !errors.As(err, &uerr) || uerr.NodeType != "code" {
						t.Fatalf("want an UnsupportedElementError naming code, got %v", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("FormatStrict: %v", err)
				}
				if got := strings.Contains(out, "CODE{verbatim}"); got != c.verbatim {
					t.Errorf("CODE{verbatim} written = %v, want %v\n%s", got, c.verbatim, strings.ReplaceAll(out, "\t", "<TAB>"))
				}
				got := codeContents(parseSlides(t, out))
				if len(got) != 1 || got[0] != want[0] {
					t.Errorf("code %q became %q\n%s", want[0], got, strings.ReplaceAll(out, "\t", "<TAB>"))
				}
				again, err := FormatStrict(parseSlides(t, out))
				if err != nil || again != out {
					t.Errorf("a second fmt pass changes the file (err %v)", err)
				}
			})
		}
	}
}
