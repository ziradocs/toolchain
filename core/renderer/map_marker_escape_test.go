// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// TestRenderMapBrowser_MarkerLabelDetails_NoDoubleEscape covers issue #68:
// renderMapBrowser ran marker.Label/marker.Details through
// ProcessVariablesSecure (which already calls EscapeHTML internally) and
// then through EscapeHTMLAttribute again, re-escaping the already-escaped
// entities — e.g. "Café & bar" became "Café &amp;amp; bar" instead of
// "Café &amp; bar". Not a security issue (double-escaping is more
// restrictive, not less), but a display bug: browsers render the
// data-label/data-details attribute values, which end up showing literal
// "&amp;" text in the marker popup instead of "&".
func TestRenderMapBrowser_MarkerLabelDetails_NoDoubleEscape(t *testing.T) {
	elem := ast.NewMapElement(diagnostics.NewPosition(1, 1), "world")
	elem.Markers = append(elem.Markers, ast.MapMarker{
		Lat:     19.4326,
		Lng:     -99.1332,
		Label:   "Café & bar",
		Details: "Open <daily> & \"cheap\"",
	})

	html := RenderElementToHTML(elem, nil, nil)

	if strings.Contains(html, "&amp;amp;") {
		t.Errorf("renderMapBrowser double-escaped marker text; got HTML containing &amp;amp;:\n%s", html)
	}

	if !strings.Contains(html, `data-label="Caf`) || !strings.Contains(html, "&amp; bar") {
		t.Errorf("expected single-escaped label (Café &amp; bar) in output, got:\n%s", html)
	}

	if !strings.Contains(html, "&lt;daily&gt;") {
		t.Errorf("expected single-escaped details (&lt;daily&gt;) in output, got:\n%s", html)
	}
}

// TestRenderMapBrowser_MarkerLabelDetails_NewlinesNormalized guards against
// a regression the #68 fix could otherwise introduce: EscapeHTMLAttribute
// (which used to run on label/details) also collapsed \n/\r to nothing and
// \t to a space so the attribute value stayed on one line.
// ProcessVariablesSecure alone does not do this (it only calls EscapeHTML),
// so simply removing the second EscapeHTMLAttribute call — without also
// keeping its whitespace normalization — would let a literal newline/tab
// in marker text flow unmodified into the data-label/data-details HTML
// attribute, breaking the single-line attribute shape.
func TestRenderMapBrowser_MarkerLabelDetails_NewlinesNormalized(t *testing.T) {
	elem := ast.NewMapElement(diagnostics.NewPosition(1, 1), "world")
	elem.Markers = append(elem.Markers, ast.MapMarker{
		Lat:     19.4326,
		Lng:     -99.1332,
		Label:   "Line one\nLine two\ttabbed",
		Details: "Detail one\r\nDetail two",
	})

	html := RenderElementToHTML(elem, nil, nil)

	if strings.Contains(html, "\n") || strings.Contains(html, "\r") {
		t.Errorf("expected renderMapBrowser to strip literal \\n/\\r from marker attributes, got:\n%q", html)
	}
	if !strings.Contains(html, "Line one") || !strings.Contains(html, "Line two tabbed") {
		t.Errorf("expected newline collapsed and tab converted to space in label, got:\n%q", html)
	}
}
