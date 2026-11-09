// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import "encoding/json"

// PruneUnusedCapabilities recalcula la declaración raíz (schemaVersion y
// capabilities) de un AST serializado que devolvió un filtro externo, para
// las capabilities que siguen declaradas aunque su contenido ya no esté.
//
// Es el caso normal de un filtro que reemplaza nodos: quien cambia cada
// gráfica con `unit` por una imagen deja declarada chart-content-v1, y quien
// cambia todas las fórmulas deja math-source-v1. Antes el filtro tenía que
// rehacer la declaración a mano con la tabla de extensiones de este core;
// si no lo hacía, DecodeAST rechazaba la salida. Una capability sin contenido
// no tiene nada que proteger, así que se quita y la versión baja a la de la
// extensión más nueva que queda (o a LegacySchemaVersion si no queda
// ninguna).
//
// Solo quita, nunca agrega: un campo opt-in presente sin su capability sigue
// siendo un error de DecodeAST, porque es contenido de un contrato que el
// filtro no declaró. Tampoco toca una declaración mal formada (versión o
// capability desconocida, repetidas, versión que no corresponde): esa la
// reporta DecodeAST tal cual. Fuera de la raíz no cambia ningún byte.
//
// Devuelve data sin cambios, y dropped vacío, cuando no hay nada que quitar
// o el JSON no se puede leer.
func PruneUnusedCapabilities(data []byte) (out []byte, dropped []string) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return data, nil
	}
	rawCaps, ok := root["capabilities"]
	if !ok {
		return data, nil
	}
	var declared []string
	if err := json.Unmarshal(rawCaps, &declared); err != nil || len(declared) == 0 {
		return data, nil
	}
	var version string
	if err := json.Unmarshal(root["schemaVersion"], &version); err != nil {
		return data, nil
	}
	if err := checkDeclaredContract(version, declared, true); err != nil {
		return data, nil
	}
	// Lectura permisiva solo para detectar qué se usa. El contrato completo
	// lo vuelve a validar DecodeAST sobre el resultado.
	var doc AST
	if err := json.Unmarshal(data, &doc); err != nil {
		return data, nil
	}
	used := map[string]bool{}
	for _, c := range UsedCapabilities(&doc) {
		used[c] = true
	}
	isDeclared := map[string]bool{}
	for _, c := range declared {
		isDeclared[c] = true
	}
	var kept []string
	for _, ext := range extensions {
		if !isDeclared[ext.capability] {
			continue
		}
		if used[ext.capability] {
			kept = append(kept, ext.capability)
		} else {
			dropped = append(dropped, ext.capability)
		}
	}
	if len(dropped) == 0 {
		return data, nil
	}
	if len(kept) == 0 {
		// Una versión legada no puede llevar la clave capabilities, ni vacía.
		delete(root, "capabilities")
	} else {
		encoded, err := json.Marshal(kept)
		if err != nil {
			return data, nil
		}
		root["capabilities"] = encoded
	}
	encodedVersion, err := json.Marshal(contractVersion(kept))
	if err != nil {
		return data, nil
	}
	root["schemaVersion"] = encodedVersion
	rewritten, err := json.Marshal(root)
	if err != nil {
		return data, nil
	}
	return rewritten, dropped
}
