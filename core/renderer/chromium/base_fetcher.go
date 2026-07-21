// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FetcherLogger interface para logging
type FetcherLogger interface {
	Info(tag, format string, args ...interface{})
	Warn(tag, format string, args ...interface{})
	Error(tag, format string, args ...interface{})
}

// BaseFetcher contiene la lógica común para todos los fetchers
type BaseFetcher struct {
	renderer    *ChromiumRenderer
	cache       map[string]string
	cacheLock   sync.RWMutex
	logger      FetcherLogger
	imageFormat string // "png", "webp", o "svg"
	webpQuality int    // 1-100, default 85
	assetType   string // "charts", "maps", "diagrams"
	logTag      string // "CHART", "MAP", "MERMAID"
}

// NewBaseFetcher crea un nuevo fetcher base
func NewBaseFetcher(renderer *ChromiumRenderer, logger FetcherLogger, assetType, logTag string) *BaseFetcher {
	return &BaseFetcher{
		renderer:    renderer,
		cache:       make(map[string]string),
		cacheLock:   sync.RWMutex{},
		logger:      logger,
		imageFormat: "png", // default
		webpQuality: 85,    // calidad default para WebP
		assetType:   assetType,
		logTag:      logTag,
	}
}

// SetImageFormat configura el formato de imagen (png, webp, svg)
func (f *BaseFetcher) SetImageFormat(format string, quality int) {
	f.imageFormat = format
	if quality > 0 && quality <= 100 {
		f.webpQuality = quality
	}
}

// GetImageFormat retorna el formato de imagen actual
func (f *BaseFetcher) GetImageFormat() string {
	return f.imageFormat
}

// FetchAndSave renderiza contenido y lo guarda como archivo
// contentHash: hash único del contenido
// renderFunc: función que realiza el renderizado real
// outputDir: directorio base donde guardar
func (f *BaseFetcher) FetchAndSave(
	contentHash string,
	outputDir string,
	renderFunc func() ([]byte, error),
) (string, error) {
	// Determinar extensión según formato
	ext := f.imageFormat
	if ext == "" {
		ext = "png"
	}

	filename := fmt.Sprintf("%s_%s.%s", f.assetType[:len(f.assetType)-1], contentHash, ext)

	// Verificar si ya existe en cache
	f.cacheLock.RLock()
	if cached, ok := f.cache[contentHash]; ok {
		f.cacheLock.RUnlock()
		f.logger.Info(f.logTag, "✅ Using cached %s: %s", f.assetType[:len(f.assetType)-1], filename)
		return cached, nil
	}
	f.cacheLock.RUnlock()

	// Crear directorio de salida si no existe
	assetsDir := filepath.Join(outputDir, f.assetType)
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create %s directory: %w", f.assetType, err)
	}

	outputPath := filepath.Join(assetsDir, filename)

	// Verificar si el archivo ya existe
	if _, err := os.Stat(outputPath); err == nil {
		f.logger.Info(f.logTag, "✅ %s already exists: %s", capitalizeFirst(f.assetType[:len(f.assetType)-1]), filename)
		relativePath := filepath.Join(f.assetType, filename)

		// Agregar a cache
		f.cacheLock.Lock()
		f.cache[contentHash] = relativePath
		f.cacheLock.Unlock()

		return relativePath, nil
	}

	// Renderizar con función personalizada
	f.logger.Info(f.logTag, "⚙️  Rendering %s: %s", f.assetType[:len(f.assetType)-1], filename)

	data, err := renderFunc()
	if err != nil {
		return "", fmt.Errorf("failed to render %s: %w", f.assetType[:len(f.assetType)-1], err)
	}

	// Guardar archivo
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save %s: %w", f.assetType[:len(f.assetType)-1], err)
	}

	f.logger.Info(f.logTag, "✅ Saved %s: %s (%.2f KB)",
		f.assetType[:len(f.assetType)-1], filename, float64(len(data))/1024)

	relativePath := filepath.Join(f.assetType, filename)

	// Agregar a cache
	f.cacheLock.Lock()
	f.cache[contentHash] = relativePath
	f.cacheLock.Unlock()

	return relativePath, nil
}

// FetchInline renderiza contenido y lo retorna inline (sin guardar archivo)
func (f *BaseFetcher) FetchInline(
	contentHash string,
	renderFunc func() ([]byte, error),
) ([]byte, error) {
	// Verificar si ya existe en cache de memoria
	f.cacheLock.RLock()
	if cached, ok := f.cache[contentHash]; ok {
		f.cacheLock.RUnlock()
		f.logger.Info(f.logTag, "✅ Using cached inline %s", f.assetType[:len(f.assetType)-1])
		// En cache inline, guardamos el path pero no lo usamos, solo para tracking
		_ = cached
	} else {
		f.cacheLock.RUnlock()
	}

	// Renderizar
	data, err := renderFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to render inline %s: %w", f.assetType[:len(f.assetType)-1], err)
	}

	// Agregar a cache para tracking
	f.cacheLock.Lock()
	f.cache[contentHash] = "inline"
	f.cacheLock.Unlock()

	return data, nil
}

// GenerateContentHash genera un hash SHA256 del contenido
func GenerateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash[:8]) // Usar solo primeros 8 bytes (16 chars hex)
}

// capitalizeFirst capitaliza la primera letra de una string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}
