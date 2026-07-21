// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"go.ziradocs.com/slidelang/internal/generator/css"
)

// GetResponsiveCSS retorna los estilos CSS responsivos desde los assets modulares
func GetResponsiveCSS() string {
	fileLoader := css.NewCSSFileLoader()

	// Intentar cargar desde el archivo modular primero
	if responsiveCSS, err := fileLoader.LoadResponsiveCSS(); err == nil {
		return responsiveCSS
	}

	// Fallback CSS responsivo en caso de que el archivo no esté disponible
	return `/* === RESPONSIVE STYLES === */
/* Responsive breakpoints */
@media (max-width: 768px) {
    .slidelang-slide {
        width: 95vw;
        height: 85vh;
        padding: 30px;
    }
    
    .slidelang-slide.slidelang-title-slide h1 {
        font-size: 2.5rem;
    }
    
    .slidelang-slide.slidelang-content-slide h1 {
        font-size: 2rem;
    }
    
    .slidelang-element.slidelang-text {
        font-size: 1.1rem;
    }
    
    .slidelang-element.slidelang-points li {
        font-size: 1rem;
        padding-left: 1.5rem;
    }
    
    .slidelang-element.slidelang-table {
        font-size: 0.9rem;
    }
    
    .slidelang-element.slidelang-table th,
    .slidelang-element.slidelang-table td {
        padding: 0.5rem;
    }
}

@media (max-width: 480px) {
    .slidelang-slide {
        width: 98vw;
        height: 90vh;
        padding: 20px;
    }
    
    .slidelang-slide.slidelang-title-slide h1 {
        font-size: 2rem;
    }
    
    .slidelang-slide.slidelang-title-slide h2 {
        font-size: 1.3rem;
    }
    
    .slidelang-slide.slidelang-content-slide h1 {
        font-size: 1.5rem;
    }
    
    .slidelang-element.slidelang-text {
        font-size: 1rem;
    }
    
    .slidelang-cards-grid {
        grid-template-columns: 1fr;
    }
    
    .slidelang-element.slidelang-image-gallery {
        grid-template-columns: 1fr;
    }
}

@media print {
    .slidelang-slide {
        width: auto;
        height: auto;
        box-shadow: none;
        page-break-after: always;
        display: block !important;
    }
    
    /* Navigation print rules are handled in navigation.css */
}
`
}
