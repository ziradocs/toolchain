// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"net/url"
	"path/filepath"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/include"
)

// rebaseIncludedAssets reescribe las fuentes locales relativas (imagen,
// video/audio y su póster) que vienen de un archivo incluido para que,
// resueltas desde sourceDir como todas las demás, apunten a lo que el autor
// escribió relativo a SU archivo. Sin esto, `![](logo.png)` dentro de
// chap/x.md se buscaba junto al archivo raíz: la expansión de @include es
// textual y el generador solo conoce un directorio base.
//
// El origen de cada elemento sale de cruzar la línea de su posición con los
// orígenes de include.ExpandWithOrigins. No toca fuentes del archivo raíz,
// de un incluido que está en el mismo sourceDir (el resultado sería el
// mismo), remotas, absolutas ni con {{variables}} (se sustituyen después,
// al renderizar, y el valor final no se conoce acá).
//
// No amplía nada: la ruta reescrita pasa por el mismo confinamiento a la
// raíz de imágenes que cualquier otra, así que un ".." que en el incluido
// salía de la raíz sigue saliendo.
func rebaseIncludedAssets(doc *ast.AST, origins include.LineOrigins, sourceDir string) {
	if doc == nil || sourceDir == "" || len(origins) == 0 {
		return
	}
	rebase := func(src string, line int) string {
		file := origins.File(line)
		if file == "" || !isRelativeLocalSource(src) {
			return src
		}
		dir := filepath.Dir(file)
		if dir == sourceDir {
			return src
		}
		rel, err := filepath.Rel(sourceDir, filepath.Join(dir, filepath.FromSlash(src)))
		if err != nil {
			return src
		}
		return filepath.ToSlash(rel)
	}
	_ = ast.Walk(doc, func(n ast.Node) error {
		switch e := n.(type) {
		case *ast.ImageElement:
			e.Source = rebase(e.Source, e.GetPosition().Line)
		case *ast.MediaElement:
			line := e.GetPosition().Line
			e.Source = rebase(e.Source, line)
			e.Poster = rebase(e.Poster, line)
		}
		return nil
	})
}

func isRelativeLocalSource(src string) bool {
	if src == "" || strings.Contains(src, "{{") || strings.HasPrefix(src, "/") || strings.HasPrefix(src, "#") || filepath.IsAbs(src) {
		return false
	}
	u, err := url.Parse(src)
	return err == nil && u.Scheme == ""
}
