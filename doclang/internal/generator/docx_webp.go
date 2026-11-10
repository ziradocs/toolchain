// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/image/webp"
)

// fileIsWebP dice si el archivo en p es una imagen WebP, por su contenido
// (un contenedor RIFF de tipo WEBP) y no por la extensión. Un error de
// lectura cuenta como "no".
func fileIsWebP(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 12)
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP"
}

// convertWebPToPNG decodifica la imagen WebP en webpPath y la escribe como
// PNG en el directorio temporal del DOCX: Word no inserta WebP, y docxgo
// tampoco lo lee. El PNG conserva la transparencia. Devuelve la ruta del PNG.
// Un WebP animado o dañado da error y la imagen queda como no disponible,
// como cualquier otra que no se puede leer.
func (g *DOCXGenerator) convertWebPToPNG(webpPath string) (string, error) {
	data, err := os.ReadFile(webpPath)
	if err != nil {
		return "", err
	}
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("cannot decode WebP image: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("cannot convert WebP image to PNG: %w", err)
	}
	sum := sha256.Sum256(data)
	pngPath := filepath.Join(g.tempDir, "webp_"+hex.EncodeToString(sum[:8])+".png")
	if err := os.WriteFile(pngPath, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	return pngPath, nil
}
