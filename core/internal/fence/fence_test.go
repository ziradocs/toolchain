// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package fence

import (
	"strings"
	"testing"
	"time"
)

func TestOpenAndCloses(t *testing.T) {
	for _, tc := range []struct {
		line string
		open int
	}{
		{"```", 3}, {"```go", 3}, {"````md", 4}, {"``` go title=\"a\"", 3}, {"`````", 5},
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
	for line, want := range map[string]string{"````md": "md", "``` go x": "go x", "```": ""} {
		if got := Info(line); got != want {
			t.Errorf("Info(%q) = %q, want %q", line, got, want)
		}
	}
}

// Differs marca las líneas que las dos reglas pueden leer distinto.
func TestDiffers(t *testing.T) {
	for line, want := range map[string]bool{
		"```": false, "```js": false, "``` js x": false, "texto": false, "``": false,
		"````": true, "````md": true, "`````": true, "```js`": true, "```ts title=`a.ts`": true,
	} {
		if got := Differs(line); got != want {
			t.Errorf("Differs(%q) = %v, want %v", line, got, want)
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
	marks := Lines(lines, 0)
	want := []bool{false, true, true, true, false, true, true}
	for i := range want {
		if marks[i] != want[i] {
			t.Errorf("Lines[%d] = %v, want %v (%v)", i, marks[i], want[i], marks)
		}
	}
}

// AtWithin busca el cierre solo dentro del contenedor: una cerca de cuatro
// que solo cerraría con CommonMark después de limit cierra en su ```.
func TestAtWithinIsBounded(t *testing.T) {
	lines := strings.Split("````js\nx\n```\n:::\n````", "\n")
	if b, _ := AtWithin(lines, 0, 3); b.End != 2 || b.Reading != ClosedByMain {
		t.Errorf("bounded: %+v", b)
	}
	if b, _ := At(lines, 0); b.End != 4 || b.Reading != CommonMark {
		t.Errorf("unbounded: %+v", b)
	}
}

// En un code-group toda línea que empieza con ``` abre una pestaña.
func TestTabWithin(t *testing.T) {
	lines := strings.Split("```js` [a]\nx\n```\n```py [b]\ny\n```", "\n")
	if _, ok := AtWithin(lines, 0, len(lines)); ok {
		t.Fatal("outside a group the backtick-info line is text here")
	}
	b, ok := TabWithin(lines, 0, len(lines))
	if !ok || b.End != 2 || b.Reading != OpenedByMain {
		t.Errorf("tab: %+v %v", b, ok)
	}
}

// EarlierAt es la regla de las versiones anteriores, con el cierre de cada
// parser.
func TestEarlierAt(t *testing.T) {
	lines := strings.Split("````md\n```\nx\n````\n```js` y", "\n")
	exact := func(t string) bool { return t == "```" }
	diagram := func(t string) bool { return t == "```" || t == "````" }
	if b, ok := EarlierAt(lines, 0, exact); !ok || b.End != 1 || b.Ticks != 3 || b.Reading != Earlier {
		t.Errorf("code: %+v", b)
	}
	if b, ok := EarlierAt(lines, 3, exact); !ok || b.End != -1 || b.Consumed() != 2 {
		t.Errorf("unclosed: %+v", b)
	}
	if b, _ := EarlierAt(lines, 2, diagram); b.Start != 0 && b.End != 3 {
		t.Errorf("diagram: %+v", b)
	}
	if _, ok := EarlierAt(lines, 2, exact); ok {
		t.Error("x does not open a fence")
	}
}

func TestMainLeavesUnclosed(t *testing.T) {
	for src, want := range map[string]bool{
		"```\na\n```":                  false,
		"````md\n```\nx\n````":         true,
		"````md\n```\nx\n````\nb\n```": false,
		"texto":                        false,
	} {
		if got := MainLeavesUnclosed(strings.Split(src, "\n"), 0, 99); got != want {
			t.Errorf("MainLeavesUnclosed(%q) = %v, want %v", src, got, want)
		}
	}
}

// Un contenedor termina en el primer ":::" en el que todo lo que se abrió
// adentro cerró: un ":::" dentro de una cerca es código.
func TestContainerEnd(t *testing.T) {
	lines := strings.Split(":::note\n````md\n:::\n````\n:::\ntexto", "\n")
	if end, closed := ContainerEnd(lines, 0, len(lines), nil); end != 4 || !closed {
		t.Errorf("end %d closed %v, want 4 true", end, closed)
	}
	lines = strings.Split(":::code-group\n````js [a]\nx\n```\n:::\n````md [b]\ny\n````\n:::", "\n")
	if end, closed := ContainerEnd(lines, 0, len(lines), nil); end != 4 || !closed {
		t.Errorf("code-group: end %d closed %v, want 4 true", end, closed)
	}
}

// Muchos bloques ::: anidados sin cerrar, con cercas y líneas con un backtick
// en el info string, no hacen el recorrido exponencial: con el cuádruple de
// líneas tarda a lo más unas veinte veces más (cuadrático en el peor caso, no
// exponencial).
func TestWalkScales(t *testing.T) {
	doc := func(n int) []string {
		var lines []string
		for i := 0; i < n; i++ {
			lines = append(lines, ":::note", "```js` x", "texto")
		}
		return lines
	}
	measure := func(n int) time.Duration {
		lines := doc(n)
		start := time.Now()
		Walk(lines, 0)
		return time.Since(start)
	}
	small, large := measure(200), measure(800)
	if small > 0 && large > 40*small && large > 200*time.Millisecond {
		t.Errorf("Walk: %v with n=200 and %v with 4n", small, large)
	}
}
