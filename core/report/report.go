package report

import (
	"fmt"
	"os"
	"path/filepath"

	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/linter"
)

// WriteReport generates the diagnostic report in the requested format and writes it to outPath.
func WriteReport(format, outPath string, active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic, docPath string) error {
	var data []byte
	var err error

	switch format {
	case "json":
		data, err = generateJSON(active, waived, docPath)
	case "sarif":
		data, err = generateSARIF(active, waived, docPath)
	default:
		return fmt.Errorf("formato de reporte no soportado: %s", format)
	}

	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(outPath, data, 0644)
}
