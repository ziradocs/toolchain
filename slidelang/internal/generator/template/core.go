// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

// GetCoreJS retorna el JavaScript core con API extendida para visualizadores externos
func GetCoreJS() string {
	return `// === ENHANCED CORE MODULE ===
const SlideLang = {
    // Propiedades básicas
    initialized: false,
    modules: {},
    metadata: null,
    
    // Inicialización básica
    init: function() {
        if (this.initialized) return;
        
        this.initialized = true;
        
        // Cargar metadata de SlideLang
        this.loadMetadata();
        
        // Emitir evento de inicialización
        this.emitEvent('coreInitialized');
    },
    
    // Cargar metadata del JSON embebido
    loadMetadata: function() {
        const metadataElement = document.getElementById('slidelang-metadata');
        if (metadataElement) {
            try {
                this.metadata = JSON.parse(metadataElement.textContent);
            } catch (e) {
                // Failed to parse metadata
            }
        }
    },
    
    // Sistema de eventos
    emitEvent: function(eventName, data = {}) {
        const event = new CustomEvent('slidelang:' + eventName, {
            detail: data
        });
        document.dispatchEvent(event);
    },
    
    // Registro de módulos
    registerModule: function(name, module) {
        this.modules[name] = module;
    },
    
    // Obtener módulo
    getModule: function(name) {
        return this.modules[name];
    },
    
    // Verificar si un módulo está disponible
    hasModule: function(name) {
        return name in this.modules;
    },
    
    // Utilidades básicas
    utils: {
        // Función para detectar características del navegador
        getBrowserInfo: function() {
            return {
                userAgent: navigator.userAgent,
                language: navigator.language,
                cookieEnabled: navigator.cookieEnabled,
                onLine: navigator.onLine
            };
        },
        
        // Función para generar IDs únicos
        generateId: function(prefix = 'slide') {
            return prefix + '-' + Math.random().toString(36).substr(2, 9);
        },
        
        // Función para sanitizar texto
        sanitizeText: function(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        },
        
        // Función para detectar si es dispositivo táctil
        isTouchDevice: function() {
            return ('ontouchstart' in window) || 
                   (navigator.maxTouchPoints > 0) || 
                   (navigator.msMaxTouchPoints > 0);
        }
    }
};

// === EXPOSE SLIDELANG TO GLOBAL SCOPE FOR MODULES ===
window.SlideLang = SlideLang;

// === SLIDELANG API FOR EXTERNAL VIEWERS ===
window.SlideLangAPI = {
    // Información de la presentación
    getMetadata: function() {
        return SlideLang.metadata ? { ...SlideLang.metadata } : null;
    },
    
    getPresentationInfo: function() {
        const metadata = this.getMetadata();
        if (!metadata) return null;
        
        return {
            title: metadata.title,
            author: metadata.author,
            date: metadata.date,
            version: metadata.version,
            theme: metadata.theme,
            totalSlides: metadata.totalSlides,
            estimatedDuration: metadata.estimatedDuration,
            features: metadata.features,
            libraries: metadata.libraries
        };
    },
    
    // Obtener tema efectivo usado en la presentación
    getTheme: function() {
        const metadata = this.getMetadata();
        return metadata ? metadata.theme : null;
    },
    
    // Slides
    getTotalSlides: function() {
        const metadata = this.getMetadata();
        return metadata ? metadata.totalSlides : document.querySelectorAll('.slide').length;
    },
    
    getCurrentSlide: function() {
        const activeSlide = document.querySelector('.slide.active');
        if (activeSlide) {
            const slideIndex = parseInt(activeSlide.dataset.slide) || 0;
            return slideIndex;
        }
        return 0;
    },
    
    getSlideInfo: function(index) {
        const metadata = this.getMetadata();
        if (metadata && metadata.slides && metadata.slides[index]) {
            return { ...metadata.slides[index] };
        }
        
        // Fallback: obtener información del DOM
        const slide = document.querySelector('.slide[data-slide="' + index + '"]');
        if (!slide) return null;
        
        return {
            id: slide.id,
            type: slide.dataset.slideType || 'content',
            title: slide.dataset.slideTitle || '',
            duration: parseInt(slide.dataset.duration) || 0,
            transition: slide.dataset.transition || 'fade',
            hasInteractive: slide.dataset.interactive === 'true',
            interactiveElements: (slide.dataset.interactiveTypes || '').split(',').filter(t => t.trim())
        };
    },
    
    getAllSlides: function() {
        const metadata = this.getMetadata();
        if (metadata && metadata.slides) {
            return metadata.slides.map(slide => ({ ...slide }));
        }
        
        // Fallback: obtener información del DOM
        const slides = document.querySelectorAll('.slide');
        return Array.from(slides).map((slide, index) => this.getSlideInfo(index));
    },
    
    // Elementos
    getSlideElements: function(slideIndex) {
        const slide = document.querySelector('.slide[data-slide="' + slideIndex + '"]');
        if (!slide) return [];
        
        const elements = slide.querySelectorAll('[data-element-type]');
        return Array.from(elements).map(el => ({
            id: el.id,
            type: el.dataset.elementType,
            slideIndex: parseInt(el.dataset.slide),
            // Información específica por tipo de elemento
            ...this._getElementSpecificData(el)
        }));
    },
    
    _getElementSpecificData: function(element) {
        const data = {};
        const type = element.dataset.elementType;
        
        switch (type) {
            case 'code':
                data.language = element.dataset.language || '';
                break;
            case 'image':
                data.context = element.dataset.context || '';
                break;
            case 'chart':
                data.chartType = element.dataset.chartType || '';
                break;
            case 'map':
                data.mapType = element.dataset.mapType || '';
                break;
            case 'special_block':
                data.blockType = element.dataset.blockType || '';
                break;
            case 'directive':
                data.directiveName = element.dataset.directiveName || '';
                break;
        }
        
        return data;
    },
    
    // Navegación
    goToSlide: function(index) {
        if (SlideLang.hasModule('navigation')) {
            const nav = SlideLang.getModule('navigation');
            if (nav && nav.showSlide) {
                nav.showSlide(index);
                return true;
            }
        }
        
        // Fallback básico
        const slides = document.querySelectorAll('.slide');
        if (index >= 0 && index < slides.length) {
            slides.forEach(slide => slide.classList.remove('active'));
            slides[index].classList.add('active');
            
            // Emitir evento personalizado
            SlideLang.emitEvent('slideChanged', { 
                currentSlide: index,
                previousSlide: this.getCurrentSlide() 
            });
            
            return true;
        }
        
        return false;
    },
    
    nextSlide: function() {
        const current = this.getCurrentSlide();
        const total = this.getTotalSlides();
        if (current < total - 1) {
            return this.goToSlide(current + 1);
        }
        return false;
    },
    
    previousSlide: function() {
        const current = this.getCurrentSlide();
        if (current > 0) {
            return this.goToSlide(current - 1);
        }
        return false;
    },
    
    // Estado de la presentación
    getState: function() {
        return {
            currentSlide: this.getCurrentSlide(),
            totalSlides: this.getTotalSlides(),
            metadata: this.getMetadata(),
            modules: Object.keys(SlideLang.modules),
            timestamp: new Date().toISOString()
        };
    },
    
    // Eventos
    on: function(eventName, callback) {
        document.addEventListener('slidelang:' + eventName, callback);
    },
    
    off: function(eventName, callback) {
        document.removeEventListener('slidelang:' + eventName, callback);
    },
    
    // Utilidades para visualizadores externos
    exportData: function() {
        return {
            metadata: this.getMetadata(),
            slides: this.getAllSlides(),
            state: this.getState(),
            exportedAt: new Date().toISOString()
        };
    },
    
    // Verificar si el API está disponible
    isReady: function() {
        return SlideLang.initialized && this.getMetadata() !== null;
    },
    
    // Verificar características disponibles
    hasFeature: function(featureName) {
        const metadata = this.getMetadata();
        if (!metadata || !metadata.features) return false;
        
        return metadata.features['has' + featureName.charAt(0).toUpperCase() + featureName.slice(1)] || false;
    },
    
    // Control de notas del presentador
    hasPresenterNotes: function() {
        return this.hasFeature('notes');
    },
    
    togglePresenterNotes: function() {
        // Prioridad 1: Usar el módulo de navegación (sistema principal)
        if (SlideLang.hasModule('navigation')) {
            const nav = SlideLang.getModule('navigation');
            if (nav && nav.togglePresenterNotes) {
                nav.togglePresenterNotes();
                return true;
            }
        }
        
        // Prioridad 2: Usar módulo de directivas (si existe)
        const directivesModule = SlideLang.getModule('directives');
        if (directivesModule && directivesModule.togglePresenterNotes) {
            directivesModule.togglePresenterNotes();
            return true;
        }
        
        return false;
    },
    
    // Obtener información de módulos
    getModules: function() {
        return Object.keys(SlideLang.modules);
    },
    
    getModuleInfo: function(moduleName) {
        const module = SlideLang.getModule(moduleName);
        if (!module) return null;
        
        return {
            name: moduleName,
            initialized: module.initialized || false,
            available: true
        };
    },
    
    isModuleAvailable: function(moduleName) {
        return SlideLang.hasModule(moduleName);
    }
};

// Auto-initialize core
if (typeof window !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() {
            SlideLang.init();
            // Emitir evento cuando la API esté lista
            setTimeout(() => {
                if (window.SlideLangAPI.isReady()) {
                    SlideLang.emitEvent('apiReady');
                }
            }, 100);
        });
    } else {
        SlideLang.init();
        // Emitir evento cuando la API esté lista
        setTimeout(() => {
            if (window.SlideLangAPI.isReady()) {
                SlideLang.emitEvent('apiReady');
            }
        }, 100);
    }
}

// Export for module usage
if (typeof module !== 'undefined' && module.exports) {
    module.exports = SlideLang;
}
`
}
