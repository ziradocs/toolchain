// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// TestPrepareTemplateData_Lang cubre el prerrequisito de los issues
// #62/#63: FrontMatter.Lang debe llegar a PresentationData.Lang para que
// buildHTMLHead (template/base.go) pueda emitir un `<html lang>` real en vez
// del "es" hardcodeado que ignoraba el frontmatter por completo.
func TestPrepareTemplateData_Lang(t *testing.T) {
	astDoc := &ast.AST{
		FrontMatter: &ast.FrontMatterNode{Lang: "fr"},
	}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())

	if got.Lang != "fr" {
		t.Errorf("Lang = %q, want %q", got.Lang, "fr")
	}
}

func TestPrepareTemplateData_Lang_AbsentFrontMatterLeavesEmpty(t *testing.T) {
	astDoc := &ast.AST{}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())

	if got.Lang != "" {
		t.Errorf("Lang = %q, want empty when there is no FrontMatter", got.Lang)
	}
}

func TestPrepareTemplateData_ThemeMode(t *testing.T) {
	astDoc := &ast.AST{FrontMatter: frontMatterWithThemeMode(t, "dark")}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	if got.ThemeMode != "dark" {
		t.Errorf("ThemeMode = %q, want dark", got.ThemeMode)
	}
}

func TestPrepareTemplateData_ThemeModeInvalidIsNotRendered(t *testing.T) {
	astDoc := &ast.AST{FrontMatter: frontMatterWithThemeMode(t, "sepia")}

	got := PrepareTemplateDataWithRenderMode(astDoc, "default", "browser", util.NewNoop(), renderer.NewDefaultRenderContext())
	if got.ThemeMode != "" {
		t.Errorf("ThemeMode = %q, want empty for a non-renderable value", got.ThemeMode)
	}
}

func frontMatterWithThemeMode(t *testing.T, mode string) *ast.FrontMatterNode {
	t.Helper()
	fm := &ast.FrontMatterNode{}
	field := reflect.ValueOf(fm).Elem().FieldByName("ThemeMode")
	if !field.IsValid() {
		t.Skip("the published core used by this compatibility build predates ThemeMode")
	}
	field.SetString(mode)
	return fm
}
