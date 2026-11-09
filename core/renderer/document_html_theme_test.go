// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

func TestGenerateThemeVariables_DropsRejectedValuesKeepsValidOnes(t *testing.T) {
	css := generateThemeVariables(map[string]string{
		"--primary-color": "#ff0000",
		"--evil":          `red; } </style><script>alert(1)</script>`,
	}, util.NewNoop())

	if !strings.Contains(css, "--primary-color: #ff0000;") {
		t.Errorf("expected valid variable to be present in generated CSS, got: %s", css)
	}
	if strings.Contains(css, "</style>") || strings.Contains(css, "<script>") {
		t.Errorf("expected rejected variable's dangerous value to be entirely absent from generated CSS, got: %s", css)
	}
	if strings.Contains(css, "--evil") {
		t.Errorf("expected rejected variable name to be dropped from generated CSS, got: %s", css)
	}
}

// spyLogger captura las llamadas a Warn/Debug para verificar que
// generateThemeVariables/GenerateDocumentHTML reportan a través del logger
// inyectado explícitamente (ctx.Logger, issue #134/G1c) en vez de depender
// del logger global de conveniencia del CLI (util.Warn/util.Debug), que
// antes se perdía en silencio para cualquier caller (como doclang) que
// nunca llama util.InitDefault.
type spyLogger struct {
	util.NoopLogger
	warnings []string
}

func (s *spyLogger) Warn(message string, args ...interface{}) {
	s.warnings = append(s.warnings, fmt.Sprintf(message, args...))
}

func TestGenerateThemeVariables_ReportsRejectedVariableToInjectedLogger(t *testing.T) {
	spy := &spyLogger{}
	generateThemeVariables(map[string]string{
		"--evil": `red; } </style><script>alert(1)</script>`,
	}, spy)

	if len(spy.warnings) != 1 {
		t.Fatalf("expected exactly 1 warning reported to the injected logger, got %d: %v", len(spy.warnings), spy.warnings)
	}
	if !strings.Contains(spy.warnings[0], "--evil") {
		t.Errorf("expected warning to name the rejected variable, got: %s", spy.warnings[0])
	}
}

func TestGenerateDocumentHTML_UsesCtxLoggerNotGlobal(t *testing.T) {
	spy := &spyLogger{}
	ctx := NewDefaultRenderContext()
	ctx.Logger = spy

	doc := &ast.AST{}
	opts := DocumentHTMLOptions{
		ThemeVariables: map[string]string{
			"--evil": `red; } </style><script>alert(1)</script>`,
		},
	}
	GenerateDocumentHTML(doc, opts, ctx)

	if len(spy.warnings) != 1 {
		t.Fatalf("expected GenerateDocumentHTML to report the rejected theme variable to ctx.Logger, got %d warnings: %v", len(spy.warnings), spy.warnings)
	}
}

// Las copias internas del CSS (--_doclang-*) no se aceptan de un tema: en
// :root, --_doclang-ink-danger pisaría la tinta danger de todas las
// superficies (1.0:1 con el color de su fondo).
func TestGenerateThemeVariables_DropsInternalVariables(t *testing.T) {
	spy := &spyLogger{}
	css := generateThemeVariables(map[string]string{
		"--doclang-danger-color":   "#dc2626",
		"--_doclang-ink-danger":    "#fff5f5",
		"--_doclang-table-link":    "#ffffff",
		"--doclang-table-ink-info": "#1e40af",
	}, spy)
	for _, gone := range []string{"--_doclang-ink-danger", "--_doclang-table-link"} {
		if strings.Contains(css, gone) {
			t.Errorf("%s llegó al CSS: %s", gone, css)
		}
	}
	for _, kept := range []string{"--doclang-danger-color: #dc2626;", "--doclang-table-ink-info: #1e40af;"} {
		if !strings.Contains(css, kept) {
			t.Errorf("falta %s en %s", kept, css)
		}
	}
	if len(spy.warnings) != 2 {
		t.Errorf("se esperaban 2 avisos, hubo %d: %v", len(spy.warnings), spy.warnings)
	}
}
