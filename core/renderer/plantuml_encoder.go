// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"bytes"
	"compress/flate"
	"strings"
)

// EncodePlantUML codifica el contenido PlantUML para usarlo en URL
// Usa el mismo formato que el servidor oficial: Deflate + Base64 personalizado
func EncodePlantUML(content string) string {
	// 1. Comprimir con Deflate
	var compressed bytes.Buffer
	writer, err := flate.NewWriter(&compressed, flate.BestCompression)
	if err != nil {
		return ""
	}

	_, err = writer.Write([]byte(content))
	if err != nil {
		_ = writer.Close()
		return ""
	}
	_ = writer.Close()

	// 2. Codificar con Base64 personalizado de PlantUML
	encoded := encodePlantUMLBase64(compressed.Bytes())

	return encoded
}

// encodePlantUMLBase64 usa el alfabeto especial de PlantUML
// Diferente del Base64 estándar
func encodePlantUMLBase64(data []byte) string {
	// Alfabeto PlantUML (diferente del RFC 4648)
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"

	var result strings.Builder

	for i := 0; i < len(data); i += 3 {
		// Tomar 3 bytes (24 bits)
		b1, b2, b3 := byte(0), byte(0), byte(0)

		if i < len(data) {
			b1 = data[i]
		}
		if i+1 < len(data) {
			b2 = data[i+1]
		}
		if i+2 < len(data) {
			b3 = data[i+2]
		}

		// Convertir a 4 caracteres de 6 bits cada uno
		c1 := (b1 >> 2) & 0x3F
		c2 := ((b1 & 0x03) << 4) | ((b2 >> 4) & 0x0F)
		c3 := ((b2 & 0x0F) << 2) | ((b3 >> 6) & 0x03)
		c4 := b3 & 0x3F

		result.WriteByte(alphabet[c1])
		result.WriteByte(alphabet[c2])

		if i+1 < len(data) {
			result.WriteByte(alphabet[c3])
		}
		if i+2 < len(data) {
			result.WriteByte(alphabet[c4])
		}
	}

	return result.String()
}

// GeneratePlantUMLURL genera la URL completa para un diagrama PlantUML
func GeneratePlantUMLURL(content string, server string, format string) string {
	// Server por defecto
	if server == "" {
		server = "https://www.plantuml.com/plantuml"
	}

	// Formato por defecto: SVG
	if format == "" {
		format = "svg"
	}

	// Asegurar que el servidor no termina en /
	server = strings.TrimSuffix(server, "/")

	// Codificar contenido
	encoded := EncodePlantUML(content)

	// Generar URL: server/format/encoded
	return server + "/" + format + "/" + encoded
}

// GeneratePlantUMLPNGURL genera URL para PNG
func GeneratePlantUMLPNGURL(content string, server string) string {
	return GeneratePlantUMLURL(content, server, "png")
}

// GeneratePlantUMLSVGURL genera URL para SVG (recomendado)
func GeneratePlantUMLSVGURL(content string, server string) string {
	return GeneratePlantUMLURL(content, server, "svg")
}

// SanitizePlantUMLContent limpia y valida el contenido PlantUML
func SanitizePlantUMLContent(content string) string {
	content = strings.TrimSpace(content)

	// Asegurar que tiene @startuml y @enduml
	if !strings.Contains(content, "@startuml") {
		content = "@startuml\n" + content
	}
	if !strings.Contains(content, "@enduml") {
		content = content + "\n@enduml"
	}

	return content
}
