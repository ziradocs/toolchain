// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// LogLevel define los niveles de logging disponibles (estándar de mercado)
type LogLevel int

const (
	LevelError LogLevel = iota // Solo errores críticos
	LevelWarn                  // Advertencias importantes
	LevelInfo                  // Información general
	LevelDebug                 // Información detallada para debugging
)

// Logger interfaz para logging estructurado (estándar de mercado)
type Logger interface {
	Error(message string, args ...interface{})              // Errores críticos
	Warn(message string, args ...interface{})               // Advertencias
	Info(category, message string, args ...interface{})     // Información general
	Debug(component, message string, args ...interface{})   // Información detallada
	Progress(stage, operation string, progress int)         // Barras de progreso
	Summary(operation string, stats map[string]interface{}) // Resúmenes
	SetLevel(level LogLevel)                                // Cambiar nivel
}

// sanitizeLogValue neutraliza, en una sola pasada rune-por-rune (no con
// strings.NewReplacer/ReplaceAll encadenados: sobre corridas de \r/\r\n
// consecutivos, varios reemplazos secuenciales pueden pisarse entre si y
// perder/fusionar caracteres -- p. ej. "\r\r\n" terminaba colapsando a un
// solo \r\n en vez de reflejar los dos separadores originales; hallazgo
// de code-review de PR #146):
//   - \r\n, \n, \r -- forjado clasico de lineas de log adicionales.
//   - U+2028/U+2029/U+0085 (separadores Unicode de linea/parrafo y NEL) --
//     algunos parsers de logs y SIEMs los tratan como salto de linea igual
//     que \n, aunque el byte stream no tenga 0x0A.
//   - \v (0x0B) / \f (0x0C) -- varios terminales (incluidos xterm-compatibles
//     comunes) los renderizan como salto a la siguiente linea.
//   - \x1b (ESC) -- inicio de toda secuencia de control ANSI/VT100.
//     ConsoleLogger.colorize ya escribe codigos ANSI crudos a stderr en modo
//     interactivo (ver mas abajo), asi que stderr se trata como una
//     terminal que SI interpreta esas secuencias -- sin neutralizar ESC, un
//     valor con p. ej. "\x1b[2K\x1b[1A" podria borrar o sobreescribir una
//     linea de log legitima anterior en vez de solo forjar una nueva.
func sanitizeLogValue(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '\r' && i+1 < len(runes) && runes[i+1] == '\n':
			b.WriteString(`\n`)
			i++ // consume tambien el \n del par
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\u2028':
			b.WriteString(`\u2028`)
		case r == '\u2029':
			b.WriteString(`\u2029`)
		case r == '\u0085':
			b.WriteString(`\u0085`)
		case r == '\v':
			b.WriteString(`\v`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\x1b':
			b.WriteString(`\x1b`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sanitizeLogArgs aplica sanitizeLogValue a los argumentos de tipo string
// antes de interpolarlos en una linea de log (issue #46, BA-6). Tambien
// cubre error y fmt.Stringer -- el patron dominante en el codebase es
// `logger.Warn("fallo: %v", err)`, y un `error` (o cualquier Stringer) NO
// es un `string` en Go, asi que un chequeo `arg.(string)` a secas los deja
// pasar sin sanitizar: %v sobre un error solo llama a su Error() e imprime
// eso literal, exactamente el vector de log forging que este fix busca
// cerrar (hallazgo de code-review de PR #146). Para estos casos se
// reemplaza el arg por su representacion ya sanitizada como string -- %v
// sobre un string produce el mismo texto que %v sobre el error/Stringer
// original, solo que limpio. Otros tipos (int, bool, etc.) no producen
// texto arbitrario via formatting y pasan sin cambios.
func sanitizeLogArgs(args []interface{}) []interface{} {
	sanitized := make([]interface{}, len(args))
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			sanitized[i] = sanitizeLogValue(v)
		case error:
			sanitized[i] = sanitizeLogValue(v.Error())
		case fmt.Stringer:
			sanitized[i] = sanitizeLogValue(v.String())
		default:
			sanitized[i] = arg
		}
	}
	return sanitized
}

// ConsoleLogger implementación de Logger para consola
type ConsoleLogger struct {
	level     LogLevel
	useColors bool
	startTime time.Time
}

// NewConsoleLogger crea un nuevo logger de consola
func NewConsoleLogger(level LogLevel, useColors bool) *ConsoleLogger {
	return &ConsoleLogger{
		level:     level,
		startTime: time.Now(),
		useColors: useColors,
	}
}

// SetLevel cambia el nivel de logging
func (l *ConsoleLogger) SetLevel(level LogLevel) {
	l.level = level
}

// Error registra un error crítico.
//
// Issue #46 (BA-6): todos los niveles escriben a os.Stderr, no a stdout —
// stdout queda reservado exclusivamente para el output real del programa
// (p. ej. contenido servido vía `--output -`/stdout en el futuro), y args se
// sanitizan para evitar log forging con \n/\r embebidos en valores de
// usuario (rutas, mensajes de diagnóstico, títulos).
func (l *ConsoleLogger) Error(message string, args ...interface{}) {
	if l.level < LevelError {
		return
	}
	fmt.Fprintf(os.Stderr, "❌ ERROR: %s\n", fmt.Sprintf(message, sanitizeLogArgs(args)...))
}

// Warn registra una advertencia
func (l *ConsoleLogger) Warn(message string, args ...interface{}) {
	if l.level < LevelWarn {
		return
	}
	fmt.Fprintf(os.Stderr, "⚠️  WARNING: %s\n", fmt.Sprintf(message, sanitizeLogArgs(args)...))
}

// Info registra un mensaje informativo categorizado
func (l *ConsoleLogger) Info(category, message string, args ...interface{}) {
	if l.level < LevelInfo {
		return
	}

	icon := l.getCategoryIcon(category)
	formatted := fmt.Sprintf(message, sanitizeLogArgs(args)...)
	if l.useColors {
		fmt.Fprintf(os.Stderr, "%s %s: %s\n", icon, l.colorize(category, getCategoryColor(category)), formatted)
	} else {
		fmt.Fprintf(os.Stderr, "%s %s: %s\n", icon, category, formatted)
	}
}

// Debug registra un mensaje de debug detallado
func (l *ConsoleLogger) Debug(component, message string, args ...interface{}) {
	if l.level < LevelDebug {
		return
	}
	fmt.Fprintf(os.Stderr, "🔧 DEBUG[%s]: %s\n", component, fmt.Sprintf(message, sanitizeLogArgs(args)...))
}

// Progress muestra el progreso de una operación
func (l *ConsoleLogger) Progress(stage, operation string, progress int) {
	if l.level < LevelInfo {
		return
	}

	icon := l.getCategoryIcon(stage)
	bar := l.createProgressBar(progress)
	fmt.Fprintf(os.Stderr, "%s %s: %s [%s] %d%%\n", icon, stage, sanitizeLogValue(operation), bar, progress)
}

// Summary muestra un resumen de la operación
func (l *ConsoleLogger) Summary(operation string, stats map[string]interface{}) {
	if l.level < LevelInfo {
		return
	}

	fmt.Fprintf(os.Stderr, "✅ %s completado en %v\n", sanitizeLogValue(operation), time.Since(l.startTime))
	for key, value := range stats {
		if s, ok := value.(string); ok {
			value = sanitizeLogValue(s)
		}
		fmt.Fprintf(os.Stderr, "   %s: %v\n", key, value)
	}
}

// getCategoryIcon retorna el icono para una categoría específica
func (l *ConsoleLogger) getCategoryIcon(category string) string {
	switch strings.ToUpper(category) {
	case "FILE":
		return "📁"
	case "PARSE":
		return "📝"
	case "AI":
		return "🔍"
	case "GEN":
		return "🎨"
	case "LINT":
		return "✅"
	case "BUILD":
		return "🔨"
	default:
		return "ℹ️"
	}
}

// getCategoryColor retorna el color para una categoría específica
func getCategoryColor(category string) string {
	switch strings.ToUpper(category) {
	case "FILE":
		return "blue"
	case "PARSE":
		return "green"
	case "AI":
		return "magenta"
	case "GEN":
		return "cyan"
	case "LINT":
		return "yellow"
	case "BUILD":
		return "red"
	default:
		return "white"
	}
}

// colorize aplica color al texto si los colores están habilitados
func (l *ConsoleLogger) colorize(text, color string) string {
	if !l.useColors {
		return text
	}

	colors := map[string]string{
		"red":     "\033[31m",
		"green":   "\033[32m",
		"yellow":  "\033[33m",
		"blue":    "\033[34m",
		"magenta": "\033[35m",
		"cyan":    "\033[36m",
		"white":   "\033[37m",
		"reset":   "\033[0m",
	}

	if colorCode, exists := colors[color]; exists {
		return colorCode + text + colors["reset"]
	}
	return text
}

// createProgressBar crea una barra de progreso visual
func (l *ConsoleLogger) createProgressBar(progress int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	barLength := 20
	filled := (progress * barLength) / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLength-filled)
	return bar
}

