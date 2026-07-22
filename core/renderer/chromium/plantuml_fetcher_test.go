// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/renderer"
)

func TestIsBlockedFetchTarget(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"::1",             // loopback IPv6
		"10.0.0.1",        // RFC1918
		"172.16.0.1",      // RFC1918
		"192.168.1.1",     // RFC1918
		"169.254.169.254", // cloud metadata (link-local)
		"169.254.0.1",     // link-local
		"0.0.0.0",         // unspecified
		"224.0.0.1",       // multicast
		"not-an-ip",       // unparseable -> fail closed
	}
	for _, ip := range blocked {
		if !isBlockedFetchTarget(ip) {
			t.Errorf("expected %q to be blocked", ip)
		}
	}

	allowed := []string{
		"8.8.8.8",
		"1.1.1.1",
		"93.184.216.34",
	}
	for _, ip := range allowed {
		if isBlockedFetchTarget(ip) {
			t.Errorf("expected %q to be allowed", ip)
		}
	}
}

func TestSSRFSafeDialControlRejectsPrivateAddress(t *testing.T) {
	if err := ssrfSafeDialControl("tcp", "169.254.169.254:80", nil); err == nil {
		t.Fatal("expected metadata address to be rejected")
	}
	if err := ssrfSafeDialControl("tcp", net.JoinHostPort("8.8.8.8", "443"), nil); err != nil {
		t.Fatalf("expected public address to be allowed, got %v", err)
	}
}

func TestFetchWithControlledRedirects_AllowsTrustedInitialServerEvenIfLoopback(t *testing.T) {
	// El servidor apuntado por --plantuml-server es confiable (flag del
	// operador, mismo trust level que --theme) — un servidor self-hosted en
	// localhost/una red privada debe seguir funcionando sin restricción.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<svg></svg>"))
	}))
	defer target.Close()

	fetcher := NewPlantUMLFetcher(target.URL, "svg", t.TempDir())
	resp, err := fetcher.fetchWithControlledRedirects(context.Background(), target.URL)
	if err != nil {
		t.Fatalf("expected direct request to a trusted (loopback) server to succeed, got: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFetchWithControlledRedirects_BlocksRedirectToLoopback(t *testing.T) {
	// El servidor "malicioso" (o comprometido) redirige a otro host loopback
	// para simular un pivot hacia una dirección interna: el redirect debe
	// rechazarse por el cliente restringido, aunque la petición inicial sí
	// se permita.
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectTarget.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer redirector.Close()

	fetcher := NewPlantUMLFetcher(redirector.URL, "svg", t.TempDir())
	_, err := fetcher.fetchWithControlledRedirects(context.Background(), redirector.URL)
	if err == nil {
		t.Fatal("expected redirect to a loopback address to be blocked, got nil error")
	}
}

func TestFetchWithControlledRedirects_BlocksRedirectToNonHTTPS(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/plantuml", http.StatusFound)
	}))
	defer redirector.Close()

	fetcher := NewPlantUMLFetcher(redirector.URL, "svg", t.TempDir())
	_, err := fetcher.fetchWithControlledRedirects(context.Background(), redirector.URL)
	if err == nil {
		t.Fatal("expected redirect to a non-https URL to be blocked, got nil error")
	}
}

func TestFetchWithControlledRedirects_StopsAfterMaxRedirects(t *testing.T) {
	// Cada hop redirige de vuelta a sí mismo — sin el cap, esto colgaría en
	// un loop infinito. Usa TLS real (no solo httptest.NewServer, que es
	// http://) para que el redirect llegue al chequeo de conteo en vez de
	// cortarse antes en el chequeo de scheme https. El test server vive en
	// 127.0.0.1 (loopback), así que se usa un restrictedClient SIN Control
	// (trust-all) para este test específico — la propia protección SSRF
	// (bloquear loopback en el segundo hop en adelante) ya tiene su test
	// dedicado arriba; mezclarla aquí haría que el segundo hop fallara por
	// SSRF antes de llegar al cap de conteo que este test quiere aislar.
	var handler http.HandlerFunc
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler(w, r) }))
	defer server.Close()
	handler = func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server.URL+"/next", http.StatusFound)
	}

	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	trustAllClient := newNoRedirectClient(5*time.Second, nil)
	trustAllClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: pool}
	fetcher := &PlantUMLFetcher{trustedClient: trustAllClient, restrictedClient: trustAllClient}

	_, err := fetcher.fetchWithControlledRedirects(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected redirect loop to be stopped by the max-redirects cap, got nil error")
	}
	if !strings.Contains(err.Error(), "redirects") {
		t.Errorf("expected a redirect-count error, got: %v", err)
	}
}

