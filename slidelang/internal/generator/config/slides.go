// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package config

// IsSlideTitle determina si un tipo de slide es de título
func IsSlideTitle(slideType string) bool {
	titleTypes := []string{"title", "title_slide", "cover", "intro"}
	for _, t := range titleTypes {
		if slideType == t {
			return true
		}
	}
	return false
}

// IsSlideContent determina si un tipo de slide es de contenido
func IsSlideContent(slideType string) bool {
	if slideType == "" || slideType == "content" {
		return true
	}
	contentTypes := []string{"content", "section", "chapter", "code_example", "with_directive"}
	for _, t := range contentTypes {
		if slideType == t {
			return true
		}
	}
	return false
}

// slideTypesWithOwnChrome son los tipos que ya se visten por su cuenta: fondo
// propio, tipografía propia. El resto hereda el vestido de "contenido".
//
// Los primeros los viste la CSS base (y los temas externos). `hero` y
// `call_to_action` los viste su propio archivo de layout (issue #254), y por
// eso pertenecen acá aunque la base no los conozca: darles ADEMÁS el vestido
// de contenido no era neutro, era destructivo. El fondo blanco de
// `.slidelang-content-slide` empata en especificidad con el fondo del layout
// y le gana por orden —el CSS del tema se escribe después del de layouts—,
// pero el `color: var(--text-on-primary)` del layout sobrevive porque nadie
// lo pisa. Resultado: un `call_to_action` quedaba con texto blanco sobre
// fondo blanco, y en el ejemplo real
// examples/18_specialized_layouts/product_launch_presentation_flex.slidelang
// sus tres viñetas de oferta no se veían en pantalla.
var slideTypesWithOwnChrome = map[string]bool{
	"title": true, "title_slide": true, "cover": true, "intro": true,
	"section": true, "chapter": true,
	"closing": true, "end": true,
	"hero": true, "call_to_action": true,
}

// UsesContentChrome reporta si un slide debe llevar TAMBIÉN la clase
// `slidelang-content-slide`, además de la suya (issue #254).
//
// Sin esto, un slide tipado se veía PEOR que uno de contenido, no distinto:
// la base y todos los temas externos hacen key en `.slidelang-content-slide`
// para el fondo y el subrayado del h1, y un `stats` solo llevaba
// `slidelang-stats-slide`, así que no matcheaba nada y perdía las dos cosas.
// Tipar un slide no puede quitarle el vestido que ya tenía.
//
// Los tipos con vestido propio (título, sección, cierre) se excluyen: agregarles
// la clase de contenido les pisaría su fondo.
func UsesContentChrome(slideType string) bool {
	// "content" ya recibe la clase por la vía normal
	// (`slidelang-{{.Type}}-slide`); agregarla otra vez la duplica en el
	// atributo, que html-validate reporta como no-dup-class.
	if slideType == "content" {
		return false
	}
	return !slideTypesWithOwnChrome[slideType]
}
