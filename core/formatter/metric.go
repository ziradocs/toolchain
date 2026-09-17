// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
)

func formatMetric(e *ast.MetricElement) (string, error) {
	body := struct {
		Label   string `yaml:"label"`
		Value   string `yaml:"value"`
		Delta   string `yaml:"delta,omitempty"`
		Trend   string `yaml:"trend,omitempty"`
		Caption string `yaml:"caption,omitempty"`
	}{e.Label, e.Value, e.Delta, e.Trend, e.Caption}
	raw, err := yaml.Marshal(body)
	if err != nil {
		return "", err
	}
	return "<<metric>>\n" + strings.TrimRight(string(raw), "\n") + "\n<<end>>", nil
}
