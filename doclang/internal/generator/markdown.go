// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"os"
	"strings"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/util"
)

// MarkdownGenerator genera documentos Markdown
type MarkdownGenerator struct {
	logger util.Logger
}

// NewMarkdownGenerator crea un nuevo generador Markdown
func NewMarkdownGenerator(log util.Logger) *MarkdownGenerator {
	return &MarkdownGenerator{
		logger: log,
	}
}

// Generate genera un documento Markdown
func (m *MarkdownGenerator) Generate(doc *ast.AST, outputFile string, opts GeneratorOptions) error {
	m.logger.Info("MARKDOWN", "Building Markdown document...")

	var md strings.Builder

	// Add frontmatter if present
	if doc.FrontMatter != nil {
		md.WriteString("---\n")
		if doc.FrontMatter.Title != "" {
			fmt.Fprintf(&md, "title: %s\n", doc.FrontMatter.Title)
		}
		if doc.FrontMatter.Author != "" {
			fmt.Fprintf(&md, "author: %s\n", doc.FrontMatter.Author)
		}
		if doc.FrontMatter.Date != "" {
			fmt.Fprintf(&md, "date: %s\n", doc.FrontMatter.Date)
		}
		md.WriteString("---\n\n")
	}

	// Add main title
	if doc.FrontMatter != nil && doc.FrontMatter.Title != "" {
		fmt.Fprintf(&md, "# %s\n\n", doc.FrontMatter.Title)
	}

	// Table of contents
	if opts.TOC {
		md.WriteString("## Table of Contents\n\n")
		sectionNum := 1
		for _, block := range doc.ContentBlocks {
			if block.Title != "" {
				anchor := strings.ToLower(strings.ReplaceAll(block.Title, " ", "-"))
				if opts.Numbering {
					fmt.Fprintf(&md, "- [%d. %s](#%s)\n", sectionNum, block.Title, anchor)
				} else {
					fmt.Fprintf(&md, "- [%s](#%s)\n", block.Title, anchor)
				}
				sectionNum++
			}
		}
		md.WriteString("\n")
	}

	// Document body
	sectionNum := 1
	for i, block := range doc.ContentBlocks {
		if block.Title != "" {
			if opts.Numbering {
				fmt.Fprintf(&md, "## %d. %s\n\n", sectionNum, block.Title)
			} else {
				fmt.Fprintf(&md, "## %s\n\n", block.Title)
			}
			sectionNum++
		}

		// Generate content for each element
		for _, element := range block.Elements {
			md.WriteString(m.renderElement(element))
			md.WriteString("\n")
		}

		// Page break marker (HTML comment)
		if opts.PageBreaks && i < len(doc.ContentBlocks)-1 {
			md.WriteString("\n---\n\n")
		}
	}

	// Write to file
	if err := os.WriteFile(outputFile, []byte(md.String()), 0644); err != nil {
		return fmt.Errorf("failed to write Markdown file: %w", err)
	}

	m.logger.Info("MARKDOWN", "Markdown document generated successfully")
	return nil
}

// renderElement convierte un elemento AST a Markdown
func (m *MarkdownGenerator) renderElement(element ast.Element) string {
	switch elem := element.(type) {
	case *ast.TextElement:
		return elem.Content + "\n"

	case *ast.PointsElement:
		var md strings.Builder
		for i, item := range elem.Items {
			if elem.ListType == "ordered" {
				fmt.Fprintf(&md, "%d. %s\n", i+1, item.Content)
			} else {
				fmt.Fprintf(&md, "- %s\n", item.Content)
			}
		}
		return md.String()

	case *ast.CodeElement:
		return fmt.Sprintf("```%s\n%s\n```\n", elem.Language, elem.Content)

	case *ast.ImageElement:
		if elem.Caption != "" {
			return fmt.Sprintf("![%s](%s)\n*%s*\n", elem.Alt, elem.Source, elem.Caption)
		}
		return fmt.Sprintf("![%s](%s)\n", elem.Alt, elem.Source)

	case *ast.TableElement:
		var md strings.Builder

		// Headers
		if len(elem.Headers) > 0 {
			md.WriteString("|")
			for _, header := range elem.Headers {
				fmt.Fprintf(&md, " %s |", header)
			}
			md.WriteString("\n|")
			for range elem.Headers {
				md.WriteString(" --- |")
			}
			md.WriteString("\n")
		}

		// Rows
		for _, row := range elem.Rows {
			md.WriteString("|")
			for _, cell := range row {
				fmt.Fprintf(&md, " %s |", cell)
			}
			md.WriteString("\n")
		}

		if elem.Caption != "" {
			fmt.Fprintf(&md, "\n*%s*\n", elem.Caption)
		}

		return md.String()

	case *ast.QuoteElement:
		return fmt.Sprintf("> %s\n", elem.Content)

	case *ast.ChecklistElement:
		var md strings.Builder
		for _, item := range elem.Items {
			checked := " "
			if item.Checked {
				checked = "x"
			}
			fmt.Fprintf(&md, "- [%s] %s\n", checked, item.Content)
		}
		return md.String()

	case *ast.MermaidElement:
		return fmt.Sprintf("```mermaid\n%s\n```\n", elem.Content)

	case *ast.ChartElement:
		// Represent chart as code block
		return fmt.Sprintf("```chart:%s\n[Chart data would be here]\n```\n", elem.ChartType)

	case *ast.SpecialBlockElement:
		var md strings.Builder
		fmt.Fprintf(&md, "> **%s: %s**\n", strings.ToUpper(elem.BlockType), elem.Title)
		fmt.Fprintf(&md, "> %s\n", elem.Content)
		return md.String()

	case *ast.GridElement:
		// No native grid equivalent in Markdown: each column is rendered as
		// its own section, separated by a divider (issue #56).
		var md strings.Builder
		if elem.Content != "" {
			md.WriteString(elem.Content + "\n\n")
		}
		for i, column := range elem.Columns {
			if i > 0 {
				md.WriteString("\n---\n\n")
			}
			md.WriteString(column.Content + "\n")
		}
		return md.String()

	default:
		m.logger.Warn("MARKDOWN: Unknown element type: %T", element)
		return ""
	}
}
