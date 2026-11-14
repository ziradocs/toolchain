// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/formatter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/transform"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/core/v2/xref"
)

// El texto de la línea de un TEXT o un QUOTE strict (`TEXT hola`, `QUOTE una
// cita`) pasa a ser su contenido, y una propiedad repetida de un SLIDE, un
// SECTION, un <<map>>, un <<quiz>>/<<poll>>/<<metric>> o una directiva avisa
// la anterior. Ningún documento que compilaba puede dejar de compilar. Estas
// pruebas corren el build como el CLI (buildErrors: parser, linter y política
// del frontmatter) y además la resolución de \ref, que los dos CLIs corren
// antes del lint y que falla con una referencia sin resolver: el texto que
// antes se descartaba ahora llega a ella.

// strictReadsBuildErrors es buildErrors más el transform de \ref
// (xref.Transform), como lo corren slidelang build y doclang build.
func strictReadsBuildErrors(src string, document bool) []string {
	if errs := buildErrors(src, document); len(errs) > 0 {
		return errs
	}
	p := parser.New(util.NewNoop())
	var doc *ast.AST
	if document {
		doc, _ = p.ParseDocument(src, "t.doclang")
	} else {
		doc, _ = p.Parse(src, "t.slidelang")
	}
	if _, err := transform.RunBuiltins(doc, []transform.Transform{xref.Transform}); err != nil {
		return []string{"xref: " + err.Error()}
	}
	return nil
}

