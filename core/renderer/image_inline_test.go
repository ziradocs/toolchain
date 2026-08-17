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

// TestRenderImageElement_DefaultBehaviorUnchanged covers the regression this
// change must not introduce: with ctx == nil (the vast majority of callers
// — RenderElementToHTML's own doc comment says a nil ctx is a normal,
// supported input), a local image reference still renders exactly as
// before issue #167's fix — verbatim src through SanitizeURL, no inlining
// attempted.
func TestRenderImageElement_DefaultBehaviorUnchanged(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	img := ast.NewImageElement(pos, "assets/logo.png", "Logo")

	got := renderImageElement(img, nil, nil)
	want := `<img src="assets/logo.png" alt="Logo">`
	if got != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

// TestRenderImageElement_BrowserModeUnchanged covers ctx != nil but
// ImageMode left at its "browser" default (or any value other than
// "offline-inline") — same as above, no inlining attempted.
func TestRenderImageElement_BrowserModeUnchanged(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	img := ast.NewImageElement(pos, "assets/logo.png", "Logo")
	ctx := NewDefaultRenderContext() // ImageMode: "browser"

	got := renderImageElement(img, nil, ctx)
	want := `<img src="assets/logo.png" alt="Logo">`
	if got != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

// TestRenderImageElement_OfflineInlineLocalSource covers the actual fix
// (issue #167): a local image reference under offline-inline mode with a
// configured AssetRoot gets read from disk and inlined as a data: URI, so
// it survives a PDF render into about:blank (no base URL to resolve a
// relative src against).
func TestRenderImageElement_OfflineInlineLocalSource(t *testing.T) {
	dir := t.TempDir()
	// A 1x1 transparent PNG, minimal valid bytes — content doesn't matter
	// for this test, only that ReadFile succeeds and the bytes round-trip
	// into the data: URI unmodified.
	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if err := os.WriteFile(filepath.Join(dir, "logo.png"), pngBytes, 0644); err != nil {
		t.Fatal(err)
	}

	pos := diagnostics.NewPosition(1, 1)
	img := ast.NewImageElement(pos, "logo.png", "Logo")
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = dir

	got := renderImageElement(img, nil, ctx)

	if strings.Contains(got, `src="logo.png"`) {
		t.Errorf("source was not inlined, still a relative path: %s", got)
	}
	if !strings.Contains(got, `src="data:image/png;base64,`) {
		t.Errorf("expected an image/png data: URI, got: %s", got)
	}
	// Roundtrip: the exact bytes written to disk must be the ones encoded.
	wantB64 := base64Encode(pngBytes)
	if !strings.Contains(got, wantB64) {
		t.Errorf("data: URI does not contain the expected base64 payload %q: %s", wantB64, got)
	}
}

// TestRenderImageElement_OfflineInlineRemoteSourceNotInlined covers that a
// remote (http/https) image reference is left alone under offline-inline —
// it already resolves fine against a real origin inside about:blank (no
// base-URL problem for an absolute URL), so reading+inlining it would only
// add cost and a needless network dependency at build time.
func TestRenderImageElement_OfflineInlineRemoteSourceNotInlined(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	img := ast.NewImageElement(pos, "https://example.com/logo.png", "Logo")
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = t.TempDir()

	got := renderImageElement(img, nil, ctx)
	want := `<img src="https://example.com/logo.png" alt="Logo">`
	if got != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

// TestRenderImageElement_OfflineInlineMissingFileFallsBack covers a local
// reference that doesn't resolve to a real file (issue #163/#167's own
// analytics_dashboard_presentation_flex.slidelang example, before it was
// fixed) — must degrade to the pre-existing broken-but-harmless
// verbatim-src behavior, not fail the whole element or the build.
func TestRenderImageElement_OfflineInlineMissingFileFallsBack(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	img := ast.NewImageElement(pos, "does-not-exist.png", "Logo")
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = t.TempDir()

	got := renderImageElement(img, nil, ctx)
	want := `<img src="does-not-exist.png" alt="Logo">`
	if got != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

// TestRenderImageElement_OfflineInlinePathEscapeRejected covers that AL-4's
// confinement (util.ResolveConfinedPath) is not weakened by this feature: a
// source that tries to escape AssetRoot via ".." must be rejected exactly
// like the DOCX/PPTX embedding path already rejects it, not silently
// resolved.
func TestRenderImageElement_OfflineInlinePathEscapeRejected(t *testing.T) {
	root := t.TempDir()
	// A real file OUTSIDE root, at a location ../secret.png from root would
	// reach — proves the escape isn't just "the file doesn't exist" but
	// actually blocked.
	parent := filepath.Dir(root)
	secretPath := filepath.Join(parent, "secret.png")
	if err := os.WriteFile(secretPath, []byte("secret"), 0644); err != nil {
		t.Skipf("could not set up escape target outside TempDir: %v", err)
	}
	defer func() {
		if err := os.Remove(secretPath); err != nil {
			t.Logf("cleanup: failed to remove %s: %v", secretPath, err)
		}
	}()

	pos := diagnostics.NewPosition(1, 1)
	img := ast.NewImageElement(pos, "../secret.png", "Logo")
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = root

	got := renderImageElement(img, nil, ctx)
	// The rejected fallback legitimately echoes the literal path the author
	// wrote (it contains "secret" as a filename, same as any other broken
	// reference would) — what must NOT happen is the file's BYTES leaking
	// out as a data: URI, which is the actual signal that the confinement
	// was bypassed.
	if strings.Contains(got, "data:") || strings.Contains(got, base64Encode([]byte("secret"))) {
		t.Errorf("path escape was not blocked, file content outside AssetRoot was inlined: %s", got)
	}
	want := `<img src="../secret.png" alt="Logo">`
	if got != want {
		t.Errorf("got:  %s\nwant: %s (rejected escape falls back to verbatim src, same as a missing file)", got, want)
	}
}

// TestRenderImageElement_OfflineInlineNoAssetRootConfigured covers the
// safety default: ImageMode == "offline-inline" without an explicit
// AssetRoot must not read the filesystem at all — same as "not offline-inline"
// — rather than defaulting to some implicit root a caller didn't ask for.
func TestRenderImageElement_OfflineInlineNoAssetRootConfigured(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	img := ast.NewImageElement(pos, "logo.png", "Logo")
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	// ctx.AssetRoot left at "" deliberately.

	got := renderImageElement(img, nil, ctx)
	want := `<img src="logo.png" alt="Logo">`
	if got != want {
		t.Errorf("got:  %s\nwant: %s", got, want)
	}
}

// TestTryInlineLocalImage_DataURISourceNotReencoded covers that a source
// which is already a data: URI (e.g. produced by some other transform
// upstream) isn't treated as a local path and re-read from "disk" —
// url.Parse gives it a non-empty Scheme ("data"), so it takes the same
// early-out as an http(s) URL.
func TestTryInlineLocalImage_DataURISourceNotReencoded(t *testing.T) {
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-inline"
	ctx.AssetRoot = t.TempDir()

	source := "data:image/png;base64,iVBORw0KGgo="
	_, ok := TryInlineLocalImage(source, ctx)
	if ok {
		t.Errorf("a data: URI source should not be treated as a local path to inline")
	}
}

// TestTryInlineLocalImage_NilLoggerDoesNotPanic covers a code-review finding:
// every other test in this file builds ctx via NewDefaultRenderContext(),
// which sets Logger to util.NewNoop() — none of them exercise a ctx that
// skipped that constructor. TryInlineLocalImage is reachable via two paths
// (renderImageElement above, and slidelang's data/converter.go directly)
// neither of which is guaranteed to have called resolveRenderContext first,
// so a hand-built *RenderContext with Logger left at its nil zero-value must
// not panic on the rejected/missing-file path, where a .Warn() call is made.
func TestTryInlineLocalImage_NilLoggerDoesNotPanic(t *testing.T) {
	ctx := &RenderContext{
		ImageMode: "offline-inline",
		AssetRoot: t.TempDir(),
		// Logger deliberately left unset (nil) — the zero-value a struct
		// literal produces if a caller forgets it, same shape the slidelang
		// literals in offline.go had before their own fix.
	}

	// Missing file: exercises the second ctx.Logger.Warn call.
	if _, ok := TryInlineLocalImage("does-not-exist.png", ctx); ok {
		t.Errorf("expected ok=false for a missing file")
	}

	// Path escape: exercises the first ctx.Logger.Warn call.
	if _, ok := TryInlineLocalImage("../secret.png", ctx); ok {
		t.Errorf("expected ok=false for a path that escapes AssetRoot")
	}
}
