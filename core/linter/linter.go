// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"fmt"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

type Linter struct {
	rules             []Rule
	policy            *PolicyConfig
	externalRulepacks []string
	externalTimeout   time.Duration
	themeVariables    map[string]string
	themeVariablesSet bool
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

// ThemeAware lo implementan las reglas que necesitan los colores resueltos
// del tema ACTIVO (issue #30 — contraste WCAG 2.2 AA) antes de correr. A
// diferencia de layoutPolicyAware (no exportada porque su único
// implementador vive dentro de este paquete), ThemeAware es EXPORTADA a
// propósito: core no posee ningún color de tema (solo el nombre del tema
// viaja en ast.FrontMatterNode.Theme; el mapa real de variables CSS lo
// resuelve cada CLI por separado — slidelang y doclang tienen
// representaciones distintas y no comparten convención de nombres). Por
// eso el propósito entero de este seam es que un consumidor FUERA de este
// paquete (un rulepack externo de contraste) implemente la interfaz; un
// método no exportado la haría imposible de implementar para ese
// consumidor.
type ThemeAware interface {
	SetThemeVariables(map[string]string)
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

// WithThemeVariables adjunta el mapa de variables CSS del tema ya resuelto
// por la CLI (nombre de variable → valor, p. ej. "--text-color" en
// slidelang o "--doclang-h1-color" en doclang) para que cualquier regla en
// l.rules que implemente ThemeAware pueda calcular contraste u otras
// propiedades derivadas del tema. Se acepta explícitamente un vars nil o
// vacío (un tema puede legítimamente no declarar variables) — por eso NO
// se guarda solo "themeVariables != nil" como señal de que ya se llamó:
// themeVariablesSet distingue "se llamó WithThemeVariables" de "el mapa
// resultante está vacío", para que AddRule siga inyectando (con un mapa
// vacío) a las reglas agregadas después, en vez de omitir la inyección
// silenciosamente.
func (l *Linter) WithThemeVariables(vars map[string]string) *Linter {
	l.themeVariables = vars
	l.themeVariablesSet = true
	for _, r := range l.rules {
		if aware, ok := r.(ThemeAware); ok {
			aware.SetThemeVariables(vars)
		}
	}
	return l
}

// WithRulepacks registers paths to external rulepack binaries to be run as subprocesses.
func (l *Linter) WithRulepacks(paths []string, timeout time.Duration) *Linter {
	l.externalRulepacks = paths
	l.externalTimeout = timeout
	return l
}

// DefaultRules devuelve el conjunto de reglas de validez del formato que
// New() usa. El ORDEN es significativo: LastSlideClosingRule debe correr
// antes de SlideLayoutValidationRule.
func DefaultRules() []Rule {
	return []Rule{
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
	}
}

// NewWithRules construye un Linter con un conjunto arbitrario de reglas.
// Para el conjunto por defecto más reglas propias:
//
//	linter.NewWithRules(append(linter.DefaultRules(), miRegla)...)
func NewWithRules(rules ...Rule) *Linter {
	return &Linter{
		rules: rules,
	}
}

// New equivale a NewWithRules(DefaultRules()...) — comportamiento idéntico
// al histórico.
func New() *Linter {
	return NewWithRules(DefaultRules()...)
}

// AddRule agrega una regla a un Linter ya construido. Si el Linter ya tiene
// una policy adjunta, cablea layoutPolicyAware en la regla nueva.
func (l *Linter) AddRule(r Rule) *Linter {
	l.rules = append(l.rules, r)
	if l.policy != nil {
		if aware, ok := r.(layoutPolicyAware); ok {
			aware.setLayoutPolicy(l.policy)
		}
	}
	if l.themeVariablesSet {
		if aware, ok := r.(ThemeAware); ok {
			aware.SetThemeVariables(l.themeVariables)
		}
	}
	return l
}

// LintUnfiltered runs all rules and returns all findings without evaluating
// the policy (without filtering waivers or modifying severities). No muta el
// receptor, así que un *Linter puede reutilizarse (incluso entre goroutines)
// para varios documentos. Descarta los descriptores de rulepacks externos;
// usa LintUnfilteredWithDescriptors si los necesitas para el reporte.
func (l *Linter) LintUnfiltered(astNode *ast.AST) []diagnostics.Diagnostic {
	diags, _ := l.LintUnfilteredWithDescriptors(astNode)
	return diags
}

// LintUnfilteredWithDescriptors corre todas las reglas y además devuelve los
// RuleDescriptor que los rulepacks externos declararon en ESTA corrida. Los
// descriptores se DEVUELVEN, no se almacenan en el receptor: por eso el
// *Linter sigue siendo seguro de reutilizar/compartir entre documentos.
func (l *Linter) LintUnfilteredWithDescriptors(astNode *ast.AST) ([]diagnostics.Diagnostic, []RuleDescriptor) {
	var allDiagnostics []diagnostics.Diagnostic
	var externalDescriptors []RuleDescriptor

	// Ejecutar todas las reglas en el AST
	for _, rule := range l.rules {
		diags := rule.Check(astNode)
		allDiagnostics = append(allDiagnostics, diags...)
	}

	// También ejecutar reglas en cada slide
	for _, slide := range astNode.ContentBlocks {
		for _, rule := range l.rules {
			diags := rule.Check(&slide)
			allDiagnostics = append(allDiagnostics, diags...)
		}
	}

	for _, packPath := range l.externalRulepacks {
		extDiags, extDescriptors, err := runExternalRulepack(astNode, packPath, l.externalTimeout, l.themeVariables, l.themeVariablesSet)
		if err != nil {
			errDiag := diagnostics.NewError(fmt.Sprintf("external rulepack %s failed: %v", packPath, err), diagnostics.Position{}, "LINTER_SYS_ERR")
			allDiagnostics = append(allDiagnostics, errDiag)
		} else {
			allDiagnostics = append(allDiagnostics, extDiags...)
			externalDescriptors = append(externalDescriptors, extDescriptors...)
		}
	}

	return allDiagnostics, externalDescriptors
}

func (l *Linter) Lint(astNode *ast.AST) []diagnostics.Diagnostic {
	allDiagnostics := l.LintUnfiltered(astNode)
	return l.policy.applyTo(allDiagnostics, astNode.FilePath, time.Now())
}
