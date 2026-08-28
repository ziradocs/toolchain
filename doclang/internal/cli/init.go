// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.ziradocs.com/core/v2/util"
)

// NewInitCommand creates the init command for doclang
func NewInitCommand() *cobra.Command {
	var (
		template string
	)

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize a new doclang document",
		Long: `Initialize creates a new .doclang file with a basic structure.

Examples:
  doclang init my-document
  doclang init policy --template strict
  doclang init technical-spec --template technical
  doclang init report --template report`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			docName := args[0]

			// Issue #47: docName debía ser un nombre, no un fragmento de ruta.
			// Sin este chequeo, `doclang init ../../evil` escribía
			// "../../evil.doclang" fuera del directorio actual — mismo
			// tratamiento que ya usan los nombres de tema (util.IsOpaquePathToken).
			if !util.IsOpaquePathToken(docName) {
				return fmt.Errorf("invalid document name %q: must not contain path separators, \"..\", or be an absolute path", docName)
			}

			fileName := docName + ".doclang"

			// Check if file already exists
			if _, err := os.Stat(fileName); err == nil {
				return fmt.Errorf("file already exists: %s", fileName)
			}

			// Generate content based on template
			content := generateDocumentTemplate(docName, template)

			// Write file
			if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			fmt.Printf("✅ Created: %s\n", fileName)
			fmt.Printf("📝 Edit the file and run: doclang build %s\n", fileName)
			return nil
		},
	}

	// Flags
	cmd.Flags().StringVarP(&template, "template", "t", "default", "Document template (default, strict, technical, report)")

	return cmd
}

func generateDocumentTemplate(name, template string) string {
	if template == "strict" {
		return generateStrictTemplate(name)
	}
	return generateFlexTemplate(name, template)
}

// generateStrictTemplate produce el esqueleto del dialecto strict: bloques
// SECTION declarados, sin normalización. Es el dialecto que se commitea y se
// revisa, así que la plantilla muestra lo que flex no puede — un `label:` en
// una tabla y su `\ref` resuelto.
func generateStrictTemplate(name string) string {
	return fmt.Sprintf(`---
title: %s
author: Your Name
mode: strict
---

SECTION "Introduction"

  TEXT
    Welcome to **DocLang** in strict mode: every block is declared, and the
    normalizer never runs. What you read is what gets parsed.

SECTION "Background"
  level: 2
  id: background

  POINTS
    - Sections are declared with SECTION, not inferred from Markdown headings
    - Nesting comes from `+"`level:`"+`, never from indentation
    - `+"`id:`"+` pins an anchor so a reference survives a title change

SECTION "Results"

  TEXT
    Labelled tables and figures can be cross-referenced; see \ref{tbl-example}.

  TABLE
    headers: ["Metric", "Value"]
    rows: [
      ["Throughput", "1200 rps"],
      ["p99 latency", "45 ms"]
    ]
    caption: "Example measurements"
    label: tbl-example
`, name)
}

