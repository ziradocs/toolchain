// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import "strings"

// codeTabWidth es el ancho, en columnas, al que se expande un tab dentro de
// un bloque de código DOCX.
const codeTabWidth = 4

// codeLineSegments prepara una línea de código para el DOCX sin perder su
// espaciado, que en Go, Makefiles o YAML es contenido.
//
// Los tabs se expanden a espacios hasta la siguiente columna múltiplo de
// codeTabWidth. Un <w:tab/> salta al siguiente tab stop del párrafo, que se
// mide en pulgadas y no en columnas: con la sangría izquierda del bloque y el
// cuerpo de la fuente de código, la alineación entre líneas con tabs y líneas
// con espacios dependería del tab stop por defecto de cada procesador. Con
// espacios en una fuente monoespaciada la columna es la misma en Word y en
// LibreOffice. Además docxgo no expone <w:tab/> en un run.
//
// docxgo solo marca xml:space="preserve" cuando el texto del run empieza o
// termina en espacio; sin esa marca un lector puede colapsar los espacios
// repetidos. Por eso la línea se parte en segmentos: cada tramo de dos o
// más espacios abre su propio run, que empieza en espacio y sale con
// preserve, y el texto entre tramos solo lleva espacios sueltos.
func codeLineSegments(line string) []string {
	line = expandTabs(line, codeTabWidth)
	if line == "" {
		return []string{""}
	}
	var segments []string
	start := 0
	for i := 0; i < len(line); {
		if line[i] != ' ' {
			i++
			continue
		}
		j := i
		for j < len(line) && line[j] == ' ' {
			j++
		}
		// Un tramo de 2+ espacios, o espacios al inicio o al final de la
		// línea, empiezan un segmento propio.
		if j-i >= 2 || i == 0 || j == len(line) {
			if i > start {
				segments = append(segments, line[start:i])
			}
			start = i
			// El tramo de espacios se pega al texto que sigue: el run
			// empieza en espacio y docxgo lo marca con preserve.
			k := j
			for k < len(line) && line[k] != ' ' {
				k++
			}
			segments = append(segments, line[i:k])
			start = k
			i = k
			continue
		}
		i = j
	}
	if start < len(line) {
		segments = append(segments, line[start:])
	}
	return segments
}

// expandTabs reemplaza cada tab por los espacios que faltan hasta la
// siguiente columna múltiplo de width, contando columnas por runa.
func expandTabs(line string, width int) string {
	if !strings.ContainsRune(line, '\t') {
		return line
	}
	var b strings.Builder
	col := 0
	for _, r := range line {
		if r == '\t' {
			n := width - col%width
			b.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}
