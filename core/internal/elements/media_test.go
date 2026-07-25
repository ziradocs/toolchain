// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func TestMediaParser_CanParse(t *testing.T) {
	parser := &MediaParser{}

	tests := []struct {
		name     string
		line     string
		mode     string
		expected bool
	}{
		{"video marker (flex)", `<<video src="demo.mp4" controls>>`, "flex", true},
		{"audio marker (strict)", `<<audio src="clip.mp3" controls>>`, "strict", true},
		{"plain text", "just some text", "flex", false},
		{"chart marker is not media", "<<chart: bar>>", "flex", false},
		{"bare video marker", "<<video>>", "flex", true},
		// Regression: HasPrefix(trimmed, "<<video") alone also matches an
		// unrelated/typo'd tag like "<<videofoo ...>>" — the prefix must be
		// followed by a word boundary (space, ">>", or end of string).
		{"videofoo is not a video tag", `<<videofoo src="demo.mp4">>`, "flex", false},
		{"audiobook is not an audio tag", `<<audiobook src="demo.mp3">>`, "flex", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parser.CanParse(tt.line, tt.mode); got != tt.expected {
				t.Errorf("CanParse(%q, %q) = %v, want %v", tt.line, tt.mode, got, tt.expected)
			}
		})
	}
}

// TestMediaParser_Parse_VideoWithAllAttributes covers issue #21: the
// single-line marker syntax must parse src (extractAttribute, quoted value)
// and the 4 boolean attributes (token presence, no value) independently of
// one another.
func TestMediaParser_Parse_VideoWithAllAttributes(t *testing.T) {
	parser := &MediaParser{}
	ctx := &ParseContext{
		Mode:  "flex",
		Lines: []string{`<<video src="demo.mp4" controls autoplay loop muted>>`},
	}

	result := parser.Parse(ctx, 0)
	if result.Error != nil {
		t.Fatalf("Parse() error = %v", result.Error)
	}
	if result.ConsumedLines != 1 {
		t.Fatalf("ConsumedLines = %d, want 1 (single-line marker)", result.ConsumedLines)
	}

	media, ok := result.Element.(*ast.MediaElement)
	if !ok {
		t.Fatalf("Element is not MediaElement: %+v", result.Element)
	}
	if media.MediaType != "video" {
		t.Errorf("MediaType = %q, want %q", media.MediaType, "video")
	}
	if media.Source != "demo.mp4" {
		t.Errorf("Source = %q, want %q", media.Source, "demo.mp4")
	}
	if !media.Controls || !media.Autoplay || !media.Loop || !media.Muted {
		t.Errorf("expected all 4 boolean attrs true, got Controls=%v Autoplay=%v Loop=%v Muted=%v",
			media.Controls, media.Autoplay, media.Loop, media.Muted)
	}
}

// TestMediaParser_Parse_AudioMinimal covers the "audio" type and that
// absent boolean attributes stay false (no surprise default of true).
func TestMediaParser_Parse_AudioMinimal(t *testing.T) {
	parser := &MediaParser{}
	ctx := &ParseContext{
		Mode:  "flex",
		Lines: []string{`<<audio src="nota.mp3">>`},
	}

	result := parser.Parse(ctx, 0)
	media := result.Element.(*ast.MediaElement)

	if media.MediaType != "audio" {
		t.Errorf("MediaType = %q, want %q", media.MediaType, "audio")
	}
	if media.Source != "nota.mp3" {
		t.Errorf("Source = %q, want %q", media.Source, "nota.mp3")
	}
	if media.Controls || media.Autoplay || media.Loop || media.Muted {
		t.Errorf("expected all boolean attrs false when absent, got Controls=%v Autoplay=%v Loop=%v Muted=%v",
			media.Controls, media.Autoplay, media.Loop, media.Muted)
	}
}

// TestMediaParser_Parse_BooleanAttrNotFalsePositiveInSource covers a
// hasBooleanAttribute edge case: a src value that CONTAINS a boolean
// attribute's name as a substring (e.g. "controls-demo.mp4") must not
// activate that attribute — hasBooleanAttribute compares whole tokens
// (strings.Fields), not substrings.
func TestMediaParser_Parse_BooleanAttrNotFalsePositiveInSource(t *testing.T) {
	parser := &MediaParser{}
	ctx := &ParseContext{
		Mode:  "flex",
		Lines: []string{`<<video src="controls-demo.mp4">>`},
	}

	result := parser.Parse(ctx, 0)
	media := result.Element.(*ast.MediaElement)

	if media.Controls {
		t.Errorf("Controls = true, want false: %q should not match \"controls\" as a substring of the src value", media.Source)
	}
}

// TestMediaParser_Parse_BooleanAttrNotFalsePositiveInQuotedSrcWithSpaces
// covers the fix for a real false positive: unlike the substring case above
// (already handled by comparing whole tokens), a quoted src value that
// CONTAINS a boolean attribute's name as a separate word — e.g.
// `src="my controls video.mp4"`, a plausible filename — used to tokenize
// to a standalone "controls" token and set Controls=true even though the
// author never declared that attribute. Quoted values must be stripped
// before tokenizing.
func TestMediaParser_Parse_BooleanAttrNotFalsePositiveInQuotedSrcWithSpaces(t *testing.T) {
	parser := &MediaParser{}
	ctx := &ParseContext{
		Mode:  "flex",
		Lines: []string{`<<video src="my controls video.mp4">>`},
	}

	result := parser.Parse(ctx, 0)
	media := result.Element.(*ast.MediaElement)

	if media.Source != "my controls video.mp4" {
		t.Fatalf("Source = %q, want %q", media.Source, "my controls video.mp4")
	}
	if media.Controls {
		t.Errorf("Controls = true, want false: the word \"controls\" inside the quoted src value must not be mistaken for the boolean attribute")
	}
}

// TestMediaParser_Parse_BooleanAttrOutsideQuotesStillDetected is the
// counterpart regression guard: stripping quoted spans must not eat a real
// boolean attribute that legitimately follows a quoted src containing
// another attribute-like word.
func TestMediaParser_Parse_BooleanAttrOutsideQuotesStillDetected(t *testing.T) {
	parser := &MediaParser{}
	ctx := &ParseContext{
		Mode:  "flex",
		Lines: []string{`<<video src="my controls video.mp4" autoplay muted>>`},
	}

	result := parser.Parse(ctx, 0)
	media := result.Element.(*ast.MediaElement)

	if !media.Autoplay || !media.Muted {
		t.Errorf("expected Autoplay and Muted true (declared outside the quoted src), got Autoplay=%v Muted=%v", media.Autoplay, media.Muted)
	}
	if media.Controls {
		t.Errorf("Controls = true, want false: only the word inside the quoted src, never actually declared")
	}
}
