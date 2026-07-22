// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

// RulePolicy es la configuración de una regla individual, identificada por
// el ID de diagnóstico que emite (p. ej. "IMG001", "LAYOUT003") — no por el
// nombre del struct Go que la implementa. Esto importa porque un solo Rule
// struct (p. ej. ElementStructureRule) puede emitir varios IDs distintos
// (TABLE001/002/003, CODEGROUP001/002, SPECIAL001, CHART001) desde la
// misma llamada a Check() — no hay forma de togglear uno sin el otro a
// nivel de struct sin partir esos structs en reglas más finas (fuera de
// scope: eso es "autorear lógica nueva", no "configurar lo que ya existe").
// Filtrar/re-severizar por ID de diagnóstico DESPUÉS de que las reglas ya
// corrieron es la granularidad real y honesta que el código actual permite
// configurar hoy.
type RulePolicy struct {
	// Enabled, si no-nil y false, descarta todo diagnóstico con este ID.
	// Puntero para distinguir "no especificado" (nil, no toca nada) de
	// "explícitamente false".
	Enabled *bool `yaml:"enabled,omitempty"`
	// Severity, si no vacío, sobreescribe la severidad ("error"|"warning"|
	// "info") de todo diagnóstico con este ID.
	Severity string `yaml:"severity,omitempty"`
	// --- waiver (aditivo) ---
	// ExpiresAt, en RFC3339, convierte esta entrada en una excepción con
	// vencimiento: mientras no expire suprime el diagnóstico; al expirar el
	// diagnóstico VUELVE y se emite POLICY001. Requiere Reason.
	ExpiresAt string `yaml:"expires_at,omitempty"`
	// Reason es obligatorio si hay ExpiresAt: una excepción sin motivo no es
	// auditable, y ese es el único valor que un waiver tiene sobre Enabled:false.
	Reason string `yaml:"reason,omitempty"`
	// ApprovedBy es texto libre y NO se verifica en este paquete — no hay
	// registro de identidades contra el cual verificarlo. Se propaga al
	// reporte para que un consumidor que sí tenga ese registro lo valide.
	ApprovedBy string `yaml:"approved_by,omitempty"`
	// Scope son globs de ruta. VACÍO significa "todo el documento".
	Scope []string `yaml:"scope,omitempty"`
}

// LayoutOverride sobreescribe los parámetros numéricos/de-lista de un
// SlideLayoutSchema (ver layout_validation.go), referenciado por tipo de
// layout ("team", "content", etc. — la MISMA clave que
// GetSlideLayoutSchemas() usa, no un ID de diagnóstico). A diferencia de
// RulePolicy (que filtra/re-severiza diagnósticos YA producidos), esto
// cambia QUÉ diagnósticos se producen en primer lugar — por eso se aplica
// en la construcción del schema (ver ResolveLayoutSchema), no en Apply().
// Deliberadamente acotado a los 3 campos numéricos/de-lista más simples de
// SlideLayoutSchema (issue #207): AllowedElements/RequiredProperties/
// OptionalProperties/ValidationRules siguen siendo Go-only — exponerlos
// por YAML implicaría serializar funciones Validator o rediseñar esos
// campos, fuera de scope de "hacer configurables los límites existentes".
type LayoutOverride struct {
	// MinElements/MaxElements, si no-nil, sobreescriben el schema base.
	// Puntero por el mismo motivo que RulePolicy.Enabled: nil (no
	// especificado) debe ser distinguible de 0 (explícitamente ilimitado
	// para MaxElements, o explícitamente "sin mínimo" para MinElements).
	MinElements *int `yaml:"min_elements,omitempty"`
	MaxElements *int `yaml:"max_elements,omitempty"`
	// ForbiddenElements, si no vacío, REEMPLAZA la lista del schema base
	// (no la extiende). Una lista vacía en YAML no se distingue de
	// "no especificado" (yaml.v3 decodifica ambos como nil/longitud-0) —
	// limitación aceptada: no hay forma de usar esta clave para "limpiar"
	// la lista prohibida de un schema, solo para reemplazarla por otra no
	// vacía.
	ForbiddenElements []string `yaml:"forbidden_elements,omitempty"`
}

