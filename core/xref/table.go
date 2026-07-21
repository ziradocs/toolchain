// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package xref implementa la numeración y resolución de referencias
// cruzadas del MVP OSS (issue #239, decisión B): figuras/tablas etiquetadas
// (`label:`) se numeran en orden de documento, y `\ref{label}` en cualquier
// campo de texto se reescribe a un link markdown a esa figura/tabla.
//
// Dos fases explícitas, no un solo walk interleaved — porque `\ref{fig:x}`
// puede PRECEDER a la definición de `fig:x` en el documento (forward
// reference): la Fase A (AssignNumbers) recorre TODO el documento primero y
// completa la tabla label→(número, ancla); solo entonces la Fase B
// (ResolveRefs) resuelve los \ref, ya con la tabla completa disponible.
package xref

// Kind identifica qué tipo de entidad referenciable asignó un label —
// determina el texto que ResolveRefs genera ("Figura N" vs "Tabla N"),
// independiente de qué prefijo haya elegido el autor para el label en sí
// (label: "fig:x" es una convención humana, no algo que el mecanismo
// interprete).
type Kind string

const (
	KindFigure   Kind = "Figura"
	KindTable    Kind = "Tabla"
	KindEquation Kind = "Ecuación"
)

// Entry es la entrada de la tabla de referencias para un label.
type Entry struct {
	Kind     Kind
	Number   int
	AnchorID string
}

// Table mapea label → Entry. Construida por AssignNumbers (Fase A),
// consumida por ResolveRefs (Fase B).
type Table map[string]Entry
