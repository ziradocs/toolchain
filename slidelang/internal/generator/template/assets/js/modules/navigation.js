// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// === SLIDE NAVIGATION MODULE ===
const SlideNavigation = {
    // Propiedades de navegación
    currentSlide: 0,
    totalSlides: 0,
    slides: null,
    isTransitioning: false,
    initialized: false,
    
    // Estados del menú
    isMenuVisible: false,
    isAdvancedMenuOpen: false,
    presentationMode: false,
    presenterNotesVisible: false,
    
    // Timer
    timer: null,
    timerStartTime: null,
    autoHideTimeout: null,
    
    // Configuración
    config: {
        loop: true,
        autoAdvance: false,
        autoAdvanceDelay: 5000,
        enableKeyboard: true,
        enableTouch: true,
        enableWheel: false,
        showProgress: true,
        showCounter: true,
        showHelp: true,
        showFloatingMenu: true,
        transitionDuration: 300,
        autoHideDelay: 3000
    },
    
    // Cache de elementos DOM
    elements: {
        progressBar: null,
        slideCounter: null,
        currentSlideEl: null,
        totalSlidesEl: null,
        helpModal: null,
        floatingMenu: null,
        advancedMenu: null,
        navButtons: null
    },
    
    // Inicialización
    init: function(userConfig = {}) {
        // Evitar doble inicialización
        if (this.initialized) {

            return;
        }
        
        // Merge configuración
        this.config = { ...this.config, ...userConfig };
        
        this.slides = document.querySelectorAll('.slidelang-slide');
        this.totalSlides = this.slides.length;
        
        this.cacheElements();
        this.createNavigationUI();
        this.bindEvents();
        this.updateUI();
        
        // Auto-advance si está habilitado
        if (this.config.autoAdvance) {
            this.startAutoAdvance();
        }
        
        // Inicializar notas del presentador
        this.initializePresenterNotes();
        
        this.initialized = true;

    },
    
    // Cache de elementos DOM para mejor performance
    cacheElements: function() {
        this.elements.progressBar = document.querySelector('.slidelang-progress-bar');
        this.elements.slideCounter = document.querySelector('.slidelang-nav-counter');
        this.elements.currentSlideEl = document.getElementById('slidelang-current-slide');
        this.elements.totalSlidesEl = document.getElementById('slidelang-total-slides');
    },
    
    // Crear UI de navegación completa
    createNavigationUI: function() {
        // Progress bar superior
        if (this.config.showProgress && !this.elements.progressBar) {
            const progressBar = document.createElement('div');
            progressBar.className = 'slidelang-progress-bar';
            document.body.appendChild(progressBar);
            this.elements.progressBar = progressBar;
        }
        
        // Crear menú flotante integrado
        if (this.config.showFloatingMenu) {
            this.createFloatingMenu();
        }
        
        // Crear menú avanzado
        this.createAdvancedMenu();
        
        // Configurar auto-ocultar
        this.setupAutoHide();
    },
    
    // Crear menú flotante unificado
    createFloatingMenu: function() {
        const menuHTML = `
            <nav id="floating-menu" class="slidelang-floating-menu" aria-label="Navegación de slides">
                <div class="slidelang-floating-menu-content">
                    <div class="slidelang-nav-counter" aria-live="polite">
                        <span id="slidelang-current-slide-floating">1</span>
                        <span class="slidelang-separator">/</span>
                        <span id="total-slides-floating">` + this.totalSlides + `</span>
                    </div>
                    <div class="slidelang-menu-buttons">
                        <button type="button" id="prev-slide-btn" class="slidelang-menu-btn" title="Slide anterior (←)" aria-label="Slide anterior (←)">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <polyline points="15,18 9,12 15,6"></polyline>
                            </svg>
                        </button>
                        <button type="button" id="next-slide-btn" class="slidelang-menu-btn" title="Siguiente slide (→)" aria-label="Siguiente slide (→)">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <polyline points="9,18 15,12 9,6"></polyline>
                            </svg>
                        </button>
                        <button type="button" id="presentation-mode-btn" class="slidelang-menu-btn" title="Modo presentación (F)" aria-label="Modo presentación (F)">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <polygon points="5,3 19,12 5,21"></polygon>
                            </svg>
                        </button>
                        <button type="button" id="advanced-menu-btn" class="slidelang-menu-btn" title="Menú avanzado (M)" aria-label="Menú avanzado (M)" aria-expanded="false" aria-controls="advanced-menu">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <circle cx="12" cy="12" r="1"></circle>
                                <circle cx="12" cy="5" r="1"></circle>
                                <circle cx="12" cy="19" r="1"></circle>
                            </svg>
                        </button>
                    </div>
                </div>
                <div class="slidelang-progress-indicator">
                    <div class="slidelang-progress-bar-floating"></div>
                </div>
            </nav>
        `;
        
        document.body.insertAdjacentHTML('beforeend', menuHTML);
        this.elements.floatingMenu = document.getElementById('floating-menu');
        this.updateFloatingCounter();
    },

    // Actualizar contador flotante
    updateFloatingCounter: function() {
        const currentSlideFloating = document.getElementById('slidelang-current-slide-floating');
        const totalSlidesFloating = document.getElementById('total-slides-floating');
        
        if (currentSlideFloating) {
            currentSlideFloating.textContent = this.currentSlide + 1;
        }
        if (totalSlidesFloating) {
            totalSlidesFloating.textContent = this.totalSlides;
        }
    },

    // Crear menú avanzado
    createAdvancedMenu: function() {
        const menuHTML = `
            <div id="advanced-menu" class="slidelang-advanced-menu" role="group" aria-label="Opciones avanzadas">
                <div class="slidelang-advanced-menu-content">
                    <div class="slidelang-advanced-menu-title">Opciones avanzadas</div>
                    <div class="slidelang-menu-section">
                        <button type="button" id="presenter-notes-btn" class="slidelang-advanced-btn">
                            <span>Notas del presentador</span>
                            <span class="slidelang-shortcut">N</span>
                        </button>
                        <button type="button" id="timer-btn" class="slidelang-advanced-btn">
                            <span>Cronómetro</span>
                            <span class="slidelang-shortcut">T</span>
                        </button>
                        <button type="button" id="help-btn" class="slidelang-advanced-btn">
                            <span>Ayuda</span>
                            <span class="slidelang-shortcut">H</span>
                        </button>
                    </div>
                    <div class="slidelang-menu-section">
                        <div class="slidelang-progress-controls">
                            <label>Progreso: <span id="progress-display">0%</span></label>
                            <input type="range" id="progress-slider" min="0" max="100" value="0">
                        </div>
                    </div>
                </div>
            </div>
        `;
        
        document.body.insertAdjacentHTML('beforeend', menuHTML);
        this.elements.advancedMenu = document.getElementById('advanced-menu');
    },

    // Configurar auto-ocultar
    setupAutoHide: function() {
        if (!this.config.showFloatingMenu) return;
        
        const showMenu = () => {
            if (this.elements.floatingMenu) {
                this.elements.floatingMenu.classList.add('slidelang-visible');
                this.isMenuVisible = true;
            }
        };
        
        const hideMenu = () => {
            if (this.elements.floatingMenu && !this.isAdvancedMenuOpen) {
                this.elements.floatingMenu.classList.remove('slidelang-visible');
                this.isMenuVisible = false;
            }
        };
        
        const resetAutoHide = () => {
            clearTimeout(this.autoHideTimeout);
            showMenu();
            this.autoHideTimeout = setTimeout(hideMenu, this.config.autoHideDelay);
        };
        
        // Eventos para mostrar/ocultar menú
        document.addEventListener('mousemove', resetAutoHide);
        document.addEventListener('keydown', resetAutoHide);
        document.addEventListener('touchstart', resetAutoHide);
        
        // Inicializar con menú visible
        resetAutoHide();
    },

    // Vincular eventos
    bindEvents: function() {
        // Navegación por teclado
        if (this.config.enableKeyboard) {
            document.addEventListener('keydown', this.handleKeydown.bind(this));
        }
        
        // Navegación táctil
        if (this.config.enableTouch) {
            this.bindTouchEvents();
        }
        
        // Botones del menú flotante
        this.bindMenuEvents();
        
        // Eventos del menú avanzado
        this.bindAdvancedMenuEvents();
    },

    // Manejar eventos de teclado
    handleKeydown: function(e) {
        if (this.isTransitioning) return;
        
        switch (e.key) {
            case 'ArrowRight':
            case ' ':
                e.preventDefault();
                this.nextSlide();
                break;
            case 'ArrowLeft':
                e.preventDefault();
                this.previousSlide();
                break;
            case 'Home':
                e.preventDefault();
                this.goToSlide(0);
                break;
            case 'End':
                e.preventDefault();
                this.goToSlide(this.totalSlides - 1);
                break;
            case 'f':
            case 'F':
                e.preventDefault();
                this.togglePresentationMode();
                break;
            case 'm':
            case 'M':
                e.preventDefault();
                this.toggleAdvancedMenu();
                break;
            case 'h':
            case 'H':
                e.preventDefault();
                this.showHelp();
                break;
            case 'n':
            case 'N':
                e.preventDefault();
                this.togglePresenterNotes();
                break;
            case 't':
            case 'T':
                e.preventDefault();
                this.toggleTimer();
                break;
        }
    },

    // Vincular eventos táctiles
    bindTouchEvents: function() {
        let touchStartX = null;
        let touchStartY = null;
        
        document.addEventListener('touchstart', (e) => {
            touchStartX = e.touches[0].clientX;
            touchStartY = e.touches[0].clientY;
        });
        
        document.addEventListener('touchend', (e) => {
            if (!touchStartX || !touchStartY) return;
            
            const touchEndX = e.changedTouches[0].clientX;
            const touchEndY = e.changedTouches[0].clientY;
            
            const deltaX = touchEndX - touchStartX;
            const deltaY = touchEndY - touchStartY;
            
            // Solo considerar swipes horizontales
            if (Math.abs(deltaX) > Math.abs(deltaY) && Math.abs(deltaX) > 50) {
                if (deltaX > 0) {
                    this.previousSlide();
                } else {
                    this.nextSlide();
                }
            }
            
            touchStartX = null;
            touchStartY = null;
        });
    },

    // Vincular eventos del menú
    bindMenuEvents: function() {
        // Botones de navegación
        const prevBtn = document.getElementById('prev-slide-btn');
        const nextBtn = document.getElementById('next-slide-btn');
        const presentationBtn = document.getElementById('presentation-mode-btn');
        const advancedBtn = document.getElementById('advanced-menu-btn');
        
        if (prevBtn) prevBtn.addEventListener('click', () => this.previousSlide());
        if (nextBtn) nextBtn.addEventListener('click', () => this.nextSlide());
        if (presentationBtn) presentationBtn.addEventListener('click', () => this.togglePresentationMode());
        if (advancedBtn) advancedBtn.addEventListener('click', () => this.toggleAdvancedMenu());
    },

    // Vincular eventos del menú avanzado
    bindAdvancedMenuEvents: function() {
        // Cerrar menú avanzado al hacer clic fuera
        document.addEventListener('click', (e) => {
            if (this.isAdvancedMenuOpen && !e.target.closest('#advanced-menu') && !e.target.closest('#advanced-menu-btn')) {
                this.toggleAdvancedMenu();
            }
        });
        
        // Botones del menú avanzado
        document.addEventListener('click', (e) => {
            if (e.target.id === 'presenter-notes-btn') {
                this.togglePresenterNotes();
            } else if (e.target.id === 'timer-btn') {
                this.toggleTimer();
            } else if (e.target.id === 'help-btn') {
                this.showHelp();
            }
        });
        
        // Control deslizante de progreso
        const progressSlider = document.getElementById('progress-slider');
        if (progressSlider) {
            progressSlider.addEventListener('input', (e) => {
                const slideIndex = Math.round((e.target.value / 100) * (this.totalSlides - 1));
                this.goToSlide(slideIndex);
            });
        }
    },

    // Navegación de slides
    goToSlide: function(index) {
        if (index < 0 || index >= this.totalSlides || index === this.currentSlide || this.isTransitioning) {
            return;
        }
        
        this.isTransitioning = true;
        
        // Ocultar slide actual
        if (this.slides[this.currentSlide]) {
            this.slides[this.currentSlide].classList.remove('slidelang-active');
        }
        
        // Mostrar nuevo slide
        this.currentSlide = index;
        if (this.slides[this.currentSlide]) {
            this.slides[this.currentSlide].classList.add('slidelang-active');
        }
        
        // Actualizar UI
        this.updateUI();
        
        // Refresh maps only if they exist on this slide (using DOM attributes)
        if (typeof SlideLang !== 'undefined' && SlideLang.modules && SlideLang.modules.maps) {
            const slideId = this.slides[this.currentSlide].id;
            const hasMap = this.slideHasMap(this.currentSlide);
            
            if (hasMap) {
                SlideLang.modules.maps.refreshMapOnSlideChange(slideId);
            }
        }
        
        // Resetear transición después de la animación
        setTimeout(() => {
            this.isTransitioning = false;
        }, this.config.transitionDuration);
        
        // Emitir evento
        this.emitSlideChange();
    },

    nextSlide: function() {
        const nextIndex = this.config.loop && this.currentSlide === this.totalSlides - 1 
            ? 0 
            : this.currentSlide + 1;
        this.goToSlide(nextIndex);
    },

    previousSlide: function() {
        const prevIndex = this.config.loop && this.currentSlide === 0 
            ? this.totalSlides - 1 
            : this.currentSlide - 1;
        this.goToSlide(prevIndex);
    },

    // Actualizar interfaz
    updateUI: function() {
        this.updateProgressBar();
        this.updateSlideCounter();
        this.updateFloatingCounter();
        this.updateProgressSlider();
    },

    updateProgressBar: function() {
        if (!this.elements.progressBar) return;
        
        const progress = ((this.currentSlide + 1) / this.totalSlides) * 100;
        this.elements.progressBar.style.width = progress + '%';
        
        // También actualizar barra flotante
        const floatingBar = document.querySelector('.slidelang-progress-bar-floating');
        if (floatingBar) {
            floatingBar.style.width = progress + '%';
        }
    },

    updateSlideCounter: function() {
        if (this.elements.currentSlideEl) {
            this.elements.currentSlideEl.textContent = this.currentSlide + 1;
        }
        if (this.elements.totalSlidesEl) {
            this.elements.totalSlidesEl.textContent = this.totalSlides;
        }
    },

    updateProgressSlider: function() {
        const progressSlider = document.getElementById('progress-slider');
        const progressDisplay = document.getElementById('progress-display');
        
        if (progressSlider) {
            const progress = ((this.currentSlide + 1) / this.totalSlides) * 100;
            progressSlider.value = progress;
        }
        
        if (progressDisplay) {
            const progress = Math.round(((this.currentSlide + 1) / this.totalSlides) * 100);
            progressDisplay.textContent = progress + '%';
        }
    },

    // Métodos de funcionalidad avanzada
    toggleAdvancedMenu: function() {
        if (!this.elements.advancedMenu) return;

        this.isAdvancedMenuOpen = !this.isAdvancedMenuOpen;

        if (this.isAdvancedMenuOpen) {
            this.elements.advancedMenu.classList.add('slidelang-visible');
        } else {
            this.elements.advancedMenu.classList.remove('slidelang-visible');
        }

        const advancedBtn = document.getElementById('advanced-menu-btn');
        if (advancedBtn) {
            advancedBtn.setAttribute('aria-expanded', String(this.isAdvancedMenuOpen));
        }
    },

    togglePresentationMode: function() {
        this.presentationMode = !this.presentationMode;
        
        if (this.presentationMode) {
            if (document.documentElement.requestFullscreen) {
                document.documentElement.requestFullscreen();
            }
            document.body.classList.add('slidelang-presentation-mode');
        } else {
            if (document.exitFullscreen) {
                document.exitFullscreen();
            }
            document.body.classList.remove('slidelang-presentation-mode');
        }
    },

    togglePresenterNotes: function() {
        this.presenterNotesVisible = !this.presenterNotesVisible;
        
        const notes = document.querySelectorAll('.slidelang-presenter-notes');
        notes.forEach(note => {
            if (this.presenterNotesVisible) {
                note.classList.add('slidelang-visible');
            } else {
                note.classList.remove('slidelang-visible');
            }
        });
    },

    toggleTimer: function() {
        if (this.timer) {
            this.stopTimer();
        } else {
            this.startTimer();
        }
    },

    startTimer: function() {
        this.timerStartTime = Date.now();
        this.timer = setInterval(() => {
            this.updateTimerDisplay();
        }, 1000);
    },

    stopTimer: function() {
        if (this.timer) {
            clearInterval(this.timer);
            this.timer = null;
        }
    },

    updateTimerDisplay: function() {
        if (!this.timerStartTime) return;
        
        const elapsed = Date.now() - this.timerStartTime;
        const minutes = Math.floor(elapsed / 60000);
        const seconds = Math.floor((elapsed % 60000) / 1000);
        
        const timerDisplay = document.querySelector('.slidelang-timer-display');
        if (timerDisplay) {
            timerDisplay.textContent = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
        }
    },

    showHelp: function() {
        // Implementar modal de ayuda

    },

    // Auto-advance
    startAutoAdvance: function() {
        if (this.autoAdvanceTimer) return;
        
        this.autoAdvanceTimer = setInterval(() => {
            this.nextSlide();
        }, this.config.autoAdvanceDelay);
    },

    stopAutoAdvance: function() {
        if (this.autoAdvanceTimer) {
            clearInterval(this.autoAdvanceTimer);
            this.autoAdvanceTimer = null;
        }
    },

    // Inicializar notas del presentador
    initializePresenterNotes: function() {
        const slides = document.querySelectorAll('.slidelang-slide');
        slides.forEach((slide, index) => {
            const notesContent = slide.querySelector('.slidelang-presenter-notes');
            if (notesContent) {
                notesContent.classList.add('slidelang-presenter-notes');
            }
        });
    },

    // Emitir eventos
    emitSlideChange: function() {
        const currentSlideElement = this.slides[this.currentSlide];
        
        // Standard event with complete slide information
        const event = new CustomEvent('slidelang:slideChanged', {
            detail: {
                currentSlide: this.currentSlide,
                totalSlides: this.totalSlides,
                progress: ((this.currentSlide + 1) / this.totalSlides) * 100,
                slideElement: currentSlideElement,
                slideId: currentSlideElement ? currentSlideElement.id : null,
                previousSlide: this.currentSlide > 0 ? this.currentSlide - 1 : null,
                isFirstSlide: this.currentSlide === 0,
                isLastSlide: this.currentSlide === this.totalSlides - 1,
                timestamp: Date.now()
            }
        });
        document.dispatchEvent(event);
        
        // Debug logging
        if (typeof console !== 'undefined' && console.log) {
            console.log('[SlideNavigation] Slide changed:', {
                current: this.currentSlide,
                total: this.totalSlides,
                slideId: currentSlideElement ? currentSlideElement.id : null
            });
        }
    },

    // Check if a slide has maps using DOM attributes
    slideHasMap: function(slideIndex) {
        // Use data-interactive-types attribute for precise detection
        const slide = this.slides[slideIndex];
        if (slide) {
            const slideElement = document.getElementById(slide.id);
            if (slideElement) {
                const interactiveTypes = slideElement.getAttribute('data-interactive-types');
                if (interactiveTypes) {
                    return interactiveTypes.includes('map');
                }
            }
        }
        
        return false;
    }
};

// Export for module usage
if (typeof module !== 'undefined' && module.exports) {
    module.exports = SlideNavigation;
}