// genStrictReadsDocs genera documentos strict (y algunos flex, para los
// bloques que flex lee igual) con texto en la línea de TEXT y QUOTE, con y
// sin cuerpo, seguidos de hermanos a su misma sangría (también los que dan
// un error al leerse como elemento), y con propiedades repetidas en cada
// contenedor y bloque con llaves. Semilla fija.
func genStrictReadsDocs(seed int64, n int) []struct {
	src      string
	document bool
} {
	headers := []string{
		"hola", "una cita", "[enlace](https://example.com)", "[roto](a b.png)", `\ref{fig:a}`, `\ref{nope}`,
		"{{autor}}", "{{nadie}}", "AUTHOR: Ana", "SOURCE: libro", "---", "TEXT x", "QUOTE y", "POINTS", "@notes",
		"@center", "| a | b |", "<<end>>", ":::", ":::note", "**negrita**", "x: y", "title: Z", "- item", "1. uno",
		"# h", "### sub", "`code`", "$x$", "<b>html</b>", "[ ] tarea", ">", "SECTION \"S\"", "IMAGE \"a.png\"",
		"CODE python", "<<math>>", "MERMAID", "  espacios  ", "\"citado\"", "ñandú", "a\tb",
		"<<column>>", "<<column typed>>", "<</map>>", "@include x.doclang", "<!-- node-id: n -->", "SLIDE content", "<<quiz>>",
	}
	textForms := []string{"TEXT %s", "TEXT: %s", "TEXT:%s", "TEXTO %s", "TEXT", "TEXT:"}
	quoteForms := []string{"QUOTE %s", "QUOTE: %s", "QUOTE:%s", "QUOTEado %s", "QUOTE", "QUOTE:"}
	siblings := []string{
		"POINTS\n  - a", "MERMAID", "CODE a b c d\n  x", "TEXT", "TEXT\n  otra", "QUOTE x", "QUOTE\n  y",
		"<<chart: bar>>\n  data: [[\"a\", 1]]", `IMAGE "x.png"`, `IMAGE`, "mundo", "TABLE\n  | a |", "| a | b |",
		"@notes\n  nota", "<<mermaid>>\n  graph TD", ":::note\nx\n:::", "PLANTUML", "<<quiz>>\n  question: Q\n  options: [a]\n<<end>>",
		`SECTION "Sub"`, "CHECKLIST\n  [ ] t",
	}
	bodies := []string{"  mundo", "  mundo\n  sigue", "  AUTHOR: Beto", "  texto\n  SOURCE: s", "", "\n  tras blanco", "  - lista\n    - sub", "    hondo"}
	slideProps := []string{`title: "A"`, `title: "B"`, `heading: "H1"`, `heading: "H2"`, `subtitle: "s1"`, `subtitle: "s2"`,
		`kicker: "k1"`, `kicker: "k2"`, `logo: "l1.png"`, `logo: "l2.png"`, "columns: 2", "columns: 3", "columns: 9",
		"align: center", "align: left", "align: diagonal", `title: ""`}
	layouts := []string{"content", "title", "hero", "comparison", "stats", "dashboard", "testimonial", "code-example", "section", "two-column"}
	headingProps := []string{"level: 4", "level: 5", "level: 9", "level: x", "id: a", "id: b", "id: !!!"}
	sectionProps := []string{"level: 1", "level: 2", "level: 3", "level: 7", "id: uno", "id: dos", "id: ***"}
	blocks := []string{
		"<<map type=\"city\" zoom=\"3\">>\n  type: world\n  type: europe\n  zoom: 2\n  zoom: x\n  center: 1, 2\n  center: 3, 4\n  title: \"T1\"\n  options:\n    title: \"T2\"\n  markers:\n  - lat: 1\n    lng: 2\n    lng: 3\n    label: \"a\"\n    label: \"b\"\n  - lat: 5\n    lng: 6\n    label: \"c\"",
		"<<map>>\n  type: world\n  heatmap: true\n  heatmap: false\n  showValues: true\n  showValues: false\n  clustering: true\n  clustering: false",
		"<<quiz>>\n  question: Q\n  options: [a, b]\n  answer: 0\n  answer: 1\n<<end>>",
		"<<quiz>>\n  {question: Q, options: [a, b], answer: 0, answer: 1}\n<<end>>",
		"<<poll>>\n  question: Q\n  question: R\n  options: [a, b]\n<<end>>",
		"<<metric>>\n  label: L\n  label: M\n  value: 1\n<<end>>",
		"@transition type=\"fade\" type=\"slide\" duration=\"300\"",
		"@highlight color=\"x\" color=\"y\"",
		"@timer duration=1 duration=2",
		"<<chart: bar>>\n  {\"labels\": [\"a\"],\n  \"labels\": [\"b\"],\n  \"datasets\": [{\"label\": \"s\", \"label\": \"t\", \"data\": [1]}]}",
	}
	type ctx struct {
		document, flex bool
		pre            string
		indent         int
		suf            string
		slide          bool
		section        bool
	}
	ctxs := []ctx{
		{document: true, pre: "SECTION \"S\"\n", indent: 2, section: true},
		{document: true, pre: "SECTION \"A\"\n  TEXT\n    a\nSECTION \"S\"\n", indent: 2, section: true},
		{pre: "SLIDE %s\n", indent: 2, slide: true},
		{pre: "SLIDE content\n  title: \"T\"\n  <<grid>>\n  <<column typed>>\n", indent: 4, suf: "  <<end>>\n"},
		{document: true, pre: "SECTION \"S\"\n  <<grid>>\n  <<column typed>>\n", indent: 4, suf: "  <<end>>\n"},
		{document: true, pre: "SECTION \"S\"\n  :::note\n", indent: 2, suf: "  :::\n"},
		{pre: "SLIDE content\n  title: \"T\"\n  :::info{typed}\n", indent: 2, suf: "  :::\n"},
		{document: true, flex: true, pre: "# Doc\n\n", indent: 0},
		{flex: true, pre: "# Deck\n\n## S\n\n", indent: 0},
	}
	rng := rand.New(rand.NewSource(seed))
	pick := func(xs []string) string { return xs[rng.Intn(len(xs))] }
	keyword := func(forms []string) string {
		f := pick(forms)
		if strings.Contains(f, "%s") {
			return fmt.Sprintf(f, pick(headers))
		}
		return f
	}
	out := make([]struct {
		src      string
		document bool
	}, n)
	for k := range out {
		c := ctxs[rng.Intn(len(ctxs))]
		mode := "strict"
		if c.flex {
			mode = "flex"
		}
		var body []string
		pre := c.pre
		if c.slide {
			pre = fmt.Sprintf(pre, pick(layouts))
			for p := rng.Intn(4); p > 0; p-- {
				body = append(body, pick(slideProps))
			}
		}
		if c.section {
			for p := rng.Intn(3); p > 0; p-- {
				body = append(body, pick(sectionProps))
			}
		}
		for el := 1 + rng.Intn(3); el > 0; el-- {
			switch r := rng.Intn(10); {
			case c.flex && r < 7:
				body = append(body, pick(blocks))
			case r < 3:
				body = append(body, keyword(textForms))
				if b := pick(bodies); b != "" {
					body = append(body, b)
				}
			case r < 6:
				body = append(body, keyword(quoteForms))
				if b := pick(bodies); b != "" {
					body = append(body, b)
				}
			case r < 7 && c.slide:
				h := []string{`SECTION "Sub"`}
				for p := 1 + rng.Intn(3); p > 0; p-- {
					h = append(h, "  "+pick(headingProps))
				}
				body = append(body, strings.Join(h, "\n"))
			case r < 9:
				body = append(body, pick(blocks))
			default:
				body = append(body, pick(siblings))
			}
			if rng.Float64() < 0.4 {
				body = append(body, pick(siblings))
			}
			if rng.Float64() < 0.2 {
				body = append(body, "")
			}
		}
		var b strings.Builder
		b.WriteString("---\nmode: " + mode + "\ntitle: \"G\"\n")
		if rng.Intn(2) == 0 {
			b.WriteString("variables:\n  autor: \"Ana\"\n")
		}
		b.WriteString("---\n\n" + pre)
		for _, block := range body {
			for _, l := range strings.Split(block, "\n") {
				if l == "" {
					b.WriteString("\n")
				} else {
					b.WriteString(strings.Repeat(" ", c.indent) + l + "\n")
				}
			}
		}
		b.WriteString(c.suf)
		out[k].src, out[k].document = b.String(), c.document
	}
	return out
}

