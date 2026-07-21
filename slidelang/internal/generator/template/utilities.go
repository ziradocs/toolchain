// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

// GetUtilitiesJS retorna utilidades básicas de JavaScript para elementos interactivos
func GetUtilitiesJS() string {
	return `// === UTILITIES MODULE ===
const SlideUtilities = {
    // Función para cambiar tabs en code groups
    switchCodeTab: function(clickedTab, blockIndex) {
        const codeGroup = clickedTab.closest('.slidelang-code-group');
        if (!codeGroup) return;
        
        // Desactivar todas las tabs
        const tabs = codeGroup.querySelectorAll('.slidelang-tab');
        tabs.forEach(tab => tab.classList.remove('active'));

        // Activar la tab clickeada
        clickedTab.classList.add('active');

        // Desactivar todos los bloques de código
        const codeBlocks = codeGroup.querySelectorAll('.slidelang-code-block');
        codeBlocks.forEach(block => block.classList.remove('active'));
        
        // Activar el bloque correspondiente
        const targetBlock = codeGroup.querySelector('#code-block-' + blockIndex);
        if (targetBlock) {
            targetBlock.classList.add('active');
        }
    },
    
    // Función para toggle details collapsibles
    toggleDetails: function(detailsElement) {
        if (!detailsElement) return;
        detailsElement.classList.toggle('expanded');
        
        // Emitir evento para extensibilidad
        const event = new CustomEvent('detailsToggled', {
            detail: {
                element: detailsElement,
                expanded: detailsElement.classList.contains('expanded')
            }
        });
        document.dispatchEvent(event);
    },
    
    // Copiar contenido de código al clipboard
    copyCodeToClipboard: function(button) {
        const codeBlock = button.closest('.slidelang-element.slidelang-code').querySelector('pre code, pre');
        if (!codeBlock) return;
        
        const code = codeBlock.textContent;
        
        if (navigator.clipboard) {
            navigator.clipboard.writeText(code).then(() => {
                this.showCopyFeedback(button, 'Copiado!');
            }).catch(() => {
                this.fallbackCopyToClipboard(code, button);
            });
        } else {
            this.fallbackCopyToClipboard(code, button);
        }
    },
    
    // Fallback para copiar sin clipboard API
    fallbackCopyToClipboard: function(text, button) {
        const textArea = document.createElement('textarea');
        textArea.value = text;
        textArea.style.position = 'fixed';
        textArea.style.left = '-999999px';
        textArea.style.top = '-999999px';
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        
        try {
            document.execCommand('copy');
            this.showCopyFeedback(button, 'Copiado!');
        } catch (err) {
            this.showCopyFeedback(button, 'Error al copiar');
        }
        
        document.body.removeChild(textArea);
    },
    
    // Mostrar feedback de copia
    showCopyFeedback: function(button, message) {
        const originalText = button.textContent;
        button.textContent = message;
        button.style.background = '#22c55e';
        
        setTimeout(() => {
            button.textContent = originalText;
            button.style.background = '';
        }, 1500);
    },
    
    // Inicializar botones de copia
    initCopyButtons: function() {
        document.querySelectorAll('.slidelang-element.slidelang-code').forEach(codeElement => {
            if (!codeElement.querySelector('.slidelang-copy-button')) {
                const button = document.createElement('button');
                button.className = 'slidelang-copy-button copy-button';
                button.textContent = 'Copiar';
                button.onclick = () => this.copyCodeToClipboard(button);
                codeElement.appendChild(button);
            }
        });
    },

    // Conectar tabs de code-group y bloques "details" colapsables. Antes
    // estos usaban onclick="..." inline en el propio elemento del template
    // — un script-src con nonce (ver core/renderer/csp.go)
    // bloquea atributos onXXX= igual que bloquearía un script inline sin nonce, así
    // que se asigna la propiedad .onclick vía JS en su lugar (eso sí lo
    // permite el CSP: es una asignación de propiedad dentro de un script ya
    // autorizado, no un atributo inline).
    initInteractiveElements: function() {
        const self = this;
        document.querySelectorAll('.slidelang-code-group .slidelang-tab').forEach(tab => {
            tab.onclick = () => self.switchCodeTab(tab, parseInt(tab.dataset.tabIndex, 10));
        });
        document.querySelectorAll('.slidelang-details').forEach(el => {
            el.onclick = () => self.toggleDetails(el);
        });
    },

    // Inicialización
    init: function() {
        this.initCopyButtons();
        this.initInteractiveElements();
    }
};

// Exponer funciones globales para compatibilidad con templates
function switchCodeTab(clickedTab, blockIndex) {
    SlideUtilities.switchCodeTab(clickedTab, blockIndex);
}

function toggleDetails(detailsElement) {
    SlideUtilities.toggleDetails(detailsElement);
}

function copyCode(button) {
    SlideUtilities.copyCodeToClipboard(button);
}

// Auto-initialize
if (typeof window !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() {
            SlideUtilities.init();
        });
    } else {
        SlideUtilities.init();
    }
}

// Export for module usage
if (typeof module !== 'undefined' && module.exports) {
    module.exports = SlideUtilities;
}
`
}
