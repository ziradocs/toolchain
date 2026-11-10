// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// warnLogger guarda los Warn y Error; lo demás lo descarta.
type warnLogger struct {
	util.Logger
	lines []string
}

func (l *warnLogger) Warn(message string, args ...interface{}) {
	l.lines = append(l.lines, "WARN "+fmt.Sprintf(message, args...))
}

func (l *warnLogger) Error(message string, args ...interface{}) {
	l.lines = append(l.lines, "ERROR "+fmt.Sprintf(message, args...))
}

const tableYAMLBody = "\n# A\n\nTABLE\n  cells:\n    - [ : ]\n"

func prepareWithPolicy(t *testing.T, log util.Logger, policy string) (*Prepared, error) {
	t.Helper()
	src := "---\ntitle: D\nmode: flex\nlint_policy:\n  rules:\n" + policy + "---\n" + tableYAMLBody
	return Prepare(log, []byte(src), PrepareOptions{InputFile: "d.md", MaxInputBytes: util.DefaultMaxInputBytes})
}

// El error de lint lleva los avisos del parser con la política aplicada: un
// TABLE004 que la política subió a error llega como error, no como el
// warning crudo del parser.
func TestPrepare_LintErrorCarriesPolicyAppliedParserDiagnostics(t *testing.T) {
	_, err := prepareWithPolicy(t, util.NewNoop(), "    TABLE004:\n      severity: error\n")
	var diagErr *DiagnosticsError
	if !errors.As(err, &diagErr) || diagErr.Stage != StageLint {
		t.Fatalf("want a lint DiagnosticsError, got %v", err)
	}
	var table004 []diagnostics.Severity
	for _, d := range diagErr.Diagnostics {
		if d.RuleID == "TABLE004" {
			table004 = append(table004, d.Severity)
		}
	}
	if len(table004) != 1 || table004[0] != diagnostics.Error {
		t.Errorf("TABLE004 severities in the error = %v, want [error]", table004)
	}
}

// Un waiver vencido sobre un ID del parser deja el POLICY001 en el log de
// LINT, como uno sobre una regla del linter.
func TestPrepare_ExpiredWaiverOnParserIDIsLogged(t *testing.T) {
	past := time.Now().AddDate(-1, 0, 0).Format(time.RFC3339)
	log := &warnLogger{Logger: util.NewNoop()}
	prepared, err := prepareWithPolicy(t, log, "    TABLE004:\n      expires_at: \""+past+"\"\n      reason: \"r\"\n      approved_by: \"e2e\"\n")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	found := false
	for _, l := range log.lines {
		if strings.HasPrefix(l, "WARN LINT:") && strings.Contains(l, "waiver for TABLE004 expired") {
			found = true
		}
	}
	if !found {
		t.Errorf("want the POLICY001 warning in the LINT log, got %v", log.lines)
	}
	policy001 := false
	for _, d := range prepared.Diagnostics() {
		if d.Code == "POLICY001" || d.RuleID == "POLICY001" {
			policy001 = true
		}
	}
	if !policy001 {
		t.Errorf("want POLICY001 in the prepared diagnostics, got %v", prepared.Diagnostics())
	}
}
