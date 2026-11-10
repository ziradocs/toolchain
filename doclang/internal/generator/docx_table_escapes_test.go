// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// El encabezado de una tabla en DOCX no pasa por el procesador inline, pero
// en el HTML sí: `H \*h\*` se ve `H *h*` ahí. El escape se resuelve igual.
func TestDOCXTable_HeaderResolvesEscapes(t *testing.T) {
	table := ast.NewTableElement(diagnostics.NewPosition(1, 1))
	table.Headers = []string{`H \*h\* y C:\Users\_tmp`}
	table.Rows = [][]string{{"c"}}
	output := filepath.Join(t.TempDir(), "t.docx")
	if err := New(newTestLogger()).Generate(astWithElements(table), output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatal(err)
	}
	xml := docxDocumentXML(t, output)
	if !strings.Contains(xml, `H *h* y C:\Users\_tmp`) || strings.Contains(xml, `\*`) {
		t.Errorf("the table header kept its escape backslash")
	}
}
