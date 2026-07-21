// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package base

// ResetCSS provides basic CSS reset and normalization
const ResetCSS = `/* Reset y normalización */
* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

html, body {
    margin: 0;
    padding: 0;
    height: 100%;
}

body {
    font-family: var(--font-main);
    background: var(--gradient-bg);
    color: var(--text-color);
    overflow: hidden;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
}

/* Elementos básicos - reset menos agresivo */
h1, h2, h3, h4, h5, h6 {
    margin: 0;
    padding: 0;
    font-weight: inherit;
}

/* Solo resetear listas que no son de contenido */
ul, ol {
    margin: 0;
    padding: 0;
}

/* Mantener estilos básicos para elementos dentro de slides */
.slide h1, .slide h2, .slide h3, .slide h4, .slide h5, .slide h6 {
    margin-bottom: 1rem;
    font-weight: 600;
    line-height: 1.2;
}

.slide h2 {
    margin-bottom: 0.8rem;
    margin-top: 1.5rem;
}

.slide h3 {
    margin-bottom: 0.6rem;
    margin-top: 1.2rem;
    font-size: 1.3rem;
}

.slide h4 {
    margin-bottom: 0.5rem;
    margin-top: 1rem;
    font-size: 1.1rem;
}

img {
    max-width: 100%;
    height: auto;
    display: block;
}

button {
    border: none;
    background: none;
    cursor: pointer;
    font-family: inherit;
}

a {
    text-decoration: none;
    color: inherit;
}

table {
    border-collapse: collapse;
    border-spacing: 0;
}

/* Scroll suave */
* {
    scroll-behavior: smooth;
}
`
