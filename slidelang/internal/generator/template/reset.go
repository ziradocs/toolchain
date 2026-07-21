// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"go.ziradocs.com/slidelang/internal/generator/css"
)

// GetResetCSS retorna solo los estilos CSS de reset
func GetResetCSS() string {
	return css.GetResetCSS()
}
