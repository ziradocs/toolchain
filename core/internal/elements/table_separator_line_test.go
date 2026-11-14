// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import "testing"

// IsMarkdownSeparatorLine es la regla de los dos dialectos: separadora si no
// hay nada más que guiones, dos puntos, espacios y "|", con "---" o (entre
// pipes) ":--", "--:" o ":-:". Cualquier otro texto es una fila de datos.
func TestIsMarkdownSeparatorLine(t *testing.T) {
	for _, tc := range []struct {
		line string
		want bool
	}{
		// Separadores de verdad, con y sin pipes de borde y con espacios.
		{"|---|", true},
		{"|---|---|", true},
		{"|:--|--:|", true},
		{"| :-: | --- |", true},
		{"|:---|:---:|---:|---|", true},
		{"| --- | --- ", true},
		{"--- | ---", true},
		{"  |  :--  |  --:  |  ", true},
		{"|\t---\t|\t:-:\t|", true},
		{"| --- | |", true},
		{"---", true},
		{"-----", true},
		{":---:", true},
		{"| --- | :: |", true},
		// Cualquier espacio Unicode cuenta como espacio, como al recortar
		// una celda.
		{"| ---\u00a0| --- |", true},
		{"|\u00a0:--\u00a0|:-:|", true},
		{"|\u3000---\u2003|", true},

		// Filas de datos: cualquier otro texto, aunque tenga guiones.
		{"| 2019 | --- |", false},
		{"| a --- b | c |", false},
		{"| x --- y |", false},
		{"| --- | n/a |", false},
		{"| :-- | x |", false},
		{"| a---b |", false},
		{`| \--- |`, false},
		{"| `---` |", false},
		{"| — | — |", false},
		{"a --- b", false},

		// Lo que no era separador en ninguno de los dos dialectos sigue
		// sin serlo: guiones cortos y líneas sin ninguna de las subcadenas.
		{"| - | - |", false},
		{"|--|--|", false},
		{"| -- | -- |", false},
		{"| | |", false},
		{"|", false},
		{"", false},
		// Sin "|" inicial, las formas con dos puntos y sin "---" tampoco:
		// strict solo tomaba por separador una línea así con "---", y flex
		// no la lee como fila.
		{":--", false},
		{":-:", false},
		{"--:", false},
		{":-- | --:", false},
		{":-- |", false},
		{"| :--", true},
	} {
		if got := IsMarkdownSeparatorLine(tc.line); got != tc.want {
			t.Errorf("IsMarkdownSeparatorLine(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}