// Variables globales para logger por defecto — conveniencia de
// slidelang únicamente (issue #134/G1c). Código de librería
// (parser/renderer/elements/ai/linter) NO debe llamar estas funciones ni
// GetDefault(): recibe su logger por inyección explícita (parser.New(log),
// renderer.RenderContext.Logger) precisamente porque un consumidor de
// core como librería (un fuzz harness, doclang, un tercero)
// nunca llama InitDefault() y silenciosamente perdía sus logs contra el
// Noop de acá abajo — parser.NewStrictParser/NewFlexParser y
// renderer.GenerateDocumentHTML dependían de este global hasta G1c, que
// los migró a logger inyectado (ver sus propios comentarios).
//
// Issue #45 (fuzzing): el valor cero de un Logger (interfaz) es nil, y
// InitDefault() no se llama nunca en un fuzz harness. Antes, cada función
// de conveniencia de este archivo se protegía por separado con un
// `if defaultLogger != nil` — GetDefault() era la única que no, y le
// entregaba un Logger nil crudo a cualquier caller, que luego panickeaba al
// invocar un método sobre él sin chequear — encontrado fuzzeando el parser
// directamente, sin pasar por el recover de util.RunGuarded que solo
// protege los call sites del CLI, no la librería en sí. Inicializar acá con
// NewNoop() en vez de dejar el nil implícito resuelve la causa raíz una sola
// vez, sin depender de que cada función (actual o futura) recuerde repetir
// el mismo chequeo.
var defaultLogger Logger = NewNoop()

