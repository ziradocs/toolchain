// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestCheckPageOverflow_EmptySelectorIsNoop confirma el contrato de
// PDFOptions.OverflowSelector (issue #175): "" es el default y debe ser un
// no-op puro, sin tocar el contexto de Chrome en absoluto — a diferencia de
// los demás tests de este paquete, no necesita un Chromium real ni se salta
// bajo -short, porque checkPageOverflow retorna antes de llamar a
// emulation/Evaluate cuando selector == "".
func TestCheckPageOverflow_EmptySelectorIsNoop(t *testing.T) {
	r := &ChromiumRenderer{logger: noopChromiumLogger{}}

	if err := r.checkPageOverflow("").Do(context.Background()); err != nil {
		t.Errorf("checkPageOverflow(\"\").Do() = %v, want nil (selector vacío debe ser no-op)", err)
	}
}

// TestRenderHTMLToPDF_OverflowSelector_WarnsWithoutFailingBuild cubre el
// caso real: un elemento cuyo contenido excede su propia caja (overflow
// intencional vía CSS) debe producir un WARNING — vía el logger, no un
// error — y el PDF debe generarse igual. "Nunca fallar el build" es el
// contrato explícito de checkPageOverflow.
func TestRenderHTMLToPDF_OverflowSelector_WarnsWithoutFailingBuild(t *testing.T) {
	r := newTestChromiumRenderer(t)
	logger := &capturingChromiumLogger{}
	r.logger = logger

	// height:50px + overflow:hidden + un párrafo largo garantiza
	// scrollHeight > clientHeight bajo cualquier motor de layout — no
	// depende de calibrar contra el tamaño real de página como #163.
	html := `<!doctype html>
<html><body>
<div class="box" style="height:50px; overflow:hidden; width:200px;">
Contenido deliberadamente largo para forzar overflow vertical dentro de una
caja de altura fija, sin importar el motor de layout ni el tamaño de fuente
por default del navegador.
</div>
</body></html>`

	outputPath := t.TempDir() + "/overflow.pdf"
	opts := DefaultPDFOptions()
	opts.OverflowSelector = ".box"

	if err := r.RenderHTMLToPDF(context.Background(), html, outputPath, opts); err != nil {
		t.Fatalf("RenderHTMLToPDF debe seguir generando el PDF pese al overflow: %v", err)
	}

	if !logger.sawWarnContaining("overflows the page and was clipped") {
		t.Errorf("esperaba un WARNING de overflow en el logger, no se vio ninguno. Warns capturados: %v", logger.warns)
	}
}

// capturingChromiumLogger implementa ChromiumLogger guardando los mensajes
// Warn para que el test pueda inspeccionarlos, en vez de solo confirmar que
// RenderHTMLToPDF no explotó.
type capturingChromiumLogger struct {
	warns []string
}

func (l *capturingChromiumLogger) Info(tag, format string, args ...interface{})  {}
func (l *capturingChromiumLogger) Error(tag, format string, args ...interface{}) {}
func (l *capturingChromiumLogger) Warn(tag, format string, args ...interface{}) {
	l.warns = append(l.warns, tag+": "+fmt.Sprintf(format, args...))
}

func (l *capturingChromiumLogger) sawWarnContaining(substr string) bool {
	for _, w := range l.warns {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}
