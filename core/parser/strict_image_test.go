// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

// collectImages devuelve todos los ImageElement de un slide, en orden de
// documento.
func collectImages(block ast.ContentBlock) []*ast.ImageElement {
	var imgs []*ast.ImageElement
	for _, el := range block.Elements {
		if img, ok := el.(*ast.ImageElement); ok {
			imgs = append(imgs, img)
		}
	}
	return imgs
}

// TestStrictParser_ConsecutiveImages_NotCollapsed cubre el bug en el que el
// bucle de continuación de caption/label de parseStrictImage se tragaba las
// IMAGE (u otros elementos) hermanas al mismo nivel de indentación: tres IMAGE
// consecutivas colapsaban a una sola. Cada IMAGE debe quedar como su propio
// ImageElement; los caption:/label: legítimos siguen adhiriéndose a su imagen.
func TestStrictParser_ConsecutiveImages_NotCollapsed(t *testing.T) {
	type wantImage struct {
		source  string
		alt     string
		caption string
		label   string
	}

	tests := []struct {
		name string
		body string
		want []wantImage
	}{
		{
			name: "tres imagenes planas consecutivas",
			body: "" +
				"  IMAGE \"a.png\" \"alt a\"\n" +
				"  IMAGE \"b.png\" \"alt b\"\n" +
				"  IMAGE \"c.png\" \"alt c\"\n",
			want: []wantImage{
				{source: "a.png", alt: "alt a"},
				{source: "b.png", alt: "alt b"},
				{source: "c.png", alt: "alt c"},
			},
		},
		{
			name: "imagenes con caption cada una",
			body: "" +
				"  IMAGE \"a.png\" \"alt a\"\n" +
				"    caption: \"pie a\"\n" +
				"  IMAGE \"b.png\" \"alt b\"\n" +
				"    caption: \"pie b\"\n",
			want: []wantImage{
				{source: "a.png", alt: "alt a", caption: "pie a"},
				{source: "b.png", alt: "alt b", caption: "pie b"},
			},
		},
		{
			name: "primera con caption+label, segunda limpia",
			body: "" +
				"  IMAGE \"a.png\" \"alt a\"\n" +
				"    caption: \"pie a\"\n" +
				"    label: \"fig:a\"\n" +
				"  IMAGE \"b.png\" \"alt b\"\n",
			want: []wantImage{
				{source: "a.png", alt: "alt a", caption: "pie a", label: "fig:a"},
				{source: "b.png", alt: "alt b"},
			},
		},
		{
			name: "imagen seguida de otro elemento y otra imagen",
			body: "" +
				"  IMAGE \"a.png\" \"alt a\"\n" +
				"  TEXT\n" +
				"    Un parrafo entre imagenes.\n" +
				"  IMAGE \"b.png\" \"alt b\"\n",
			want: []wantImage{
				{source: "a.png", alt: "alt a"},
				{source: "b.png", alt: "alt b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := "---\nmode: strict\ntitle: M\n---\nSLIDE content\n" + tt.body

			p := New(util.NewNoop())
			astNode, diags := p.Parse(content, "test.slidelang")
			for _, d := range diags {
				if d.IsError() {
					t.Fatalf("unexpected error diagnostic: %s", d.Message)
				}
			}
			if astNode == nil || len(astNode.ContentBlocks) == 0 {
				t.Fatal("expected at least one content block")
			}

			imgs := collectImages(astNode.ContentBlocks[0])
			if len(imgs) != len(tt.want) {
				got := make([]string, len(imgs))
				for i, im := range imgs {
					got[i] = im.Source
				}
				t.Fatalf("image count = %d %v, want %d", len(imgs), got, len(tt.want))
			}

			for i, w := range tt.want {
				got := imgs[i]
				if got.Source != w.source {
					t.Errorf("image[%d].Source = %q, want %q", i, got.Source, w.source)
				}
				if got.Alt != w.alt {
					t.Errorf("image[%d].Alt = %q, want %q", i, got.Alt, w.alt)
				}
				if got.Caption != w.caption {
					t.Errorf("image[%d].Caption = %q, want %q", i, got.Caption, w.caption)
				}
				if got.Label != w.label {
					t.Errorf("image[%d].Label = %q, want %q", i, got.Label, w.label)
				}
			}
		})
	}
}

// IMAGE accepts an optional `context:`. It overrides the value the parser would
// infer from the position of the image, so that fmt can keep the context a flex
// source had. An unknown value is a warning and the inferred one is used.
func TestStrictParser_ImageContextProperty(t *testing.T) {
	parse := func(t *testing.T, body string) (*ast.ImageElement, []string) {
		t.Helper()
		content := "---\nmode: strict\ntitle: M\n---\nSLIDE content\n" + body
		astNode, diags := New(util.NewNoop()).Parse(content, "test.slidelang")
		var ruleIDs []string
		for _, d := range diags {
			if d.IsError() {
				t.Fatalf("unexpected error diagnostic: %s", d.Message)
			}
			ruleIDs = append(ruleIDs, d.RuleID)
		}
		imgs := collectImages(astNode.ContentBlocks[0])
		if len(imgs) != 1 {
			t.Fatalf("got %d images, want 1", len(imgs))
		}
		return imgs[0], ruleIDs
	}

	t.Run("declared", func(t *testing.T) {
		for _, c := range []ast.ImageContext{ast.ImageContextTitle, ast.ImageContextHero, ast.ImageContextGallery, ast.ImageContextContent, ast.ImageContextStandalone} {
			img, ids := parse(t, "  TEXT\n    a\n  TEXT\n    b\n  TEXT\n    c\n  IMAGE \"a.png\" \"alt\"\n    context: "+string(c)+"\n")
			if img.Context != c {
				t.Errorf("Context = %q, want %q", img.Context, c)
			}
			if len(ids) != 0 {
				t.Errorf("unexpected diagnostics %v", ids)
			}
		}
	})

	t.Run("absent keeps the inferred value", func(t *testing.T) {
		img, _ := parse(t, "  TEXT\n    a\n  TEXT\n    b\n  TEXT\n    c\n  IMAGE \"a.png\" \"alt\"\n")
		if img.Context == "" {
			t.Error("Context should still be inferred")
		}
	})

	t.Run("unknown value warns", func(t *testing.T) {
		img, ids := parse(t, "  TEXT\n    a\n  TEXT\n    b\n  TEXT\n    c\n  IMAGE \"a.png\" \"alt\"\n    context: banner\n")
		if img.Context == "" || img.Context == "banner" {
			t.Errorf("Context = %q, want an inferred value", img.Context)
		}
		if len(ids) != 1 || ids[0] != "IMG003" {
			t.Errorf("rule IDs = %v, want [IMG003]", ids)
		}
	})
}
