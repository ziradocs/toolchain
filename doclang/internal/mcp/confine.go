// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.ziradocs.com/core/v2/util"
)

// allowedInputExts son las extensiones que acepta `export` como documento
// de entrada.
var allowedInputExts = map[string]bool{
	".md":       true,
	".markdown": true,
	".doclang":  true,
}

// exportRoot es el directorio al que `export` confina todo lo que lee y
// escribe (--root de `doclang mcp`). dir es la ruta ya resuelta con
// filepath.EvalSymlinks al arrancar el servidor; alias es la ruta absoluta
// tal como llegó, para aceptar una ruta absoluta del cliente escrita con el
// mismo prefijo aunque contenga un symlink (p. ej. /tmp → /private/tmp).
type exportRoot struct {
	dir   string
	alias string
}

func newExportRoot(dir string) (exportRoot, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return exportRoot{}, fmt.Errorf("cannot determine working directory for --root: %w", err)
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return exportRoot{}, fmt.Errorf("invalid root %q: %w", dir, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return exportRoot{}, fmt.Errorf("invalid root %q: %w", dir, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return exportRoot{}, fmt.Errorf("invalid root %q: %w", dir, err)
	}
	if !info.IsDir() {
		return exportRoot{}, fmt.Errorf("invalid root %q: not a directory", dir)
	}
	// La raíz del sistema de archivos ("/", o "C:\" en Windows) no confina
	// nada, así que no se acepta. Además util.ResolveConfinedPath arma con
	// ella el prefijo "//" y rechazaría toda ruta, imágenes incluidas.
	if filepath.Dir(resolved) == resolved {
		return exportRoot{}, fmt.Errorf("invalid root %q: it is the filesystem root, which confines nothing; choose a project directory", dir)
	}
	return exportRoot{dir: resolved, alias: filepath.Clean(abs)}, nil
}

// relative convierte una ruta del cliente en una ruta relativa a la raíz.
// Una ruta absoluta solo se acepta si cae dentro de la raíz; el resto del
// confinamiento (.., symlinks) lo hace util.ResolveConfinedPath, que
// rechaza las absolutas, por eso se convierten aquí.
func (r exportRoot) relative(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("%w: empty path", util.ErrPathEscapesBase)
	}
	if !filepath.IsAbs(p) {
		return p, nil
	}
	clean := filepath.Clean(p)
	for _, base := range []string{r.dir, r.alias} {
		rel, err := filepath.Rel(base, clean)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rel, nil
		}
	}
	return "", fmt.Errorf("%w: absolute path %q is outside the root", util.ErrPathEscapesBase, p)
}

// redact reemplaza la ruta absoluta de la raíz por "<root>" en un mensaje
// para el cliente.
func (r exportRoot) redact(msg string) string {
	msg = strings.ReplaceAll(msg, r.dir, "<root>")
	if r.alias != r.dir {
		msg = strings.ReplaceAll(msg, r.alias, "<root>")
	}
	return msg
}

// display devuelve abs relativa a la raíz, con "/" — es lo que ve el
// cliente en el resultado, nunca la ruta absoluta de la máquina.
func (r exportRoot) display(abs string) string {
	rel, err := filepath.Rel(r.dir, abs)
	if err != nil {
		return filepath.Base(abs)
	}
	return filepath.ToSlash(rel)
}

// resolveInput confina el documento de entrada y lo lee: dentro de la raíz
// (también tras resolver symlinks), archivo regular, extensión permitida y
// bajo el tope de tamaño, chequeado con el Stat del archivo abierto antes
// de leer y otra vez sobre lo leído.
func (r exportRoot) resolveInput(p string, maxBytes int) (abs string, content []byte, err error) {
	rel, err := r.relative(p)
	if err != nil {
		return "", nil, err
	}
	ext := strings.ToLower(filepath.Ext(rel))
	if !allowedInputExts[ext] {
		return "", nil, fmt.Errorf("unsupported input extension %q (allowed: .md, .markdown, .doclang)", filepath.Ext(rel))
	}
	joined, err := util.ResolveConfinedPath(r.dir, rel)
	if err != nil {
		return "", nil, err
	}

	// Un FIFO o un dispositivo con nombre .md bloquearía os.Open antes de
	// que empiece el timeout del export. Se exige archivo regular antes de
	// abrir (Stat y no Lstat: un symlink a un archivo dentro de la raíz es
	// válido y ya lo confinó util.ResolveConfinedPath) y se abre sin bloquear donde el
	// sistema lo permite, por si el archivo cambia entre el Stat y el Open.
	if info, err := os.Stat(joined); err == nil && !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("input is not a regular file: %s", filepath.ToSlash(rel))
	}
	f, err := os.OpenFile(joined, os.O_RDONLY|openNonBlock, 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil, fmt.Errorf("input file not found: %s", filepath.ToSlash(rel))
		}
		return "", nil, fmt.Errorf("cannot open input: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", nil, fmt.Errorf("cannot stat input: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("input is not a regular file: %s", filepath.ToSlash(rel))
	}
	if info.Size() > int64(maxBytes) {
		return "", nil, util.CheckInputSize(int(min(info.Size(), int64(maxBytes)+1)), maxBytes)
	}
	content, err = io.ReadAll(io.LimitReader(f, int64(maxBytes)+1))
	if err != nil {
		return "", nil, fmt.Errorf("failed to read file: %w", err)
	}
	if err := util.CheckInputSize(len(content), maxBytes); err != nil {
		return "", nil, err
	}
	return joined, content, nil
}

