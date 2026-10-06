// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"errors"
	"testing"
)

const typedColumnHead = "---\nmode: flex\n---\n\n## One\n\n"

// The newlines at the end of a fenced code body are part of the code, and the
// strict parser reads the blank lines after a CODE as that code. A typed column
// used to trim every newline off the end of each element it wrote, so a code
// element followed by another one in the same column came back shorter.
func TestFlexToStrict_TypedColumnCodeKeepsItsTrailingNewlines(t *testing.T) {
	cases := map[string]string{
		"followed by a text":     typedColumnHead + "::: grid\n::: column typed\n```go\na\n\n\n```\n\nafter\n::: column\nright\n:::\n",
		"followed by a fence":    typedColumnHead + "::: grid\n::: column typed\n```go\na\n\n```\n```js\nb\n```\n::: column\nright\n:::\n",
		"no trailing newline":    typedColumnHead + "::: grid\n::: column typed\n```go\na\n```\n::: column\nright\n:::\n",
		"last element, no blank": typedColumnHead + "::: grid\n::: column\nleft\n::: column typed\n```go\na\n```\n:::\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			want := codeContents(parseSlides(t, src))
			out, got := transpile(t, src)
			g := codeContents(got)
			if len(g) != len(want) {
				t.Fatalf("code blocks %q became %q\n%s", want, g, out)
			}
			for i := range want {
				if g[i] != want[i] {
					t.Errorf("code %q became %q\n%s", want[i], g[i], out)
				}
			}
			again, err := FormatStrict(parseSlides(t, out))
			if err != nil || again != out {
				t.Errorf("a second fmt pass changes the file (err %v)", err)
			}
		})
	}
}

// The body of a typed column ends at its last non-blank line, so newlines that
// end a code body at the end of the column cannot be written back. fmt says so
// by name instead of writing a column whose code is shorter.
func TestFlexToStrict_TypedColumnCodeEndingWithNewlinesIsRefused(t *testing.T) {
	cases := map[string]string{
		"one blank line":     typedColumnHead + "::: grid\n::: column typed\n```go\na\n\n```\n::: column\nright\n:::\n",
		"two blank lines":    typedColumnHead + "::: grid\n::: column typed\n```go\na\n\n\n```\n::: column\nright\n:::\n",
		"in the last column": typedColumnHead + "::: grid\n::: column\nleft\n::: column typed\n```go\na\n\n```\n:::\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := FormatStrict(parseSlides(t, src))
			var uerr *UnsupportedElementError
			if !errors.As(err, &uerr) || uerr.NodeType != "code" {
				t.Fatalf("want an UnsupportedElementError naming code, got %v\n%s", err, out)
			}
		})
	}
}
