// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// === MERMAID MODULE ===
const SlideLangMermaid = {
    initialized: false,
    processedDiagrams: new Set(),
    diagramsConfig: null,
    
    init: function() {
        if (typeof mermaid === 'undefined') {
            if (typeof console !== 'undefined' && console.warn) {
                console.warn('[SlideLang.mermaid] Mermaid library not found');
            }
            return;
        }
        
        if (this.initialized) {
            return;
        }
        
        // Configurar Mermaid
        this.configureMermaid();
        
        this.initialized = true;
        
        // Load diagrams configuration from metadata
        this.loadDiagramsFromMetadata();
        
        // Subscribe to navigation events
        this.subscribeToEvents();
        
        // Don't process diagrams immediately - wait for slide events
        // This ensures we only render when a slide is actually active
        // console.log('[SlideLang.mermaid] Mermaid module initialized, waiting for slide events');
    },
    
    configureMermaid: function() {
        // Nota (issue #85): fuente de verdad de la config del lado CLIENTE.
        // El par de seguridad securityLevel:'strict' + htmlLabels:false debe
        // mantenerse en sync con renderer.MermaidInitConfigJS (Go); este asset
        // se embebe con //go:embed y no puede importar la constante Go. El test
        // TestMermaidAsset_SafeAndConsistent bloquea cualquier htmlLabels:true.
        try {
            mermaid.initialize({
                startOnLoad: false,
                theme: 'default',
                securityLevel: 'strict',
                htmlLabels: false,
                flowchart: {
                    useMaxWidth: true,
                    htmlLabels: false,
                    curve: 'basis'
                },
                gantt: {
                    numberSectionStyles: 4
                },
                themeVariables: {
                    fontFamily: 'arial'
                },
                sequence: {
                    diagramMarginX: 50,
                    diagramMarginY: 10
                },
                // Add parsing configuration to be more strict
                parseEscape: false,
                suppressErrors: false
            });
            
            // console.log('[SlideLang.mermaid] Mermaid configured with version:', mermaid.version || 'unknown');
            
        } catch (error) {
            if (typeof console !== 'undefined' && console.error) {
                console.error('[SlideLang.mermaid] Error configuring Mermaid:', error);
            }
        }
    },
    
    loadDiagramsFromMetadata: function() {
        // Prioridad 1: Obtener datos de metadata
        const metadata = SlideLang.metadata || {};
        if (metadata.diagrams && metadata.diagrams.length > 0) {
            this.diagramsConfig = metadata.diagrams;
        } else {
            // Fallback: No hay metadata de diagramas
            this.diagramsConfig = [];
        }
    },
    
    subscribeToEvents: function() {
        // Listen to the standard navigation event
        document.addEventListener('slidelang:slideChanged', (event) => {
            // console.log('[SlideLang.mermaid] Slide changed event received:', event.detail);
            this.handleSlideChange(event.detail);
        });
        
        // Listen to core initialization to ensure we process diagrams when ready
        document.addEventListener('slidelang:coreInitialized', () => {
            // console.log('[SlideLang.mermaid] Core initialized, processing current slide');
            this.loadDiagramsFromMetadata();
            this.processCurrentSlide();
        });
        
        // Also listen for DOMContentLoaded in case core is already initialized
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', async () => {
                // Small delay to ensure slides are set up
                setTimeout(async () => {
                    // console.log('[SlideLang.mermaid] DOM loaded, checking for active slide');
                    await this.processCurrentSlide();
                }, 100);
            });
        } else {
            // DOM already loaded, check after a short delay
            setTimeout(async () => {
                // console.log('[SlideLang.mermaid] DOM already loaded, checking for active slide');
                await this.processCurrentSlide();
            }, 100);
        }
    },
    
    handleSlideChange: async function(slideInfo) {
        // Process diagrams on the new active slide
        await this.processCurrentSlide();
    },
    
    processCurrentSlide: async function() {
        // Only process diagrams in the currently active slide
        const activeSlide = document.querySelector('.slidelang-slide.slidelang-active');
        if (!activeSlide) {
            // console.log('[SlideLang.mermaid] No active slide found, skipping diagram processing');
            return;
        }
        
        // console.log('[SlideLang.mermaid] Processing diagrams for active slide:', activeSlide.id);
        
        const processedDiagrams = new Set();
        
        // Priority 1: Process diagrams from metadata
        if (this.diagramsConfig && this.diagramsConfig.length > 0) {
            for (const diagramConfig of this.diagramsConfig) {
                const element = document.getElementById(diagramConfig.id);
                
                // Only process if element exists AND is in the active slide
                if (element && activeSlide.contains(element) && !this.processedDiagrams.has(diagramConfig.id)) {
                    // console.log('[SlideLang.mermaid] Rendering diagram in active slide:', diagramConfig.id);
                    await this.renderDiagramFromConfig(element, diagramConfig);
                    processedDiagrams.add(diagramConfig.id);
                } else if (element && !activeSlide.contains(element)) {
                    // console.log('[SlideLang.mermaid] Skipping diagram not in active slide:', diagramConfig.id);
                }
            }
        }
        
        // Priority 2: Fallback to legacy DOM processing for unprocessed diagrams
        this.processLegacyDiagrams(processedDiagrams, activeSlide);
    },
    
    processLegacyDiagrams: function(processedDiagrams, activeSlide) {
        // Use the provided activeSlide or find it
        if (!activeSlide) {
            activeSlide = document.querySelector('.slidelang-slide.slidelang-active');
        }
        
        if (!activeSlide) {
            // console.log('[SlideLang.mermaid] No active slide found for legacy processing');
            return;
        }
        
        // Find mermaid elements in the active slide
        const elements = activeSlide.querySelectorAll('.slidelang-element.slidelang-mermaid .slidelang-mermaid');
        
        if (elements.length === 0) {
            return;
        }
        
        let legacyProcessed = 0;
        
        // Process each element that wasn't processed by metadata
        elements.forEach((element, index) => {
            const elementId = element.id || 'mermaid-legacy-' + activeSlide.id + '-' + index;
            element.id = elementId; // Ensure element has an ID
            
            // Skip if already processed by metadata or previously
            if (processedDiagrams.has(elementId) || this.processedDiagrams.has(elementId)) {
                return;
            }
            
            this.processDiagram(element, elementId);
            legacyProcessed++;
        });
    },
    
    renderDiagramFromConfig: async function(element, diagramConfig) {
        try {
            // Content llega ya como string plano (JSON.parse del metadata lo decodifica una sola vez)
            let graphDefinition = diagramConfig.content;

            // Basic validation
            if (!graphDefinition || graphDefinition.trim() === '') {
                if (typeof console !== 'undefined' && console.warn) {
                    console.warn('[SlideLang.mermaid] Empty diagram content for:', diagramConfig.id);
                }
                return;
            }

            // Minimal processing - content should already be normalized
            graphDefinition = this.miniminalProcessing(graphDefinition);
            
            // Validate the diagram content before rendering
            if (!this.validateDiagramContent(graphDefinition)) {
                console.warn('[SlideLang.mermaid] Invalid diagram content detected for:', diagramConfig.id);
                this.showMermaidMessage(element, 'Invalid diagram content', 'mermaid-error');
                return;
            }
            
            // Clear the element and set basic styles
            element.innerHTML = '';
            element.style.width = '100%';
            element.style.minHeight = '200px';
            
            // Generate unique diagram ID
            const diagramId = 'mermaid-svg-' + diagramConfig.id;
            
            // console.log('[SlideLang.mermaid] About to render diagram:', diagramConfig.id);
            // console.log('[SlideLang.mermaid] Content to render:', graphDefinition);
            // console.log('[SlideLang.mermaid] Content length:', graphDefinition.length);
            // console.log('[SlideLang.mermaid] Content bytes:', Array.from(graphDefinition).map(c => c.charCodeAt(0)));
            // console.log('[SlideLang.mermaid] First 10 chars:', graphDefinition.substring(0, 10).split('').map(c => `'${c}' (${c.charCodeAt(0)})`));
            
            // Use the correct Mermaid v11 API
            try {
                // For Mermaid v11, use mermaid.render with proper async/await pattern
                const { svg, bindFunctions } = await mermaid.render(diagramId, graphDefinition);
                
                // Insert the SVG into the element
                element.innerHTML = svg;
                
                // Bind any interactive functions if available
                if (bindFunctions) {
                    bindFunctions(element);
                }
                
                // Adjust SVG for responsive display
                this.adjustSVG(element);
                
                // Mark as processed
                this.processedDiagrams.add(diagramConfig.id);
                
                // console.log('[SlideLang.mermaid] Successfully rendered diagram:', diagramConfig.id);
                
            } catch (error) {
                console.error('[SlideLang.mermaid] Mermaid.render() failed for:', diagramConfig.id);
                console.error('[SlideLang.mermaid] Error details:', error);
                console.error('[SlideLang.mermaid] Content that failed:');
                console.error(graphDefinition);
                
                // Try alternative approach using mermaid.run
                // console.log('[SlideLang.mermaid] Trying with mermaid.run()...');
                try {
                    // Build the mermaid div via the DOM (not innerHTML): the
                    // diagram source is never parsed as HTML, only as text.
                    // Nota (issue #84): este sink es el equivalente cliente de
                    // renderer.buildMermaidDiv (Go). Queda exento del constructor
                    // Go a propósito porque usa textContent y nunca interpreta el
                    // contenido como HTML; si se cambia a interpolación de string,
                    // debe volver a pasar por un escape como EscapeHTML.
                    const mermaidDiv = document.createElement('div');
                    mermaidDiv.className = 'mermaid';
                    mermaidDiv.textContent = graphDefinition;
                    element.replaceChildren(mermaidDiv);

                    // Use mermaid.run to process the element
                    await mermaid.run({
                        nodes: [mermaidDiv],
                        suppressErrors: false
                    });

                    this.processedDiagrams.add(diagramConfig.id);
                    // console.log('[SlideLang.mermaid] Successfully processed with mermaid.run:', diagramConfig.id);

                } catch (runError) {
                    console.error('[SlideLang.mermaid] mermaid.run also failed:', runError);
                    this.showMermaidMessage(element, 'Error rendering diagram: ' + error.message, 'mermaid-error');
                }
            }
            
        } catch (error) {
            if (typeof console !== 'undefined' && console.error) {
                console.error('[SlideLang.mermaid] Error in renderDiagramFromConfig:', error);
            }
            this.showMermaidMessage(element, 'Error rendering diagram: ' + error.message, 'mermaid-error');
        }
    },

    processDiagram: function(element, elementId) {
        // Legacy mode no longer supported - all diagrams should come from metadata
        if (typeof console !== 'undefined' && console.warn) {
            console.warn('[SlideLang.mermaid] Legacy diagram processing attempted for element:', elementId, 'All Mermaid diagrams should be loaded from metadata.');
        }

        // Show informational message instead of rendering
        this.showMermaidMessage(element, 'Mermaid diagram should be loaded from metadata. Please ensure the diagram metadata is properly generated.', 'info');
    },
    
    renderDiagram: function(element, elementId, graphDefinition) {
        try {
            // Generar ID único para el diagrama
            const diagramId = 'mermaid-diagram-' + elementId;
            
            // Renderizar el diagrama
            mermaid.render(diagramId, graphDefinition).then(({ svg }) => {
                // Insertar el SVG en el elemento
                element.innerHTML = svg;
                
                // Ajustar SVG
                this.adjustSVG(element);
                
                // Mark as processed
                this.processedDiagrams.add(elementId);
                
            }).catch(error => {
                if (typeof console !== 'undefined' && console.error) {
                    console.error('[SlideLang.mermaid] Error rendering diagram:', error);
                }
                this.showMermaidMessage(element, 'Error rendering diagram: ' + error.message, 'error');
            });
        } catch (error) {
            if (typeof console !== 'undefined' && console.error) {
                console.error('[SlideLang.mermaid] Error in renderDiagram:', error);
            }
            this.showMermaidMessage(element, 'Error rendering diagram: ' + error.message, 'error');
        }
    },
    
    // showMermaidMessage evita innerHTML con contenido potencialmente
    // controlado por el atacante (error.message puede ecoar el source del
    // diagrama); usa textContent para que nunca se parsee como HTML.
    showMermaidMessage: function(element, message, className) {
        const div = document.createElement('div');
        div.className = className || 'mermaid-error';
        div.textContent = message;
        element.replaceChildren(div);
    },

    adjustSVG: function(element) {
        const svg = element.querySelector('svg');
        if (!svg) return;
        
        svg.style.maxWidth = '100%';
        svg.style.height = 'auto';
        svg.style.minHeight = '200px';
        svg.setAttribute('preserveAspectRatio', 'xMidYMid meet');
        
        // Obtener dimensiones originales y ajustar al contenedor
        const container = svg.closest('.slidelang-element.mermaid');
        if (container) {
            const containerHeight = container.clientHeight;
            
            // Ajustar si el diagrama es muy alto
            try {
                if (svg.getBBox && svg.getBBox().height > containerHeight * 0.8) {
                    svg.style.maxHeight = (containerHeight * 0.8) + 'px';
                }
            } catch(e) {
                // Silently handle getBBox errors
            }
        }
    },
    
    detectDiagramType: function(content) {
        const trimmed = content.trim();
        const firstLine = trimmed.split('\n')[0].trim().toLowerCase();
        
        if (firstLine.startsWith('flowchart')) {
            return 'flowchart';
        } else if (firstLine.startsWith('graph ')) {
            return 'flowchart';
        } else if (firstLine.startsWith('gantt')) {
            return 'gantt';
        } else if (firstLine.startsWith('sequencediagram')) {
            return 'sequence';
        } else if (firstLine.startsWith('classdiagram')) {
            return 'class';
        } else if (firstLine.startsWith('statediagram')) {
            return 'state';
        } else if (firstLine.startsWith('pie')) {
            return 'pie';
        } else if (firstLine.startsWith('journey')) {
            return 'journey';
        } else if (firstLine.startsWith('gitgraph')) {
            return 'gitgraph';
        } else {
            // Additional checks for content patterns
            if (trimmed.includes('-->') || trimmed.includes('---')) {
                return 'flowchart';
            } else if (trimmed.includes('section ') && (trimmed.includes(':active') || trimmed.includes('dateformat'))) {
                return 'gantt';
            }
            return 'unknown';
        }
    },
    
    miniminalProcessing: function(content) {
        if (!content || typeof content !== 'string') {
            return content;
        }

        // Clean and normalize the content more thoroughly
        let processed = content.trim();

        // Remove any potential double-encoding artifacts
        if (processed.startsWith('"') && processed.endsWith('"')) {
            try {
                // Try to parse as JSON string in case of double encoding
                processed = JSON.parse(processed);
            } catch (e) {
                // If parsing fails, just remove quotes manually
                processed = processed.slice(1, -1);
            }
        }

        // SECURITY NOTE: do NOT decode HTML entities here. Content already
        // arrives via JSON.parse() of the embedded metadata (the generator's
        // toJSON escapes <,>,& as \u00xx), so it's already literal
        // characters. Decoding &lt;/&amp;/etc. would reconstruct a payload
        // that the generation pipeline intentionally escaped (#73).

        // Clean up any problematic characters that might confuse Mermaid
        processed = processed
            .replace(/\r\n/g, '\n')  // Normalize line endings
            .replace(/\r/g, '\n')    // Convert remaining \r to \n
            .replace(/\t/g, '    ')  // Convert tabs to spaces
            .trim();

        // Remove any leading/trailing whitespace from each line
        processed = processed.split('\n')
            .map(line => line.trim())
            .filter(line => line.length > 0)  // Remove empty lines
            .join('\n');

        return processed;
    },
    
    validateDiagramContent: function(content) {
        if (!content || typeof content !== 'string') {
            return false;
        }
        
        const trimmed = content.trim();
        
        // Check for minimum viable diagram
        if (trimmed.length < 5) {
            return false;
        }
        
        // Check for valid diagram types
        const validStarts = [
            'flowchart', 'graph', 'sequenceDiagram', 'classDiagram', 
            'stateDiagram', 'erDiagram', 'gantt', 'pie', 'journey',
            'gitgraph', 'mindmap', 'timeline', 'c4context'
        ];
        
        const firstLine = trimmed.split('\n')[0].trim().toLowerCase();
        const hasValidStart = validStarts.some(start => firstLine.startsWith(start));
        
        if (!hasValidStart) {
            console.warn('[SlideLang.mermaid] Diagram does not start with valid type. First line:', firstLine);
            // Allow it anyway but log warning
        }
        
        // Check for problematic patterns that might confuse Mermaid
        const problematicPatterns = [
            /^\s*-\s/m,  // Lines starting with markdown list syntax
            /^\s*\*\s/m, // Lines starting with markdown list syntax
            /^\s*\d+\.\s/m, // Lines starting with numbered list syntax
        ];
        
        for (const pattern of problematicPatterns) {
            if (pattern.test(trimmed)) {
                console.warn('[SlideLang.mermaid] Detected potentially problematic pattern in diagram content');
                return false;
            }
        }
        
        // Additional diagnostic logging (commented for cleaner output)
        // console.log('[SlideLang.mermaid] Diagram content validated successfully');
        // console.log('[SlideLang.mermaid] First line:', firstLine);
        // console.log('[SlideLang.mermaid] Content length:', trimmed.length);
        // console.log('[SlideLang.mermaid] Contains style commands:', trimmed.includes('style '));
        // console.log('[SlideLang.mermaid] Contains class commands:', trimmed.includes('class '));
        
        return true;
    },
};

// Export for module usage
if (typeof module !== 'undefined' && module.exports) {
    module.exports = SlideLangMermaid;
}

// Auto-register mermaid module
(function() {
    function registerMermaid() {
        if (typeof window !== 'undefined' && window.SlideLang) {
            SlideLang.registerModule('mermaid', SlideLangMermaid);
            SlideLangMermaid.init();
        } else {
            setTimeout(registerMermaid, 50);
        }
    }

    // Iniciar el proceso de registro
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', registerMermaid);
    } else {
        registerMermaid();
    }
})();