// PolicyConfig es el motor de políticas configurable: Rules es un mapa de
// ID de diagnóstico → override post-hoc (filtra/re-severiza diagnósticos ya
// producidos). Layouts es un mapa de tipo-de-layout → override de
// parámetros del schema, aplicado ANTES de que SlideLayoutValidationRule
// corra (issue #207). Un ID/tipo que no aparece en ningún Rule/schema hoy
// (typo, o algo que se elimine en el futuro) simplemente no tiene efecto —
// no es un error de validación en esta primera versión (ver
// LoadPolicyConfig).
type PolicyConfig struct {
	Rules   map[string]RulePolicy     `yaml:"rules"`
	Layouts map[string]LayoutOverride `yaml:"layouts"`
}

// ResolveLayoutSchema aplica el LayoutOverride configurado para layoutType
// (si existe) sobre base, devolviendo una copia — base nunca se muta,
// consistente con GetSlideLayoutSchemas() devolviendo un mapa fresco en
// cada llamada. nil-safe: un *PolicyConfig nil devuelve base sin tocar.
func (p *PolicyConfig) ResolveLayoutSchema(layoutType string, base SlideLayoutSchema) SlideLayoutSchema {
	if p == nil {
		return base
	}
	override, ok := p.Layouts[layoutType]
	if !ok {
		return base
	}
	resolved := base
	if override.MinElements != nil {
		resolved.MinElements = *override.MinElements
	}
	if override.MaxElements != nil {
		resolved.MaxElements = *override.MaxElements
	}
	if len(override.ForbiddenElements) > 0 {
		resolved.ForbiddenElements = override.ForbiddenElements
	}
	return resolved
}

// LoadPolicyConfig lee y valida un archivo de política YAML. Retorna nil,
// nil si path es "" (sin política configurada, comportamiento por defecto:
// todas las reglas activas con su severidad original).
func LoadPolicyConfig(path string) (*PolicyConfig, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("linter: no se pudo leer el archivo de política %q: %w", path, err)
	}

	return parsePolicyConfig(data, path)
}

// parsePolicyConfig decodifica y valida un PolicyConfig desde bytes YAML ya
// en memoria — compartido por LoadPolicyConfig (lee de un path, operador
// confiable) y ResolvePolicyConfig (issue #208, política embebida en el
// frontmatter del propio documento, nunca toca el filesystem). source se usa
// solo en los mensajes de error para identificar el origen.
func parsePolicyConfig(data []byte, source string) (*PolicyConfig, error) {
	var cfg PolicyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("linter: %q no es YAML válido: %w", source, err)
	}

	for id, policy := range cfg.Rules {
		switch policy.Severity {
		case "", string(diagnostics.Error), string(diagnostics.Warning), string(diagnostics.Info):
			// válido
		default:
			return nil, fmt.Errorf(
				"linter: %q: regla %q tiene severity %q inválida (debe ser 'error', 'warning', 'info', o vacío)",
				source, id, policy.Severity)
		}

		if policy.ExpiresAt != "" {
			if _, err := time.Parse(time.RFC3339, policy.ExpiresAt); err != nil {
				return nil, fmt.Errorf("linter: %q: regla %q tiene expires_at %q inválido (debe ser RFC3339): %w", source, id, policy.ExpiresAt, err)
			}
			if policy.Reason == "" {
				return nil, fmt.Errorf("linter: %q: regla %q tiene expires_at pero no tiene reason", source, id)
			}
		}
	}

	// schemas se usa solo para resolver el valor EFECTIVO de Min/MaxElements
	// cuando un override toca un solo lado (ver el chequeo min>max más
	// abajo) — GetSlideLayoutSchemas() devuelve un mapa fresco, barato de
	// construir, así que llamarlo acá (validación, no hot path de lint) es
	// aceptable.
	schemas := GetSlideLayoutSchemas()
	for layoutType, override := range cfg.Layouts {
		if override.MinElements != nil && *override.MinElements < 0 {
			return nil, fmt.Errorf(
				"linter: %q: layout %q tiene min_elements %d inválido (no puede ser negativo)",
				source, layoutType, *override.MinElements)
		}
		if override.MaxElements != nil && *override.MaxElements < 0 {
			return nil, fmt.Errorf(
				"linter: %q: layout %q tiene max_elements %d inválido (no puede ser negativo)",
				source, layoutType, *override.MaxElements)
		}

		// Código de review (PR #220): chequear min>max solo cuando AMBOS
		// están en el MISMO override deja pasar un override parcial que
		// contradice el schema base — p.ej. layouts: {team: {min_elements:
		// 10}} contra el default MaxElements:8 de "team" (layout_validation.go)
		// pasaba validación y producía un schema resuelto imposible de
		// satisfacer (LAYOUT_MIN_ELEMENTS y LAYOUT_MAX_ELEMENTS simultáneos
		// para cualquier cantidad de elementos). Se resuelve el valor
		// EFECTIVO de cada lado (el override si está presente, si no el del
		// schema base) antes de comparar. Si layoutType no existe en
		// schemas (ID desconocido — PolicyConfig documenta que eso no es un
		// error), no hay base contra la cual cruzar: el chequeo queda
		// exactamente como antes, solo sobre los valores que el override
		// mismo trae.
		effectiveMin := override.MinElements
		effectiveMax := override.MaxElements
		if base, ok := schemas[layoutType]; ok {
			if effectiveMin == nil {
				m := base.MinElements
				effectiveMin = &m
			}
			if effectiveMax == nil {
				m := base.MaxElements
				effectiveMax = &m
			}
		}
		if effectiveMin != nil && effectiveMax != nil &&
			*effectiveMax > 0 && *effectiveMin > *effectiveMax {
			return nil, fmt.Errorf(
				"linter: %q: layout %q resulta en min_elements (%d) mayor que max_elements (%d) al combinar el override con el schema base — ver GetSlideLayoutSchemas()",
				source, layoutType, *effectiveMin, *effectiveMax)
		}
	}

	return &cfg, nil
}

