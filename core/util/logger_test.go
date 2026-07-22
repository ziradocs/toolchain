// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdoutStderr redirige temporalmente os.Stdout/os.Stderr, ejecuta fn,
// y devuelve lo que cada uno capturó.
func captureStdoutStderr(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	origStdout, origStderr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe (stdout) failed: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe (stderr) failed: %v", err)
	}
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origStdout, origStderr }()

	fn()

	if err := outW.Close(); err != nil {
		t.Fatalf("outW.Close failed: %v", err)
	}
	if err := errW.Close(); err != nil {
		t.Fatalf("errW.Close failed: %v", err)
	}

	outBytes, _ := io.ReadAll(outR)
	errBytes, _ := io.ReadAll(errR)
	return string(outBytes), string(errBytes)
}

// Issue #46 (BA-6): Error/Warn/Info/Debug/Progress/Summary deben salir por
// stderr, dejando stdout limpio para el output real del programa.
func TestConsoleLogger_WritesToStderrNotStdout(t *testing.T) {
	logger := NewConsoleLogger(LevelDebug, false)

	stdout, stderr := captureStdoutStderr(t, func() {
		logger.Error("boom")
		logger.Warn("careful")
		logger.Info("CAT", "info line")
		logger.Debug("comp", "debug line")
		logger.Progress("BUILD", "compiling", 50)
		logger.Summary("Build", map[string]interface{}{"files": 3})
	})

	if stdout != "" {
		t.Errorf("stdout debería quedar vacío, tiene: %q", stdout)
	}
	for _, want := range []string{"boom", "careful", "info line", "debug line", "compiling", "Build"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr debería contener %q, tiene: %q", want, stderr)
		}
	}
}

// Issue #46 (BA-6): un \n/\r embebido en un valor interpolado no debe
// producir líneas de log adicionales sin el prefijo del logger (log forging).
func TestConsoleLogger_SanitizesNewlinesInArgs(t *testing.T) {
	logger := NewConsoleLogger(LevelError, false)

	_, stderr := captureStdoutStderr(t, func() {
		logger.Error("archivo inválido: %s", "foo.slidelang\n❌ ERROR: mensaje forjado")
	})

	// Solo debe haber UN salto de línea real: el que el propio Fprintf agrega
	// al final. El \n embebido en el valor de usuario debe llegar como texto
	// escapado (dos caracteres: '\' + 'n'), nunca como salto de línea real —
	// de lo contrario "mensaje forjado" aparecería en su propia línea con
	// pinta de venir del proceso, no de un valor interpolado.
	if got := strings.Count(stderr, "\n"); got != 1 {
		t.Errorf("se esperaba exactamente 1 salto de línea real (el \\n embebido no debe crear uno nuevo), tiene %d, stderr: %q", got, stderr)
	}
	if !strings.Contains(stderr, `foo.slidelang\n❌ ERROR: mensaje forjado`) {
		t.Errorf("el \\n embebido debería aparecer escapado literalmente en la misma línea, stderr: %q", stderr)
	}
}

// Ampliado tras security-review de PR #146: no solo \n/\r ASCII forjan
// líneas — separadores Unicode de línea/párrafo, NEL, y ESC (que arranca
// secuencias de control ANSI capaces de borrar/sobreescribir una línea
// previa en una terminal real) logran el mismo efecto.
func TestConsoleLogger_SanitizesOtherLineBreakingAndControlChars(t *testing.T) {
	logger := NewConsoleLogger(LevelError, false)

	cases := []struct {
		name  string
		value string
	}{
		{"line separator U+2028", "a\u2028b"},
		{"paragraph separator U+2029", "a\u2029b"},
		{"NEL U+0085", "a\u0085b"},
		{"vertical tab", "a\vb"},
		{"form feed", "a\fb"},
		{"ANSI escape", "a\x1b[2K\x1b[1Ab"},
	}

	rawChars := []string{"\u2028", "\u2029", "\u0085", "\v", "\f", "\x1b"}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr := captureStdoutStderr(t, func() {
				logger.Error("valor: %s", tc.value)
			})
			if got := strings.Count(stderr, "\n"); got != 1 {
				t.Errorf("se esperaba exactamente 1 salto de línea real, tiene %d, stderr: %q", got, stderr)
			}
			for _, raw := range rawChars {
				if strings.Contains(stderr, raw) {
					t.Errorf("stderr no debería contener el carácter crudo %U sin escapar: %q", []rune(raw)[0], stderr)
				}
			}
		})
	}
}

