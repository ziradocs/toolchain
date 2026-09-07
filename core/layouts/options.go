// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package layouts declara las opciones de presentación que un slide puede
// llevar además de su layout (issue #255).
//
// Vive en su propio paquete y no en core/linter, que es donde están los
// schemas, por una razón de dependencias: los parsers necesitan saber qué
// llaves aceptar y de qué tipo, y `core/parser` no puede importar
// `core/linter` sin invertir la dirección. Este paquete no importa nada del
// toolchain, así que lo pueden usar los dos.
package layouts

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// OptionType es el tipo de una opción. Deliberadamente corto: cada tipo nuevo
// agrega una rama en el coercer, una regla en el linter y una columna en la
// documentación, así que se agrega uno cuando hay una opción que lo necesita,
// no antes.
type OptionType int

const (
	// OptionInt es un entero acotado por Min/Max.
	OptionInt OptionType = iota
	// OptionEnum es uno de un conjunto cerrado de strings.
	OptionEnum
)

// OptionSpec declara una opción: su nombre, su tipo y sus límites.
type OptionSpec struct {
	Name string
	Type OptionType
	// Min/Max acotan una OptionInt (inclusive).
	Min, Max int
	// Values son los valores aceptados de una OptionEnum.
	Values []string
	// Doc es la línea que la documentación y los mensajes de error usan.
	Doc string
}

// columnsOption y alignOption son las dos opciones del primer corte.
//
// Se eligieron porque el CSS de layouts (issue #254) las puede honrar sin
// código nuevo: el template las emite como `data-layout-*` y una regla las
// lee. Una opción que ningún renderer consume sería una promesa vacía, que es
// justo lo que #255 existe para evitar.
var (
	columnsOption = OptionSpec{
		Name: "columns", Type: OptionInt, Min: 1, Max: 4,
		Doc: "cuántas columnas usa la retícula del slide (1-4)",
	}
	alignOption = OptionSpec{
		Name: "align", Type: OptionEnum, Values: []string{"left", "center"},
		Doc: "alineación del contenido (left o center)",
	}
)

// registry mapea cada layout a las opciones que acepta.
//
// Que una opción esté declarada para un layout y no para otro no es
// burocracia: es lo que permite reportar `columns: 2` sobre un `hero` como un
// error de escritura en vez de ignorarlo en silencio, que es la clase de
// defecto que el issue #255 pide evitar explícitamente.
var registry = map[string][]OptionSpec{
	"comparison":     {columnsOption},
	"stats":          {columnsOption},
	"dashboard":      {columnsOption},
	"call_to_action": {columnsOption, alignOption},
	"hero":           {alignOption},
	"testimonial":    {alignOption},
}

// Options devuelve las opciones que un layout acepta, ordenadas por nombre.
func Options(layout string) []OptionSpec {
	specs := append([]OptionSpec(nil), registry[layout]...)
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs
}

// OptionNames devuelve los nombres de esas opciones, para los mensajes.
func OptionNames(layout string) []string {
	specs := Options(layout)
	names := make([]string, len(specs))
	for i, s := range specs {
		names[i] = s.Name
	}
	return names
}

// LayoutsWithOptions devuelve todos los layouts que declaran alguna opción.
func LayoutsWithOptions() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Config lleva las opciones ya resueltas de un slide. Es un struct tipado y no
// un mapa: el contrato JSON y el tipo TypeScript salen por reflexión, el
// formatter sabe qué emitir, y el linter puede hablar de campos concretos. Un
// `map[string]any` haría las cuatro cosas más difíciles y volvería a hacer
// invisible un typo, que es lo que #255 quiere evitar.
//
// El cero de cada campo significa "no declarado".
type Config struct {
	Columns int    `json:"columns,omitempty"`
	Align   string `json:"align,omitempty"`
}

// IsZero reporta si no se declaró ninguna opción, para que el llamador no
// adjunte un Config vacío al AST.
func (c Config) IsZero() bool {
	return c == Config{}
}

// Accepts reporta si layout declara una opción con ese nombre.
func Accepts(layout, key string) bool {
	for _, spec := range registry[layout] {
		if spec.Name == key {
			return true
		}
	}
	return false
}

// IsKnownOption reporta si key es una opción de ALGÚN layout. Sirve para
// distinguir "esta llave no va en este layout" (un error que vale la pena
// nombrar) de "esta llave no es una opción" (que es otro mensaje).
func IsKnownOption(key string) bool {
	for _, specs := range registry {
		for _, spec := range specs {
			if spec.Name == key {
				return true
			}
		}
	}
	return false
}

// Apply valida y asigna una opción sobre cfg.
//
// Devuelve error en vez de escribir un diagnóstico porque los dos llamadores
// —el parser flex y el strict— los reportan distinto, y porque el linter
// necesita la misma validación sobre un AST que no pasó por ningún parser (el
// modo JSON).
func Apply(cfg *Config, layout, key, value string) error {
	var spec *OptionSpec
	for i := range registry[layout] {
		if registry[layout][i].Name == key {
			spec = &registry[layout][i]
			break
		}
	}
	if spec == nil {
		if IsKnownOption(key) {
			return fmt.Errorf("%q is not an option of layout %q; it accepts: %s",
				key, layout, strings.Join(OptionNames(layout), ", "))
		}
		return fmt.Errorf("%q is not a layout option", key)
	}

	switch spec.Type {
	case OptionInt:
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("%q must be a whole number between %d and %d, got %q",
				key, spec.Min, spec.Max, value)
		}
		if n < spec.Min || n > spec.Max {
			return fmt.Errorf("%q must be between %d and %d, got %d", key, spec.Min, spec.Max, n)
		}
		if key == "columns" {
			cfg.Columns = n
		}
	case OptionEnum:
		v := strings.ToLower(strings.TrimSpace(value))
		ok := false
		for _, allowed := range spec.Values {
			if v == allowed {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("%q must be one of %s, got %q", key, strings.Join(spec.Values, "/"), value)
		}
		if key == "align" {
			cfg.Align = v
		}
	}
	return nil
}