// Tres reglas que las plantillas de acá tienen que respetar, y que no son
// cosméticas — cada una salió de una plantilla que no sobrevivía al propio
// toolchain (ni a `doclang build`, no solo a `doclang fmt`):
//
//  1. El contenido de un bloque <<...>> va INDENTADO 2 espacios. Un
//     <<mermaid>> con su `graph TD` pegado al margen es exactamente lo que
//     normalizer.Detector.detectMalformedMermaidDiagrams puntúa 0.8 como
//     "contenido generado por AI" — por encima del umbral de 0.3 — y eso
//     enciende el set ENTERO de reglas del normalizador sobre el documento.
//     Ahí HeadersRule (una heurística pensada para slidelang: un solo "##"
//     por slide, el resto es contenido) degradaba a "**negrita**" el segundo
//     "##" de cada sección. `## 1.2 Scope` y `## 2.2 Components` salían del
//     build como texto en negrita, fuera de la jerarquía y fuera del TOC.
//     Con el contenido indentado el detector no dispara y no se normaliza
//     nada.
//
//  2. Un bloque <<...>> se cierra con <<end>>, nunca con un `>>` suelto.
//     `>>` no es un terminador del lenguaje (core/spec/language-specification.md:
//     `element_terminator ::= "<<end>>" | block_boundary | EOF`); el bloque
//     terminaba por dedent y el `>>` sobrante se parseaba como una cita
//     anidada, que salía renderizada como un "> >" suelto. Ojo con
//     ElementClosingTagsRule, que dice normalizar `>>`: su guard
//     `!HasSuffix(trimmed, ">>")` la deja inerte para cualquier apertura
//     bien formada (`<<mermaid>>`, `<<chart...>>` y `<<map>>` terminan en
//     `>>`), así que no rescata nada.
//
//  3. Si los títulos ya traen su propio número escrito ("# 1. Introduction"),
//     la plantilla declara `numbering: false`. Con la numeración automática
//     encendida salía "1. 1. Introduction" — es el caso de uso exacto para
//     el que existe el tri-estado de FrontMatterNode.Numbering.
//
//  4. Las llaves de un bloque van en la forma que el parser realmente lee.
//     El chart de `report` decía `<<chart:bar title="...">>` con un
//     `datasets:`/`backgroundColor:` adentro, y ninguna de las tres cosas
//     existe en el DSL: el título no llegaba al AST (el chart se renderizaba
//     sin título), backgroundColor se perdía, y el `data:` anidado dentro de
//     `datasets:` se colaba como si fuera la llave de nivel superior. Las
//     llaves del cuerpo son data/series/labels/options/title/type, y la
//     apertura solo acepta width/height. Desde CHART005 el parser avisa en
//     vez de tragárselo, pero la plantilla no debería necesitar el aviso.
//
//     Ojo con "devolverle" el color al chart: hoy NO hay forma desde el DSL
//     de fijar el color de un dataset — renderer/html.go lo asigna desde la
//     paleta del tema (dataset["backgroundColor"] = colors[...]), pisando
//     cualquier cosa que venga de options.
//
// TestInit_TemplatesSurviveTheirOwnToolchain (init_roundtrip_test.go) cubre
// las tres.
func generateFlexTemplate(name, template string) string {
	switch template {
	case "technical":
		return fmt.Sprintf(`---
title: %s
author: Your Name
date: 2025-10-08
mode: flex
toc:
  enabled: true
  depth: 3
numbering: false
page:
  size: A4
  margins: 2cm
---

# Executive Summary

Brief overview of the document.

---

# 1. Introduction

## 1.1 Purpose

Describe the purpose of this document.

## 1.2 Scope

Define the scope of this specification.

---

# 2. System Architecture

## 2.1 Overview

High-level architecture description.

<<mermaid>>
  graph TD
  A[Client] --> B[API Gateway]
  B --> C[Service Layer]
  C --> D[Database]
<<end>>

## 2.2 Components

### 2.2.1 API Gateway

Component details...

---

# 3. Requirements

## 3.1 Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-001 | User authentication | High |
| FR-002 | Data validation | High |
| FR-003 | Reporting | Medium |

---

# 4. Conclusion

Summary and next steps.
`, name)

	case "report":
		return fmt.Sprintf(`---
title: %s
author: Your Name
date: 2025-10-08
mode: flex
toc:
  enabled: true
numbering: false
header:
  enabled: true
  text:
    center: %s
footer:
  enabled: true
  page_numbers:
    enabled: true
---

# Executive Summary

Key findings and recommendations.

---

# 1. Introduction

## 1.1 Background

Context and background information.

## 1.2 Objectives

Main objectives of this report.

---

# 2. Methodology

How the analysis was conducted.

---

# 3. Findings

## 3.1 Key Metrics

<<chart: bar>>
  title: "Performance Metrics"
  labels: ["Q1", "Q2", "Q3", "Q4"]
  data: [85, 90, 88, 95]
<<end>>

## 3.2 Analysis

Detailed analysis of findings.

---

# 4. Recommendations

Actionable recommendations based on findings.

---

# 5. Conclusion

Summary and next steps.
`, name, name)

	default:
		return fmt.Sprintf(`---
title: %s
author: Your Name
date: 2025-10-08
mode: flex
---

# Introduction

Welcome to **DocLang**! This is a sample document.

---

# Section 1

## Subsection 1.1

Your content here with **bold** and *italic* text.

- Point 1
- Point 2
- Point 3

---

# Section 2

## Code Example

`+"```python\n"+`def hello():
    print("Hello, DocLang!")
`+"```\n"+`

---

# Conclusion

Summary and next steps.
`, name)
	}
}
