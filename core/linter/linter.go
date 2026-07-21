// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

type Linter struct {
	rules  []Rule
	policy *PolicyConfig
}

type Rule interface {
	Check(node ast.Node) []diagnostics.Diagnostic
}

// layoutPolicyAware lo implementan las reglas que necesitan el
// *PolicyConfig ANTES de correr (no solo después, vía Apply) — hoy solo
// SlideLayoutValidationRule, para resolver overrides de
// Min/MaxElements/ForbiddenElements por tipo de layout (issue #207).
// Separado de PolicyConfig.Apply (que solo filtra/re-severiza diagnósticos
// ya producidos) porque un override de parámetro cambia QUÉ diagnósticos
// se producen, no solo cómo se muestran.
type layoutPolicyAware interface {
	setLayoutPolicy(*PolicyConfig)
}

// WithPolicy adjunta un motor de políticas configurable: Lint() filtrará y
// re-severizará los diagnósticos según policy antes de devolverlos (ver
// PolicyConfig.Apply), y cualquier regla en l.rules que implemente
// layoutPolicyAware recibe la misma policy para resolver sus propios
// parámetros antes de Check() (ver SlideLayoutValidationRule). policy nil
// es un no-op — deja New() con su comportamiento por defecto (todas las
// reglas, severidad y parámetros originales).
func (l *Linter) WithPolicy(policy *PolicyConfig) *Linter {
	l.policy = policy
	for _, r := range l.rules {
		if aware, ok := r.(layoutPolicyAware); ok {
			aware.setLayoutPolicy(policy)
		}
	}
	return l
}

func New() *Linter {
	return &Linter{
		rules: []Rule{
			&PresentationHasSlidesRule{},
			&FrontMatterValidRule{},
			&SlideNotEmptyRule{},
			&ImageHasSourceRule{},
			&CodeHasContentRule{},
			&ParseErrorDetectionRule{},
			// Nuevas reglas de validación estricta
			&StrictModeValidationRule{},
			&ElementStructureRule{},
			&PropertyValidationRule{},
			// Layout-specific validation
			&LastSlideClosingRule{}, // Debe ejecutarse antes de SlideLayoutValidationRule
			&SlideLayoutValidationRule{},
		},
	}
}

func (l *Linter) Lint(astNode *ast.AST) []diagnostics.Diagnostic {
	var allDiagnostics []diagnostics.Diagnostic

	// Ejecutar todas las reglas en el AST
	for _, rule := range l.rules {
		diagnostics := rule.Check(astNode)
		allDiagnostics = append(allDiagnostics, diagnostics...)
	}

	// También ejecutar reglas en cada slide
	for _, slide := range astNode.ContentBlocks {
		for _, rule := range l.rules {
			diagnostics := rule.Check(&slide)
			allDiagnostics = append(allDiagnostics, diagnostics...)
		}
	}

	return l.policy.Apply(allDiagnostics)
}
