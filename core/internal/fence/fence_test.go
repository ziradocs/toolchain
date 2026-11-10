// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package fence

import "testing"

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
