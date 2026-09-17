// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"reflect"

	"go.ziradocs.com/core/v2/ast"
)

// frontMatterThemeMode reads the additive core AST field without making this
// module stop compiling against the currently published core. CI deliberately
// builds consumers both against that release and against the core in this
// repository; once the core release is bumped, the reflection path resolves
// the same exported string field normally. An older core simply has no visual
// scheme metadata and therefore keeps the system-preference behavior.
func frontMatterThemeMode(fm *ast.FrontMatterNode) string {
	if fm == nil {
		return ""
	}
	field := reflect.ValueOf(fm).Elem().FieldByName("ThemeMode")
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	mode := field.String()
	if mode == "light" || mode == "dark" {
		return mode
	}
	return ""
}
