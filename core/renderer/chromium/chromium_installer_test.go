// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type noopChromiumLogger struct{}

func (noopChromiumLogger) Info(tag, format string, args ...interface{})  {}
func (noopChromiumLogger) Warn(tag, format string, args ...interface{})  {}
func (noopChromiumLogger) Error(tag, format string, args ...interface{}) {}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestGetDownloadURL_UsesPinnedVersionForCurrentPlatform(t *testing.T) {
	if _, ok := chromiumSHA256[chromiumPlatformKey()]; !ok {
		t.Skipf("no pinned checksum for this platform (%s); nothing to verify here", chromiumPlatformKey())
	}

	ci := NewChromiumInstaller(noopChromiumLogger{})
	url, err := ci.getDownloadURL()
	if err != nil {
		t.Fatalf("getDownloadURL returned error: %v", err)
	}
	if !strings.Contains(url, chromiumVersion) {
		t.Errorf("expected URL to contain pinned version %q, got %q", chromiumVersion, url)
	}
}

func TestDownloadFile_RejectsHashMismatch(t *testing.T) {
	if _, ok := chromiumSHA256[chromiumPlatformKey()]; !ok {
		t.Skipf("no pinned checksum for this platform (%s)", chromiumPlatformKey())
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is definitely not a real chrome zip"))
	}))
	defer server.Close()

	ci := NewChromiumInstaller(noopChromiumLogger{})
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "chromium.zip")

	err := ci.downloadFile(context.Background(), server.URL, destPath)
	if err == nil {
		t.Fatal("expected a SHA-256 mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Errorf("expected a SHA-256 mismatch error, got: %v", err)
	}
	if _, statErr := os.Stat(destPath); statErr == nil {
		t.Error("destPath should not exist after a hash mismatch (no partial/corrupt file left behind)")
	}
	entries, _ := os.ReadDir(destDir)
	for _, e := range entries {
		t.Errorf("expected no leftover files in destDir after a failed download, found: %s", e.Name())
	}
}

func TestDownloadFile_RejectsOversizedResponse(t *testing.T) {
	if _, ok := chromiumSHA256[chromiumPlatformKey()]; !ok {
		t.Skipf("no pinned checksum for this platform (%s)", chromiumPlatformKey())
	}

	original := chromiumMaxDownloadBytes
	chromiumMaxDownloadBytes = 10 // bytes; fuerza el cap a dispararse de inmediato
	defer func() { chromiumMaxDownloadBytes = original }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this response is way bigger than the 10-byte test cap"))
	}))
	defer server.Close()

	ci := NewChromiumInstaller(noopChromiumLogger{})
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "chromium.zip")

	err := ci.downloadFile(context.Background(), server.URL, destPath)
	if err == nil {
		t.Fatal("expected a max-size error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds max allowed size") {
		t.Errorf("expected a size-cap error, got: %v", err)
	}
	if _, statErr := os.Stat(destPath); statErr == nil {
		t.Error("destPath should not exist after exceeding the size cap")
	}
}

func TestDownloadFile_AcceptsMatchingHash(t *testing.T) {
	expectedHash, ok := chromiumSHA256[chromiumPlatformKey()]
	if !ok {
		t.Skipf("no pinned checksum for this platform (%s)", chromiumPlatformKey())
	}
	// No descargamos el ZIP real (cientos de MB); en su lugar confirmamos que
	// downloadFile acepta y mueve a destPath un contenido que SÍ produce el
	// hash esperado, sustituyendo temporalmente el mapa por uno con el hash
	// de un payload de prueba conocido.
	testPayload := []byte("known payload for hash-match test")
	testHash := sha256Hex(testPayload)

	originalMap := chromiumSHA256
	chromiumSHA256 = map[string]string{chromiumPlatformKey(): testHash}
	defer func() { chromiumSHA256 = originalMap }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(testPayload)
	}))
	defer server.Close()

	ci := NewChromiumInstaller(noopChromiumLogger{})
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "chromium.zip")

	if err := ci.downloadFile(context.Background(), server.URL, destPath); err != nil {
		t.Fatalf("expected download to succeed with matching hash, got: %v", err)
	}
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected destPath to exist after a successful download: %v", err)
	}
	if string(data) != string(testPayload) {
		t.Error("downloaded content does not match the served payload")
	}
	_ = expectedHash // solo usado para el Skipf de arriba
}

// zipTestEntry describe una entrada a escribir en un zip de prueba.
type zipTestEntry struct {
	name      string
	content   []byte
	isSymlink bool
}

// writeTestZip crea un zip en zipPath a partir de entries; una entry con
// isSymlink=true se escribe con el modo symlink de Unix (el contenido de la
// entry es el "target" del link, tal como lo hace un zip real con symlinks).
func writeTestZip(t *testing.T, zipPath string, entries []zipTestEntry) {
	t.Helper()
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create test zip: %v", err)
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	for _, e := range entries {
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.isSymlink {
			hdr.SetMode(os.ModeSymlink | 0777)
		} else {
			hdr.SetMode(0644)
		}
		writer, err := w.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", e.name, err)
		}
		if _, err := writer.Write(e.content); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", e.name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
}

