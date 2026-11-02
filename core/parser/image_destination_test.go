// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// Dos imágenes en una línea son un párrafo con dos imágenes inline; antes se
// leían como una imagen de bloque y la segunda se perdía. Una ruta suelta con
// espacios sigue siendo imagen, con un aviso LINK001.
func TestParseDocument_ImageDestinations(t *testing.T) {
	src := "# T\n\n![a](a.png) ![b](b.png)\n\n![c](<mi dir/c.png> \"Título\")\n\n![d](d.png otra)\n"
	doc, diags := New(util.NewNoop()).ParseDocument(src, "t.doclang")
	spacedWarned := false
	for _, d := range diags {
		if d.RuleID == "LINK001" && strings.Contains(d.Message, "d.png otra") {
			spacedWarned = true
		}
	}
	if !spacedWarned {
		t.Errorf("sin aviso LINK001 para la ruta con espacios: %v", diags)
	}
	var images []*ast.ImageElement
	var texts []string
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch e := n.(type) {
		case *ast.ImageElement:
			images = append(images, e)
		case *ast.TextElement:
			texts = append(texts, e.Content)
		}
		return nil
	})
	if len(images) != 2 || images[0].Source != "mi dir/c.png" || images[0].Title != "Título" || images[0].Alt != "c" || images[1].Source != "d.png otra" {
		t.Fatalf("imágenes de bloque = %#v", images)
	}
	joined := strings.Join(texts, "\n")
	for _, want := range []string{"![a](a.png) ![b](b.png)"} {
		if !strings.Contains(joined, want) {
			t.Errorf("falta el párrafo %q en %q", want, joined)
		}
	}
	ast.SetTableContract(doc)
	if doc.SchemaVersion != ast.ImageTitleSchemaVersion {
		t.Errorf("schemaVersion = %s, capabilities %v", doc.SchemaVersion, doc.Capabilities)
	}
}
