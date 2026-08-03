package cli

import (
	"net"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// TestE2E_NoNetworkEgressOnDefaultBuildPath is E2E scenario #4: the default
// build path (--format html, no offline rendering, no rulepack fetching a
// remote resource) must make zero network calls. This is a POSITIVE test —
// it counts connections through a real listener acting as an HTTP(S) proxy,
// rather than asserting by absence of observation.
//
// LIMITATION, declared deliberately: this detector only sees traffic routed
// through net/http honoring HTTP_PROXY/HTTPS_PROXY. That covers the three
// real build-time network paths in this codebase — plantuml_fetcher.go
// explicitly sets Proxy: http.ProxyFromEnvironment, native_map.go uses
// http.DefaultClient, and chromium_installer.go uses the default transport
// — but a raw net.Dial or chromedp's websocket connection would bypass it.
// Neither is reachable from the path this test exercises: they only
// activate under --format pdf, --render-mode offline-*, or
// --install-chromium, none of which are used here.
func TestE2E_NoNetworkEgressOnDefaultBuildPath(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start proxy-recorder listener: %v", err)
	}
	defer func() { _ = ln.Close() }()

	var connections int64
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt64(&connections, 1)
			_ = conn.Close()
		}
	}()

	proxyURL := "http://" + ln.Addr().String()
	t.Setenv("HTTP_PROXY", proxyURL)
	t.Setenv("HTTPS_PROXY", proxyURL)
	t.Setenv("ALL_PROXY", proxyURL)
	// golang.org/x/net/http/httpproxy.FromEnvironment falls back to the
	// lowercase name whenever the uppercase one is empty (getEnvAny), so an
	// ambient no_proxy=* (common in some CI/shell setups) would survive
	// clearing only NO_PROXY and silently disable proxying — clear both
	// cases. REQUEST_METHOD is cleared too: its mere presence makes the same
	// package refuse HTTP_PROXY outright for http:// requests (the httpoxy
	// CGI-vulnerability mitigation), which would fail this test in any
	// environment where it happens to be set for unrelated reasons.
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	t.Setenv("REQUEST_METHOD", "")

	dir := t.TempDir()
	rulepack := writeToyRulepack(t, dir)
	fixture := writeFixture(t, dir, true)

	err = runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", fixture, "--format", "html", "--output", filepath.Join(dir, "out"),
			"--report", "sarif", "--report-out", filepath.Join(dir, "report.sarif")})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	afterBuild := atomic.LoadInt64(&connections)

	// Prove the detector is actually armed: net/http's ProxyFromEnvironment
	// caches the HTTP_PROXY/HTTPS_PROXY lookup process-wide on first use
	// (sync.Once), so if anything in this test binary resolved it before
	// t.Setenv ran above, the env vars set here would be silently ignored
	// and afterBuild==0 would mean nothing. ".invalid" (RFC 2606) guarantees
	// this probe itself never reaches the real network even if the proxy
	// weren't consulted — only proves whether IT WAS consulted.
	if resp, err := http.Get("http://detector-control.invalid"); err == nil {
		_ = resp.Body.Close()
	}
	if atomic.LoadInt64(&connections) == afterBuild {
		t.Fatal("proxy detector not armed (HTTP_PROXY was resolved before this test set it) — the zero-egress assertion below would be meaningless")
	}

	if afterBuild != 0 {
		t.Errorf("expected 0 connections through the proxy during the build, got %d — the default build path made a network call", afterBuild)
	}
}
