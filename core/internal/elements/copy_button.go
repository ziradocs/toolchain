// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

// GetCopyButtonJS returns a minimal JavaScript function for copy button functionality
// This can be integrated directly into code.go if needed
func GetCopyButtonJS() string {
	return `// === COPY BUTTON FOR CODE BLOCKS ===
function addCopyButtonToCodeBlock(codeElement) {
    if (codeElement.querySelector('.copy-button')) return;
    
    const button = document.createElement('button');
    button.className = 'copy-button';
    button.innerHTML = '📋';
    button.title = 'Copiar código';
    button.setAttribute('aria-label', 'Copiar código al portapapeles');
    
    button.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        
        const pre = codeElement.querySelector('pre');
        if (pre) {
            const code = pre.textContent || pre.innerText;
            
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(code).then(() => {
                    showCopySuccess(button);
                }).catch(() => {
                    fallbackCopy(code, button);
                });
            } else {
                fallbackCopy(code, button);
            }
        }
    });
    
    codeElement.appendChild(button);
}

function showCopySuccess(button) {
    button.innerHTML = '✓';
    button.title = '¡Copiado!';
    button.classList.add('success');
    setTimeout(() => {
        button.innerHTML = '📋';
        button.title = 'Copiar código';
        button.classList.remove('success');
    }, 2000);
}

function fallbackCopy(text, button) {
    try {
        const textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'fixed';
        textarea.style.left = '-999999px';
        textarea.style.top = '-999999px';
        document.body.appendChild(textarea);
        textarea.focus();
        textarea.select();
        
        const successful = document.execCommand('copy');
        document.body.removeChild(textarea);
        
        if (successful) {
            showCopySuccess(button);
        } else {
            showCopyError(button);
        }
    } catch (err) {
        showCopyError(button);
    }
}

function showCopyError(button) {
    button.innerHTML = '✗';
    button.title = 'Error al copiar';
    button.classList.add('error');
    setTimeout(() => {
        button.innerHTML = '📋';
        button.title = 'Copiar código';
        button.classList.remove('error');
    }, 2000);
}

// Initialize copy buttons for all code blocks
function initCodeCopyButtons() {
    document.querySelectorAll('.slidelang-element.code').forEach(addCopyButtonToCodeBlock);
}

// Auto-initialize
if (typeof document !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initCodeCopyButtons);
    } else {
        initCodeCopyButtons();
    }
}
`
}

// GetCopyButtonCSS returns the minimal CSS styles for the copy button
func GetCopyButtonCSS() string {
	return `/* === COPY BUTTON STYLES === */
.element.code {
    position: relative;
}

.copy-button {
    position: absolute;
    top: 0.5rem;
    right: 0.5rem;
    background: rgba(255, 255, 255, 0.1);
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 4px;
    color: #fff;
    cursor: pointer;
    font-size: 14px;
    padding: 0.25rem 0.5rem;
    transition: all 0.2s ease;
    z-index: 10;
    backdrop-filter: blur(4px);
}

.copy-button:hover {
    background: rgba(255, 255, 255, 0.2);
    border-color: rgba(255, 255, 255, 0.3);
    transform: scale(1.05);
}

.copy-button.success {
    background: rgba(34, 197, 94, 0.2);
    border-color: rgba(34, 197, 94, 0.4);
    color: #22c55e;
}

.copy-button.error {
    background: rgba(239, 68, 68, 0.2);
    border-color: rgba(239, 68, 68, 0.4);
    color: #ef4444;
}

/* Dark theme adjustments */
.theme-dark .copy-button {
    background: rgba(0, 0, 0, 0.3);
    border-color: rgba(255, 255, 255, 0.1);
    color: #e2e8f0;
}

.theme-dark .copy-button:hover {
    background: rgba(0, 0, 0, 0.4);
    border-color: rgba(255, 255, 255, 0.2);
}
`
}
