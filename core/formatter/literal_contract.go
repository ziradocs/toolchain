// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
 "strings"
 "go.ziradocs.com/core/v2/ast"
)

func validateLiteralRepresentation(doc *ast.AST) error {
 if ast.DeclaresExtendedContract(doc) { if err := ast.ValidateTableContract(doc); err != nil { return err } }
 source := ast.UsesMathSource(doc)
 return ast.Walk(doc,func(n ast.Node) error {
  m,ok := n.(*ast.MathElement); if !ok { return nil }
  lines := strings.Split(m.Content,"\n")
  for _, line := range lines {
   trimmed := strings.TrimSpace(line)
   if trimmed == "<<end>>" || trimmed == "---" || strings.HasPrefix(line,"caption:") || strings.HasPrefix(line,"label:") { return newUnsupported("math","content collides with a math delimiter or metadata") }
   if !source && (line != trimmed || trimmed == "" && m.Content != "") { return newUnsupported("math","legacy math cannot preserve whitespace; math-source-v1 is required") }
  }
  return nil
 })
}
