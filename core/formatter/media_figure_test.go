// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// poster= y caption= dejan de perderse: el parser los llena, el documento
// pasa a 2.18.0 con media-figure-v1 inferido de la sintaxis, y fmt los
// conserva en los dos DSL.
func TestMediaPosterCaptionRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name     string
		src      string
		document bool
	}{
		{"slidelang strict", "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  <<video src=\"demo.mp4\" poster=\"poster.jpg\" caption=\"Product demo\" controls>>\n", false},
		{"doclang flex", "---\ntitle: D\n---\n# Doc\n\n<<video src=\"demo.mp4\" poster=\"poster.jpg\" caption=\"Product demo\" controls>>\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := mustParse(t, tc.src, tc.document)
			m := firstOfType[*ast.MediaElement](t, doc)
			if m.Poster != "poster.jpg" || m.Caption != "Product demo" || !m.Controls {
				t.Fatalf("media = %#v", m)
			}
			if doc.SchemaVersion != ast.MediaFigureSchemaVersion {
				t.Fatalf("version = %s", doc.SchemaVersion)
			}
			format := FormatStrict
			if tc.document {
				format = FormatDocument
			}
			out, err := format(doc)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, `poster="poster.jpg"`) || !strings.Contains(out, `caption="Product demo"`) {
				t.Fatalf("fmt lost poster/caption:\n%s", out)
			}
			again := firstOfType[*ast.MediaElement](t, mustParse(t, out, tc.document))
			if again.Poster != m.Poster || again.Caption != m.Caption {
				t.Fatalf("round trip = %#v", again)
			}
		})
	}
}

// Un atributo desconocido ya no desaparece sin aviso.
func TestMediaUnknownAttributeWarns(t *testing.T) {
	_, diags := parseAny("---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  <<video src=\"a.mp4\" width=\"300\" caption=\"x y\">>\n")
	found := false
	for _, d := range diags {
		if d.RuleID == "MEDIA001" && strings.Contains(d.Message, `"width"`) {
			found = true
		}
		if strings.Contains(d.Message, `"y"`) || strings.Contains(d.Message, `"x`) {
			t.Fatalf("caption text reported as attribute: %v", d)
		}
	}
	if !found {
		t.Fatalf("no MEDIA001 warning: %v", diags)
	}
}
