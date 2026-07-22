package linter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

type externalReport struct {
	Findings []externalFinding `json:"findings"`
}

type externalFinding struct {
	diagnostics.Diagnostic
}

func runExternalRulepack(doc *ast.AST, binaryPath string, timeout time.Duration) ([]diagnostics.Diagnostic, error) {
	// Raw AST: json.Marshal directly on *ast.AST.
	input, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("serializing AST for rulepack: %w", err)
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
			return nil, fmt.Errorf("timed out after %s", timeout)
		}
		return nil, fmt.Errorf("exited with error: %w (stderr: %s)", err, stderr.String())
	}

	var report externalReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		return nil, fmt.Errorf("decoding rulepack output: %w", err)
	}

	diags := make([]diagnostics.Diagnostic, 0, len(report.Findings))
	for _, f := range report.Findings {
		diags = append(diags, f.Diagnostic)
	}

	return diags, nil
}
