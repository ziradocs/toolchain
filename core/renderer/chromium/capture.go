// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// Plazos de las capturas. Antes solo los mapas tenían uno: una página de
// chart cuyo Chart.js no cargaba (sin red, el script venía de jsDelivr) se
// quedaba esperando un data-ready que nunca llegaba, y `doclang build -f pdf`
// o `-f docx` se colgaba sin límite. Con el plazo, un recurso que falta
// termina en un error con un mensaje claro y el documento sale con un aviso
// en vez de no salir.
//
// Son variables y no constantes solo para que los tests puedan acortarlas.
var (
	// captureTimeout acota cada captura de chart, mermaid y math: un render
	// normal tarda 1-3 s (la espera fija de la página es de 1.5 s).
	captureTimeout = 30 * time.Second
	// pdfPrintTimeout acota RenderHTMLToPDF, que imprime el documento
	// entero; es más holgado porque un documento de cientos de páginas
	// tarda en imprimirse.
	pdfPrintTimeout = 5 * time.Minute
)

// newCaptureContext abre un tab nuevo sobre el browser vivo y devuelve un
// contexto para chromedp.Do acotado por timeout y por el ctx del caller.
// El cancel cierra el tab. Mismo armado que ya usaban los mapas
// (WithTimeout sobre el tab, no sobre r.ctx), así que un plazo vencido nunca
// envenena el browser para los renders siguientes.
func (r *ChromiumRenderer) newCaptureContext(callerCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	tabCtx, tabCancel := chromedp.NewContext(r.ctx)
	timeoutCtx, timeoutCancel := context.WithTimeout(tabCtx, timeout)
	runCtx, cancel := withCallerCancel(timeoutCtx, callerCtx)
	return runCtx, func() {
		cancel()
		timeoutCancel()
		tabCancel()
	}
}

// captureError envuelve el error de un chromedp.Do de captura. Si venció el
// plazo propio (y no el ctx del caller), lo dice con el plazo y la causa
// probable, en vez del "context deadline exceeded" pelado.
func captureError(what string, timeout time.Duration, callerCtx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) && (callerCtx == nil || callerCtx.Err() == nil) {
		return fmt.Errorf("%s rendering timed out after %s: the page never signaled it was done (a script or resource it needs did not load)", what, timeout)
	}
	return fmt.Errorf("%s rendering failed: %w", what, err)
}

// renderStateJS es el estado que publican las páginas de captura:
// window.__renderError (string) si algo falló, window.__renderReady (true)
// cuando ya se puede capturar. waitRenderState lo lee por polling.
const renderStateJS = `(() => {
  if (window.__renderError) { return {error: String(window.__renderError)}; }
  if (window.__renderReady === true) { return {ready: true}; }
  return null;
})()`

// renderState es lo que devuelve renderStateJS.
type renderState struct {
	Ready bool   `json:"ready"`
	Error string `json:"error"`
}

// waitRenderState espera a que la página publique listo o error (ver
// renderStateJS) y convierte el error de la página en un error de Go. La
// espera la acota el plazo del contexto (newCaptureContext): una página que
// no publica nada termina en timeout, nunca en una espera sin fin.
func waitRenderState(expr string) chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		st, err := chromedp.Poll[renderState](expr, chromedp.WithPollingInterval(50*time.Millisecond))(ctx, t)
		if err != nil {
			return err
		}
		if st.Error != "" {
			return &pageRenderError{msg: st.Error}
		}
		return nil
	})
}

// store corre la acción a y guarda su valor en dst. Desde chromedp 0.21 una
// acción devuelve su valor en vez de escribirlo en un puntero, y chromedp.Do
// solo acepta pasos sin valor (Action[Void]); con store cada captura sigue
// siendo una sola secuencia de pasos que lee el resultado de uno intermedio.
func store[T any](dst *T, a chromedp.Action[T]) chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		v, err := a(ctx, t)
		if err != nil {
			return err
		}
		*dst = v
		return nil
	})
}

// pageRenderError es un error que reportó la propia página (un script que
// lanzó, un recurso que no cargó). Se distingue del resto para que math
// pueda reintentar con el recurso que faltó (ver RenderMathToSVGWithTheme).
type pageRenderError struct{ msg string }

func (e *pageRenderError) Error() string { return e.msg }

// renderErrorHookJS se inyecta antes de los scripts de la página: cualquier
// excepción no atrapada (p. ej. "Chart is not defined" si el bundle no cargó)
// queda en window.__renderError, y waitRenderState la devuelve de inmediato
// en vez de esperar al plazo. Los avisos "ResizeObserver loop ..." no
// cuentan: Chrome los despacha como error de window, Chart.js usa
// ResizeObserver, y no impiden dibujar.
const renderErrorHookJS = `<script>window.addEventListener('error', function (e) { var m = (e && e.message) ? e.message : 'script error'; if (m.indexOf('ResizeObserver loop') !== -1) { return; } if (!window.__renderError) { window.__renderError = m; } });</script>`

// mapStateJS es renderStateJS para la página de Leaflet, que publica
// window.mapReady en vez de window.__renderReady.
const mapStateJS = `(() => {
  if (window.__renderError) { return {error: String(window.__renderError)}; }
  if (window.mapReady === true) { return {ready: true}; }
  return null;
})()`
