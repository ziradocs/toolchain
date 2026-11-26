// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/internal/normalize"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer"
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/util"
)

// Las posiciones del AST y de los diagnósticos cuentan las líneas del
// archivo aunque el normalizador haya cambiado el número de líneas antes de
// que el parser lo leyera (ver normalized_lines.go). Cada caso arma un
// documento cuyo texto normalizado tiene otro número de líneas que el
// fuente, y pide la línea exacta de tres cosas que vienen después del
// cambio: un TABLE003 del linter (una fila con una celda de más), un error
// del parser (un fence de código con un argumento de más) y el encabezado
// de una sección.

const (
	badRow     = "| 1 | 2 | 3 | sobra |"
	badFence   = "```go malo extra"
	lastHeader = "# Cierre"
)

func mermaidBlock(i int) string {
	return fmt.Sprintf("Párrafo %d.\n\n```mermaid\ngraph TD\n  A%d-->B%d\n```\n", i, i, i)
}

// repeatedNoAnchors son dos mermaid y dos tablas iguales con una fila de
// más: ninguna de sus líneas es única.
func repeatedNoAnchors() string {
	m := "```mermaid\n\n\ngraph TD\n\n\n```\n"
	tbl := "| a | b |\n|---|---|\n| 1 | 2 | 3 |\n"
	return m + "\n" + tbl + "\n" + tbl + "\n" + m + "\n"
}

// Las dos filas con una celda de más de repeatedNoAnchors dan TABLE003 en
// su línea (13 y 17), no en las que daba el reparto en proporción (11 y 18).
func TestParseDocument_RepeatedLinesWithoutAnchors(t *testing.T) {
	src := "# T\n\n" + repeatedNoAnchors() + "Fin.\n"
	requireLineCountChange(t, src, base.DialectDocuments)
	doc, _ := New(util.NewNoop()).ParseDocument(src, "d.doclang")
	var got []int
	for _, d := range linter.New().Lint(doc) {
		if d.RuleID == "TABLE003" {
			got = append(got, d.Position.Line)
		}
	}
	if fmt.Sprint(got) != "[13 17]" {
		t.Errorf("TABLE003 on lines %v, want [13 17]", got)
	}
}

// lcsPairs da una subsecuencia común de largo máximo: se compara con la
// programación dinámica de siempre sobre entradas chicas al azar.
func TestLCSPairsIsALongestCommonSubsequence(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	alphabet := []string{"", "a", "b", "```", "|x|"}
	gen := func() []string {
		out := make([]string, rng.Intn(12))
		for i := range out {
			out[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return out
	}
	for i := 0; i < 3000; i++ {
		a, b := gen(), gen()
		m := lineMatcher{src: a, seen: b, out: make([]int, len(b)+2)}
		pairs, ok := m.lcsPairs(0, len(a), 0, len(b))
		if !ok {
			t.Fatalf("lcsPairs gave up on %q %q", a, b)
		}
		pa, pb := -1, -1
		for _, p := range pairs {
			if p.a <= pa || p.b <= pb || a[p.a] != b[p.b] {
				t.Fatalf("%q %q: bad pairs %v", a, b, pairs)
			}
			pa, pb = p.a, p.b
		}
		if want := lcsLength(a, b); len(pairs) != want {
			t.Fatalf("%q %q: %d pairs, want %d", a, b, len(pairs), want)
		}
	}
}

func lcsLength(a, b []string) int {
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				dp[i][j] = dp[i+1][j+1] + 1
			case dp[i+1][j] > dp[i][j+1]:
				dp[i][j] = dp[i+1][j]
			default:
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	return dp[0][0]
}

func tail() string {
	return "\n| a | b | c |\n|---|---|---|\n| 1 | 2 | 3 |\n" + badRow + "\n\n" +
		badFence + "\nx := 1\n```\n\n" + lastHeader + "\n\nFin.\n"
}

// lineOf es la línea (1-based) de la primera línea de src que es exactamente
// want, ignorando espacios a los lados.
func lineOf(t *testing.T, src, want string) int {
	t.Helper()
	for i, l := range strings.Split(src, "\n") {
		if strings.TrimSpace(l) == want {
			return i + 1
		}
	}
	t.Fatalf("%q not found in source", want)
	return 0
}

// requireLineCountChange falla si el normalizador no cambia el número de
// líneas de content: sin eso el caso no prueba nada.
func requireLineCountChange(t *testing.T, content string, dialect base.Dialect) {
	t.Helper()
	content = normalizeDocumentStart(content)
	det := normalizer.NewDetector().Detect(content)
	out, _ := normalize.ProcessWithDetection(content, det, dialect, util.NewNoop())
	if strings.Count(out, "\n") == strings.Count(content, "\n") {
		t.Fatalf("the normalizer keeps the line count of this case (detected=%v); it proves nothing", det.Detected)
	}
}

type gotLines struct{ table, fence, header int }

func collectLines(doc *ast.AST, diags []diagnostics.Diagnostic) gotLines {
	var g gotLines
	all := append(append([]diagnostics.Diagnostic{}, diags...), linter.New().Lint(doc)...)
	for _, d := range all {
		switch {
		case d.RuleID == "TABLE003":
			g.table = d.Position.Line
		case d.IsError() && strings.Contains(d.Message, "takes at most a language and a filename"):
			g.fence = d.Position.Line
		}
	}
	for _, b := range doc.ContentBlocks {
		if b.Title == "Cierre" || b.Heading == "Cierre" {
			g.header = b.Position.Line
		}
	}
	return g
}

func checkLines(t *testing.T, src string, g gotLines) {
	t.Helper()
	want := gotLines{lineOf(t, src, badRow), lineOf(t, src, badFence), lineOf(t, src, lastHeader)}
	if g != want {
		t.Errorf("lines (table, fence, header) = %v, want %v", g, want)
	}
}

func TestParseDocument_PositionsSurviveNormalization(t *testing.T) {
	var mermaid10 strings.Builder
	for i := 0; i < 10; i++ {
		mermaid10.WriteString(mermaidBlock(i))
		mermaid10.WriteString("\n---\n\n")
	}
	cases := map[string]string{
		// El caso del reporte: sin frontmatter (missing_frontmatter enciende
		// el normalizador), diez bloques mermaid y reglas horizontales.
		"no frontmatter, ten mermaid fences and rules": "# Inicio\n\n" + mermaid10.String() + tail(),
		// Con frontmatter el Detector necesita otra señal: una imagen con
		// nombre de ejemplo.
		"frontmatter, mermaid fences": "---\ntitle: \"D\"\n---\n\n# Inicio\n\n![x](example.png)\n\n" +
			mermaidBlock(1) + "\n" + mermaidBlock(2) + "\n```js\nlet a = 1\n```\n\n***\n" + tail(),
		"leading blank lines before frontmatter": "\n\n---\ntitle: \"D\"\n---\n\n# Inicio\n\n![x](example.png)\n\n" +
			mermaidBlock(1) + tail(),
		"BOM and no frontmatter":                 "\xef\xbb\xbf# Inicio\n\n" + mermaidBlock(1) + "\n" + mermaidBlock(2) + tail(),
		"leading blank lines and no frontmatter": "\n\n\n# Inicio\n\n" + mermaidBlock(1) + tail(),
		// TablesRule convierte líneas separadas por tabs en una tabla y le
		// agrega el encabezado y el separador: dos líneas más.
		"tab-separated lines turned into a table": "# Inicio\n\nNombre\tEdad\tCiudad\nAna\t30\tLeón\nLuis\t41\tMérida\nEva\t25\tTepic\n" + tail(),
		// Sin ninguna línea única entre el primer y el último renglón: el
		// mismo mermaid dos veces y la misma tabla dos veces. El mapa se
		// arma con la subsecuencia común más larga, no en proporción.
		"no unique line, repeated mermaid and tables": "# Inicio\n\n" + repeatedNoAnchors() + tail(),
		// Un mermaid cuyo cuerpo empieza y termina con líneas en blanco, que
		// MermaidRule también quita.
		"mermaid with blank lines inside": "# Inicio\n\n```mermaid\n\n\ngraph TD\n  A-->B\n\n\n```\n" + tail(),
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			requireLineCountChange(t, src, base.DialectDocuments)
			doc, diags := New(util.NewNoop()).ParseDocument(src, "d.doclang")
			// La línea se cuenta sobre el fuente sin BOM: el BOM no ocupa
			// línea.
			checkLines(t, strings.TrimPrefix(src, "\xef\xbb\xbf"), collectLines(doc, diags))
		})
	}
}

