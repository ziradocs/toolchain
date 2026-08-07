// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// writeDoc escribe content en <dir>/<name> y devuelve la ruta.
func writeDoc(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	return path
}

// writeModeFixture escribe un .doclang flex mínimo cuyo frontmatter declara
// mode (o ninguno, si mode == "").
func writeModeFixture(t *testing.T, dir, mode string) string {
	t.Helper()
	frontmatter := "title: \"Mode Fixture\"\n"
	if mode != "" {
		frontmatter += "mode: " + mode + "\n"
	}
	return writeDoc(t, dir, "fixture.doclang",
		"---\n"+frontmatter+"---\n\n# Fixture\n\nHello world.\n")
}

const strictFixture = `---
mode: strict
title: "Strict Fixture"
---

SECTION "Introduction"

  TEXT
    This document is written in the strict dialect.

SECTION "Details"
  level: 2
  id: details

  POINTS
    - First point
    - Second point
`

// El contrato de la fase 2: un documento strict construye de verdad, con su
// propio dialecto. (En la fase 0 este mismo archivo fallaba a propósito, y
// antes de eso se construía en silencio como flex.)
func TestE2E_StrictModeBuilds(t *testing.T) {
	dir := t.TempDir()
	fixture := writeDoc(t, dir, "strict.doclang", strictFixture)

	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", fixture, "--output", dir, "--toc"}); err != nil {
		t.Fatalf("expected a strict document to build, got: %v", err)
	}

	html, err := os.ReadFile(filepath.Join(dir, "strict.html"))
	if err != nil {
		t.Fatalf("expected an HTML output file: %v", err)
	}
	out := string(html)

	for _, want := range []string{"Introduction", "First point", `id="details"`} {
		if !strings.Contains(out, want) {
			t.Errorf("expected the rendered document to contain %q", want)
		}
	}
}

// Un error de gramática strict tiene que salir como error de parseo con su
// línea, no como un documento vacío que "construyó bien".
func TestE2E_StrictGrammarErrorIsReported(t *testing.T) {
	dir := t.TempDir()
	fixture := writeDoc(t, dir, "broken.doclang",
		"---\nmode: strict\ntitle: \"Broken\"\n---\n\nJust some prose, no SECTION.\n")

	err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", fixture, "--output", dir, "--lint-only"})
	if err == nil {
		t.Fatal("expected a strict document with no SECTION to fail")
	}
	if !strings.Contains(err.Error(), "parse errors found") {
		t.Errorf("expected the build to abort on the parse-error path, got: %v", err)
	}
}

// El mismo texto en los dos dialectos rinde el mismo documento. Es la
// promesa que hace útil al par flex/strict: se elige el dialecto por cómo se
// quiere AUTORAR, no por lo que sale.
func TestE2E_StrictAndFlexRenderEquivalently(t *testing.T) {
	dir := t.TempDir()

	strictPath := writeDoc(t, dir, "s.doclang", `---
mode: strict
title: "T"
---

SECTION "Guide"

  TEXT
    Intro paragraph.

SECTION "Install"
  level: 2

  TEXT
    Run it.
`)
	flexPath := writeDoc(t, dir, "f.doclang", `---
mode: flex
title: "T"
---

# Guide

Intro paragraph.

## Install

Run it.
`)

	for _, p := range []string{strictPath, flexPath} {
		if err := runCLI(t, Options{Name: "doclang", Version: "test"},
			[]string{"build", p, "--output", dir}); err != nil {
			t.Fatalf("build failed for %s: %v", filepath.Base(p), err)
		}
	}

	strictHTML := readFileString(t, filepath.Join(dir, "s.html"))
	flexHTML := readFileString(t, filepath.Join(dir, "f.html"))

	// Los encabezados de subsección son lo que prueba que la forma del AST
	// coincide: los produce el mismo helper compartido en ambos dialectos.
	for _, want := range []string{`<h2 id="install">Install</h2>`} {
		if !strings.Contains(strictHTML, want) {
			t.Errorf("strict output is missing %q", want)
		}
		if !strings.Contains(flexHTML, want) {
			t.Errorf("flex output is missing %q", want)
		}
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(b)
}

// `fmt` sin banderas PRESERVA el dialecto: un documento strict se reescribe
// en strict. Lo contrario —emitir flex por defecto— transpilaría en silencio
// cada documento strict del repo en la primera corrida de `fmt --write`.
func TestE2E_FmtPreservesTheStrictDialect(t *testing.T) {
	dir := t.TempDir()
	fixture := writeDoc(t, dir, "strict.doclang", strictFixture)

	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"fmt", fixture, "--write"}); err != nil {
		t.Fatalf("expected fmt to format a strict document, got: %v", err)
	}

	formatted := readFileString(t, fixture)
	if !strings.Contains(formatted, "mode: strict") || !strings.Contains(formatted, `SECTION "Introduction"`) {
		t.Fatalf("fmt did not keep the document in the strict dialect:\n%s", formatted)
	}

	// Segunda pasada: byte-idéntica.
	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"fmt", fixture, "--write"}); err != nil {
		t.Fatalf("second fmt pass failed: %v", err)
	}
	if again := readFileString(t, fixture); again != formatted {
		t.Errorf("fmt is not idempotent on strict documents:\n--- first ---\n%s\n--- second ---\n%s", formatted, again)
	}
}

