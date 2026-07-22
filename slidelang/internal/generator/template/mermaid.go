// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"embed"
	"fmt"

	"go.ziradocs.com/core/v2/renderer"
)

//go:embed assets/js/modules/mermaid.js
var mermaidAssets embed.FS

// GetMermaidJS retorna el módulo JavaScript para Mermaid desde archivo externo
func GetMermaidJS() string {
	// Load external mermaid module file
	content, err := mermaidAssets.ReadFile("assets/js/modules/mermaid.js")
	if err != nil {
		// Fallback to inline module if external file not found
		fmt.Printf("Warning: Could not load external mermaid.js, using fallback: %v\n", err)
		return getMermaidJSFallback()
	}

	return string(content)
}

// getMermaidJSFallback provides a minimal fallback if external file is not available
func getMermaidJSFallback() string {
	return `// === MERMAID MODULE (FALLBACK) ===
// Note: This is a fallback implementation. External mermaid.js module is recommended.
const SlideLangMermaid = {
    initialized: false,
    
    init: function() {
        if (typeof console !== 'undefined' && console.warn) {
            console.warn('[SlideLang.mermaid] Using fallback module - external mermaid.js not found');
        }
        
        if (typeof mermaid === 'undefined') {
            if (typeof console !== 'undefined' && console.error) {
                console.error('[SlideLang.mermaid] Mermaid library not loaded');
            }
            return;
        }
        
        if (this.initialized) return;
        
        this.initialized = true;
        
        // Basic mermaid configuration.
        // La config canónica vive en renderer.MermaidInitConfigJS (issue #85):
        // securityLevel:'strict'+htmlLabels:false garantizados en un solo lugar.
        mermaid.initialize(` + renderer.MermaidInitConfigJS(false) + `);
        
        if (typeof console !== 'undefined' && console.log) {
            console.log('[SlideLang.mermaid] Fallback module initialized');
        }
    }
};

// Auto-register fallback module
(function() {
    function registerMermaid() {
        if (typeof window !== 'undefined' && window.SlideLang) {
            SlideLang.registerModule('mermaid', SlideLangMermaid);
            SlideLangMermaid.init();
        } else {
            setTimeout(registerMermaid, 50);
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', registerMermaid);
    } else {
        registerMermaid();
    }
})();
`
}
