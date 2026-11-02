// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// coreBlocksRemoteImages dice si el core con el que se compila ya tiene el
// opt-in de imágenes remotas (RenderContext.SetAllowRemoteImages); contra
// uno publicado que no lo tiene, el PDF las sigue pidiendo y el test se salta.
func coreBlocksRemoteImages() bool {
	_, ok := any(renderer.NewDefaultRenderContext()).(remoteImagesSetter)
	return ok
}

// TestPDF_RemoteImagesNotFetchedByDefault (Chromium real): el PDF se imprime
// en esta máquina, así que una imagen http(s) de un documento no confiable
// le hacía pedir esa URL. Sin AllowRemoteImages el servidor no recibe ningún
// pedido (ni del bloque ni de las inline) y quedan cuatro avisos; con el
// opt-in ninguna imagen se bloquea.
func TestPDF_RemoteImagesNotFetchedByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Chromium")
	}
	if !coreBlocksRemoteImages() {
		t.Skip("this core has no remote-image opt-in")
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngOfSize(t, 8, 8))
	}))
	defer srv.Close()

	src := "---\ntitle: R\nlang: es\nmode: flex\n---\n\n# R\n\n![bloque](" + srv.URL + "/a.png)\n\n" +
		"Texto ![p](" + srv.URL + "/p.png) y\n\n- ítem ![l](" + srv.URL + "/l.png)\n\n| A |\n|---|\n| ![c](" + srv.URL + "/c.png) |\n"
	for _, allow := range []bool{false, true} {
		hits.Store(0)
		dir := t.TempDir()
		out := filepath.Join(t.TempDir(), "out.pdf")
		logger := newTestLogger()
		err := New(logger).Generate(parseInline(t, src), out, GeneratorOptions{Format: "pdf", AssetRoot: dir, SourceDir: dir, AllowRemoteImages: allow})
		if err != nil {
			if strings.Contains(err.Error(), "hromium") {
				t.Skipf("no Chromium: %v", err)
			}
			t.Fatal(err)
		}
		// Con el opt-in no se cuenta: Chromium no deja que una página
		// about:blank pida a 127.0.0.1 (acceso a red local), así que acá el
		// servidor no recibe nada de todos modos. La prueba con el opt-in
		// es la manual, contra una IP no loopback (ver el PR).
		if n := hits.Load(); !allow && n != 0 {
			t.Errorf("default: the server got %d requests, want 0", n)
		}
		if allow {
			for _, w := range logger.warns {
				if strings.Contains(w, "remote image") {
					t.Errorf("with AllowRemoteImages no image should be blocked: %s", w)
				}
			}
		} else {
			warned := 0
			for _, w := range logger.warns {
				if strings.Contains(w, "remote image") && strings.Contains(w, "--allow-remote-images") {
					warned++
				}
			}
			if warned != 4 {
				t.Errorf("want 4 remote-image warnings, got %d: %v", warned, logger.warns)
			}
		}
	}
}
