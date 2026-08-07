// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// `@include` se expande ANTES del parser (core/include, textual): la línea
// de la directiva se reemplaza por el contenido del fragmento TAL CUAL, con
// la indentación que el fragmento trae escrita — y descartando la de la
// directiva.
//
// En flex eso da igual. En strict, donde la indentación ES sintaxis (columna
// 0 abre una sección, indentado es su cuerpo), la regla se vuelve visible:
// **manda la indentación del fragmento, no la del `@include`**. Un fragmento
// de secciones completas se escribe en columna 0; uno pensado para el cuerpo
// de una sección se escribe ya indentado. Indentar la directiva no indenta
// nada.
//
// Estos tests fijan esa regla en sus dos direcciones. Es la clase de
// interacción que solo aparece end-to-end: ni el parser ni include saben el
// uno del otro.

func TestE2E_StrictIncludeOfCompleteSections(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "chapter.doclang", `SECTION "Included Chapter"

  TEXT
    Content from the included file.
`)
	main := writeDoc(t, dir, "main.doclang", `---
mode: strict
title: "Main"
---

SECTION "Introduction"

  TEXT
    Local content.

@include chapter.doclang
`)

	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", main, "--output", dir}); err != nil {
		t.Fatalf("expected an include of complete sections to build, got: %v", err)
	}

	html := readFileString(t, filepath.Join(dir, "main.html"))
	for _, want := range []string{"Introduction", "Included Chapter", "Content from the included file"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the rendered document to contain %q", want)
		}
	}
}

// Un fragmento escrito YA indentado aterriza dentro del cuerpo de la
// sección, aunque el `@include` esté en columna 0: lo que manda es la
// indentación del fragmento.
func TestE2E_StrictIncludeOfIndentedBodyFragment(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "fragment.doclang", "  TEXT\n    A fragment meant to sit inside a section.\n")
	main := writeDoc(t, dir, "main.doclang", `---
mode: strict
title: "Main"
---

SECTION "Introduction"

@include fragment.doclang
`)

	if err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", main, "--output", dir}); err != nil {
		t.Fatalf("expected an indented fragment to land inside the section, got: %v", err)
	}
	if html := readFileString(t, filepath.Join(dir, "main.html")); !strings.Contains(html, "sit inside a section") {
		t.Error("the included fragment did not make it into the output")
	}
}

// La trampa que esta regla esconde, fijada explícitamente: indentar el
// `@include` NO indenta el fragmento. Un fragmento sin indentar sigue
// aterrizando en columna 0 —donde solo vive SECTION— y el parser lo rechaza
// ruidosamente. Es el comportamiento correcto (el silencioso sería que el
// contenido desapareciera), pero contradice la intuición de quien viene de
// flex, así que queda pineado.
func TestE2E_StrictIndentingTheIncludeDirectiveDoesNotIndentTheFragment(t *testing.T) {
	dir := t.TempDir()
	writeDoc(t, dir, "fragment.doclang", "TEXT\n  Unindented fragment.\n")
	main := writeDoc(t, dir, "main.doclang", `---
mode: strict
title: "Main"
---

SECTION "Introduction"

  @include fragment.doclang
`)

	err := runCLI(t, Options{Name: "doclang", Version: "test"},
		[]string{"build", main, "--output", dir, "--lint-only"})
	if err == nil {
		t.Fatal("expected an unindented fragment to be rejected even though the directive was indented")
	}
	if !strings.Contains(err.Error(), "parse errors found") {
		t.Errorf("expected a parse error, got: %v", err)
	}
}
