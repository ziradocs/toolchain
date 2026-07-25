// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

// TestMediaElement_DecodeRoundTrip cubre issue #21: DecodeElement (el
// dispatcher polimórfico usado por ContentBlock.UnmarshalJSON/DecodeAST, p.
// ej. al reconstruir un *AST desde la salida de un --filter externo, issue
// #240) debe reconocer el discriminador "media" y reconstruir un
// *MediaElement con todos sus campos intactos — antes de agregar el case
// NodeTypeMedia en decode.go, esto fallaba con "unknown element
// discriminator" para cualquier documento que contuviera un MediaElement.
func TestMediaElement_DecodeRoundTrip(t *testing.T) {
	pos := diagnostics.NewPosition(3, 1)
	original := NewMediaElement(pos, "video", "demo.mp4")
	original.Controls = true
	original.Autoplay = true
	original.Loop = true
	original.Muted = true

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	decoded, err := DecodeElement(data)
	if err != nil {
		t.Fatalf("DecodeElement error: %v", err)
	}

	media, ok := decoded.(*MediaElement)
	if !ok {
		t.Fatalf("decoded element is %T, want *MediaElement", decoded)
	}
	if media.MediaType != "video" || media.Source != "demo.mp4" {
		t.Errorf("MediaType/Source = %q/%q, want video/demo.mp4", media.MediaType, media.Source)
	}
	if !media.Controls || !media.Autoplay || !media.Loop || !media.Muted {
		t.Errorf("expected all 4 boolean fields true after round-trip, got Controls=%v Autoplay=%v Loop=%v Muted=%v",
			media.Controls, media.Autoplay, media.Loop, media.Muted)
	}
	if media.GetType() != NodeTypeMedia {
		t.Errorf("GetType() = %q, want %q", media.GetType(), NodeTypeMedia)
	}
}
