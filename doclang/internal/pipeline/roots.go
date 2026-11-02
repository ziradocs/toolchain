// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Roots son los tres directorios con los que un build resuelve y confina
// lo que lee fuera del archivo de entrada. Todos absolutos.
type Roots struct {
	// SourceDir es desde dónde se resuelven las fuentes de imagen
	// relativas (GenerateOptions.SourceDir).
	SourceDir string
	// AssetRoot es hasta dónde se puede leer una imagen local
	// (GenerateOptions.AssetRoot).
	AssetRoot string
	// IncludeRoot es desde dónde se resuelven los @include, y a la vez
	// hasta dónde pueden leer (PrepareOptions.IncludeRoot): include.Expand
	// usa un solo directorio para las dos cosas, en todos los niveles.
	IncludeRoot string
}

// ResolveRoots arma los Roots de `doclang build` a partir de sus flags
// (--base-dir, --asset-root, --include-root; vacío = no pasado).
//
// El ancla es baseDir si se pasó y, si no, el directorio del archivo de
// entrada. Las tres cosas parten de ella: las imágenes relativas se
// resuelven desde el ancla, y las dos raíces que no se pasaron
// explícitamente valen el ancla. Con --base-dir el archivo de entrada es
// solo un portador del texto (la copia temporal de un buffer sin guardar,
// un manifiesto generado que hace @include de archivos reales): su
// directorio no dice nada del proyecto, así que tampoco debe acotar lo que
// se puede leer. Sin --base-dir el resultado es el de siempre.
//
// baseDir tiene que existir, ser un directorio y quedar dentro de la raíz
// de imágenes, también tras resolver symlinks: si quedara fuera, toda
// fuente relativa sería rechazada y el documento perdería sus imágenes sin
// otro aviso que un warning por imagen. Con baseDir, ancla y raíz de
// imágenes se devuelven con los symlinks resueltos, para que el chequeo
// léxico de util.ResolveConfinedPathFrom las compare en la misma forma.
func ResolveRoots(inputFile, baseDir, assetRoot, includeRoot string) (Roots, error) {
	if baseDir == "" {
		return resolveRootsFrom(filepath.Dir(inputFile), assetRoot, includeRoot)
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return Roots{}, fmt.Errorf("invalid base dir: %w", err)
	}
	info, err := os.Stat(absBase)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Roots{}, fmt.Errorf("invalid base dir: %s does not exist", baseDir)
		}
		return Roots{}, fmt.Errorf("invalid base dir: %w", err)
	}
	if !info.IsDir() {
		return Roots{}, fmt.Errorf("invalid base dir: %s is not a directory", baseDir)
	}
	realBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		return Roots{}, fmt.Errorf("invalid base dir: %w", err)
	}

	roots, err := resolveRootsFrom(realBase, assetRoot, includeRoot)
	if err != nil {
		return Roots{}, err
	}
	if assetRoot != "" {
		realRoot, err := filepath.EvalSymlinks(roots.AssetRoot)
		if err != nil {
			return Roots{}, fmt.Errorf("invalid asset root: %w", err)
		}
		if !within(realBase, realRoot) {
			return Roots{}, fmt.Errorf("base dir %s is outside the asset root %s: relative image sources could never be read; pass an --asset-root that contains it", baseDir, assetRoot)
		}
		roots.AssetRoot = realRoot
	}
	return roots, nil
}

func resolveRootsFrom(anchor, assetRoot, includeRoot string) (Roots, error) {
	if assetRoot == "" {
		assetRoot = anchor
	}
	if includeRoot == "" {
		includeRoot = anchor
	}
	absSource, err := filepath.Abs(anchor)
	if err != nil {
		return Roots{}, fmt.Errorf("invalid input directory: %w", err)
	}
	absAsset, err := filepath.Abs(assetRoot)
	if err != nil {
		return Roots{}, fmt.Errorf("invalid asset root: %w", err)
	}
	absInclude, err := filepath.Abs(includeRoot)
	if err != nil {
		return Roots{}, fmt.Errorf("invalid include root: %w", err)
	}
	return Roots{SourceDir: absSource, AssetRoot: absAsset, IncludeRoot: absInclude}, nil
}

// within dice si path es base o está debajo de ella. Ambas rutas tienen
// que venir ya limpias y en la misma forma (absolutas, symlinks resueltos).
func within(path, base string) bool {
	if path == base {
		return true
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
