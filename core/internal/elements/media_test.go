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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parser.CanParse(tt.line, tt.mode); got != tt.expected {
				t.Errorf("CanParse(%q, %q) = %v, want %v", tt.line, tt.mode, got, tt.expected)
			}
		})
	}
}

// TestMediaParser_Parse_VideoWithAllAttributes cubre issue #21: la sintaxis
// de marcador de una sola línea debe parsear src (extractAttribute, valor
// entre comillas) y los 4 atributos booleanos (presencia de token, sin
// valor) independientemente unos de otros.
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

// TestMediaParser_Parse_AudioMinimal cubre el tipo "audio" y que atributos
// booleanos ausentes queden en false (no un default sorpresivo a true).
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

// TestMediaParser_Parse_BooleanAttrNotFalsePositiveInSource cubre un caso
// límite de hasBooleanAttribute: un valor de src que CONTIENE el nombre de
// un atributo booleano como substring (p. ej. "controls-demo.mp4") no debe
// activar ese atributo — hasBooleanAttribute compara tokens completos
// (strings.Fields), no substrings.
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
