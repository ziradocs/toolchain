// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// ErrPathEscapesBase indica que la ruta resultante no queda confinada dentro
// del directorio base permitido.
var ErrPathEscapesBase = errors.New("path escapes the allowed base directory")

// ResolveConfinedPath confina userPath (contenido no confiable: nombre de
// tema, fuente de imagen, etc.) dentro de base. Rechaza rutas absolutas y
// cualquier resultado que, tras filepath.Join+Clean, quede fuera de base —
// filepath.Join colapsa ".." de forma puramente léxica, así que suficientes
// ".." pueden escapar la base por completo antes de llegar aquí; ese es
// justo el caso que esta función detecta comparando el resultado final
// contra base, no solo el userPath de entrada.
//
// La comprobación léxica por sí sola no basta: un symlink DENTRO de base
// cuyo destino apunte fuera (p. ej. un documento compartido en un zip/
// carpeta junto a un "logo.png" que en realidad es un symlink a
// /etc/passwd) la evadiría por completo. Si el resultado existe, se
// resuelven los symlinks de ambos lados (filepath.EvalSymlinks) y se
// re-verifica el confinamiento contra la base real — necesario también
// porque la propia base puede llegar a través de un symlink en sistemas
// donde un directorio común lo es (p. ej. /tmp -> /private/tmp en macOS),
// así que hay que resolver ambos lados simétricamente. Si el resultado
// todavía no existe, no hay symlink que seguir: la posterior lectura del
// llamador fallará por su cuenta con "not found", que no es un vector de
// fuga de datos.
//
// Ver docs/SECURITY_AUDIT_2026-07.md, AL-4 (DOCX lee imágenes fuera del
// árbol del documento), AL-5 (PDF incluye archivos locales vía <img src>
// absoluto) y ME-2 (path traversal por nombre de tema).
func ResolveConfinedPath(base, userPath string) (string, error) {
	if userPath == "" {
		return "", fmt.Errorf("%w: empty path", ErrPathEscapesBase)
	}
	if filepath.IsAbs(userPath) {
		return "", fmt.Errorf("%w: absolute path not allowed: %q", ErrPathEscapesBase, userPath)
	}

	cleanBase := filepath.Clean(base)
	joined := filepath.Join(cleanBase, userPath)

	if !isWithin(joined, cleanBase) {
		return "", fmt.Errorf("%w: %q escapes %q", ErrPathEscapesBase, userPath, base)
	}

	resolvedBase, err := filepath.EvalSymlinks(cleanBase)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return joined, nil
		}
		return "", fmt.Errorf("failed to resolve base directory %q: %w", base, err)
	}
	resolvedPath, err := filepath.EvalSymlinks(joined)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return joined, nil
		}
		return "", fmt.Errorf("failed to resolve path %q: %w", joined, err)
	}
	if !isWithin(resolvedPath, resolvedBase) {
		return "", fmt.Errorf("%w: %q resolves via symlink outside %q", ErrPathEscapesBase, userPath, base)
	}

	return joined, nil
}

func isWithin(path, base string) bool {
	return path == base || strings.HasPrefix(path, base+string(filepath.Separator))
}

// IsOpaquePathToken rechaza un nombre (p. ej. de tema) que contenga
// separadores de ruta, ".." o sea una ruta absoluta — lo trata como un
// token opaco en vez de un fragmento de ruta. Para casos donde ni siquiera
// se quiere permitir la resolución dentro de un directorio (a diferencia de
// ResolveConfinedPath, que sí permite subdirectorios legítimos).
func IsOpaquePathToken(name string) bool {
	if name == "" || filepath.IsAbs(name) {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	if strings.Contains(name, "..") {
		return false
	}
	return true
}
