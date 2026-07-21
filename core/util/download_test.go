// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// doGet es un helper de test para obtener un *http.Response real de un
// httptest.Server — las funciones bajo prueba consumen la respuesta ya
// obtenida, no hacen ellas mismas la petición (ver comentario en
// download.go sobre por qué).
func doGet(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url) //nolint:gosec // URL de un httptest.Server local, no user input
	if err != nil {
		t.Fatalf("http.Get failed: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestConsumeResponseToTempFile_AcceptsMatchingSize(t *testing.T) {
	body := []byte("contenido de prueba, nada especial")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	resp := doGet(t, srv.URL)
	tmpPath, written, err := ConsumeResponseToTempFile(resp, tmpDir, "dl-*.tmp", int64(len(body)))
	if err != nil {
		t.Fatalf("ConsumeResponseToTempFile failed: %v", err)
	}
	defer func() { _ = os.Remove(tmpPath) }()

	if written != int64(len(body)) {
		t.Errorf("written = %d, want %d", written, len(body))
	}
	got, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("os.ReadFile failed: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("contenido del temporal = %q, want %q", got, body)
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		t.Fatalf("os.Stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("permisos del temporal = %o, want 0600", info.Mode().Perm())
	}
}

// Issue #75: la respuesta debe rechazarse SIN esperar a que el servidor
// termine de enviar un body arbitrariamente grande — LimitReader(max+1)
// detecta el exceso apenas se supera el cap.
func TestConsumeResponseToTempFile_RejectsOversizedResponse(t *testing.T) {
	oversized := bytes.Repeat([]byte("x"), 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(oversized)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	maxBytes := int64(100) // muy por debajo de los 1024 bytes reales
	resp := doGet(t, srv.URL)
	tmpPath, written, err := ConsumeResponseToTempFile(resp, tmpDir, "dl-*.tmp", maxBytes)

	if err == nil {
		t.Fatal("se esperaba un error por exceso de tamaño")
	}
	if !strings.Contains(err.Error(), "exceeds max allowed size") {
		t.Errorf("mensaje de error inesperado: %v", err)
	}
	if written <= maxBytes {
		t.Errorf("written = %d, se esperaba > maxBytes (%d) para confirmar que se detectó el exceso", written, maxBytes)
	}
	if tmpPath != "" {
		t.Errorf("tmpPath debería estar vacío en error, tiene %q", tmpPath)
	}
	assertTempDirEmpty(t, tmpDir)
}

func TestConsumeResponseToTempFile_RejectsNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	resp := doGet(t, srv.URL)
	tmpPath, _, err := ConsumeResponseToTempFile(resp, tmpDir, "dl-*.tmp", 1024)
	if err == nil {
		t.Fatal("se esperaba un error por status no-200")
	}
	if tmpPath != "" {
		t.Errorf("tmpPath debería estar vacío en error, tiene %q", tmpPath)
	}
	assertTempDirEmpty(t, tmpDir)
}

// assertTempDirEmpty confirma que ConsumeResponseToTempFile limpió su
// propio temporal en el camino de error (issue #75, hallazgo de
// code-review de PR #148) — el caller ya no necesita gestionar ese
// cleanup por su cuenta.
func assertTempDirEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("os.ReadDir failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("se esperaba tmpDir vacío tras el error (el helper debe autolimpiarse), tiene %d entradas", len(entries))
	}
}

// Issue #75: chromium_installer.go necesita alimentar un hasher SHA-256 en
// paralelo a la escritura a disco, sin releer el archivo — extraWriters
// cubre exactamente ese caso.
func TestConsumeResponseToTempFile_TeesToExtraWriters(t *testing.T) {
	body := []byte("contenido para verificar con un hash en paralelo")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	hasher := sha256.New()
	resp := doGet(t, srv.URL)
	tmpPath, _, err := ConsumeResponseToTempFile(resp, tmpDir, "dl-*.tmp", int64(len(body)), hasher)
	if err != nil {
		t.Fatalf("ConsumeResponseToTempFile failed: %v", err)
	}
	defer func() { _ = os.Remove(tmpPath) }()

	want := sha256.Sum256(body)
	got := hasher.Sum(nil)
	if hex.EncodeToString(got) != hex.EncodeToString(want[:]) {
		t.Errorf("hash del extraWriter no coincide con el del body real")
	}
}

func TestConsumeResponseWithLimit_AcceptsMatchingSize(t *testing.T) {
	body := []byte("<svg>diagrama de prueba</svg>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	resp := doGet(t, srv.URL)
	got, err := ConsumeResponseWithLimit(resp, int64(len(body)))
	if err != nil {
		t.Fatalf("ConsumeResponseWithLimit failed: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("contenido = %q, want %q", got, body)
	}
}

func TestConsumeResponseWithLimit_RejectsOversizedResponse(t *testing.T) {
	oversized := bytes.Repeat([]byte("y"), 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(oversized)
	}))
	defer srv.Close()

	resp := doGet(t, srv.URL)
	_, err := ConsumeResponseWithLimit(resp, 100)
	if err == nil {
		t.Fatal("se esperaba un error por exceso de tamaño")
	}
	if !strings.Contains(err.Error(), "exceeds max allowed size") {
		t.Errorf("mensaje de error inesperado: %v", err)
	}
}

func TestConsumeResponseWithLimit_RejectsNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	resp := doGet(t, srv.URL)
	_, err := ConsumeResponseWithLimit(resp, 1024)
	if err == nil {
		t.Fatal("se esperaba un error por status no-200")
	}
}
