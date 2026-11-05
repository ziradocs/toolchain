// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// oracleRewriteImageDestinations es el recorrido de MarkdownImageDestinations
// (y del rebase de @include) tal como estaba en v2.49.2: el regex de apertura
// anclado en cada "![" y renderer.ScanLinkDestination desde cada apertura.
func oracleRewriteImageDestinations(md string, rewrite func(string) string) string {
	open := regexp.MustCompile(`^!\[[^\]]*\]\(`)
	if !strings.Contains(md, "![") {
		return md
	}
	var b strings.Builder
	i := 0
	for i < len(md) {
		switch {
		case md[i] == '`':
			end := MarkdownCodeSpanEnd(md, i)
			b.WriteString(md[i:end])
			i = end
		case strings.HasPrefix(md[i:], "!["):
			m := open.FindStringIndex(md[i:])
			if m == nil {
				b.WriteString("![")
				i += 2
				continue
			}
			openEnd := i + m[1]
			dest, n, ok := renderer.ScanLinkDestination(md[openEnd:])
			if !ok {
				b.WriteString(md[i:openEnd])
				i = openEnd
				continue
			}
			b.WriteString(md[i:openEnd])
			b.WriteString(rewrite(dest))
			b.WriteString(md[openEnd+len(dest) : openEnd+n])
			i = openEnd + n
		default:
			b.WriteByte(md[i])
			i++
		}
	}
	return b.String()
}

func TestRewriteInlineImageDestinationsMatchesOld(t *testing.T) {
	mark := func(d string) string { return "{" + d + "}" }
	r := rand.New(rand.NewSource(5))
	for i := 0; i < 20000; i++ {
		s := randomLinkText(r, 1+r.Intn(30))
		if got, want := RewriteInlineImageDestinations(s, mark), oracleRewriteImageDestinations(s, mark); got != want {
			t.Fatalf("%q:\n nuevo %q\n viejo %q", s, got, want)
		}
	}
	for _, s := range linkScannerCorpus {
		if got, want := RewriteInlineImageDestinations(s, mark), oracleRewriteImageDestinations(s, mark); got != want {
			t.Fatalf("%q:\n nuevo %q\n viejo %q", s, got, want)
		}
	}
}

func TestRewriteInlineImageDestinationsScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("mide tiempos")
	}
	inputs := map[string]func(n int) string{
		"barra y dos backticks": func(n int) string { return strings.Repeat("![a](b) \\``a", n) },
		"largos crecientes": func(n int) string {
			var b strings.Builder
			for i := 1; b.Len() < 6*n; i++ {
				b.WriteString("![a](b)" + strings.Repeat("`", i%300+1))
			}
			return b.String()
		},
	}
	for name, gen := range scaleInputs {
		inputs[name] = gen
	}
	for name, gen := range inputs {
		assertLinear(t, name, func(s string) { MarkdownImageDestinations(s) }, gen)
	}
}

// backtickRuns.spanEnd es MarkdownCodeSpanEnd en cada backtick.
func TestBacktickRunsMatchesCodeSpanEnd(t *testing.T) {
	r := rand.New(rand.NewSource(9))
	alphabet := []byte("```a \\")
	for i := 0; i < 20000; i++ {
		b := make([]byte, 1+r.Intn(30))
		for j := range b {
			b[j] = alphabet[r.Intn(len(alphabet))]
		}
		s := string(b)
		runs := newBacktickRuns(s)
		for p := 0; p < len(s); p++ {
			if s[p] != '`' {
				continue
			}
			if got, want := runs.spanEnd(p), MarkdownCodeSpanEnd(s, p); got != want {
				t.Fatalf("spanEnd(%d) en %q: %d, MarkdownCodeSpanEnd %d", p, s, got, want)
			}
		}
	}
}
