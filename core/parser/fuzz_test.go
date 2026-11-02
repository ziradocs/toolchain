// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// Issue #45: el parser consume input no confiable (contenido de un archivo
// .slidelang/.doclang cualquiera) sin ningún fuzz target — el único recover
// existente vive en util.RunGuarded, en los call sites del CLI (build.go),
// no en el parser mismo. Estos targets llaman al parser DIRECTAMENTE, sin
// ese recover, para que un panic real quede expuesto como fallo de fuzzing
// (y termine como test de regresión) en vez de quedar oculto detrás del
// recover del CLI en producción.
//
// Semillas embebidas como literales (no se leen desde examples/, que vive
// fuera de este módulo — core eventualmente será su propio repo,
// ver plan de lanzamiento OSS, y una ruta relativa a examples/ se rompería
// en ese momento).

const fuzzSeedFrontMatter = `---
title: Fuzz Seed
mode: flex
---

# Título

Contenido de prueba.
`

const fuzzSeedStrict = `---
mode: strict
title: Fuzz Seed Strict
---

SLIDE title
  TITLE: Título
  CONTENT: Contenido de prueba.
END
`

const fuzzSeedNoFrontMatter = `# Solo un título

Sin front matter.
`

const fuzzSeedMalformedYAML = `---
title: [unclosed
mode: flex
---

# X
`

const fuzzSeedUnicodeAndSpecials = `---
title: "Ünïcödé 🎉 テスト"
mode: flex-full
---

# Título con ***énfasis*** y ` + "`código`" + `

- [ ] tarea
- [x] hecha

<<chart: type=bar>>
{"labels": ["a"], "datasets": []}
<<end>>
`

const fuzzSeedDocumentStrict = `---
mode: strict
title: Fuzz Seed Document Strict
---

SECTION "Introducción"

  TEXT
    Contenido de prueba.

SECTION "Detalle"
  level: 2
  id: detalle

  POINTS
    - uno
    - dos
`

// Listas con nested-list-types-v1 y list-start-v1: la capacidad cambia de
// parser de listas, y el fuzzer casi nunca escribe ese frontmatter por su
// cuenta. "1)" con la capacidad colgaba la CLI en v2.49.0.
const fuzzSeedTypedLists = `---
mode: flex
ast_capabilities: ["nested-list-types-v1"]
---
# T

1) uno
2) dos
1. tres
   - a) cuatro
     1) cinco
`

const fuzzSeedListStart = `---
mode: flex
ast_capabilities: ["nested-list-types-v1", "list-start-v1"]
---
# T

3) uno
4) dos
- x
  2. y
`

const fuzzSeedTypedStrict = `---
mode: strict
ast_capabilities: [nested-list-types-v1]
---
SLIDE content
  POINTS
    1) uno
      - dos
    2. tres
`

// fuzzTypedPrefix se antepone a la entrada en los embudos para que el
// fuzzer recorra también el parser de listas tipado.
const fuzzTypedPrefix = "---\nmode: flex\nast_capabilities: [\"nested-list-types-v1\", \"list-start-v1\"]\n---\n"

// failOnZeroConsumption falla si un bucle flex tuvo que saltarse una línea
// porque un parser devolvió un elemento sin consumir nada: el guard evita el
// cuelgue, pero sigue siendo un bug de parser que el fuzzing debe encontrar.
func failOnZeroConsumption(t *testing.T, diags []diagnostics.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if strings.Contains(d.Message, "consumed no lines") {
			t.Fatalf("parser did not advance: %s", d.Message)
		}
	}
}

func fuzzSeeds() []string {
	return []string{
		"",
		"---",
		"---\n---",
		fuzzSeedFrontMatter,
		fuzzSeedStrict,
		fuzzSeedDocumentStrict,
		fuzzSeedNoFrontMatter,
		fuzzSeedMalformedYAML,
		fuzzSeedUnicodeAndSpecials,
		fuzzSeedTypedLists,
		fuzzSeedListStart,
		fuzzSeedTypedStrict,
	}
}

// FuzzParse cubre el embudo real que usan ambos CLIs: parser.New(log).Parse.
func FuzzParse(f *testing.F) {
	for _, s := range fuzzSeeds() {
		f.Add(s)
	}
	logger := util.NewNoop()
	f.Fuzz(func(t *testing.T, content string) {
		_, diags := New(logger).Parse(content, "fuzz.slidelang")
		failOnZeroConsumption(t, diags)
		_, diags = New(logger).Parse(fuzzTypedPrefix+content, "fuzz.slidelang")
		failOnZeroConsumption(t, diags)
	})
}

// FuzzFrontMatter cubre específicamente la frontera de yaml.Unmarshal.
func FuzzFrontMatter(f *testing.F) {
	for _, s := range fuzzSeeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, content string) {
		p := &FrontMatterParser{}
		p.Parse(content)
	})
}

// FuzzStrictParse cubre el modo strict (sintaxis con keywords).
func FuzzStrictParse(f *testing.F) {
	for _, s := range fuzzSeeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, content string) {
		p := NewStrictParser(content, util.NewNoop())
		_, diags := p.Parse()
		failOnZeroConsumption(t, diags)
	})
}

// FuzzFlexParse cubre el modo flex (Markdown extendido).
func FuzzFlexParse(f *testing.F) {
	for _, s := range fuzzSeeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, content string) {
		p := NewFlexParser(content, util.NewNoop())
		_, diags := p.Parse()
		failOnZeroConsumption(t, diags)
	})
}

// FuzzDocumentFlexParse cubre el parser de doclang, con y sin normalización
// AI habilitada (dos formas de construcción, mismo Parse()).
func FuzzDocumentFlexParse(f *testing.F) {
	for _, s := range fuzzSeeds() {
		f.Add(s)
	}
	logger := util.NewNoop()
	f.Fuzz(func(t *testing.T, content string) {
		p := NewDocumentFlexParser(content, logger)
		_, diags := p.Parse()
		failOnZeroConsumption(t, diags)

		pAI := NewDocumentFlexParserWithNormalization(content, logger)
		_, diags = pAI.Parse()
		failOnZeroConsumption(t, diags)
	})
}

// FuzzDocumentStrictParse cubre el dialecto strict documental (bloques
// SECTION). Comparte el despacho de elementos con FuzzStrictParse, pero su
// bucle de nivel superior y su manejo de propiedades son propios — y es ahí,
// en las ramas que deciden si una línea abre un bloque, donde vivieron los
// hangs de forward-progress que motivaron estos targets (issues #45/#155).
func FuzzDocumentStrictParse(f *testing.F) {
	for _, s := range fuzzSeeds() {
		f.Add(s)
	}
	logger := util.NewNoop()
	f.Fuzz(func(t *testing.T, content string) {
		p := NewDocumentStrictParser(content, logger)
		_, diags := p.Parse()
		failOnZeroConsumption(t, diags)
	})
}

// FuzzParseDocument cubre el embudo real de doclang: el despacho por modo,
// normalizador incluido.
func FuzzParseDocument(f *testing.F) {
	for _, s := range fuzzSeeds() {
		f.Add(s)
	}
	logger := util.NewNoop()
	f.Fuzz(func(t *testing.T, content string) {
		_, diags := New(logger).ParseDocument(content, "fuzz.doclang")
		failOnZeroConsumption(t, diags)
		_, diags = New(logger).ParseDocument(fuzzTypedPrefix+content, "fuzz.doclang")
		failOnZeroConsumption(t, diags)
	})
}
