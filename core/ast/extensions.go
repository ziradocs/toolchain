// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"fmt"
	"strings"
)

// extension describe una extensión opt-in del contrato AST: la capability que
// la declara, la versión de schema que la introdujo y cómo detectar si un
// documento la usa de verdad.
//
// La regla de versionado es una sola para todas: un documento declara
// exactamente las capabilities de las extensiones que usa, y su schemaVersion
// es la de la extensión más nueva entre ellas. Sin extensiones queda en
// LegacySchemaVersion (y los lectores aceptan PreviousSchemaVersion). Así una
// extensión nueva es una fila más de esta tabla y no una combinación más de
// casos escritos a mano.
type extension struct {
	capability string
	version    string
	used       func(*AST) bool
}

// extensions está en orden de versión ascendente; ese orden es también el
// orden canónico de AST.Capabilities.
var extensions = []extension{
	{TableRowsCapability, TableSchemaVersion, UsesTableRows},
	{NestedListTypesCapability, NestedListSchemaVersion, UsesNestedListTypes},
	{TypedHeadingsCapability, TypedHeadingsSchemaVersion, UsesTypedHeadings},
	{MediaFigureCapability, MediaFigureSchemaVersion, UsesMediaFigure},
	{CodeFilenameCapability, CodeFilenameSchemaVersion, UsesCodeFilename},
}

// UsesCodeFilename reporta si algún CodeElement declara filename.
func UsesCodeFilename(doc *AST) bool {
	used := false
	_ = Walk(doc, func(n Node) error {
		if c, ok := n.(*CodeElement); ok && c.Filename != "" {
			used = true
		}
		return nil
	})
	return used
}

// UsesMediaFigure reporta si algún MediaElement declara poster o caption.
func UsesMediaFigure(doc *AST) bool {
	used := false
	_ = Walk(doc, func(n Node) error {
		if m, ok := n.(*MediaElement); ok && (m.Poster != "" || m.Caption != "") {
			used = true
		}
		return nil
	})
	return used
}

// UsesTypedHeadings reporta si doc contiene algún HeadingElement, incluidos
// los anidados en bloques especiales o columnas.
func UsesTypedHeadings(doc *AST) bool {
	used := false
	_ = Walk(doc, func(n Node) error {
		if _, ok := n.(*HeadingElement); ok {
			used = true
		}
		return nil
	})
	return used
}

// UsedCapabilities devuelve, en orden canónico, las capabilities de las
// extensiones que doc usa realmente.
func UsedCapabilities(doc *AST) []string {
	var caps []string
	for _, ext := range extensions {
		if ext.used(doc) {
			caps = append(caps, ext.capability)
		}
	}
	return caps
}

// UsesExtensions reporta si doc usa al menos una extensión opt-in.
func UsesExtensions(doc *AST) bool {
	return len(UsedCapabilities(doc)) > 0
}

// IsExtendedSchemaVersion reporta si version pertenece a alguna extensión.
func IsExtendedSchemaVersion(version string) bool {
	for _, ext := range extensions {
		if ext.version == version {
			return true
		}
	}
	return false
}

// DeclaresExtendedContract reporta si doc se presenta como extendido, ya sea
// por lo que usa o por lo que declara. Sirve para decidir cuándo validar.
func DeclaresExtendedContract(doc *AST) bool {
	return UsesExtensions(doc) || IsExtendedSchemaVersion(doc.SchemaVersion) || len(doc.Capabilities) > 0
}

// KnownCapabilities lista las capabilities que este core entiende.
func KnownCapabilities() []string {
	out := make([]string, len(extensions))
	for i, ext := range extensions {
		out[i] = ext.capability
	}
	return out
}

func extensionFor(capability string) (extension, int, bool) {
	for i, ext := range extensions {
		if ext.capability == capability {
			return ext, i, true
		}
	}
	return extension{}, -1, false
}

// contractVersion es la schemaVersion que corresponde a un conjunto de
// capabilities usadas (ya en orden canónico).
func contractVersion(caps []string) string {
	if len(caps) == 0 {
		return LegacySchemaVersion
	}
	ext, _, _ := extensionFor(caps[len(caps)-1])
	return ext.version
}

// checkDeclaredContract valida una declaración version/capabilities por sí
// sola, antes de mirar el contenido: versión conocida, capabilities conocidas
// y sin repetir, y que la versión sea exactamente la de la capability más
// nueva declarada.
func checkDeclaredContract(version string, caps []string, capsPresent bool) error {
	if version == PreviousSchemaVersion || version == LegacySchemaVersion {
		if capsPresent {
			return fmt.Errorf("legacy schemaVersion cannot declare capabilities")
		}
		return nil
	}
	if !IsExtendedSchemaVersion(version) {
		return fmt.Errorf("unsupported schemaVersion %q", version)
	}
	if len(caps) == 0 {
		return fmt.Errorf("schemaVersion %s requires capability declarations", version)
	}
	seen := map[string]bool{}
	newest := -1
	for _, c := range caps {
		_, idx, ok := extensionFor(c)
		if !ok {
			return fmt.Errorf("schemaVersion %s requires capability from %s; unknown %q", version, strings.Join(KnownCapabilities(), ", "), c)
		}
		if seen[c] {
			return fmt.Errorf("schemaVersion %s requires capability declarations without duplicates (%q)", version, c)
		}
		seen[c] = true
		if idx > newest {
			newest = idx
		}
	}
	if extensions[newest].version != version {
		return fmt.Errorf("schemaVersion %s requires capability declarations whose newest extension is %s (declared %v)", version, version, caps)
	}
	return nil
}

// validateUsedContract compara lo declarado contra lo usado: las
// capabilities deben ser exactamente las de las extensiones presentes y la
// versión la de la más nueva. Nada se repara en silencio.
func validateUsedContract(doc *AST) error {
	used := UsedCapabilities(doc)
	if len(used) == 0 {
		if (doc.SchemaVersion != LegacySchemaVersion && doc.SchemaVersion != PreviousSchemaVersion) || len(doc.Capabilities) != 0 {
			return fmt.Errorf("unsupported AST version/capabilities without extensions: %q %v", doc.SchemaVersion, doc.Capabilities)
		}
		return nil
	}
	want := contractVersion(used)
	if doc.SchemaVersion != want || !hasCapabilities(doc.Capabilities, used...) {
		return fmt.Errorf("%s requires schemaVersion %s and capabilities %v (got %q %v)", describeUsed(used), want, used, doc.SchemaVersion, doc.Capabilities)
	}
	return nil
}

func describeUsed(caps []string) string {
	names := map[string]string{
		TableRowsCapability:       "tableRows",
		NestedListTypesCapability: "nested lists",
		TypedHeadingsCapability:   "typed headings",
		MediaFigureCapability:     "media poster/caption",
		CodeFilenameCapability:    "code filenames",
	}
	parts := make([]string, len(caps))
	for i, c := range caps {
		parts[i] = names[c]
	}
	return strings.Join(parts, " and ")
}
