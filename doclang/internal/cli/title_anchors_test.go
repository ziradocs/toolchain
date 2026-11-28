// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"regexp"
	"strings"
	"testing"
)

// Límite conocido de --title con los ids únicos de encabezado (issue #267):
// el parser reparte los ids con el título de la fuente, así que una sección
// `# {{title}}` cuenta como "Notas" aunque --title la dibuje "Otra". Los ids
// siguen siendo únicos y cada enlace del TOC llega a su encabezado; lo que
// no se consigue es que `## Notas` conserve notas, así que un enlace del
// autor a #notas queda sin destino. Esta prueba fija las dos cosas: si
// alguna vez se reparten los ids con el título efectivo, el segundo id pasa
// a notas y hay que actualizarla (y el README y la ayuda de --title).
func TestTitleFlagKeepsHeadingIDsUnique(t *testing.T) {
	const src = "---\ntitle: Notas\n---\n\n# {{title}}\n\nVer [a](#notas).\n\n## Notas\n\nTexto.\n"
	out, err := runTitleBuild(t, src, "html", "--title", "Otra", "--toc")
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	body := html[strings.Index(html, "<body"):]
	var ids []string
	seen := map[string]int{}
	for _, m := range regexp.MustCompile(`<h[1-6] id="([^"]*)"`).FindAllStringSubmatch(body, -1) {
		ids = append(ids, m[1])
		seen[m[1]]++
	}
	if strings.Join(ids, " ") != "otra notas-2" {
		t.Errorf("ids = %v, want [otra notas-2]", ids)
	}
	// El límite documentado: el [a](#notas) de la fuente iba a `## Notas`,
	// que ahora es notas-2, y ningún encabezado tiene notas, así que el
	// enlace queda sin destino.
	if !strings.Contains(body, `<a href="#notas">a</a>`) {
		t.Errorf("no está el enlace a #notas de la fuente")
	}
	if seen["notas"] != 0 {
		t.Errorf("un encabezado tiene #notas: el límite cambió, actualiza el README y la ayuda de --title")
	}
	all := map[string]bool{}
	for _, m := range regexp.MustCompile(`\bid="([^"]*)"`).FindAllStringSubmatch(body, -1) {
		if all[m[1]] {
			t.Errorf("id %q repetido", m[1])
		}
		all[m[1]] = true
	}
	// El TOC de --toc en HTML es el de la barra lateral del visor.
	links := regexp.MustCompile(`<a href="#([^"]*)" class="toc-link`).FindAllStringSubmatch(body, -1)
	if len(links) != 2 {
		t.Fatalf("el TOC tiene %d enlaces, want 2", len(links))
	}
	for _, l := range links {
		if seen[l[1]] != 1 {
			t.Errorf("el TOC enlaza #%s, que no es el id de un encabezado", l[1])
		}
	}
}
