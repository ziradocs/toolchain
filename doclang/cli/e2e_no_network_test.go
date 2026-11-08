package cli

import (
	"bufio"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Variables con las que el proceso padre le dice al hijo qué construir. Que el
// hijo exista se decide por la primera: vacía = soy el padre.
const (
	noNetChildOutDir   = "ZIRA_E2E_NONET_CHILD_OUT"
	noNetChildRulepack = "ZIRA_E2E_NONET_CHILD_RULEPACK"
	noNetChildFixture  = "ZIRA_E2E_NONET_CHILD_FIXTURE"
	noNetControlHost   = "detector-control.invalid"
)

// TestE2E_NoNetworkEgressOnDefaultBuildPath is E2E scenario #4: the default
// build path (--format html, no offline rendering, no rulepack fetching a
// remote resource) must make zero network calls. This is a POSITIVE test —
// it counts connections through a real listener acting as an HTTP(S) proxy,
// rather than asserting by absence of observation.
//
// The build runs in a CHILD process (this same test binary, re-executed), not
// in the test process. net/http resolves HTTP_PROXY/HTTPS_PROXY once per
// process (sync.Once inside ProxyFromEnvironment), and any package linked
// into the binary may trigger that lookup from an init(): go-sdk >= 1.8.0 does
// (oauthex builds a transport at package level and probes its Proxy func).
// Setting the variables with t.Setenv after that point is silently ignored, so
// an in-process version of this test can only fail, or worse pass vacuously.
// A child started with the variables already in its environment resolves them
// correctly no matter what its dependencies do at init, which makes the test
// independent of dependency upgrades.
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
	if out := os.Getenv(noNetChildOutDir); out != "" {
		runNoNetworkChild(t, out)
		return
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start proxy-recorder listener: %v", err)
	}
	defer func() { _ = ln.Close() }()

	// Registra la primera línea de cada petición que llega al proxy: para HTTP
	// plano es "GET http://host/ HTTP/1.1", para HTTPS "CONNECT host:443 ...".
	// Con eso el test dice A DÓNDE intentó salir el build, no solo cuántas veces.
	var (
		mu    sync.Mutex
		lines []string
	)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				line, _ := bufio.NewReader(conn).ReadString('\n')
				mu.Lock()
				lines = append(lines, strings.TrimSpace(line))
				mu.Unlock()
			}()
		}
	}()

	dir := t.TempDir()
	rulepack := writeToyRulepack(t, dir)
	fixture := writeFixture(t, dir, true)

	proxyURL := "http://" + ln.Addr().String()
	cmd := exec.Command(os.Args[0], "-test.run=^TestE2E_NoNetworkEgressOnDefaultBuildPath$", "-test.count=1")
	cmd.Env = append(noNetBaseEnv(),
		"HTTP_PROXY="+proxyURL,
		"HTTPS_PROXY="+proxyURL,
		"ALL_PROXY="+proxyURL,
		noNetChildOutDir+"="+filepath.Join(dir, "out"),
		noNetChildRulepack+"="+rulepack,
		noNetChildFixture+"="+fixture,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child build process failed: %v\n%s", err, out)
	}

	// Dar tiempo a que el listener termine de anotar las últimas conexiones.
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()

	// Prove the detector is actually armed: the child makes one deliberate
	// request AFTER the build, to a ".invalid" host (RFC 2606, never resolves,
	// so the probe itself cannot reach the real network). If the proxy was not
	// consulted the probe never shows up here and a zero-egress result would
	// mean nothing.
	var unexpected []string
	armed := false
	for _, l := range lines {
		if strings.Contains(l, noNetControlHost) {
			armed = true
			continue
		}
		unexpected = append(unexpected, l)
	}
	if !armed {
		t.Fatalf("proxy detector not armed: the child's control request to %s never reached the proxy (seen: %q), so the zero-egress assertion below would be meaningless", noNetControlHost, lines)
	}
	if len(unexpected) != 0 {
		t.Errorf("expected 0 connections through the proxy during the build, got %d — the default build path made a network call: %q", len(unexpected), unexpected)
	}
}

// runNoNetworkChild es el cuerpo que corre en el proceso hijo: construye con
// los mismos argumentos que el test original y, solo después, dispara la
// petición de control que prueba que el proxy estaba armado.
func runNoNetworkChild(t *testing.T, outDir string) {
	t.Helper()
	rulepack := os.Getenv(noNetChildRulepack)
	fixture := os.Getenv(noNetChildFixture)
	reportOut := filepath.Join(filepath.Dir(outDir), "report.sarif")

	err := runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", fixture, "--format", "html", "--output", outDir,
			"--report", "sarif", "--report-out", reportOut})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if resp, err := http.Get("http://" + noNetControlHost); err == nil {
		_ = resp.Body.Close()
	}
}

// noNetBaseEnv devuelve el entorno del padre sin ninguna variable que altere
// la resolución del proxy. httpproxy.FromEnvironment cae al nombre en
// minúsculas cuando el de mayúsculas está vacío, así que un no_proxy=* ambiente
// (común en algunos CI/shells) sobreviviría si solo se limpiara NO_PROXY: se
// quitan ambas formas. REQUEST_METHOD también: su sola presencia hace que ese
// paquete rechace HTTP_PROXY para peticiones http:// (mitigación httpoxy).
func noNetBaseEnv() []string {
	drop := map[string]bool{
		"HTTP_PROXY": true, "http_proxy": true,
		"HTTPS_PROXY": true, "https_proxy": true,
		"ALL_PROXY": true, "all_proxy": true,
		"NO_PROXY": true, "no_proxy": true,
		"REQUEST_METHOD": true,
	}
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if drop[key] {
			continue
		}
		env = append(env, kv)
	}
	return env
}
