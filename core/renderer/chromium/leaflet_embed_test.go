// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"bufio"
	"context"
	"io/fs"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"go.ziradocs.com/core/v2/renderer"
)

// leafletThirdPartyHosts son los hosts de los que la página de Leaflet
// cargaba el script, la hoja de estilos y los íconos antes de embeberlos
// (issue #472).
var leafletThirdPartyHosts = []string{"unpkg.com", "raw.githubusercontent.com", "cdnjs.cloudflare.com"}

// leafletProbeHost es el host de la sonda de control (RFC 2606, nunca
// resuelve).
const leafletProbeHost = "detector-control.invalid"

func TestEmbeddedLeaflet_MatchesCDNIntegrity(t *testing.T) {
	assertMatchesSRI(t, "leaflet.js", renderer.LeafletCDNScriptTag, embeddedLeafletJS)
	assertMatchesSRI(t, "leaflet.css", renderer.LeafletCDNCSSTag, embeddedLeafletCSS)
}

// TestEmbeddedLeafletMarkers_CoverAllowlist: hay un ícono por cada color
// del allowlist de SanitizeLeafletMarkerColor y ninguno más.
func TestEmbeddedLeafletMarkers_CoverAllowlist(t *testing.T) {
	colors := []string{"black", "blue", "gold", "green", "grey", "orange", "red", "violet", "yellow"}
	for _, c := range colors {
		if renderer.SanitizeLeafletMarkerColor(c) != c {
			t.Fatalf("%s is not in the marker allowlist", c)
		}
		if _, err := embeddedLeafletMarkers.ReadFile("third_party/leaflet-color-markers/img/marker-icon-2x-" + c + ".png"); err != nil {
			t.Errorf("no embedded icon for %s: %v", c, err)
		}
	}
	icons, err := fs.Glob(embeddedLeafletMarkers, "third_party/leaflet-color-markers/img/*.png")
	if err != nil || len(icons) != len(colors) {
		t.Errorf("embedded icons = %v (err %v), want %d", icons, err, len(colors))
	}
}

// TestGenerateLeafletHTML_NoThirdPartyAssets: la página del respaldo de
// mapas lleva Leaflet, su hoja de estilos y los íconos embebidos; no
// nombra ningún host de terceros y su CSP solo admite los tiles.
func TestGenerateLeafletHTML_NoThirdPartyAssets(t *testing.T) {
	r := &ChromiumRenderer{}
	config := renderer.MapConfig{Zoom: 5, Markers: []renderer.MapMarker{{Lat: 1, Lng: 2, Label: "A", Color: "red"}}}
	html := r.generateLeafletHTML(config, 800, 600)
	for _, host := range leafletThirdPartyHosts {
		if strings.Contains(html, host) {
			t.Errorf("map page references %s", host)
		}
	}
	for _, want := range []string{"Leaflet 1.9.4", ".leaflet-container", "iconUrl: 'data:image/png;base64,", "shadowUrl: 'data:image/png;base64,"} {
		if !strings.Contains(html, want) {
			t.Errorf("map page lacks %q", want)
		}
	}
	if strings.Contains(mapRenderCSP, "https://unpkg.com") || !strings.Contains(mapRenderCSP, "script-src 'unsafe-inline'") {
		t.Errorf("CSP still allows a script host: %s", mapRenderCSP)
	}
}

// TestRenderMapToPNG_NoThirdPartyRequests (Chromium real): el respaldo de
// Leaflet, con todo el tráfico de Chromium por un proxy (--proxy-server)
// que anota cada pedido, no le pide nada a unpkg.com, raw.githubusercontent.com ni
// cdnjs.cloudflare.com, y Leaflet carga (el render no falla con "Leaflet
// did not load"). Que Chromium pasaba por el proxy lo prueba una navegación
// de control. Con Leaflet desde unpkg (antes de embeberlo) el proxy
// anotaba CONNECT unpkg.com:443 y el render fallaba.
func TestRenderMapToPNG_NoThirdPartyRequests(t *testing.T) {
	path := requireTestChromiumPath(t)

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

	// Todo el tráfico de Chromium va al proxy, también el de loopback.
	proxy := ln.Addr().String()
	prev := extraAllocatorOptions
	extraAllocatorOptions = []chromedp.ExecAllocatorOption{
		chromedp.ProxyServer("http://" + proxy),
		chromedp.Flag("proxy-bypass-list", "<-loopback>"),
	}
	t.Cleanup(func() { extraAllocatorOptions = prev })
	r, err := NewChromiumRenderer(context.Background(), path, false, noopChromiumLogger{})
	if err != nil {
		t.Fatalf("NewChromiumRenderer: %v", err)
	}
	defer r.Close()

	// Los tiles los descarga Go de un servidor local, fuera del proxy.
	srv, _ := tileTestServer(t, func(string) bool { return true })
	config := tileServerMap(t, srv.URL, "")
	config.Markers = []renderer.MapMarker{{Lat: 19.43, Lng: -99.13, Label: "CDMX", Color: "red"}}
	_, renderErr := r.RenderMapToPNG(context.Background(), config, 400, 300)

	// Sonda de control: un pedido de Chromium que tiene que pasar por el
	// proxy, para probar que el detector veía el tráfico del navegador.
	if err := r.ensureBrowser(context.Background()); err == nil {
		tab, cancel := chromedp.NewContext(r.ctx)
		probeCtx, cancelProbe := context.WithTimeout(tab, 10*time.Second)
		_ = chromedp.Do(probeCtx, chromedp.Navigate("http://"+leafletProbeHost+"/"))
		cancelProbe()
		cancel()
	}

	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	armed := false
	for _, l := range lines {
		if strings.Contains(l, leafletProbeHost) {
			armed = true
		}
		for _, host := range leafletThirdPartyHosts {
			if strings.Contains(l, host) {
				t.Errorf("Leaflet fallback requested %s: %q", host, l)
			}
		}
	}
	if renderErr != nil {
		t.Errorf("Leaflet fallback failed: %v", renderErr)
	}
	if !armed {
		t.Errorf("the control request never reached the proxy (seen %q): the detector did not observe Chromium's traffic", lines)
	}
}
