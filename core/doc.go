// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package core es el motor compartido de parsing, AST, linting y renderizado
// que usan los CLIs slidelang y doclang. A partir de v2.0.0, este módulo
// provee una API pública y estable para integraciones de terceros.
//
// # Modelo de consumo
//
// SlideLang/DocLang se pueden usar como binarios independientes, o se pueden
// embeber en otros programas Go para inyectar reglas de validación custom.
//
//	import (
//		"go.ziradocs.com/slidelang/cli"
//		"go.ziradocs.com/core/linter"
//	)
//
//	func main() {
//		cli.Execute(cli.Options{
//			CustomRules: []linter.Rule{MiRegla{}},
//		})
//	}
//
// # Contratos públicos estables (v2.x)
//
// Lo que este proyecto promete mantener y versionar según SemVer:
//
//  1. La API de entrada a los CLIs (paquetes slidelang/cli y doclang/cli),
//     en particular la estructura cli.Options que permite inyectar políticas,
//     reglas custom, y ganchos PostLint.
//
//  2. El esquema serializado (json) del AST se versiona de forma independiente
//     bajo ast.SchemaVersion (ver @ziradocs/ast-types y docs/architecture/json-ast-contract.md).
//
// El resto de la API Go (core/ast, core/linter, etc.) NO tiene garantías SemVer
// y puede cambiar en versiones menores. La estructura HTML generada y sus
// clases CSS tampoco son parte de este contrato estable.
package core