// ResolvePolicyConfig implementa la precedencia flag > frontmatter >
// default para la política del linter — el mismo patrón de 3 niveles que
// la resolución de tema (ver CLAUDE.md "Theme resolution priority",
// doclang's getThemeName), aplicado a --lint-config (issue #208).
//
// flagPath, si no vacío, es input de operador confiable: se lee del
// filesystem exactamente igual que hoy (LoadPolicyConfig), rutas
// arbitrarias permitidas — sin cambios de comportamiento.
//
// Si flagPath está vacío, se busca una política embebida INLINE en el
// frontmatter del propio documento bajo la clave "lint_policy:" (mismo
// schema YAML que un archivo de --lint-config). El frontmatter es input
// NO confiable per el modelo de amenaza ME-2
// (docs/SECURITY_AUDIT_2026-07.md) — es contenido del documento, que puede
// venir de un tercero — así que, a diferencia de flagPath, esto NUNCA
// interpreta un valor del frontmatter como ruta de archivo: se parsea
// enteramente en memoria desde el YAML ya presente en fm.Raw. No hay ruta
// que recorrer (path traversal es imposible por construcción, no porque se
// valide un path — no existe ningún path en este camino).
func ResolvePolicyConfig(flagPath string, fm *ast.FrontMatterNode) (*PolicyConfig, error) {
	if flagPath != "" {
		return LoadPolicyConfig(flagPath)
	}
	if fm == nil || strings.TrimSpace(fm.Raw) == "" {
		return nil, nil
	}

	var wrapper struct {
		LintPolicy yaml.Node `yaml:"lint_policy"`
	}
	if err := yaml.Unmarshal([]byte(fm.Raw), &wrapper); err != nil {
		return nil, fmt.Errorf("linter: no se pudo reparsear el frontmatter buscando lint_policy: %w", err)
	}
	if wrapper.LintPolicy.IsZero() {
		return nil, nil // no hay clave lint_policy: en este documento
	}

	data, err := yaml.Marshal(&wrapper.LintPolicy)
	if err != nil {
		return nil, fmt.Errorf("linter: no se pudo re-serializar lint_policy del frontmatter: %w", err)
	}
	return parsePolicyConfig(data, "frontmatter lint_policy")
}

