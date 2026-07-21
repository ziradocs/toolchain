// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

// GetSpecializedCSS retorna CSS específico para layouts especializados
func GetSpecializedCSS() string {
	return `
/* =================================
   SPECIALIZED LAYOUTS CSS - COMPLEMENTARY
   ================================= */

/* Title Layout - Enhance existing title-slide styles */
.slide[data-slide-type="title"] {
    /* Inherit existing title-slide styles from base CSS */
}

.slide[data-slide-type="title"] .slide-logo {
    margin-top: 2rem;
    filter: drop-shadow(0 4px 8px rgba(0,0,0,0.2));
}

/* Section Layout - For section dividers */
.slide[data-slide-type="section"] {
    background: var(--title-gradient, var(--gradient-bg));
    color: var(--text-on-primary);
    justify-content: center;
}

.slide[data-slide-type="section"] h1 {
    font-size: 2.5rem !important;
    font-weight: 600;
    margin-bottom: 0.5rem;
    border-left: 5px solid rgba(255,255,255,0.8) !important;
    border-bottom: none !important;
    padding-left: 1rem;
    color: var(--text-on-primary) !important;
}

/* Comparison Layout - Enhanced for side-by-side content */
.slide[data-slide-type="comparison"] .special-block {
    display: inline-block;
    width: calc(50% - 1rem);
    margin: 0.5rem;
    vertical-align: top;
    border-radius: 12px;
    padding: 1.5rem;
    box-shadow: 0 4px 12px rgba(0,0,0,0.1);
    transition: transform 0.2s ease;
}

.slide[data-slide-type="comparison"] .special-block.info {
    background: var(--bg-info, linear-gradient(135deg, var(--bg-light), var(--bg-white)));
    border-left: 4px solid var(--info-color);
}

.slide[data-slide-type="comparison"] .special-block.success {
    background: var(--bg-success, linear-gradient(135deg, var(--bg-light), var(--bg-white)));
    border-left: 4px solid var(--success-color);
}

/* Stats Layout - Enhanced tables and data presentation */
.slide[data-slide-type="stats"] table {
    margin: 1.5rem 0;
    box-shadow: 0 4px 12px rgba(0,0,0,0.1);
    border-radius: 8px;
    overflow: hidden;
}

.slide[data-slide-type="stats"] th {
    background: var(--gradient-bg);
    color: var(--text-on-primary);
    padding: 1rem;
    font-weight: 600;
}

/* Code Example Layout - Enhanced code presentation */
.slide[data-slide-type="code_example"] pre {
    background: var(--bg-code);
    color: var(--text-on-dark);
    border-radius: 8px;
    padding: 1.5rem;
    margin: 1.5rem 0;
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    font-family: 'Fira Code', 'Monaco', 'Cascadia Code', monospace;
    line-height: 1.5;
}

.slide[data-slide-type="code_example"] ul {
    background: var(--bg-light);
    border-radius: 8px;
    padding: 1.5rem;
    margin: 1.5rem 0;
    border-left: 4px solid var(--info-color);
}

/* Responsive adjustments */
@media (max-width: 768px) {
    .slide[data-slide-type="comparison"] .special-block {
        width: 100%;
        margin: 0.5rem 0;
    }
}`
}
