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