// resolveOutput confina la ruta de salida. util.ResolveConfinedPath solo
// revisa symlinks de rutas que existen, y el archivo de salida normalmente
// no existe todavía, así que se confina el DIRECTORIO padre (que debe
// existir: export no crea directorios) y se arma la ruta final sobre ese
// directorio ya resuelto. Si el destino existe y es un symlink se rechaza
// aunque apunte dentro de la raíz, para que `overwrite` nunca escriba a
// través de un enlace. La extensión tiene que ser la del formato, así un
// `overwrite: true` no puede pisar el .md de origen ni otros archivos.
func (r exportRoot) resolveOutput(p, format string) (string, error) {
	rel, err := r.relative(p)
	if err != nil {
		return "", err
	}
	rel = filepath.Clean(rel)
	if want := "." + format; strings.ToLower(filepath.Ext(rel)) != want {
		return "", fmt.Errorf("output %q must have the %s extension for format %s", filepath.ToSlash(rel), want, format)
	}

	dir, err := util.ResolveConfinedPath(r.dir, filepath.Dir(rel))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("output directory does not exist: %s", filepath.ToSlash(filepath.Dir(rel)))
		}
		return "", fmt.Errorf("cannot stat output directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("output directory is not a directory: %s", filepath.ToSlash(filepath.Dir(rel)))
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve output directory: %w", err)
	}
	final := filepath.Join(realDir, filepath.Base(rel))

	if fi, err := os.Lstat(final); err == nil {
		if fi.Mode()&fs.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: output %q is a symlink", util.ErrPathEscapesBase, filepath.ToSlash(rel))
		}
		if !fi.Mode().IsRegular() {
			return "", fmt.Errorf("output %q exists and is not a regular file", filepath.ToSlash(rel))
		}
	}
	return final, nil
}

// defaultOutput es la salida por defecto: junto al input, con la extensión
// del formato. inputAbs es la ruta confinada que devolvió resolveInput (sin
// resolver symlinks), así que el directorio es el que el cliente nombró.
func (r exportRoot) defaultOutput(inputAbs, format string) string {
	rel, err := filepath.Rel(r.dir, inputAbs)
	if err != nil {
		rel = filepath.Base(inputAbs)
	}
	return strings.TrimSuffix(rel, filepath.Ext(rel)) + "." + format
}

// resolveDir confina un directorio pedido por el cliente (baseDir,
// includeRoot): dentro de la raíz, también tras resolver symlinks, y
// existente. Devuelve la ruta con los symlinks ya resueltos y vuelve a
// comprobar que esa ruta esté dentro de la raíz (r.dir ya viene resuelta).
// Así el directorio que se usa después es el que se validó: si un symlink
// de la ruta pedida se cambia tras la validación para apuntar fuera de
// --root, la raíz de includes o el baseDir no se mueven con él.
func (r exportRoot) resolveDir(p string) (string, error) {
	rel, err := r.relative(p)
	if err != nil {
		return "", err
	}
	dir, err := util.ResolveConfinedPath(r.dir, filepath.Clean(rel))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("directory does not exist: %s", filepath.ToSlash(rel))
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", filepath.ToSlash(rel))
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	within, err := filepath.Rel(r.dir, real)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) || filepath.IsAbs(within) {
		return "", fmt.Errorf("%w: %s resolves outside the root", util.ErrPathEscapesBase, filepath.ToSlash(rel))
	}
	return real, nil
}
