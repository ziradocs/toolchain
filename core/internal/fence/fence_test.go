// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package fence

import (
	"strings"
	"testing"
)

func TestOpenAndCloses(t *testing.T) {
	for _, tc := range []struct {
		line string
		open int
	}{
		{"```", 3}, {"```go", 3}, {"````md", 4}, {"``` go title=\"a\"", 3},
		{"``", 0}, {"```js`", 0}, {"``` a ` b", 0}, {"texto", 0}, {"", 0},
	} {
		if got := Open(tc.line); got != tc.open {
			t.Errorf("Open(%q) = %d, want %d", tc.line, got, tc.open)
		}
	}
	for _, tc := range []struct {
		line string
		open int
		want bool
	}{
		{"```", 3, true}, {"````", 3, true}, {"`````", 4, true},
		{"```", 4, false}, {"``` x", 3, false}, {"``", 3, false}, {"", 3, false}, {"```", 0, false},
	} {
		if got := Closes(tc.line, tc.open); got != tc.want {
			t.Errorf("Closes(%q, %d) = %v, want %v", tc.line, tc.open, got, tc.want)
		}
	}
	for content, want := range map[string]string{
		"x := 1":        "```",
		"```go\nx\n```": "````",
		"````\n```":     "`````",
		"a ``` b":       "```",
	} {
		if got := For(content); got != want {
			t.Errorf("For(%q) = %q, want %q", content, got, want)
		}
	}
}

// At lee cada cerca con CommonMark si da un resultado bien formado y, si no,
// con la regla de las versiones anteriores.
func TestAt(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		ok        bool
		end       int
		reading   Reading
		mainEnd   int
	}{
		{"closed by as many", "```\na\n```", true, 2, CommonMark, 2},
		{"closed by more", "```\na\n````\nb\n```", true, 2, CommonMark, 4},
		{"four keeps three as content", "````md\n```\nx\n````", true, 3, CommonMark, 1},
		{"four closed only by three", "````js\nq\n```\nfin", true, 2, ClosedByMain, 2},
		{"never closed", "````js\nq", true, -1, Unclosed, -1},
		{"backtick info, as text a fence stays open", "```ts title=`a.ts`\ncode\n```\nfin", true, 2, OpenedByMain, 2},
		{"backtick info, as text nothing stays open", "```js` x\ntexto", false, 0, 0, 0},
		{"backtick info, later fence closes", "```js`\na\n```\nb\n```", false, 0, 0, 0},
		{"not a fence", "texto\n```", false, 0, 0, 0},
	} {
		lines := strings.Split(tc.src, "\n")
		b, ok := At(lines, 0)
		if ok != tc.ok {
			t.Errorf("%s: ok %v, want %v", tc.name, ok, tc.ok)
			continue
		}
		if ok && (b.End != tc.end || b.Reading != tc.reading || b.MainEnd != tc.mainEnd) {
			t.Errorf("%s: %+v, want end %d reading %d mainEnd %d", tc.name, b, tc.end, tc.reading, tc.mainEnd)
		}
	}
	lines := strings.Split("a\n````js\nq\n```\nb\n```\nc", "\n")
	if got := End(lines, 1); got != 3 {
		t.Errorf("End = %d, want 3", got)
	}
	if got := End(lines, 0); got != -1 {
		t.Errorf("End of a non-fence = %d, want -1", got)
	}
	marks := Lines(lines, 0)
	want := []bool{false, true, true, true, false, true, true}
	for i := range want {
		if marks[i] != want[i] {
			t.Errorf("Lines[%d] = %v, want %v (%v)", i, marks[i], want[i], marks)
		}
	}
}