func TestFetchWithControlledRedirects_PreservesProxyFromEnvironment(t *testing.T) {
	fetcher := NewPlantUMLFetcher("https://example.com/plantuml", "svg", t.TempDir())
	for _, client := range []*http.Client{fetcher.trustedClient, fetcher.restrictedClient} {
		transport, ok := client.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("expected client.Transport to be *http.Transport, got %T", client.Transport)
		}
		if transport.Proxy == nil {
			t.Fatal("expected Transport.Proxy to be set (e.g. http.ProxyFromEnvironment); got nil, which drops HTTP_PROXY/HTTPS_PROXY support")
		}
	}
}

func TestGeneratePlantUMLURLStillValid(t *testing.T) {
	url := renderer.GeneratePlantUMLURL("@startuml\nA->B\n@enduml", "https://example.com/plantuml", "svg")
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	expectedPrefix := "https://example.com/plantuml/svg/"
	if len(url) <= len(expectedPrefix) || url[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("expected URL to start with %q, got %q", expectedPrefix, url)
	}
}

func TestFetchDiagramToAssets_DoesNotCachePartialFileOnSizeCapExceeded(t *testing.T) {
	original := plantumlMaxResponseBytes
	plantumlMaxResponseBytes = 10 // bytes; fuerza el cap a dispararse de inmediato
	defer func() { plantumlMaxResponseBytes = original }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this response is way bigger than the 10-byte test cap"))
	}))
	defer server.Close()

	outputDir := t.TempDir()
	fetcher := NewPlantUMLFetcher(server.URL, "svg", outputDir)

	_, err := fetcher.FetchDiagramToAssets(context.Background(), "@startuml\nA->B\n@enduml")
	if err == nil {
		t.Fatal("expected an oversized-response error, got nil")
	}

	assetsDir := filepath.Join(outputDir, "assets", "diagrams")
	entries, readErr := os.ReadDir(assetsDir)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatalf("failed to read assets dir: %v", readErr)
	}
	for _, e := range entries {
		t.Errorf("expected no leftover files in assetsDir after exceeding the size cap, found: %s", e.Name())
	}
}

// TestFetchDiagramInline_RespectsCallerCancellation es la prueba de
// cancelación que exige issue #134/G1d: un ctx que se cancela ANTES de que
// el servidor responda debe abortar la petición HTTP en curso (no solo
// bloquear indefinidamente hasta el timeout fijo de 30s del cliente) —
// distinto de simplemente verificar que el parámetro ctx existe en la
// firma sin comprobar que en verdad gobierna la cancelación.
func TestFetchDiagramInline_RespectsCallerCancellation(t *testing.T) {
	unblock := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-unblock // nunca responde hasta que el test lo libere (o termine)
	}))
	defer func() {
		close(unblock)
		server.Close()
	}()

	fetcher := NewPlantUMLFetcher(server.URL, "svg", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // ya cancelado antes de la llamada

	done := make(chan error, 1)
	go func() {
		_, err := fetcher.FetchDiagramInline(ctx, "@startuml\nA->B\n@enduml")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error from a request made with an already-canceled context")
		}
		if !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("expected the error to mention context cancellation, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("FetchDiagramInline did not return promptly after ctx was canceled — cancellation is not reaching the HTTP request")
	}
}