const strictReadsSeed, strictReadsDocs = 259, 4000

// strictReadsDump, con STRICT_READS_DUMP=<dir>, escribe el AST y los
// errores de build de cada documento generado, para comparar el AST contra
// la base.
func strictReadsDump(t *testing.T, i int, src string, document bool, errs []string) {
	dir := os.Getenv("STRICT_READS_DUMP")
	if dir == "" {
		return
	}
	p := parser.New(util.NewNoop())
	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if document {
		doc, diags = p.ParseDocument(src, "t.doclang")
	} else {
		doc, diags = p.Parse(src, "t.slidelang")
	}
	var parseErrs []diagnostics.Diagnostic
	var warnings []string
	for _, d := range diags {
		if d.IsError() {
			parseErrs = append(parseErrs, d)
			continue
		}
		end := d.Position.Line
		if d.EndPosition != nil {
			end = d.EndPosition.Line
		}
		warnings = append(warnings, fmt.Sprintf("%s:%d-%d %s", d.RuleID, d.Position.Line, end, d.Message))
	}
	raw, err := json.MarshalIndent(struct {
		Src         string
		AST         *ast.AST
		ParseErrors []diagnostics.Diagnostic
		Warnings    []string
		Build       []string
		Fmt         []string
	}{src, doc, parseErrs, warnings, errs, strictReadsFmtChecks(src, document)}, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("%05d-%x.json", i, sha256.Sum256([]byte(src)))
	if err := os.WriteFile(filepath.Join(dir, name[:21]+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// strictReadsParse parsea src como lo hace cada CLI; ok es false si hay un
// error.
func strictReadsParse(src string, document bool) (*ast.AST, bool) {
	p := parser.New(util.NewNoop())
	var doc *ast.AST
	var diags []diagnostics.Diagnostic
	if document {
		doc, diags = p.ParseDocument(src, "t.doclang")
	} else {
		doc, diags = p.Parse(src, "t.slidelang")
	}
	for _, d := range diags {
		if d.IsError() {
			return doc, false
		}
	}
	return doc, true
}

// strictReadsTree es el AST como árbol JSON sin posiciones ni frontmatter,
// para comparar un documento con su salida de fmt.
func strictReadsTree(doc *ast.AST) string {
	raw, _ := json.Marshal(doc)
	var v any
	_ = json.Unmarshal(raw, &v)
	var strip func(any) any
	strip = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, val := range x {
				if k == "position" || k == "endPosition" || k == "frontMatter" || strings.HasSuffix(k, "Positions") {
					continue
				}
				out[k] = strip(val)
			}
			return out
		case []any:
			for i := range x {
				x[i] = strip(x[i])
			}
		}
		return v
	}
	out, _ := json.Marshal(strip(v))
	return string(out)
}

// strictReadsRepeatWarnings cuenta los avisos de propiedad repetida de src.
// fmt escribe una vez cada propiedad que arma desde el AST y copia tal cual
// los bloques que no arma (<<map>>, un <<chart>> JSON, las directivas), así
// que su salida no puede tener más que el documento.
func strictReadsRepeatWarnings(src string, document bool) int {
	p := parser.New(util.NewNoop())
	var diags []diagnostics.Diagnostic
	if document {
		_, diags = p.ParseDocument(src, "t.doclang")
	} else {
		_, diags = p.Parse(src, "t.slidelang")
	}
	n := 0
	for _, d := range diags {
		switch d.RuleID {
		case "STRICT005", "MAP003", "QUOTE001", "MATH001":
			n++
		case "DIRECTIVE004", "CHART007", "IMG005", "TABLE008":
			if strings.Contains(d.Message, "replaces it") || strings.Contains(d.Message, "twice") {
				n++
			}
		case "QUIZ007", "POLL007", "METRIC006":
			if strings.Contains(d.Message, "written again") || strings.Contains(d.Message, "twice") {
				n++
			}
		}
	}
	return n
}

// strictReadsFmtChecks corre fmt sobre un documento que parsea sin errores,
// como cada CLI (slidelang fmt escribe strict; doclang fmt conserva el
// dialecto y --strict transpila), y devuelve lo que falla: "fmt:<forma>" si
// fmt da un error, "reread:<forma>" si su salida no se lee igual que el
// documento, "idem:<forma>" si fmt de su salida no la deja igual y
// "warn:<forma>" si su salida tiene más avisos de propiedad repetida que el
// documento.
func strictReadsFmtChecks(src string, document bool) []string {
	doc, ok := strictReadsParse(src, document)
	if !ok {
		return nil
	}
	type form struct {
		name string
		f    func(*ast.AST) (string, error)
	}
	forms := []form{{"strict", formatter.FormatStrict}}
	if document {
		forms = []form{{"doc-strict", formatter.FormatDocumentStrict}}
		if doc.FrontMatter == nil || doc.FrontMatter.Mode != "strict" {
			forms = append(forms, form{"doc", formatter.FormatDocument})
		}
	}
	var out []string
	for _, fm := range forms {
		doc, _ := strictReadsParse(src, document)
		text, err := fm.f(doc)
		if err != nil {
			out = append(out, "fmt:"+fm.name)
			continue
		}
		again, ok := strictReadsParse(text, document)
		if !ok || strictReadsTree(again) != strictReadsTree(doc) {
			out = append(out, "reread:"+fm.name)
			continue
		}
		if text2, err := fm.f(again); err != nil || text2 != text {
			out = append(out, "idem:"+fm.name)
			continue
		}
		if strictReadsRepeatWarnings(text, document) > strictReadsRepeatWarnings(src, document) {
			out = append(out, "warn:"+fm.name)
		}
	}
	return out
}

// strictReadsBaseLines lee un archivo de testdata generado en la base: una
// entrada por línea, sin las vacías ni los comentarios.
func strictReadsBaseLines(t *testing.T, name string) map[string]bool {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	out := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			out[line] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// strictReadsWriteBase, con STRICT_READS_WRITE_BASE=<dir>, escribe en dir los
// dos archivos de testdata de estas pruebas. Se corre una vez en la base
// (84e227e, origin/integration/v2.59.0) con este mismo archivo de prueba; en
// la rama no se regeneran.
func strictReadsWriteBase(t *testing.T, dir string) {
	docs := genStrictReadsDocs(strictReadsSeed, strictReadsDocs)
	head := fmt.Sprintf("# genStrictReadsDocs(%d, %d) en 84e227e (origin/integration/v2.59.0)\n", strictReadsSeed, strictReadsDocs)
	var build, fmts strings.Builder
	build.WriteString(head + "# Índices de los documentos que no compilan\n")
	fmts.WriteString(head + "# Índice y comprobación de fmt que falla\n")
	for i, d := range docs {
		errs := strictReadsBuildErrors(d.src, d.document)
		strictReadsDump(t, i, d.src, d.document, errs)
		if len(errs) > 0 {
			fmt.Fprintf(&build, "%d\n", i)
		}
		for _, c := range strictReadsFmtChecks(d.src, d.document) {
			fmt.Fprintf(&fmts, "%d %s\n", i, c)
		}
	}
	for name, text := range map[string]string{"strict_reads_main_failures.txt": build.String(), "strict_reads_main_fmt.txt": fmts.String()} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Diferencial de build contra la base: testdata/strict_reads_main_failures.txt
// tiene los índices de los documentos generados que no compilaban allá, con el
// mismo generador y la misma semilla. Todo documento que compilaba tiene que
// seguir compilando.
func TestStrictReadsBuildDifferential(t *testing.T) {
	if dir := os.Getenv("STRICT_READS_WRITE_BASE"); dir != "" {
		strictReadsWriteBase(t, dir)
		return
	}
	failedOnBase := strictReadsBaseLines(t, "strict_reads_main_failures.txt")
	docs := genStrictReadsDocs(strictReadsSeed, strictReadsDocs)
	broke, fixed := 0, 0
	for i, d := range docs {
		errs := strictReadsBuildErrors(d.src, d.document)
		strictReadsDump(t, i, d.src, d.document, errs)
		base := failedOnBase[strconv.Itoa(i)]
		switch {
		case len(errs) > 0 && !base:
			broke++
			if broke <= 5 {
				t.Errorf("document %d built on the base and now fails: %v\n%s", i, errs, d.src)
			}
		case len(errs) == 0 && base:
			fixed++
		}
	}
	if broke > 0 {
		t.Errorf("%d documents that built on the base fail now", broke)
	}
	t.Logf("%d of %d documents failed on the base; %d of them build now", len(failedOnBase), len(docs), fixed)
}

// Diferencial de fmt contra la base (testdata/strict_reads_main_fmt.txt): en
// cada documento que parsea, fmt no puede empezar a fallar, su salida tiene
// que leerse igual que el documento y fmt de su salida tiene que dejarla
// igual, en cada forma donde eso ya pasaba en la base.
func TestStrictReadsFmtDifferential(t *testing.T) {
	if os.Getenv("STRICT_READS_WRITE_BASE") != "" {
		return
	}
	failedOnBase := strictReadsBaseLines(t, "strict_reads_main_fmt.txt")
	docs := genStrictReadsDocs(strictReadsSeed, strictReadsDocs)
	broke, checked := 0, 0
	for i, d := range docs {
		if _, ok := strictReadsParse(d.src, d.document); !ok {
			continue
		}
		checked++
		for _, c := range strictReadsFmtChecks(d.src, d.document) {
			// Una forma que ya fallaba en la base, por la comprobación que
			// sea, no pasaba allá.
			form := c[strings.Index(c, ":")+1:]
			if failedOnBase[fmt.Sprintf("%d fmt:%s", i, form)] || failedOnBase[fmt.Sprintf("%d reread:%s", i, form)] ||
				failedOnBase[fmt.Sprintf("%d idem:%s", i, form)] || failedOnBase[fmt.Sprintf("%d warn:%s", i, form)] {
				continue
			}
			broke++
			if broke <= 5 {
				t.Errorf("document %d: %s, which passed on the base\n%s", i, c, d.src)
			}
		}
	}
	if broke > 0 {
		t.Errorf("%d fmt checks that passed on the base fail now", broke)
	}
	t.Logf("%d of %d documents parse; %d fmt checks failed on the base", checked, len(docs), len(failedOnBase))
}
