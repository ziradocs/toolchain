// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"go.ziradocs.com/slidelang/v2/internal/generator/css"
)

// GetResponsiveCSS retorna los estilos CSS responsivos desde los assets
// modulares (assets/css/modules/responsive.css, vía go:embed). Tenía un
// fallback de CSS copiado a mano para cuando la carga fallara — pero
// moduleCSSFiles es un //go:embed de esa ruta exacta, así que la lectura no
// puede fallar en runtime en ningún binario que haya compilado, y el
// literal ya había divergido del archivo real (le faltaba el fix de
// paginación de #163, y ni siquiera tenía el reset del container que la
// otra copia en css/builder.go sí traía). Una copia que solo puede
// desactualizarse es peor que no tenerla — issue #163 encontró tres de
// estas, dos ya divergentes entre sí.
func GetResponsiveCSS() string {
	fileLoader := css.NewCSSFileLoader()
	responsiveCSS, err := fileLoader.LoadResponsiveCSS()
	if err != nil {
		return ""
	}
	return responsiveCSS
}
