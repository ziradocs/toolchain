// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// TestRenderMediaElement_OfflineInlineLocalSource cubre issue #181, el
// equivalente de TestRenderImageElement_OfflineInlineLocalSource (#167)
// para <video>/<audio>: una fuente local bajo offline-inline con AssetRoot
// configurado se lee del disco y se inlinea como data: URI.
func TestRenderMediaElement_OfflineInlineLocalSource(t *testing.T) {
	dir := t.TempDir()
	mp3Bytes := []byte("not-real-mp3-bytes-content-does-not-matter-here")
	if err := os.WriteFile(filepath.Join(dir, "clip.mp3"), mp3Bytes, 0644); err != nil {
		t.Fatal(err)
	}

	pos := diagnostics.NewPosition(1, 1)
	media := ast.NewMediaElement(pos, "audio", "clip.mp3")
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = dir

	got := renderMediaElement(media, nil, ctx)

	if strings.Contains(got, `src="clip.mp3"`) {
		t.Errorf("source was not inlined, still a relative path: %s", got)
	}
	if !strings.Contains(got, `src="data:audio/mpeg;base64,`) {
		t.Errorf("expected an audio/mpeg data: URI, got: %s", got)
	}
	wantB64 := base64Encode(mp3Bytes)
	if !strings.Contains(got, wantB64) {
		t.Errorf("data: URI does not contain the expected base64 payload: %s", got)
	}
}

// TestRenderMediaElement_DefaultBehaviorUnchanged cubre la no-regresión:
// con ctx == nil (el contrato histórico de renderMediaElement, antes de
// #181), una fuente local sigue renderizando tal cual — sin intento de
// inlinear.
func TestRenderMediaElement_DefaultBehaviorUnchanged(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	media := ast.NewMediaElement(pos, "video", "demo.mp4")

	got := renderMediaElement(media, nil, nil)
	want := `<video src="demo.mp4"></video>`
	if got != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

// TestTryInlineLocalMedia_ExceedsCapFallsBack cubre el cap de tamaño de
// issue #181: un archivo por encima de maxInlineMediaBytes no se inlinea
// — a diferencia de TryInlineLocalImage (que no tiene cap, para no
// regresionar #167), TryInlineLocalMedia se degrada al mismo
// comportamiento "no se pudo inlinear" que un archivo faltante o una
// ruta rechazada.
func TestTryInlineLocalMedia_ExceedsCapFallsBack(t *testing.T) {
	dir := t.TempDir()
	bigPath := filepath.Join(dir, "big.mp4")
	// Escribir un archivo escaso (sparse) del tamaño justo por encima del
	// cap — no hace falta escribir bytes reales de más de 64MiB, solo que
	// el tamaño reportado por Stat lo sea.
	f, err := os.Create(bigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxInlineMediaBytes + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()

	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = dir

	if _, ok := TryInlineLocalMedia("big.mp4", ctx); ok {
		t.Errorf("expected ok=false for a file over the %d byte cap", maxInlineMediaBytes)
	}
}

// TestTryInlineLocalMedia_UnderCapInlines confirma el otro lado del cap:
// un archivo justo POR DEBAJO del límite sí se inlinea normalmente —
// para que el test de arriba no esconda un cap puesto en cero o un bug
// que rechace todo.
func TestTryInlineLocalMedia_UnderCapInlines(t *testing.T) {
	dir := t.TempDir()
	data := []byte("small clip")
	if err := os.WriteFile(filepath.Join(dir, "small.mp4"), data, 0644); err != nil {
		t.Fatal(err)
	}

	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = dir

	got, ok := TryInlineLocalMedia("small.mp4", ctx)
	if !ok {
		t.Fatal("expected ok=true for a file well under the cap")
	}
	if !strings.HasPrefix(got, "data:video/mp4;base64,") {
		t.Errorf("expected a video/mp4 data: URI, got: %s", got)
	}
}

// TestTryInlineLocalImage_NoCapRegression confirma que agregar el cap a
// TryInlineLocalMedia (una función NUEVA) no le agregó por accidente un
// cap a TryInlineLocalImage (ya publicada en v2.19.0) — una imagen por
// encima de lo que sería el cap de media debe seguir inlineándose igual
// que siempre.
func TestTryInlineLocalImage_NoCapRegression(t *testing.T) {
	dir := t.TempDir()
	bigPath := filepath.Join(dir, "big.png")
	f, err := os.Create(bigPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxInlineMediaBytes + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()

	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = dir

	if _, ok := TryInlineLocalImage("big.png", ctx); !ok {
		t.Errorf("TryInlineLocalImage must not have gained a size cap — expected ok=true for a file over maxInlineMediaBytes")
	}
}