// InitDefault inicializa el logger por defecto. Solo slidelang lo llama
// (doclang arma sus propios *util.Logger e inyecta directamente, sin
// tocar este global) — ver el comentario de defaultLogger arriba.
func InitDefault(level LogLevel, useColors bool) {
	defaultLogger = NewConsoleLogger(level, useColors)
}

// Funciones de conveniencia para slidelang que usan el logger por
// defecto — no llamar desde código de librería (ver defaultLogger arriba).
func Error(message string, args ...interface{}) {
	defaultLogger.Error(message, args...)
}

func Warn(message string, args ...interface{}) {
	defaultLogger.Warn(message, args...)
}

func Info(category, message string, args ...interface{}) {
	defaultLogger.Info(category, message, args...)
}

func Debug(component, message string, args ...interface{}) {
	defaultLogger.Debug(component, message, args...)
}

func Progress(stage, operation string, progress int) {
	defaultLogger.Progress(stage, operation, progress)
}

func Summary(operation string, stats map[string]interface{}) {
	defaultLogger.Summary(operation, stats)
}

func SetLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

// GetDefault retorna el logger por defecto — nunca nil (ver comentario en
// defaultLogger arriba).
func GetDefault() Logger {
	return defaultLogger
}

// NoopLogger implementa Logger sin hacer nada (para tests)
type NoopLogger struct{}

// NewNoop crea un logger que no hace nada (para tests)
func NewNoop() Logger {
	return &NoopLogger{}
}

func (n *NoopLogger) Error(message string, args ...interface{})              {}
func (n *NoopLogger) Warn(message string, args ...interface{})               {}
func (n *NoopLogger) Info(category, message string, args ...interface{})     {}
func (n *NoopLogger) Debug(component, message string, args ...interface{})   {}
func (n *NoopLogger) Progress(stage, operation string, progress int)         {}
func (n *NoopLogger) Summary(operation string, stats map[string]interface{}) {}
func (n *NoopLogger) SetLevel(level LogLevel)                                {}
