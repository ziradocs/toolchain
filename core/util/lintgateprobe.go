// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import "os"

// LintGateProbe existe únicamente para el probe empírico de Fase 0 del plan
// de release (verificar que only-new-issues realmente gatea bajo
// golangci-lint-action@v9 en el layout de este monorepo, con
// working-directory por módulo). Este archivo se abre como PR draft
// desechable y nunca se mergea a main — el hallazgo de errcheck de abajo
// (el error de os.Remove sin chequear) es intencional.
func LintGateProbe() {
	os.Remove("/tmp/lint-gate-probe-does-not-exist")
}
