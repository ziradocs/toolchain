package report

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/linter"
)

// WriteReport generates the diagnostic report in the requested format and writes it to outPath.
func WriteReport(format, outPath string, active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic, doc *ast.AST, content []byte, rulepacks []string) error {
	var data []byte
	var err error

	docPath := ""
	schemaVersion := "2.0.0" // fallback
	if doc != nil {
		docPath = doc.FilePath
		if doc.SchemaVersion != "" {
			schemaVersion = doc.SchemaVersion
		}
	}

	switch format {
	case "json":
		checksum := ""
		if content != nil {
			checksum = fmt.Sprintf("%x", sha256.Sum256(content))
		}
		data, err = generateJSON(active, waived, docPath, schemaVersion, checksum, rulepacks)
	case "sarif":
		data, err = generateSARIF(active, waived, docPath)
	default:
		return fmt.Errorf("formato de reporte no soportado: %s", format)
	}

	if err != nil {
		return err
	}

	if outPath == "" || outPath == "-" {
		_, err = os.Stdout.Write(data)
		if err == nil {
			// Añadir un salto de línea si no lo tiene
			if len(data) > 0 && data[len(data)-1] != '\n' {
				_, _ = os.Stdout.WriteString("\n")
			}
		}
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(outPath, data, 0644)
}