func TestExtractZip_AllowsSymlinkWithConfinedRelativeTarget(t *testing.T) {
	// Un bundle de framework de macOS legítimo (Chrome for Testing) trae
	// symlinks relativos como "Versions/Current -> A" — deben seguir
	// funcionando, no rechazarse de plano.
	ci := NewChromiumInstaller(noopChromiumLogger{})
	srcDir := t.TempDir()
	destDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "test.zip")

	writeTestZip(t, zipPath, []zipTestEntry{
		{name: "Versions/A/lib.txt", content: []byte("real content")},
		{name: "Versions/Current", content: []byte("A"), isSymlink: true},
	})

	if err := ci.extractZip(zipPath, destDir); err != nil {
		t.Fatalf("expected a confined relative symlink to extract successfully, got: %v", err)
	}
	linkPath := filepath.Join(destDir, "Versions", "Current")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("expected symlink to exist at %s: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to be a symlink", linkPath)
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to read symlink target: %v", err)
	}
	if target != "A" {
		t.Errorf("expected symlink target %q, got %q", "A", target)
	}
}

func TestExtractZip_RejectsChainedSymlinkEscape(t *testing.T) {
	// Ataque de dos symlinks encadenados: la primera entrada (x/a -> ../b)
	// pasa el chequeo de confinamiento porque, LEXICAMENTE, su padre "x"
	// resuelve a destDir/b (confinado). Una vez creada, x/a es un symlink
	// REAL a destDir/b. La segunda entrada (x/a/c -> ../../secret) se
	// valida contra el padre LÉXICO "x/a" (luce confinado: resuelve a
	// destDir/secret... espera, en realidad el ataque depende de que el
	// padre FÍSICO real de x/a/c sea destDir/b, no destDir/x/a, porque el
	// kernel resuelve el symlink x/a al crear/leer x/a/c). Sin el chequeo de
	// ancestro-symlink, extractSymlink solo ve el padre léxico y aprueba un
	// target que en realidad escapa una vez resuelto contra el padre físico.
	ci := NewChromiumInstaller(noopChromiumLogger{})
	srcDir := t.TempDir()
	destDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "test.zip")

	writeTestZip(t, zipPath, []zipTestEntry{
		{name: "b/", isSymlink: false},
		{name: "x/", isSymlink: false},
		{name: "x/a", content: []byte("../b"), isSymlink: true},
		{name: "x/a/c", content: []byte("../../secret"), isSymlink: true},
	})

	err := ci.extractZip(zipPath, destDir)
	if err == nil {
		t.Fatal("expected the chained-symlink escape to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("expected a symlink-ancestor error, got: %v", err)
	}
	// destDir entero debió limpiarse tras el fallo (ver
	// TestExtractZip_CleansUpDestDirOnFailure) — confirmar que no quedó
	// ninguna pieza del ataque en disco.
	entries, _ := os.ReadDir(destDir)
	for _, e := range entries {
		t.Errorf("expected destDir to be cleaned up after rejecting the chained escape, found leftover: %s", e.Name())
	}
}

func TestExtractZip_RejectsSymlinkWithAbsoluteTarget(t *testing.T) {
	ci := NewChromiumInstaller(noopChromiumLogger{})
	srcDir := t.TempDir()
	destDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "test.zip")

	writeTestZip(t, zipPath, []zipTestEntry{
		{name: "evil-link", content: []byte("/etc/passwd"), isSymlink: true},
	})

	err := ci.extractZip(zipPath, destDir)
	if err == nil {
		t.Fatal("expected symlink with an absolute target to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("expected an absolute-target error, got: %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(destDir, "evil-link")); statErr == nil {
		t.Error("symlink entry should not have been written to disk")
	}
}

func TestExtractZip_RejectsSymlinkEscapingViaDotDot(t *testing.T) {
	ci := NewChromiumInstaller(noopChromiumLogger{})
	srcDir := t.TempDir()
	destDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "test.zip")

	writeTestZip(t, zipPath, []zipTestEntry{
		{name: "subdir/escape-link", content: []byte("../../../../etc/passwd"), isSymlink: true},
	})

	err := ci.extractZip(zipPath, destDir)
	if err == nil {
		t.Fatal("expected symlink escaping destDir via '..' to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "escapes") {
		t.Errorf("expected an escapes-destination error, got: %v", err)
	}
}

func TestExtractZip_RejectsOversizedEntry(t *testing.T) {
	originalEntry := chromiumMaxUncompressedEntryBytes
	chromiumMaxUncompressedEntryBytes = 10 // bytes; fuerza el cap a dispararse de inmediato
	defer func() { chromiumMaxUncompressedEntryBytes = originalEntry }()

	ci := NewChromiumInstaller(noopChromiumLogger{})
	srcDir := t.TempDir()
	destDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "test.zip")

	writeTestZip(t, zipPath, []zipTestEntry{
		{name: "big-file.bin", content: []byte("this content is way bigger than the 10-byte test cap")},
	})

	err := ci.extractZip(zipPath, destDir)
	if err == nil {
		t.Fatal("expected oversized entry to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "exceeds max uncompressed size") {
		t.Errorf("expected a per-entry size-cap error, got: %v", err)
	}
}

