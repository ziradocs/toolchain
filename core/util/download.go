// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// Issue #75: renderer/chromium_installer.go (downloadFile) y
// renderer/plantuml_fetcher.go (FetchDiagramToAssets/FetchDiagramInline)
// implementaban el mismo patrón de descarga acotada (LimitReader(max+1) +
// atomic temp-file+rename) de forma independiente — dos copias mantenidas
// por separado del mismo mecanismo de seguridad. Este archivo extrae ese
// mecanismo a un solo lugar.
//
// Estas funciones consumen un *http.Response ya obtenido, en vez de hacer
// ellas mismas la petición HTTP: cada caller tiene su propia forma legítima
// de obtener la respuesta (chromium_installer.go usa un client.Get simple;
// plantuml_fetcher.go usa fetchWithControlledRedirects, que valida cada
// redirect contra IPs privadas — ver docs/SECURITY_AUDIT_2026-07.md, ME-3).
// El mecanismo realmente compartido es "consumir el body con un cap de
// tamaño sin exponerse a una respuesta arbitrariamente grande o lenta", no
// cómo se obtuvo esa respuesta.

// ConsumeResponseToTempFile valida que resp tenga status 200 y descarga su
// body hacia un archivo temporal nuevo creado en tmpDir (patrón tmpPattern,
// ver os.CreateTemp) con permisos 0600, acotando el tamaño total a
// maxBytes.
//
// io.LimitReader(resp.Body, maxBytes+1) permite detectar un exceso sin
// esperar a agotar el body completo: si el cuerpo real supera el cap,
// io.Copy retorna written == maxBytes+1 en vez de bloquear indefinidamente
// en una respuesta que nunca termina.
//
// Si extraWriters no está vacío, cada byte descargado también se escribe
// ahí (p. ej. un hash.Hash, para verificar un checksum sin releer el
// archivo del disco).
//
// En CUALQUIER camino de error, esta función limpia su propio temporal
// (os.Remove) — el caller nunca necesita gestionar tmpPath en el camino de
// error; tmpPath solo es válido (no vacío) cuando err == nil, listo para
// que el caller lo renombre al destino final (issue #75, hallazgo de
// code-review de PR #148: la versión anterior dejaba el temporal en
// tmpPath incluso en error, obligando a cada caller a duplicar un
// `if tmpPath != "" { defer os.Remove(tmpPath) }` — ninguno de los dos usa
// esa flexibilidad para inspeccionar el archivo antes de decidir, así que
// el helper ahora se encarga por completo).
func ConsumeResponseToTempFile(resp *http.Response, tmpDir, tmpPattern string, maxBytes int64, extraWriters ...io.Writer) (tmpPath string, written int64, err error) {
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("bad status: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp(tmpDir, tmpPattern)
	if err != nil {
		return "", 0, err
	}
	path := tmpFile.Name()
	cleanup := func() { _ = os.Remove(path) }

	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return "", 0, err
	}

	writers := make([]io.Writer, 0, len(extraWriters)+1)
	writers = append(writers, tmpFile)
	writers = append(writers, extraWriters...)
	dest := io.MultiWriter(writers...)

	written, err = io.Copy(dest, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		_ = tmpFile.Close()
		cleanup()
		return "", written, err
	}

	// El tamaño excedido se reporta ANTES que cualquier error de Close():
	// si el body superó el cap, esa es la causa de fondo que el caller debe
	// ver, sin importar si además el flush final falló (p. ej. disco lleno)
	// — el archivo se descarta de todos modos. Invertir este orden (chequear
	// Close() primero) enmascararía la señal de seguridad "respuesta
	// desmedida" detrás de un error de I/O genérico (hallazgo de
	// code-review de PR #148).
	if written > maxBytes {
		_ = tmpFile.Close()
		cleanup()
		return "", written, fmt.Errorf("response exceeds max allowed size (%d MB)", maxBytes/(1024*1024))
	}

	if closeErr := tmpFile.Close(); closeErr != nil {
		cleanup()
		return "", written, closeErr
	}

	return path, written, nil
}

// ConsumeResponseWithLimit valida que resp tenga status 200 y lee su body
// completo a memoria, acotado a maxBytes — para respuestas que no
// necesitan persistir a disco (p. ej. contenido que se embebe inline en el
// HTML generado).
func ConsumeResponseWithLimit(resp *http.Response, maxBytes int64) ([]byte, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response exceeds max allowed size (%d MB)", maxBytes/(1024*1024))
	}

	return data, nil
}
