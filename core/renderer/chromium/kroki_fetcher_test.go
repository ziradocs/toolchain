// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestKrokiFetcher_PostsSourceAndDiagramType(t *testing.T) {
	var gotPath, gotBody, gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg>ok</svg>"))
	}))
	defer server.Close()

	fetcher := NewKrokiFetcher(server.URL, "mermaid", t.TempDir())
	svg, err := fetcher.FetchInline(context.Background(), "graph TD; A-->B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svg != "<svg>ok</svg>" {
		t.Errorf("got %q", svg)
	}
	if gotPath != "/mermaid/svg" {
		t.Errorf("expected path /mermaid/svg, got %q", gotPath)
	}
	if gotContentType != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %q", gotContentType)
	}
	if gotBody != "graph TD; A-->B" {
		t.Errorf("expected diagram source in body, got %q", gotBody)
	}
}

func TestKrokiFetcher_CachesAcrossInlineAndSave(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg>cached</svg>"))
	}))
	defer server.Close()

	outputDir := t.TempDir()
	fetcher := NewKrokiFetcher(server.URL, "plantuml", outputDir)

	if _, err := fetcher.FetchDiagramInline(context.Background(), "@startuml\nA->B\n@enduml"); err != nil {
		t.Fatalf("unexpected error on inline fetch: %v", err)
	}
	if _, err := fetcher.FetchDiagramToAssets(context.Background(), "@startuml\nA->B\n@enduml"); err != nil {
		t.Fatalf("unexpected error on save fetch: %v", err)
	}

	if calls != 1 {
		t.Errorf("expected a single HTTP call shared across FetchDiagramInline and FetchDiagramToAssets, got %d", calls)
	}
}

func TestKrokiFetcher_FetchDiagramToAssets_WritesUnderAssetsDiagrams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg>plantuml</svg>"))
	}))
	defer server.Close()

	outputDir := t.TempDir()
	fetcher := NewKrokiFetcher(server.URL, "plantuml", outputDir)

	relPath, err := fetcher.FetchDiagramToAssets(context.Background(), "@startuml\nA->B\n@enduml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Dir(relPath) != filepath.Join("assets", "diagrams") {
		t.Errorf("expected path under assets/diagrams, got %q", relPath)
	}
	if _, err := os.Stat(filepath.Join(outputDir, relPath)); err != nil {
		t.Errorf("expected file to exist on disk: %v", err)
	}
}

func TestKrokiFetcher_FetchAndSave_WritesUnderDiagrams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg>mermaid</svg>"))
	}))
	defer server.Close()

	outputDir := t.TempDir()
	fetcher := NewKrokiFetcher(server.URL, "mermaid", outputDir)

	relPath, err := fetcher.FetchAndSave(context.Background(), "graph TD; A-->B", outputDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Dir(relPath) != "diagrams" {
		t.Errorf("expected path under diagrams/, got %q", relPath)
	}
	if _, err := os.Stat(filepath.Join(outputDir, relPath)); err != nil {
		t.Errorf("expected file to exist on disk: %v", err)
	}
}

func TestKrokiFetcher_AllowsTrustedInitialServerEvenIfLoopback(t *testing.T) {
	// Mismo modelo de confianza que PlantUMLFetcher: --kroki-server viene de
	// un flag del operador, así que un servidor on-prem en localhost/una red
	// privada sigue funcionando sin restricción en la petición inicial.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg></svg>"))
	}))
	defer server.Close()

	fetcher := NewKrokiFetcher(server.URL, "mermaid", t.TempDir())
	if _, err := fetcher.FetchInline(context.Background(), "graph TD; A-->B"); err != nil {
		t.Fatalf("expected direct request to a trusted (loopback) server to succeed, got: %v", err)
	}
}

func TestKrokiFetcher_BlocksRedirectToLoopback(t *testing.T) {
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectTarget.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	fetcher := NewKrokiFetcher(redirector.URL, "mermaid", t.TempDir())
	_, err := fetcher.FetchInline(context.Background(), "graph TD; A-->B")
	if err == nil {
		t.Fatal("expected redirect to a loopback address to be blocked, got nil error")
	}
}

// TestKrokiFetcher_DiskCacheIncludesServerInKey cubre un hallazgo de
// code-review: dos KrokiFetcher apuntando a servidores Kroki distintos
// (p. ej. un usuario cambia --kroki-server de staging a producción entre
// builds) pero con el mismo outputDir y el mismo contenido de diagrama
// escribían el MISMO nombre de archivo — el os.Stat de
// FetchDiagramToAssets/FetchAndSave veía el archivo del servidor viejo y
// nunca volvía a pedirlo al servidor nuevo, sirviendo en silencio el SVG
// equivocado. cacheKey ahora incluye f.server.
func TestKrokiFetcher_DiskCacheIncludesServerInKey(t *testing.T) {
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg>from-server-a</svg>"))
	}))
	defer serverA.Close()

	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg>from-server-b</svg>"))
	}))
	defer serverB.Close()

	outputDir := t.TempDir()
	const diagram = "@startuml\nA->B\n@enduml"

	fetcherA := NewKrokiFetcher(serverA.URL, "plantuml", outputDir)
	relPathA, err := fetcherA.FetchDiagramToAssets(context.Background(), diagram)
	if err != nil {
		t.Fatalf("unexpected error fetching from server A: %v", err)
	}
	dataA, err := os.ReadFile(filepath.Join(outputDir, relPathA))
	if err != nil {
		t.Fatalf("failed to read server A output: %v", err)
	}
	if string(dataA) != "<svg>from-server-a</svg>" {
		t.Fatalf("expected server A content, got %q", dataA)
	}

	fetcherB := NewKrokiFetcher(serverB.URL, "plantuml", outputDir)
	relPathB, err := fetcherB.FetchDiagramToAssets(context.Background(), diagram)
	if err != nil {
		t.Fatalf("unexpected error fetching from server B: %v", err)
	}
	dataB, err := os.ReadFile(filepath.Join(outputDir, relPathB))
	if err != nil {
		t.Fatalf("failed to read server B output: %v", err)
	}
	if string(dataB) != "<svg>from-server-b</svg>" {
		t.Fatalf("expected fresh content from server B, got the stale server A file instead: %q", dataB)
	}
	if relPathA == relPathB {
		t.Errorf("expected distinct filenames for the same diagram fetched from different servers, both resolved to %q", relPathA)
	}
}

func TestKrokiFetcher_DefaultsToPublicServer(t *testing.T) {
	fetcher := NewKrokiFetcher("", "mermaid", t.TempDir())
	if fetcher.server != "https://kroki.io" {
		t.Errorf("expected default server https://kroki.io, got %q", fetcher.server)
	}
}
