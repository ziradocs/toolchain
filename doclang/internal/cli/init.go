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
	cmd.Flags().StringVarP(&template, "template", "t", "default", "Document template (default, technical, report)")

	return cmd
}

func generateDocumentTemplate(name, template string) string {
	return generateFlexTemplate(name, template)
}

func generateFlexTemplate(name, template string) string {
	switch template {
	case "technical":
		return fmt.Sprintf(`---
title: %s
doctype: technical-specification
author: Your Name
date: 2025-10-08
mode: flex
toc:
  enabled: true
  depth: 3
numbering:
  enabled: true
  style: 1.1.1
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
>>

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
doctype: report
author: Your Name
date: 2025-10-08
mode: flex
toc:
  enabled: true
numbering:
  enabled: true
header:
  enabled: true
  text: %s
footer:
  enabled: true
  page-numbers: true
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

<<chart:bar title="Performance Metrics">>
  labels: ["Q1", "Q2", "Q3", "Q4"]
  datasets:
    data: [85, 90, 88, 95]
    backgroundColor: "#3498db"
>>

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
