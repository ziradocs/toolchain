package report

import (
	"encoding/json"
	"time"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
)

type ReportJSON struct {
	ReportVersion string        `json:"reportVersion"`
	SchemaVersion string        `json:"schemaVersion"`
	Rulepacks     []string      `json:"rulepacks,omitempty"`
	Document      DocumentInfo  `json:"document"`
	ProducedAt    time.Time     `json:"producedAt"`
	Findings      []FindingJSON `json:"findings"`
}

type DocumentInfo struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum,omitempty"`
}

type FindingJSON struct {
	diagnostics.Diagnostic
	Waived bool               `json:"waived"`
	Waiver *linter.RulePolicy `json:"waiver,omitempty"`
}

func generateJSON(active []diagnostics.Diagnostic, waived []linter.WaivedDiagnostic, docPath string, astVersion string, checksum string, rulepacks []string) ([]byte, error) {
	report := ReportJSON{
		ReportVersion: "1.0.0",
		SchemaVersion: astVersion,
		Rulepacks:     rulepacks,
		Document: DocumentInfo{
			Path:     docPath,
			Checksum: checksum,
		},
		ProducedAt: time.Now().UTC(),
		Findings:   make([]FindingJSON, 0, len(active)+len(waived)),
	}

	for _, d := range active {
		report.Findings = append(report.Findings, FindingJSON{
			Diagnostic: d,
			Waived:     false,
		})
	}
	for _, w := range waived {
		report.Findings = append(report.Findings, FindingJSON{
			Diagnostic: w.Diagnostic,
			Waived:     true,
			Waiver:     w.Policy,
		})
	}

	return json.MarshalIndent(report, "", "  ")
}
