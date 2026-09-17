// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// MetricParser parsea un KPI con estructura explícita; no busca signos de
// moneda ni porcentajes dentro de prosa como hacía el linter histórico.
type MetricParser struct{}

func (p *MetricParser) CanParse(line string, mode string) bool {
	return strings.TrimSpace(line) == "<<metric>>"
}

func (p *MetricParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	pos := ctx.Position(startIndex)
	raw, consumed, closedBy := readQuizPollBody(ctx.Lines, startIndex, "metric")
	diags := make([]diagnostics.Diagnostic, 0, 3)
	if closedBy != closedByCloser {
		diags = append(diags, diagnostics.NewWarning("El bloque metric no se cerró con '<<end>>'", pos, "metric-parser").WithRuleID("METRIC001"))
	}

	type metricBody struct {
		Label   string `yaml:"label"`
		Value   string `yaml:"value"`
		Delta   string `yaml:"delta"`
		Trend   string `yaml:"trend"`
		Caption string `yaml:"caption"`
	}
	var body metricBody
	yamlText := dedentBlock(raw)
	if strings.TrimSpace(yamlText) != "" {
		if err := yaml.Unmarshal([]byte(yamlText), &body); err != nil {
			diags = append(diags, diagnostics.NewWarning(fmt.Sprintf("El YAML de metric es inválido y fue ignorado: %v", err), pos, "metric-parser").WithRuleID("METRIC002"))
			body = metricBody{}
		} else {
			var keys map[string]interface{}
			_ = yaml.Unmarshal([]byte(yamlText), &keys)
			unknown := make([]string, 0)
			for key := range keys {
				if key != "label" && key != "value" && key != "delta" && key != "trend" && key != "caption" {
					unknown = append(unknown, key)
				}
			}
			if len(unknown) > 0 {
				sort.Strings(unknown)
				diags = append(diags, diagnostics.NewWarning("Llave(s) no reconocida(s) en metric: "+strings.Join(unknown, ", "), pos, "metric-parser").WithRuleID("METRIC003"))
			}
		}
	}
	body.Trend = strings.ToLower(strings.TrimSpace(body.Trend))
	if body.Trend != "" && body.Trend != "up" && body.Trend != "down" && body.Trend != "flat" {
		diags = append(diags, diagnostics.NewWarning("trend de metric debe ser up, down o flat", pos, "metric-parser").WithRuleID("METRIC004"))
		body.Trend = ""
	}
	m := ast.NewMetricElement(pos)
	m.Label, m.Value, m.Delta, m.Trend, m.Caption = body.Label, body.Value, body.Delta, body.Trend, body.Caption
	return &ParseResult{Element: m, ConsumedLines: consumed, Diagnostics: diags}
}