// Hallazgo de code-review de PR #146: sanitizeLogArgs solo chequeaba
// arg.(string) — un error (patrón dominante: logger.Warn("fallo: %v", err))
// no es un string en Go, así que pasaba sin sanitizar (%v sobre un error
// solo llama a Error() e imprime eso literal).
func TestConsoleLogger_SanitizesErrorArgs(t *testing.T) {
	logger := NewConsoleLogger(LevelWarn, false)

	_, stderr := captureStdoutStderr(t, func() {
		logger.Warn("fallo: %v", errors.New("boom\n❌ ERROR: mensaje forjado"))
	})

	if got := strings.Count(stderr, "\n"); got != 1 {
		t.Errorf("se esperaba exactamente 1 salto de línea real, tiene %d, stderr: %q", got, stderr)
	}
	if !strings.Contains(stderr, `boom\n❌ ERROR: mensaje forjado`) {
		t.Errorf("el \\n embebido en el error debería aparecer escapado literalmente, stderr: %q", stderr)
	}
}

// Hallazgo de code-review de PR #146: corridas de \r/\r\n consecutivos con
// reemplazos secuenciales podían perder/fusionar caracteres. La versión
// rune-por-rune debe reflejar cada separador sin colapsar información.
func TestConsoleLogger_SanitizeLogValue_HandlesConsecutiveCRLF(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"CR CR LF", "\r\r\n", `\r\n`},
		{"CRLF CRLF", "\r\n\r\n", `\n\n`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeLogValue(tc.input); got != tc.want {
				t.Errorf("sanitizeLogValue(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// Hallazgo de code-review de PR #146: Summary()'s `operation` no se
// sanitizaba, a diferencia de Progress() que sí sanitiza su parámetro
// análogo — inconsistencia entre los 6 métodos.
func TestConsoleLogger_Summary_SanitizesOperation(t *testing.T) {
	logger := NewConsoleLogger(LevelInfo, false)

	_, stderr := captureStdoutStderr(t, func() {
		logger.Summary("Build\n❌ ERROR: forjado", map[string]interface{}{})
	})

	if got := strings.Count(stderr, "\n"); got != 1 {
		t.Errorf("se esperaba exactamente 1 salto de línea real, tiene %d, stderr: %q", got, stderr)
	}
}

func TestConsoleLogger_SetLevel(t *testing.T) {
	logger := NewConsoleLogger(LevelInfo, false)

	logger.SetLevel(LevelDebug)
	// Cannot directly test level without exposing it, but we can test that it doesn't panic
	logger.Debug("test", "test message")
}

func TestConsoleLogger_Error(t *testing.T) {
	logger := NewConsoleLogger(LevelError, false)
	// Just ensure it doesn't panic
	logger.Error("test error")
	logger.Error("test error with args: %d", 42)
}

func TestConsoleLogger_Warn(t *testing.T) {
	logger := NewConsoleLogger(LevelWarn, false)
	logger.Warn("test warning")
	logger.Warn("test warning with args: %s", "test")
}

func TestConsoleLogger_Info(t *testing.T) {
	logger := NewConsoleLogger(LevelInfo, false)
	logger.Info("FILE", "test info")
	logger.Info("PARSE", "test with args: %d", 123)
}

func TestConsoleLogger_Debug(t *testing.T) {
	logger := NewConsoleLogger(LevelDebug, false)
	logger.Debug("component", "debug message")
	logger.Debug("test", "debug with args: %v", []int{1, 2, 3})
}

func TestConsoleLogger_Progress(t *testing.T) {
	logger := NewConsoleLogger(LevelInfo, false)
	logger.Progress("BUILD", "compiling", 50)
	logger.Progress("GEN", "generating", 100)
}

func TestConsoleLogger_Summary(t *testing.T) {
	logger := NewConsoleLogger(LevelInfo, false)
	stats := map[string]interface{}{
		"files":  10,
		"size":   "1.2MB",
		"errors": 0,
	}
	logger.Summary("Build", stats)
}

func TestConsoleLogger_GetCategoryIcon(t *testing.T) {
	logger := NewConsoleLogger(LevelInfo, false)

	tests := []struct {
		category string
		expected string
	}{
		{"FILE", "📁"},
		{"PARSE", "📝"},
		{"AI", "🔍"},
		{"GEN", "🎨"},
		{"LINT", "✅"},
		{"BUILD", "🔨"},
		{"UNKNOWN", "ℹ️"},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			result := logger.getCategoryIcon(tt.category)
			if result != tt.expected {
				t.Errorf("getCategoryIcon(%s) = %s, want %s", tt.category, result, tt.expected)
			}
		})
	}
}

func TestGetCategoryColor(t *testing.T) {
	tests := []struct {
		category string
		expected string
	}{
		{"FILE", "blue"},
		{"PARSE", "green"},
		{"AI", "magenta"},
		{"GEN", "cyan"},
		{"LINT", "yellow"},
		{"BUILD", "red"},
		{"UNKNOWN", "white"},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			result := getCategoryColor(tt.category)
			if result != tt.expected {
				t.Errorf("getCategoryColor(%s) = %s, want %s", tt.category, result, tt.expected)
			}
		})
	}
}