func TestExtractZip_RejectsOversizedTotal(t *testing.T) {
	originalTotal := chromiumMaxUncompressedTotalBytes
	chromiumMaxUncompressedTotalBytes = 15 // bytes; cada entry individual pasa el cap por-entrada pero no el total
	defer func() { chromiumMaxUncompressedTotalBytes = originalTotal }()

	ci := NewChromiumInstaller(noopChromiumLogger{})
	srcDir := t.TempDir()
	destDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "test.zip")

	writeTestZip(t, zipPath, []zipTestEntry{
		{name: "a.bin", content: []byte("0123456789")},
		{name: "b.bin", content: []byte("0123456789")},
	})

	err := ci.extractZip(zipPath, destDir)
	if err == nil {
		t.Fatal("expected total-size cap to be exceeded, got nil error")
	}
	if !strings.Contains(err.Error(), "exceeds max total uncompressed size") {
		t.Errorf("expected a total-size-cap error, got: %v", err)
	}
}

func TestExtractZip_CleansUpDestDirOnFailure(t *testing.T) {
	// Un install parcial (algunos archivos ya escritos cuando una entrada
	// posterior dispara un cap) no debe dejar un árbol incompleto que
	// IsChromiumInstalled confunda con uno completo.
	originalEntry := chromiumMaxUncompressedEntryBytes
	chromiumMaxUncompressedEntryBytes = 10 // bytes
	defer func() { chromiumMaxUncompressedEntryBytes = originalEntry }()

	ci := NewChromiumInstaller(noopChromiumLogger{})
	srcDir := t.TempDir()
	destDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "test.zip")

	writeTestZip(t, zipPath, []zipTestEntry{
		{name: "chrome", content: []byte("ok")},
		{name: "big-file.bin", content: []byte("this content is way bigger than the 10-byte test cap")},
	})

	if err := ci.extractZip(zipPath, destDir); err == nil {
		t.Fatal("expected extraction to fail on the oversized second entry")
	}

	if _, statErr := os.Stat(destDir); statErr == nil {
		entries, _ := os.ReadDir(destDir)
		for _, e := range entries {
			t.Errorf("expected destDir to be cleaned up after a failed extraction, found leftover: %s", e.Name())
		}
	}
}

func TestExtractZip_TotalCapOvershootIsBoundedByOneByte(t *testing.T) {
	// Antes, el cap total solo se chequeaba DESPUÉS de escribir la entrada
	// completa, permitiendo un overshoot de hasta un cap-por-entrada entero.
	// Con el presupuesto por-llamada acotado al remanente, el overshoot debe
	// quedar acotado a, como mucho, el +1 byte del truco de detección de
	// io.LimitReader.
	originalTotal := chromiumMaxUncompressedTotalBytes
	chromiumMaxUncompressedTotalBytes = 15 // bytes
	defer func() { chromiumMaxUncompressedTotalBytes = originalTotal }()

	ci := NewChromiumInstaller(noopChromiumLogger{})
	srcDir := t.TempDir()
	destDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "test.zip")

	writeTestZip(t, zipPath, []zipTestEntry{
		{name: "a.bin", content: []byte("0123456789")}, // 10 bytes, cabe en el cupo de 15
		{name: "b.bin", content: []byte("0123456789")}, // 10 bytes más, excede el remanente (5)
	})

	err := ci.extractZip(zipPath, destDir)
	if err == nil {
		t.Fatal("expected total-size cap to be exceeded, got nil error")
	}
	if !strings.Contains(err.Error(), "exceeds max total uncompressed size") {
		t.Errorf("expected a total-size-cap error (not a per-entry one), got: %v", err)
	}
}

func TestExtractZip_AcceptsWellFormedZip(t *testing.T) {
	ci := NewChromiumInstaller(noopChromiumLogger{})
	srcDir := t.TempDir()
	destDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "test.zip")

	writeTestZip(t, zipPath, []zipTestEntry{
		{name: "chrome", content: []byte("fake chrome binary")},
		{name: "subdir/lib.so", content: []byte("fake lib")},
	})

	if err := ci.extractZip(zipPath, destDir); err != nil {
		t.Fatalf("expected well-formed zip to extract successfully, got: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(destDir, "chrome"))
	if err != nil {
		t.Fatalf("expected extracted file to exist: %v", err)
	}
	if string(data) != "fake chrome binary" {
		t.Error("extracted content does not match source")
	}
	if _, err := os.Stat(filepath.Join(destDir, "subdir", "lib.so")); err != nil {
		t.Errorf("expected nested file to exist: %v", err)
	}
}
