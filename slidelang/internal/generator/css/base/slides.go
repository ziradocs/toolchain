// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package base

// SlidesCSS provides core slide structure and presentation styles
const SlidesCSS = `/* Container principal */
.presentation-container {
    width: 100vw;
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
}

/* Slide base */
.slide {
    width: 90vw;
    max-width: 1000px;
    height: 80vh;
    background: var(--bg-white);
    border-radius: var(--border-radius-lg);
    box-shadow: var(--shadow-lg);
    padding: 60px;
    display: none;
    flex-direction: column;
    justify-content: flex-start;
    position: relative;
    overflow-y: auto;
    transition: var(--transition);
}

.slide.active {
    display: flex;
    animation: slideIn 0.5s ease-out;
}

/* Animaciones de slide */
@keyframes slideIn {
    from {
        opacity: 0;
        transform: translateX(50px);
    }
    to {
        opacity: 1;
        transform: translateX(0);
    }
}

.slide.title-slide,
.slide.cover-slide,
.slide.intro-slide {
    justify-content: center;
    text-align: center;
    background: var(--bg-title-slide);
    color: var(--bg-white);
}

.slide.section-slide {
    justify-content: center;
    text-align: center;
    background: var(--bg-section-slide);
    color: var(--bg-white);
}

.slide.content-slide {
    justify-content: flex-start;
    background: var(--bg-content-slide);
}

.slide.end-slide {
    justify-content: center;
    text-align: center;
    background: var(--bg-end-slide);
    color: var(--bg-white);
}

/* Closing slide styles - similar to title but optimized for conclusions */
.slide.closing-slide,
.slide.closing {
    justify-content: center;
    text-align: center;
    background: var(--bg-closing-slide, var(--bg-title-slide));
    color: var(--text-on-closing, var(--bg-white));
}

.slide.closing-slide h1,
.slide.closing h1 {
    font-size: 3rem;
    margin-bottom: 1.5rem;
    font-weight: 600;
    text-shadow: 0 2px 4px var(--shadow-text);
}

.slide.closing-slide h2,
.slide.closing h2 {
    font-size: 1.6rem;
    font-weight: 300;
    opacity: 0.9;
    margin-bottom: 2rem;
}

/* Contact info styling in closing slides */
.slide.closing-slide .contact-info,
.slide.closing .contact-info {
    margin-top: 2rem;
    font-size: 1.1rem;
    opacity: 0.8;
}

.slide.closing-slide .contact-info a,
.slide.closing .contact-info a {
    color: inherit;
    text-decoration: none;
    border-bottom: 1px solid transparent;
    transition: var(--transition);
}

.slide.closing-slide .contact-info a:hover,
.slide.closing .contact-info a:hover {
    border-bottom-color: currentColor;
}

/* Logo positioning in closing slides */
.slide.closing-slide .logo,
.slide.closing .logo {
    max-height: 80px;
    margin: 2rem 0;
    opacity: 0.8;
}

/* Headers en slides */
.slide.title-slide h1 {
    font-size: 3.5rem;
    margin-bottom: 1rem;
    font-weight: 700;
    text-shadow: 0 2px 4px var(--shadow-text);
}

.slide.title-slide h2 {
    font-size: 1.8rem;
    font-weight: 300;
    opacity: 0.9;
}

/* Nuevo layout inteligente para slides de título */
.title-slide-container {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    height: 100%;
    gap: 2rem;
    text-align: center;
}

.title-slide-container .title-section {
    flex-shrink: 0;
    margin-bottom: 1rem;
}

.title-slide-container .main-title {
    font-size: 3.5rem;
    margin-bottom: 0.5rem;
    font-weight: 700;
    text-shadow: 0 2px 4px var(--shadow-text);
    line-height: 1.1;
}

.title-slide-container .subtitle {
    font-size: 1.8rem;
    font-weight: 300;
    opacity: 0.9;
    margin: 0;
    line-height: 1.3;
}

.title-slide-container .logo-section {
    flex-shrink: 0;
    margin: 1rem 0;
    max-height: 30vh;
    display: flex;
    justify-content: center;
    align-items: center;
}

.title-slide-container .logo-image .title-logo {
    max-width: 280px;
    max-height: 200px;
    object-fit: contain;
    filter: drop-shadow(0 4px 8px rgba(0, 0, 0, 0.1));
}

.title-slide-container .logo-caption {
    font-size: 0.9rem;
    opacity: 0.7;
    margin-top: 0.5rem;
}

.title-slide-container .meta-section {
    flex-shrink: 0;
    margin-top: 1rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    align-items: center;
}

.title-slide-container .author-info {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.title-slide-container .author-label {
    font-size: 0.9rem;
    opacity: 0.7;
    font-weight: 300;
}

.title-slide-container .author-name {
    font-size: 1.4rem;
    font-weight: 500;
    margin: 0;
}

.title-slide-container .date-info {
    margin-top: 0.5rem;
}

.title-slide-container .presentation-date {
    font-size: 0.95rem;
    opacity: 0.8;
    margin: 0;
    font-weight: 300;
}

.title-slide-container .title-content {
    margin-top: 1rem;
    max-width: 90%;
}

/* Responsive para slides de título */
@media (max-width: 768px) {
    .title-slide-container .main-title {
        font-size: 2.5rem;
    }
    
    .title-slide-container .subtitle {
        font-size: 1.4rem;
    }
    
    .title-slide-container .logo-image .title-logo {
        max-width: 200px;
        max-height: 150px;
    }
    
    .title-slide-container .title-content {
        max-width: 95%;
    }
}

/* Layout simple para casos básicos */
.slide.title-slide:not(:has(.title-slide-container)) {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    gap: 1.5rem;
    text-align: center;
}

.slide.content-slide h1 {
    font-size: 2.5rem;
    color: var(--primary-color);
    margin-bottom: 2rem;
    border-bottom: 3px solid var(--primary-color);
    padding-bottom: 0.5rem;
}

.slide.section-slide h1 {
    font-size: 3rem;
    margin-bottom: 1rem;
    font-weight: 600;
}

/* Logo en slide título */
.slide-logo {
    max-width: 200px;
    max-height: 150px;
    margin: 1rem 0;
    object-fit: contain;
}

/* Metadatos del slide */
.slide-meta {
    position: absolute;
    bottom: 20px;
    right: 30px;
    font-size: 0.9rem;
    opacity: 0.7;
    color: var(--text-light);
}

/* Navigation elements removed - now handled by navigation module */
`
