// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestMediaFigureHTML(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	video := ast.NewMediaElement(pos, "video", "https://example.com/demo.mp4")
	video.Poster = "https://example.com/poster.jpg"
	video.Caption = "Demo <b>{{who}}</b>"
	html := RenderElementToHTML(video, map[string]interface{}{"who": "team"}, nil)
	for _, want := range []string{`poster="https://example.com/poster.jpg"`, `<figure class="media-figure">`, `<figcaption>Demo &lt;b&gt;team&lt;/b&gt;</figcaption>`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in %s", want, html)
		}
	}
	video.Poster = "javascript:alert(1)"
	if html := RenderElementToHTML(video, nil, nil); strings.Contains(html, "poster=") || !strings.Contains(html, "<video") {
		t.Fatalf("dangerous poster not dropped: %s", html)
	}
	audio := ast.NewMediaElement(pos, "audio", "https://example.com/a.mp3")
	audio.Poster = "https://example.com/p.jpg"
	if html := RenderElementToHTML(audio, nil, nil); strings.Contains(html, "poster=") {
		t.Fatalf("audio got a poster: %s", html)
	}
}
