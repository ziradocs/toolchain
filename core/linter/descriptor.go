// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import "sort"

// RuleDescriptor es metadata estática por regla (helpUri, propiedades libres)
// que un autor de regla/rulepack puede declarar antes de que corra ningún
// Check. core no le asigna significado a Properties — es un lugar genérico
// para que un rulepack externo (p.ej. reglas de contraste WCAG) cuelgue lo
// que necesite. Los json tags sirven tanto al protocolo de rulepack externo
// (ver external.go) como al reporte nativo (ver core/report/json.go).
//
// Properties es map[string]any (no map[string]string) a propósito: un
// property bag SARIF admite valores JSON arbitrarios (números, arreglos,
// objetos), así que restringirlo a strings haría fallar el decodificado de
// la salida de un rulepack que emita, p.ej., {"minContrast": 4.5}.
//
// Nota SARIF: al emitirse en driver.rules[].properties, la clave `tags`
// está RESERVADA por el estándar como arreglo de strings — un autor que la
// use debe darle un []string, no un escalar, o el reporte no validará contra
// el schema. core no reescribe la clave; es responsabilidad del autor.
type RuleDescriptor struct {
	ID         string         `json:"id"`
	Name       string         `json:"name,omitempty"`
	HelpURI    string         `json:"helpUri,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

// Describable lo implementan opcionalmente Rule y/o RulePack para exponer
// descriptores estáticos por regla. Detección vía type-assertion, mismo
// idioma que layoutPolicyAware (linter.go).
type Describable interface {
	Descriptors() []RuleDescriptor
}

// CollectDescriptors junta los descriptores de reglas y rulepacks in-process
// que implementen Describable. Se recolecta aquí (no desde *Linter) porque
// build.go aplana cada RulePack en reglas sueltas vía AddRule antes de que
// el Linter llegue a sostener el pack: un descriptor a nivel de pack solo es
// visible mientras rules/packs siguen siendo los objetos originales.
func CollectDescriptors(rules []Rule, packs []RulePack) []RuleDescriptor {
	var out []RuleDescriptor

	for _, r := range rules {
		if d, ok := r.(Describable); ok {
			out = append(out, d.Descriptors()...)
		}
	}

	for _, p := range packs {
		if d, ok := p.(Describable); ok {
			out = append(out, d.Descriptors()...)
		}
		for _, r := range p.Rules() {
			if d, ok := r.(Describable); ok {
				out = append(out, d.Descriptors()...)
			}
		}
	}

	return out
}

// NormalizeDescriptors ordena por ID, deduplica y descarta los de ID vacío
// (SARIF exige un `id` único en cada reportingDescriptor de driver.rules[],
// así que un catálogo con IDs repetidos no es representable). El orden de
// entrada de reglas/packs/rulepacks no es determinista respecto a un mapa,
// así que esto es lo que hace estable la salida del reporte.
//
// Dedup por ID: si dos descriptores declaran el MISMO id con metadata
// distinta (p.ej. dos rulepacks que colisionan en "WCAG001"), gana el que
// aparezca de último en el orden de recolección de CollectDescriptors —
// una colisión de id entre packs es una mala configuración del autor, no
// algo que core intente fusionar. El resultado es determinista dado un
// orden de registro fijo.
func NormalizeDescriptors(ds []RuleDescriptor) []RuleDescriptor {
	byID := make(map[string]RuleDescriptor, len(ds))
	for _, d := range ds {
		if d.ID == "" {
			continue
		}
		byID[d.ID] = d
	}

	out := make([]RuleDescriptor, 0, len(byID))
	for _, d := range byID {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out
}
