// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

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

func fuzzSeeds() []string {
	return []string{
		"",
		"---",
		"---\n---",
		fuzzSeedFrontMatter,
		fuzzSeedStrict,
		fuzzSeedNoFrontMatter,
		fuzzSeedMalformedYAML,
		fuzzSeedUnicodeAndSpecials,
	}
}

// FuzzParse cubre el embudo real que usan ambos CLIs: parser.New(log).Parse.
func FuzzParse(f *testing.F) {
	for _, s := range fuzzSeeds() {
		f.Add(s)
	}
	logger := util.NewNoop()
	f.Fuzz(func(t *testing.T, content string) {
		p := New(logger)
		p.Parse(content, "fuzz.slidelang")
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
		p.Parse()
	})
}

// FuzzFlexParse cubre el modo flex (Markdown extendido).
func FuzzFlexParse(f *testing.F) {
	for _, s := range fuzzSeeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, content string) {
		p := NewFlexParser(content, util.NewNoop())
		p.Parse()
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
		p.Parse()

		pAI := NewDocumentFlexParserWithNormalization(content, logger)
		pAI.Parse()
	})
}