func TestConsoleLogger_CreateProgressBar(t *testing.T) {
	logger := NewConsoleLogger(LevelInfo, false)

	tests := []struct {
		name     string
		progress int
		hasBlock bool
	}{
		{"0 percent", 0, false},
		{"50 percent", 50, true},
		{"100 percent", 100, true},
		{"negative clamped", -10, false}, // Should clamp to 0
		{"over 100 clamped", 150, true},  // Should clamp to 100
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := logger.createProgressBar(tt.progress)
			if tt.hasBlock && !strings.Contains(bar, "█") {
				t.Errorf("createProgressBar(%d) should contain filled blocks", tt.progress)
			}
			// Check character count, not byte length (Unicode characters)
			charCount := strings.Count(bar, "█") + strings.Count(bar, "░")
			if charCount != 20 {
				t.Errorf("createProgressBar(%d) character count = %d, want 20", tt.progress, charCount)
			}
		})
	}
}

func TestConsoleLogger_Colorize(t *testing.T) {
	logger := NewConsoleLogger(LevelInfo, true)

	colored := logger.colorize("test", "red")
	if !strings.Contains(colored, "\033[31m") {
		t.Error("colorize() should contain red color code when colors enabled")
	}

	loggerNoColor := NewConsoleLogger(LevelInfo, false)
	plain := loggerNoColor.colorize("test", "red")
	if plain != "test" {
		t.Errorf("colorize() with colors disabled should return plain text, got %s", plain)
	}
}

func TestInitDefault(t *testing.T) {
	InitDefault(LevelInfo, false)

	if defaultLogger == nil {
		t.Error("InitDefault() should set defaultLogger")
	}
}

func TestGetDefault(t *testing.T) {
	InitDefault(LevelDebug, true)
	logger := GetDefault()

	if logger == nil {
		t.Error("GetDefault() should return non-nil logger after InitDefault()")
	}
}

func TestGlobalLogFunctions(t *testing.T) {
	InitDefault(LevelDebug, false)

	// Test that global functions don't panic
	Error("test error")
	Warn("test warning")
	Info("TEST", "test info")
	Debug("component", "test debug")
	Progress("BUILD", "operation", 50)
	Summary("operation", map[string]interface{}{"test": 1})
	SetLevel(LevelInfo)
}

func TestNoopLogger(t *testing.T) {
	logger := NewNoop()

	// Test that all methods don't panic
	logger.Error("error")
	logger.Warn("warning")
	logger.Info("cat", "info")
	logger.Debug("comp", "debug")
	logger.Progress("stage", "op", 50)
	logger.Summary("op", map[string]interface{}{})
	logger.SetLevel(LevelDebug)
}
