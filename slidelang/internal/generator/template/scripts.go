// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

// GetInitJS retorna el código de inicialización modular
func GetInitJS(modules []string) string {
	initModules := ""

	// Generar inicialización específica por módulo
	for _, module := range modules {
		switch module {
		case "navigation":
			initModules += `        if (typeof SlideNavigation !== 'undefined') {
            SlideLang.registerModule('navigation', SlideNavigation);
            SlideNavigation.init();
        }
`
		case "utilities":
			initModules += `        if (typeof SlideUtilities !== 'undefined') {
            SlideLang.registerModule('utilities', SlideUtilities);
            SlideUtilities.init();
        }
`
		case "mermaid":
			initModules += `        if (SlideLang.mermaid) {
            SlideLang.registerModule('mermaid', SlideLang.mermaid);
            SlideLang.mermaid.init();
        }
`
		case "charts":
			initModules += `        if (SlideLang.charts) {
            SlideLang.registerModule('charts', SlideLang.charts);
            SlideLang.charts.init();
        }
`
		case "maps":
			initModules += `        if (SlideLang.maps) {
            SlideLang.registerModule('maps', SlideLang.maps);
            SlideLang.maps.init();
        }
`
		case "directives":
			initModules += `        if (SlideLang.directives) {
            SlideLang.registerModule('directives', SlideLang.directives);
        }
`
		case "floatingMenu":
			initModules += `        if (typeof SlideFloatingMenu !== 'undefined') {
            SlideLang.registerModule('floatingMenu', SlideFloatingMenu);
            SlideFloatingMenu.init();
        }
`
		}
	}

	return `// === INITIALIZATION ===
// Inicializar SlideLang cuando el DOM esté listo
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function() {
        SlideLang.init();
        
        // Inicializar módulos disponibles
` + initModules + `        
        // Configurar integración entre módulos
        if (SlideLang.hasModule('navigation') && SlideLang.hasModule('maps')) {
            // Refrescar mapas cuando cambie de slide
            document.addEventListener('slideChanged', function(e) {
                if (SlideLang.maps && SlideLang.maps.refreshMaps) {
                    setTimeout(() => SlideLang.maps.refreshMaps(), 100);
                }
            });
        }
        
    });
} else {
    SlideLang.init();
    
    // Inicializar módulos disponibles
` + initModules + `    
    // Configurar integración entre módulos
    if (SlideLang.hasModule('navigation') && SlideLang.hasModule('maps')) {
        // Refrescar mapas cuando cambie de slide
        document.addEventListener('slideChanged', function(e) {
            if (SlideLang.maps && SlideLang.maps.refreshMaps) {
                setTimeout(() => SlideLang.maps.refreshMaps(), 100);
            }
        });
    }
    
}
`
}
