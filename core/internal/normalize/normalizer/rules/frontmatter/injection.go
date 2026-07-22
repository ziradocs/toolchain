// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package frontmatter

import (
	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// InjectionRule inyecta frontmatter faltante
type InjectionRule struct {
	parser *base.FrontmatterParser
}

// NewInjectionRule crea una nueva instancia de la regla
func NewInjectionRule() *InjectionRule {
	return &InjectionRule{
		parser: base.NewFrontmatterParser(),
	}
}

func (r *InjectionRule) Apply(content string) (string, error) {
	// Si no tiene frontmatter, crear uno completo
	if !r.parser.HasFrontmatter(content) {
		frontmatter := r.parser.CreateBasicFrontmatter(content)
		return frontmatter + content, nil
	}

	// Si tiene frontmatter pero no tiene mode, agregarlo
	if !r.parser.HasMode(content) {
		return r.parser.AddModeToFrontmatter(content), nil
	}

	// Ya tiene frontmatter válido con mode
	return content, nil
}

func (r *InjectionRule) Description() string {
	return "Inyecta frontmatter faltante con configuración básica"
}

func (r *InjectionRule) Priority() int {
	return 1
}

func (r *InjectionRule) Category() base.RuleCategory {
	return base.CategoryFrontmatter
}
