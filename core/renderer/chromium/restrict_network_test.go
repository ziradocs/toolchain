// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"go.ziradocs.com/core/v2/renderer"
)

// lanIPv4 es una dirección IPv4 de esta máquina que no es loopback, o "".
func lanIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() && !n.IP.IsLinkLocalUnicast() {
			return n.IP.String()
		}
	}
	return ""
}

// TestRestrictNetwork (Chromium real): con RestrictNetwork, Chrome no llega
// a un servidor en una IP de la red (no loopback) aunque se le pida navegar
// ahí, y un render sigue funcionando (los scripts van embebidos). Sin
// RestrictNetwork, la misma navegación sí llega: es el control.
func TestRestrictNetwork(t *testing.T) {
	path := requireTestChromiumPath(t)
	ip := lanIPv4()
	if ip == "" {
		t.Skip("no non-loopback IPv4 address on this machine")
	}
	ln, err := net.Listen("tcp", ip+":0")
	if err != nil {
		t.Skipf("cannot listen on %s: %v", ip, err)
	}
	var hits atomic.Int32
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("<html>ok</html>"))
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	target := "http://" + ln.Addr().String() + "/"

	for _, restrict := range []bool{false, true} {
		hits.Store(0)
		r, err := NewChromiumRenderer(context.Background(), path, false, noopChromiumLogger{})
		if err != nil {
			t.Fatal(err)
		}
		if restrict {
			if err := r.RestrictNetwork(); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.ensureBrowser(context.Background()); err != nil {
			r.Close()
			t.Fatal(err)
		}
		tab, cancel := chromedp.NewContext(r.ctx)
		navCtx, cancelNav := context.WithTimeout(tab, 10*time.Second)
		_ = chromedp.Do(navCtx, chromedp.Navigate(target))
		cancelNav()
		cancel()
		if restrict {
			if hits.Load() != 0 {
				t.Errorf("with RestrictNetwork Chrome reached %s (%d requests)", target, hits.Load())
			}
			if _, err := r.RenderMermaidToPNGWithTheme(context.Background(), "flowchart LR\n A --> B", 400, 300, renderer.DiagramThemeColors{}); err != nil {
				t.Errorf("render with RestrictNetwork failed: %v", err)
			}
			if err := r.RestrictNetwork(); err == nil || !strings.Contains(err.Error(), "before the browser starts") {
				t.Errorf("RestrictNetwork after start: err = %v", err)
			}
		} else if hits.Load() == 0 {
			t.Errorf("control: Chrome without RestrictNetwork did not reach %s", target)
		}
		r.Close()
	}
}
