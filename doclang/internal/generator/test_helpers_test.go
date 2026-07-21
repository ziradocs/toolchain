// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/util"
)

type testLogger struct {
	infos  []string
	warns  []string
	errors []string
}

func newTestLogger() *testLogger {
	return &testLogger{}
}

func (l *testLogger) Error(message string, args ...interface{}) {
	l.errors = append(l.errors, fmt.Sprintf(message, args...))
}

func (l *testLogger) Warn(message string, args ...interface{}) {
	l.warns = append(l.warns, fmt.Sprintf(message, args...))
}

func (l *testLogger) Info(category, message string, args ...interface{}) {
	l.infos = append(l.infos, fmt.Sprintf("%s: %s", category, fmt.Sprintf(message, args...)))
}

func (l *testLogger) Debug(component, message string, args ...interface{}) {}

func (l *testLogger) Progress(stage, operation string, progress int) {}

func (l *testLogger) Summary(operation string, stats map[string]interface{}) {}

func (l *testLogger) SetLevel(level util.LogLevel) {}

func newTestAST() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)

	front := ast.NewFrontMatterNode(pos)
	front.Title = "Sample Document"
	front.Author = "Test Author"
	front.Date = "2025-01-01"
	doc.FrontMatter = front

	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "Introduction"
	block.Elements = append(block.Elements,
		ast.NewTextElement(diagnostics.NewPosition(3, 1), "Welcome"),
	)

	points := ast.NewPointsElement(diagnostics.NewPosition(4, 1))
	points.Items = append(points.Items,
		*ast.NewPointItem(diagnostics.NewPosition(4, 1), "First item"),
		*ast.NewPointItem(diagnostics.NewPosition(5, 1), "Second item"),
	)
	block.Elements = append(block.Elements, points)

	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}
