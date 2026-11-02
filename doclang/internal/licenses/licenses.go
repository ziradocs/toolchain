// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package licenses lleva dentro del binario LICENSE, NOTICE y los textos de
// licencia del código y las fuentes de terceros que el binario embebe
// (mermaid, Chart.js, MathJax y los glifos de New Computer Modern), para que
// el binario solo, sin el archivo del release, los traiga. texts/ es una
// copia de la raíz y de core/renderer/chromium/third_party que regenera
// scripts/sync-licenses.sh; el test falla si se desincroniza.
package licenses

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

//go:embed texts
var texts embed.FS

// Text devuelve LICENSE, NOTICE y cada texto de terceros, con un
// encabezado por archivo.
func Text() string {
	var b strings.Builder
	write := func(name string) {
		data, err := texts.ReadFile("texts/" + name)
		if err != nil {
			return
		}
		b.WriteString("==== " + name + " ====\n\n")
		b.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	write("LICENSE")
	write("NOTICE")
	for _, name := range Files() {
		if strings.HasPrefix(name, "third_party/") {
			write(name)
		}
	}
	return b.String()
}

// Files lista los archivos embebidos, relativos a texts/, en orden.
func Files() []string {
	var names []string
	_ = fs.WalkDir(texts, "texts", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, strings.TrimPrefix(p, "texts/"))
		}
		return nil
	})
	sort.Strings(names)
	return names
}
