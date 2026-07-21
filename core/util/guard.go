// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// DefaultMaxInputBytes es el límite de tamaño de entrada por defecto aplicado
// antes de parsear. Contiene la amplificación del normalizer AI y cualquier
// loop del parser (ver docs/SECURITY_AUDIT_2026-07.md, hallazgo ME-8).
const DefaultMaxInputBytes = 10 << 20 // 10 MB

// DefaultParseTimeout es el presupuesto de tiempo compartido para el parsing
// (defensa en profundidad; ver docs/SECURITY_AUDIT_2026-07.md, ME-8/BA-5).
const DefaultParseTimeout = 30 * time.Second

// maxAllowedInputBytes acota el resultado de ResolveMaxInputBytes: un valor de
// --max-size o de la env var expresado en MB que, al convertirlo a bytes,
// desbordaría un int (o resultaría en un límite absurdamente alto) se recorta
// a este techo en vez de envolver a un número negativo y rechazar toda entrada.
const maxAllowedInputBytes = 1 << 40 // 1 TB

// CheckInputSize rechaza contenido más grande que max con un error claro,
// antes de que llegue al parser.
func CheckInputSize(size, max int) error {
	if size > max {
		return fmt.Errorf("input file too large: %s exceeds the %s limit (increase the configured max size if this is expected)", SizeString(size), SizeString(max))
	}
	return nil
}

// ResolveMaxInputBytes calcula el límite efectivo de tamaño de entrada según
// prioridad: flag explícito (flagMB > 0) > variable de entorno > default.
func ResolveMaxInputBytes(flagMB int, envVar string) int {
	if flagMB > 0 {
		return mbToBytesClamped(flagMB)
	}
	if v := os.Getenv(envVar); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return mbToBytesClamped(n)
		}
	}
	return DefaultMaxInputBytes
}

// mbToBytesClamped convierte mb a bytes sin desbordar un int: si mb<<20
// excedería maxAllowedInputBytes (o desbordara a negativo), se recorta al
// techo en vez de envolver silenciosamente a un límite negativo/incorrecto.
func mbToBytesClamped(mb int) int {
	if mb > maxAllowedInputBytes>>20 {
		return maxAllowedInputBytes
	}
	return mb << 20
}

// RecoverGuard ejecuta fn y convierte cualquier panic en un error controlado,
// sin exponer stack trace ni rutas absolutas (ver docs/SECURITY_AUDIT_2026-07.md,
// hallazgo BA-5: no existía recover() en todo el repo).
func RecoverGuard(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal error while processing input: %v", r)
		}
	}()
	return fn()
}

// RunWithTimeout ejecuta fn en una goroutine y retorna un error si no termina
// dentro de d. Backstop de defensa en profundidad: como el parser no soporta
// cancelación, la goroutine sigue corriendo en segundo plano tras el timeout
// (queda detached); el cap de tamaño de entrada sigue siendo la defensa
// primaria contra loops o entradas patológicas.
func RunWithTimeout(d time.Duration, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return fmt.Errorf("processing timed out after %s", d)
	}
}

// RunGuarded combina RecoverGuard y RunWithTimeout en un solo boundary: fn
// corre bajo recover() (panics no se propagan) y bajo un presupuesto de
// tiempo d. El orden de composición importa (recover debe ser lo más interno,
// para capturar panics que ocurran dentro de fn incluso si el timeout ya
// venció) — RunGuarded lo fija una sola vez en vez de dejar que cada call
// site lo repita y arriesgue invertirlo.
func RunGuarded(d time.Duration, fn func() error) error {
	return RunWithTimeout(d, func() error {
		return RecoverGuard(fn)
	})
}
