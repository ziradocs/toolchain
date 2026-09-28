// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func typedHeadingFilterFixture() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	first := ast.NewHeadingElement(pos, 3, "First heading", "heading-first-heading")
	first.NodeID = "HeadingA"
	second := ast.NewHeadingElement(pos, 4, "Second heading", "heading-second-heading")
	second.NodeID = "HeadingB"
	block.Elements = append(block.Elements, first, second)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	ast.SetTableContract(doc)
	return doc
}

const headingHandshake = `if [ "$1" = "--ziradocs-capabilities" ]; then
echo '{"astSchemaVersions":["2.17.0"],"features":["typed-headings-v1"]}'
exit 0
fi
`

// Un filtro compatible puede editar texto y anchor; uno viejo no recibe el
// AST; uno que cambia el nivel de un encabezado identificado, lo borra o baja
// la versión se rechaza.
func TestTypedHeadingFilterNegotiationAndPreservation(t *testing.T) {
	good := writeFilter(t, headingHandshake+"cat")
	if _, err := RunFilters(typedHeadingFilterFixture(), []string{good}, time.Second); err != nil {
		t.Fatal(err)
	}
	edit := writeFilter(t, headingHandshake+`sed 's/First heading/Edited heading/; s/heading-first-heading/heading-edited-heading/'`)
	doc, err := RunFilters(typedHeadingFilterFixture(), []string{edit}, time.Second)
	if err != nil {
		t.Fatalf("text edit rejected: %v", err)
	}
	if h := doc.ContentBlocks[0].Elements[0].(*ast.HeadingElement); h.Text != "Edited heading" || h.NodeID != "HeadingA" {
		t.Fatalf("edit not applied: %#v", h)
	}
	marker := filepath.Join(t.TempDir(), "ran")
	old := writeFilter(t, `if [ "$1" = "--ziradocs-capabilities" ]; then echo '{"astSchemaVersions":["2.16.0"],"features":["nested-list-types-v1"]}'; exit 0; fi
echo ran > `+marker+`
cat`)
	if _, err := RunFilters(typedHeadingFilterFixture(), []string{old}, time.Second); err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("old filter accepted: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("old filter received AST bytes")
	}
	level := writeFilter(t, headingHandshake+`sed 's/"level":4/"level":5/'`)
	if _, err := RunFilters(typedHeadingFilterFixture(), []string{level}, time.Second); err == nil || !strings.Contains(err.Error(), "levels changed") {
		t.Fatalf("level change accepted: %v", err)
	}
	renamed := writeFilter(t, headingHandshake+`sed 's/HeadingB/HeadingZ/'`)
	if _, err := RunFilters(typedHeadingFilterFixture(), []string{renamed}, time.Second); err == nil || !strings.Contains(err.Error(), "nodeId set") {
		t.Fatalf("identity rename accepted: %v", err)
	}
	downgrade := writeFilter(t, headingHandshake+`sed 's/"schemaVersion":"2.17.0"/"schemaVersion":"2.14.0"/'`)
	if _, err := RunFilters(typedHeadingFilterFixture(), []string{downgrade}, time.Second); err == nil {
		t.Fatal("downgraded AST accepted")
	}
}