// La entrada pública que normaliza por su cuenta lleva las posiciones al
// mismo lugar que ParseDocument.
func TestNewDocumentFlexParserWithNormalization_PositionsSurviveNormalization(t *testing.T) {
	src := "# Inicio\n\n" + mermaidBlock(1) + "\n" + mermaidBlock(2) + tail()
	requireLineCountChange(t, src, base.DialectDocuments)
	doc, diags := NewDocumentFlexParserWithNormalization(src, util.NewNoop()).Parse()
	checkLines(t, src, collectLines(doc, diags))
}

// Slidelang comparte el normalizador: un deck flex cuyo normalizador cambia
// el número de líneas da también la línea del archivo.
func TestParse_SlidesPositionsSurviveNormalization(t *testing.T) {
	for _, mode := range []string{"flex", "flex-full", "auto"} {
		t.Run(mode, func(t *testing.T) {
			src := "---\nmode: " + mode + "\ntitle: \"D\"\n---\n\n# Inicio\n\n![x](example.png)\n\n" +
				mermaidBlock(1) + "\n" + mermaidBlock(2) + tail()
			requireLineCountChange(t, src, base.DialectSlides)
			doc, diags := New(util.NewNoop()).Parse(src, "d.slidelang")
			want := gotLines{lineOf(t, src, badRow), lineOf(t, src, badFence), 0}
			g := collectLines(doc, diags)
			g.header = 0 // en slides "# Cierre" es otro slide; basta con las dos líneas de diagnóstico
			if g != want {
				t.Errorf("lines (table, fence) = %v, want %v", g, want)
			}
		})
	}
}

func TestNormalizedLineMap(t *testing.T) {
	cases := []struct {
		name, src, seen string
		want            []int
	}{
		{"identity", "a\nb\nc", "a\nb\nc", []int{0, 1, 2, 3, 4}},
		// El ``` de cierre se quita: lo que sigue sube una línea.
		{"line removed", "x\n```mermaid\ngraph\n```\ny", "x\n<<mermaid>>\n  graph\ny", []int{0, 1, 2, 3, 5, 6}},
		// Dos líneas insertadas antes de c caen en c.
		{"lines inserted", "a\nc\nd", "a\nh\ns\nc\nd", []int{0, 1, 2, 2, 2, 3, 4}},
		{"inserted at the end", "a\nb", "a\nb\nz", []int{0, 1, 2, 2, 3}},
		{"repeated lines only", "-\n-\n-\n-", "-\n-", []int{0, 1, 2, 5}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizedLineMap(c.src, strings.Split(c.seen, "\n"))
			if fmt.Sprint(got) != fmt.Sprint(c.want) {
				t.Errorf("normalizedLineMap = %v, want %v", got, c.want)
			}
		})
	}
}
