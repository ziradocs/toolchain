// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

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

const (
	noNetFlagChildOut  = "ZIRA_E2E_NONET_FLAG_CHILD_OUT"
	noNetFlagChildSrc  = "ZIRA_E2E_NONET_FLAG_CHILD_SRC"
	noNetFlagChildFlag = "ZIRA_E2E_NONET_FLAG_CHILD_NONET"
)

// noNetFlagFixture tiene un diagrama PlantUML y un mapa: los dos bloques que
// el DOCX pedía a terceros aun en los modos offline (issue #472).
const noNetFlagFixture = "---\ntitle: Red\nlang: es\nmode: flex\n---\n\n# Red\n\n" +
	"<<plantuml>>\n@startuml\nA -> B\n@enduml\n<</plantuml>>\n\n" +
	"<<map type=\"city\" zoom=\"11\">>\ncenter: 40.7128, -74.0060\nmarker: 40.7128, -74.0060, \"NY\", \"\", \"blue\"\n<</map>>\n"

// TestE2E_NoNetworkFlagDOCX (issue #472): `doclang build --format docx
// --render-mode offline-inline` pedía el PlantUML a www.plantuml.com y los
// tiles a OpenStreetMap; con --no-network no sale ningún pedido. Mismo
// detector que TestE2E_NoNetworkEgressOnDefaultBuildPath: el build corre en
// un proceso hijo con HTTP(S)_PROXY apuntando a un listener que anota cada
// pedido, y una sonda de control a un host .invalid prueba que el detector
// estaba armado. Sin la bandera, el mismo build sí pasa por el proxy (el
// control de que el test ve lo que dice ver).
func TestE2E_NoNetworkFlagDOCX(t *testing.T) {
	if out := os.Getenv(noNetFlagChildOut); out != "" {
		runNoNetworkFlagChild(t, out)
		return
	}
	for _, noNet := range []bool{false, true} {
		lines := runBehindRecordingProxy(t, noNet)
		armed := false
		var build []string
		for _, l := range lines {
			if strings.Contains(l, noNetControlHost) {
				armed = true
				continue
			}
			build = append(build, l)
		}
		if !armed {
			t.Fatalf("proxy detector not armed (seen %q)", lines)
		}
		if noNet && len(build) != 0 {
			t.Errorf("--no-network build went through the proxy: %q", build)
		}
		if !noNet {
			joined := strings.Join(build, "\n")
			for _, host := range []string{"www.plantuml.com", "openstreetmap.org"} {
				if !strings.Contains(joined, host) {
					t.Errorf("control build without --no-network did not request %s (seen %q)", host, build)
				}
			}
		}
	}
}

// runBehindRecordingProxy corre el build hijo con un proxy que anota la
// primera línea de cada pedido y devuelve esas líneas.
func runBehindRecordingProxy(t *testing.T, noNet bool) []string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
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
	src := filepath.Join(dir, "red.doclang")
	if err := os.WriteFile(src, []byte(noNetFlagFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	proxyURL := "http://" + ln.Addr().String()
	flag := ""
	if noNet {
		flag = "1"
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestE2E_NoNetworkFlagDOCX$", "-test.count=1")
	cmd.Env = append(noNetBaseEnv(),
		"HTTP_PROXY="+proxyURL, "HTTPS_PROXY="+proxyURL, "ALL_PROXY="+proxyURL,
		noNetFlagChildOut+"="+filepath.Join(dir, "out"),
		noNetFlagChildSrc+"="+src,
		noNetFlagChildFlag+"="+flag,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child build (no-network=%v) failed: %v\n%s", noNet, err, out)
	}
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	return append([]string(nil), lines...)
}

func runNoNetworkFlagChild(t *testing.T, outDir string) {
	t.Helper()
	args := []string{"build", os.Getenv(noNetFlagChildSrc), "--format", "docx", "--render-mode", "offline-inline", "--output", outDir}
	if os.Getenv(noNetFlagChildFlag) != "" {
		args = append(args, "--no-network")
	}
	if err := runCLI(t, Options{}, args); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if resp, err := http.Get("http://" + noNetControlHost); err == nil {
		_ = resp.Body.Close()
	}
}
