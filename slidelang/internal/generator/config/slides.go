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

// chromeAliases mapea un tipo de slide al nombre CANÓNICO de su familia para
// efectos de la clase CSS. La clave es que estos alias ya son alias en todas
// partes menos en el CSS: config.IsSlideTitle trata `title_slide`, `cover` e
// `intro` como slides de título, y IsSlideContent pone a `chapter` junto a
// `section`. Pero la clase se armaba con el tipo verbatim, así que un
// `title_slide` emitía `slidelang-title_slide-slide` y ningún tema —ni el
// empaquetado ni uno externo— tiene una regla con ese nombre.
//
// El resultado medido en el navegador: un slide `title_slide` salía con fondo
// blanco, texto oscuro y justify-content: flex-start, o sea igual que un slide
// de contenido, mientras `title` salía con el degradado, texto blanco y
// centrado. Elegir el alias cambiaba el render, que es exactamente lo que un
// alias no debe hacer.
//
// Se resuelve acá y no agregando selectores a la CSS base porque los temas
// —incluidos los externos, que este repo no controla— hacen key en
// `.slidelang-title-slide` y `.slidelang-section-slide`. Emitir la clase
// canónica arregla los cuatro alias en todos los temas a la vez, presentes y
// futuros; parchear el CSS solo arreglaría los que vienen en el paquete.
//
// `data-slide-type` NO se toca: sigue siendo el BlockType verbatim, que es lo
// que el linter valida, lo que viaja en el JSON del AST y lo que el CSS de
// layouts (issue #254) usa como selector.
var chromeAliases = map[string]string{
	"title_slide": "title",
	"cover":       "title",
	"intro":       "title",
	"chapter":     "section",
}

// ChromeClassType devuelve el nombre que va en la clase
// `slidelang-<nombre>-slide`. Para casi todos es el tipo mismo; para los alias
// de arriba, el canónico de su familia.
func ChromeClassType(slideType string) string {
	if canonical, ok := chromeAliases[slideType]; ok {
		return canonical
	}
	return slideType
}