// `--strict` sobre un documento flex lo promueve a artefacto auditable. La
// prueba de que la transpilación es correcta no es que el texto se parezca,
// sino que el documento transpilado RINDE lo mismo.
func TestE2E_FmtTranspilesFlexToStrict(t *testing.T) {
	dir := t.TempDir()
	flexSource := `---
mode: flex
title: "Draft"
---

# Guide

Intro paragraph.

## Install

Run it.
`
	flexPath := writeDoc(t, dir, "draft.doclang", flexSource)

	// HTML de referencia, antes de transpilar.
	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", flexPath, "--output", dir}); err != nil {
		t.Fatalf("baseline build failed: %v", err)
	}
	before := readFileString(t, filepath.Join(dir, "draft.html"))

	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"fmt", flexPath, "--strict", "--write"}); err != nil {
		t.Fatalf("transpile failed: %v", err)
	}

	transpiled := readFileString(t, flexPath)
	if !strings.Contains(transpiled, "mode: strict") || !strings.Contains(transpiled, `SECTION "Guide"`) {
		t.Fatalf("expected a strict document after --strict, got:\n%s", transpiled)
	}

	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", flexPath, "--output", dir}); err != nil {
		t.Fatalf("the transpiled document does not build: %v", err)
	}
	after := readFileString(t, filepath.Join(dir, "draft.html"))
	if stripNonces(after) != stripNonces(before) {
		t.Error("transpiling flex → strict changed the rendered output")
	}
}

// stripNonces normaliza los nonces CSP, que se regeneran al azar en cada
// build: sin esto, dos builds del MISMO archivo ya diferirían y la
// comparación no probaría nada.
var nonceRe = regexp.MustCompile(`nonce-?[=">]?['"]?[A-Za-z0-9+/=]{16,}`)

func stripNonces(html string) string {
	return nonceRe.ReplaceAllString(html, "nonce-NORMALIZED")
}

// Degradar strict → flex descartaría la declaración del autor sin forma de
// recuperarla, así que se niega en vez de hacerlo.
func TestE2E_FmtRefusesToDowngradeStrictToFlex(t *testing.T) {
	dir := t.TempDir()
	fixture := writeDoc(t, dir, "strict.doclang", strictFixture)

	err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"fmt", fixture, "--strict=false"})
	if err == nil {
		t.Fatal("expected fmt to refuse downgrading a strict document to flex")
	}
	if !strings.Contains(err.Error(), "degradaría") {
		t.Errorf("expected the error to explain the downgrade, got: %v", err)
	}
	if got := readFileString(t, fixture); got != strictFixture {
		t.Error("fmt modified a document it refused to convert")
	}
}

// Los waivers viven en el frontmatter y se resuelven desde `fm.Raw`, que es
// agnóstico al dialecto — pero eso hay que probarlo, no suponerlo.
func TestE2E_LintPolicyWorksInStrictFrontMatter(t *testing.T) {
	dir := t.TempDir()
	rulepack := writeToyRulepack(t, dir)

	fixture := writeDoc(t, dir, "waived.doclang", "---\nmode: strict\ntitle: \"Waived\"\n"+
		waiverBlock(nil)+"---\n\nSECTION \"Intro\"\n\n  TEXT\n    Hello.\n")

	if err := runCLI(t, Options{Name: "doclang", Version: "test", ExternalRulepacks: []string{rulepack}},
		[]string{"build", fixture, "--output", dir, "--lint-only"}); err != nil {
		t.Fatalf("expected the waiver in a strict document's frontmatter to apply, got: %v", err)
	}
}

// La plantilla que `doclang init --template strict` escribe tiene que
// construir. Una plantilla rota es peor que no tener plantilla: es lo
// primero que ve alguien que prueba el dialecto.
func TestE2E_StrictInitTemplateBuilds(t *testing.T) {
	dir := t.TempDir()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"init", "sample", "--template", "strict"}); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", "sample.doclang", "--output", dir}); err != nil {
		t.Fatalf("the strict init template does not build: %v", err)
	}

	// La plantilla promete un `label:` con su `\ref` resuelto — es
	// justamente lo que el dialecto flex no puede hacer para tablas.
	html := readFileString(t, filepath.Join(dir, "sample.html"))
	if strings.Contains(html, `\ref{`) {
		t.Error("the template's cross-reference was left unresolved")
	}
	if !strings.Contains(html, `href="#tbl-example"`) {
		t.Error("the template's labelled table did not become a resolvable anchor")
	}
}

// Control negativo, y la garantía de compatibilidad que importa: los modos
// que el corpus de examples/ realmente usa siguen construyendo igual. Sin
// esto, un despacho demasiado goloso pasaría desapercibido.
func TestE2E_NonStrictModesStillBuild(t *testing.T) {
	for _, mode := range []string{"flex", "flex-full", "auto", ""} {
		name := mode
		if name == "" {
			name = "unset"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			fixture := writeModeFixture(t, dir, mode)

			if err := runCLI(t, Options{Name: "doclang", Version: "test"},
				[]string{"build", fixture, "--output", dir, "--lint-only"}); err != nil {
				t.Fatalf("expected mode %q to keep building, got: %v", mode, err)
			}
		})
	}
}
