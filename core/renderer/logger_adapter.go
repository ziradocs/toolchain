// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"

	"go.ziradocs.com/core/v2/util"
)

// ChromiumLoggerAdapter adapta un util.Logger genérico a ChromiumLogger (y
// también a FetcherLogger, que tiene la misma forma) — fuente única para
// ambos CLIs, que antes reimplementaban este wrapper cada uno por su cuenta
// (issue #124: doclang's htmlLoggerAdapter y slidelang's
// chromiumLogAdapter eran copias divergentes del mismo mecanismo; la de
// doclang además duplicaba el tag — lo pasaba como categoría a Info Y lo
// horneaba en el mensaje al mismo tiempo).
//
// util.Logger.Warn/Error no reciben una categoría separada (a diferencia de
// Info/Debug), así que acá se prefija manualmente en el formato.
type ChromiumLoggerAdapter struct {
	Logger util.Logger
}

func (l *ChromiumLoggerAdapter) Info(tag, format string, args ...interface{}) {
	l.Logger.Info(tag, format, args...)
}

func (l *ChromiumLoggerAdapter) Warn(tag, format string, args ...interface{}) {
	l.Logger.Warn(fmt.Sprintf("[%s] %s", tag, format), args...)
}

func (l *ChromiumLoggerAdapter) Error(tag, format string, args ...interface{}) {
	l.Logger.Error(fmt.Sprintf("[%s] %s", tag, format), args...)
}

// NoopFetcherLogger silencia los logs por-diagrama de los fetchers. Antes
// existía como noOpFetcherLogger sin exportar acá (solo usable dentro de
// este paquete) y se reimplementaba de forma independiente en slidelang
// como quietFetcherLogger (issue #124) — exportada para que ambos CLIs
// compartan una única implementación.
type NoopFetcherLogger struct{}

func (NoopFetcherLogger) Info(tag, format string, args ...interface{})  {}
func (NoopFetcherLogger) Warn(tag, format string, args ...interface{})  {}
func (NoopFetcherLogger) Error(tag, format string, args ...interface{}) {}
