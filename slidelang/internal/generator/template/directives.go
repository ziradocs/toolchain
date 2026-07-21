// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

// GetDirectivesJS retorna la funcionalidad JavaScript para directivas
func GetDirectivesJS() string {
	return `/* Directivas JavaScript */

// Sistema de gestión de directivas
class DirectiveManager {
    constructor() {
        this.timers = new Map();
        this.animations = new Map();
        this.presenterNotesVisible = false;
        this.init();
    }

    init() {
        this.initTimers();
        this.initAnimations();
        this.initPresenterNotes();
        this.initAutoPlay();
        this.initTransitions();
    }

    // Timer Management
    initTimers() {
        const timers = document.querySelectorAll('[data-directive="timer"]');
        timers.forEach(timer => {
            const duration = parseInt(timer.dataset.duration) || 300;
            this.startTimer(timer, duration);
        });
    }

    startTimer(timerElement, duration) {
        const timeDisplay = timerElement.querySelector('.timer-time');
        const progressBar = timerElement.querySelector('.timer-progress');
        
        let remaining = duration;
        
        // Set CSS variable for animation duration
        timerElement.style.setProperty('--timer-duration', duration + 's');
        
        const timerId = setInterval(() => {
            remaining--;
            
            if (timeDisplay) {
                timeDisplay.textContent = remaining + 's';
            }
            
            // Color coding based on remaining time
            if (remaining <= 30) {
                timerElement.style.background = 'rgba(239, 68, 68, 0.9)'; // red
            } else if (remaining <= 60) {
                timerElement.style.background = 'rgba(245, 158, 11, 0.9)'; // orange
            }
            
            if (remaining <= 0) {
                clearInterval(timerId);
                this.onTimerExpired(timerElement);
            }
        }, 1000);
        
        this.timers.set(timerElement, timerId);
    }

    onTimerExpired(timerElement) {
        timerElement.style.background = 'rgba(239, 68, 68, 0.9)';
        timerElement.classList.add('timer-expired');
        
        // Flash effect
        let flashes = 0;
        const flashInterval = setInterval(() => {
            timerElement.style.opacity = timerElement.style.opacity === '0.3' ? '1' : '0.3';
            flashes++;
            if (flashes >= 6) {
                clearInterval(flashInterval);
                timerElement.style.opacity = '1';
            }
        }, 200);
    }

    // Animation Management
    initAnimations() {
        // Intersection Observer for triggering animations when elements come into view
        const animatedElements = document.querySelectorAll('.animate-fade-in, .animate-slide-up, .animate-bounce');
        
        if (animatedElements.length === 0) return;
        
        const observer = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    entry.target.style.animationDelay = '0s';
                    entry.target.style.animationPlayState = 'running';
                }
            });
        }, { threshold: 0.1 });
        
        animatedElements.forEach(el => {
            el.style.animationPlayState = 'paused';
            observer.observe(el);
        });
    }

    // Presenter Notes Management
    initPresenterNotes() {
        // Solo agregar el manejador si no hay floating menu
        if (!document.getElementById('floating-menu')) {
            // Keyboard shortcut: 'N' to toggle presenter notes
            document.addEventListener('keydown', (e) => {
                if (e.key === 'n' || e.key === 'N') {
                    this.togglePresenterNotes();
                }
            });
        }
        
        // Presenter notes are now handled via metadata in navigation.js
        // No DOM-based notes initialization needed
    }

    // Legacy method - presenter notes are now handled via metadata in navigation.js
    togglePresenterNotes() {
        // This functionality is now handled by the navigation module
        // which reads notes from metadata instead of DOM elements
        // This method is deprecated. Use navigation module instead.
    }

    // Auto-play Management
    initAutoPlay() {
        const autoPlayElements = document.querySelectorAll('[data-directive="auto-play"]');
        autoPlayElements.forEach(el => {
            const interval = parseInt(el.dataset.interval) || 5000;
            this.setupAutoPlay(el, interval);
        });
    }

    setupAutoPlay(element, interval) {
        // Auto-advance to next slide if this element is visible
        setTimeout(() => {
            if (this.isElementVisible(element)) {
                window.slideManager?.nextSlide();
            }
        }, interval);
    }

    // Transition Management
    initTransitions() {
        const transitionMarkers = document.querySelectorAll('[data-directive="transition"]');
        transitionMarkers.forEach(marker => {
            const type = marker.dataset.type || 'fade';
            const duration = marker.dataset.duration || '500ms';
            
            // Apply transition to parent slide
            const slide = marker.closest('.slide');
            if (slide) {
                slide.style.transition = this.getTransitionCSS(type, duration);
            }
        });
    }

    getTransitionCSS(type, duration) {
        switch (type) {
            case 'fade':
                return ` + "`" + `opacity $` + `{duration} ease-in-out` + "`" + `;
            case 'slide':
                return ` + "`" + `transform $` + `{duration} ease-in-out` + "`" + `;
            case 'none':
                return 'none';
            default:
                return ` + "`" + `all $` + `{duration} ease-in-out` + "`" + `;
        }
    }

    // Utility Methods
    isElementVisible(element) {
        const rect = element.getBoundingClientRect();
        return rect.top >= 0 && rect.left >= 0 && 
               rect.bottom <= window.innerHeight && 
               rect.right <= window.innerWidth;
    }

    // Full-screen Management
    initFullScreen() {
        const fullScreenElements = document.querySelectorAll('.full-screen');
        fullScreenElements.forEach(el => {
            el.addEventListener('click', (e) => {
                if (e.target === el) {
                    this.exitFullScreen(el);
                }
            });
            
            // ESC key to exit
            document.addEventListener('keydown', (e) => {
                if (e.key === 'Escape') {
                    this.exitFullScreen(el);
                }
            });
        });
    }

    exitFullScreen(element) {
        element.classList.remove('full-screen');
    }

    // Public API
    resetTimers() {
        this.timers.forEach((timerId, element) => {
            clearInterval(timerId);
        });
        this.timers.clear();
        this.initTimers();
    }

    pauseAnimations() {
        const animatedElements = document.querySelectorAll('.animate-fade-in, .animate-slide-up, .animate-bounce');
        animatedElements.forEach(el => {
            el.style.animationPlayState = 'paused';
        });
    }

    resumeAnimations() {
        const animatedElements = document.querySelectorAll('.animate-fade-in, .animate-slide-up, .animate-bounce');
        animatedElements.forEach(el => {
            el.style.animationPlayState = 'running';
        });
    }
}

// Global instance
window.directiveManager = new DirectiveManager();

// Integration with slide navigation
document.addEventListener('slideChanged', (e) => {
    // Reset timers when slide changes
    if (window.directiveManager) {
        window.directiveManager.resetTimers();
    }
});

// Keyboard shortcuts info
document.addEventListener('keydown', (e) => {
    if (e.key === '?' || (e.key === 'h' && e.ctrlKey)) {
        showDirectiveHelp();
    }
});

function showDirectiveHelp() {
    const helpText = 
        'Directive Keyboard Shortcuts:\n' +
        'N - Toggle presenter notes\n' +
        'ESC - Exit full-screen mode\n' +
        'Ctrl+H or ? - Show this help';
    
    alert(helpText);
}
`
}