// diagnosticRuleID devuelve el identificador efectivo de un diagnóstico:
// RuleID si está poblado (la mayoría de rules.go, vía .WithRuleID(...)), o
// Code si no (layout_validation.go y algunas reglas de rules.go, que
// asignan Code directamente en el literal). Ambos campos existen en
// diagnostics.Diagnostic y se usan de forma inconsistente entre archivos —
// esta función es el único punto que necesita conocer esa inconsistencia.
func diagnosticRuleID(d diagnostics.Diagnostic) string {
	if d.RuleID != "" {
		return d.RuleID
	}
	return d.Code
}

// matchScopeGlob chequea si una ruta cumple con el patrón glob simple.
// Soporta ** (cualquier cosa) y * (cualquier cosa excepto /).
func matchScopeGlob(path, pattern string) bool {
	// Reemplazos simples para convertir a expresión regular.
	// Nota: esto es un helper rápido y sin dependencias.
	rx := regexp.QuoteMeta(pattern)
	rx = strings.ReplaceAll(rx, `\*\*`, `.*`)
	rx = strings.ReplaceAll(rx, `\*`, `[^/]*`)
	rx = strings.ReplaceAll(rx, `\?`, `.`)
	matched, _ := regexp.MatchString("^"+rx+"$", path)
	return matched
}

// Apply filtra diagnósticos deshabilitados y sobreescribe severidad según
// la política configurada. nil-safe: un *PolicyConfig nil (sin política
// cargada) devuelve diags sin tocar.
func (p *PolicyConfig) Apply(diags []diagnostics.Diagnostic) []diagnostics.Diagnostic {
	return p.applyTo(diags, "", time.Now())
}

// WaivedDiagnostic wraps a suppressed diagnostic along with the policy
// that suppressed it (useful for SARIF reports).
type WaivedDiagnostic struct {
	Diagnostic diagnostics.Diagnostic
	Policy     *RulePolicy
}

// applyTo executes the policy by injecting the current timestamp and the file path.
// It evaluates waivers using Scope and ExpiresAt. Returns only active diagnostics.
func (p *PolicyConfig) applyTo(diags []diagnostics.Diagnostic, filePath string, now time.Time) []diagnostics.Diagnostic {
	active, _ := p.Evaluate(diags, filePath, now)
	return active
}

// Evaluate classifies the diagnostics into active and suppressed (waived).
func (p *PolicyConfig) Evaluate(diags []diagnostics.Diagnostic, filePath string, now time.Time) (active []diagnostics.Diagnostic, waived []WaivedDiagnostic) {
	if p == nil || len(p.Rules) == 0 {
		return diags, nil
	}

	active = make([]diagnostics.Diagnostic, 0, len(diags))
	for _, d := range diags {
		if diagnosticRuleID(d) == "POLICY001" {
			// POLICY001 no se puede suprimir
			active = append(active, d)
			continue
		}

		policy, ok := p.Rules[diagnosticRuleID(d)]
		if !ok {
			active = append(active, d)
			continue
		}

		isWaived := false
		expired := false
		inScope := true

		if policy.ExpiresAt != "" {
			if len(policy.Scope) > 0 {
				if filePath == "" {
					inScope = false
				} else {
					inScope = false
					for _, scopeGlob := range policy.Scope {
						if matchScopeGlob(filePath, scopeGlob) {
							inScope = true
							break
						}
					}
				}
			}

			if inScope {
				exp, _ := time.Parse(time.RFC3339, policy.ExpiresAt)
				if now.After(exp) {
					expired = true
				} else {
					isWaived = true
				}
			}
		}

		if policy.Enabled != nil && !*policy.Enabled {
			// Si expiró el waiver, enabled:false deja de tener efecto
			if !expired {
				isWaived = true
			}
		}

		if expired {
			if policy.Severity != "" {
				d.Severity = diagnostics.Severity(policy.Severity)
			}
			active = append(active, d)

			msg := fmt.Sprintf("waiver for %s expired on %s", diagnosticRuleID(d), policy.ExpiresAt)
			policyDiag := diagnostics.NewWarning(msg, d.Position, "linter").WithCode("POLICY001")
			active = append(active, policyDiag)
			continue
		}

		if isWaived {
			polCopy := policy
			waived = append(waived, WaivedDiagnostic{
				Diagnostic: d,
				Policy:     &polCopy,
			})
			continue
		}

		if policy.Severity != "" {
			d.Severity = diagnostics.Severity(policy.Severity)
		}
		active = append(active, d)
	}
	return active, waived
}
