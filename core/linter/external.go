package linter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

type externalManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Prefix  string `json:"prefix"`
}

type externalReport struct {
	ReportVersion string            `json:"reportVersion"`
	Manifest      externalManifest  `json:"manifest"`
	Findings      []externalFinding `json:"findings"`
	// Descriptors se decodifica aparte (json.RawMessage), no inline: un
	// descriptor malformado NO debe hacer fallar el Unmarshal del reporte
	// completo y descartar los findings reales del pack (pérdida silenciosa
	// de diagnósticos = falso "limpio").
	Descriptors json.RawMessage `json:"descriptors,omitempty"`
}

type externalFinding struct {
	diagnostics.Diagnostic
}

func runExternalRulepack(doc *ast.AST, binaryPath string, timeout time.Duration) ([]diagnostics.Diagnostic, []RuleDescriptor, error) {
	// Raw AST: json.Marshal directly on *ast.AST.
	input, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing AST for rulepack: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, nil, fmt.Errorf("timed out after %s", timeout)
		}
		return nil, nil, fmt.Errorf("exited with error: %w (stderr: %s)", err, stderr.String())
	}

	var report externalReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		return nil, nil, fmt.Errorf("decoding rulepack output: %w", err)
	}

	if report.ReportVersion == "" {
		return nil, nil, fmt.Errorf("missing reportVersion in rulepack output")
	}
	if report.Manifest.Name == "" || report.Manifest.Version == "" || report.Manifest.Prefix == "" {
		return nil, nil, fmt.Errorf("incomplete manifest in rulepack output (name, version, and prefix are required)")
	}

	provenance := fmt.Sprintf("%s@%s", report.Manifest.Name, report.Manifest.Version)

	diags := make([]diagnostics.Diagnostic, 0, len(report.Findings))
	for _, f := range report.Findings {
		d := f.Diagnostic
		d.Source = provenance
		diags = append(diags, d)
	}

	// Los descriptores son best-effort: si vienen malformados se ignoran y se
	// conservan los findings, en vez de perder todo el reporte del pack.
	var descriptors []RuleDescriptor
	if len(report.Descriptors) > 0 {
		if err := json.Unmarshal(report.Descriptors, &descriptors); err != nil {
			descriptors = nil
		}
	}

	return diags, descriptors, nil
}
