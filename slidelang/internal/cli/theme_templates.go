// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

// CSS templates for different theme types

const businessThemeCSS = `/* Business Professional Theme */

/* Slide Layouts */
.slide {
    background: linear-gradient(135deg, var(--background-color) 0%, #f8f9fa 100%);
    border: 1px solid #dee2e6;
}

.slide h1, .slide h2 {
    color: var(--primary-color);
    font-weight: 600;
    margin-bottom: 1rem;
    border-bottom: 2px solid var(--accent-color);
    padding-bottom: 0.5rem;
}

.slide h3 {
    color: var(--text-color);
    font-weight: 500;
    margin-bottom: 0.75rem;
}

/* Professional Lists */
.slide ul {
    list-style: none;
    padding-left: 0;
}

.slide ul li {
    position: relative;
    padding-left: 1.5rem;
    margin-bottom: 0.5rem;
}

.slide ul li::before {
    content: "▶";
    color: var(--primary-color);
    position: absolute;
    left: 0;
    font-weight: bold;
}

/* Business Cards & Callouts */
.slide .info-block {
    background: linear-gradient(90deg, var(--accent-color), #17a2b8);
    color: white;
    padding: 1rem;
    border-radius: var(--border-radius);
    margin: 1rem 0;
}

.slide .warning-block {
    background: linear-gradient(90deg, #ffc107, #ffb300);
    color: #212529;
    padding: 1rem;
    border-radius: var(--border-radius);
    margin: 1rem 0;
}

/* Professional Tables */
.slide table {
    border-collapse: separate;
    border-spacing: 0;
    box-shadow: var(--box-shadow);
    border-radius: var(--border-radius);
    overflow: hidden;
}

.slide table th {
    background: var(--primary-color);
    color: white;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
}

.slide table td {
    border-bottom: 1px solid #dee2e6;
}

.slide table tr:hover td {
    background-color: #f8f9fa;
}

/* Code Blocks */
.slide pre {
    background: #f8f9fa;
    border: 1px solid #dee2e6;
    border-left: 4px solid var(--primary-color);
}

/* Charts & Diagrams */
.slide .chart-container,
.slide .diagram-container {
    background: white;
    border-radius: var(--border-radius);
    box-shadow: var(--box-shadow);
    padding: 1rem;
    margin: 1rem 0;
}
`

const academicThemeCSS = `/* Academic Research Theme */

/* Slide Layouts */
.slide {
    background: var(--background-color);
    border: none;
    font-family: var(--font-family);
}

.slide h1 {
    color: var(--primary-color);
    font-weight: 700;
    text-align: center;
    margin-bottom: 1.5rem;
    font-size: 2rem;
}

.slide h2 {
    color: var(--primary-color);
    font-weight: 600;
    margin-bottom: 1rem;
    font-size: 1.5rem;
    border-bottom: 1px solid var(--accent-color);
    padding-bottom: 0.25rem;
}

.slide h3 {
    color: var(--text-color);
    font-weight: 500;
    margin-bottom: 0.75rem;
    font-style: italic;
}

/* Academic Lists */
.slide ul {
    list-style-type: disc;
    padding-left: 2rem;
}

.slide ol {
    list-style-type: decimal;
    padding-left: 2rem;
}

.slide li {
    margin-bottom: 0.5rem;
    line-height: var(--line-height-base);
}

/* Citations and References */
.slide .citation {
    font-style: italic;
    color: #666;
    font-size: 0.9rem;
}

.slide .reference {
    font-size: 0.85rem;
    color: #555;
    border-left: 3px solid var(--accent-color);
    padding-left: 1rem;
    margin: 1rem 0;
}

/* Academic Blocks */
.slide .theorem {
    background: #f8f9fa;
    border: 1px solid var(--accent-color);
    border-radius: var(--border-radius);
    padding: 1rem;
    margin: 1rem 0;
}

.slide .theorem::before {
    content: "Theorem: ";
    font-weight: bold;
    color: var(--primary-color);
}

.slide .proof {
    background: #fafafa;
    border-left: 4px solid var(--accent-color);
    padding: 1rem;
    margin: 1rem 0;
    font-style: italic;
}

.slide .proof::before {
    content: "Proof: ";
    font-weight: bold;
    font-style: normal;
    color: var(--primary-color);
}

/* Academic Tables */
.slide table {
    border-collapse: collapse;
    margin: 1rem auto;
    max-width: 100%;
}

.slide table th {
    background: var(--secondary-color);
    color: var(--primary-color);
    font-weight: 600;
    text-align: center;
    border: 1px solid #ddd;
}

.slide table td {
    border: 1px solid #ddd;
    text-align: center;
    padding: 0.5rem;
}

/* Equations and Code */
.slide .equation {
    text-align: center;
    font-size: 1.2rem;
    margin: 1.5rem 0;
    padding: 1rem;
    background: #fafafa;
    border-radius: var(--border-radius);
}

.slide pre {
    background: #f5f5f5;
    border: 1px solid #ddd;
    font-family: 'Courier New', monospace;
}
`

const creativeThemeCSS = `/* Creative Modern Theme */

/* Slide Layouts */
.slide {
    background: linear-gradient(45deg, var(--background-color) 0%, #ffeaa7 50%, var(--background-color) 100%);
    position: relative;
    overflow: hidden;
}

.slide::before {
    content: "";
    position: absolute;
    top: -50%;
    left: -50%;
    width: 200%;
    height: 200%;
    background: radial-gradient(circle, transparent 20%, rgba(231, 76, 60, 0.05) 21%, rgba(231, 76, 60, 0.05) 34%, transparent 35%, transparent);
    animation: float 20s ease-in-out infinite;
}

@keyframes float {
    0%, 100% { transform: translateY(0px) rotate(0deg); }
    50% { transform: translateY(-20px) rotate(180deg); }
}

.slide h1, .slide h2 {
    background: linear-gradient(45deg, var(--primary-color), var(--secondary-color));
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    font-weight: 700;
    text-shadow: 2px 2px 4px rgba(0,0,0,0.1);
}

.slide h1 {
    font-size: 2.5rem;
    text-align: center;
    margin-bottom: 1.5rem;
}

.slide h2 {
    font-size: 1.8rem;
    margin-bottom: 1rem;
}

/* Creative Lists */
.slide ul {
    list-style: none;
    padding-left: 0;
}

.slide ul li {
    position: relative;
    padding-left: 2rem;
    margin-bottom: 0.75rem;
    transition: transform 0.3s ease;
}

.slide ul li::before {
    content: "●";
    color: var(--accent-color);
    position: absolute;
    left: 0;
    font-size: 1.2rem;
    animation: pulse 2s infinite;
}

@keyframes pulse {
    0%, 100% { transform: scale(1); }
    50% { transform: scale(1.2); }
}

.slide ul li:hover {
    transform: translateX(10px);
}

/* Creative Blocks */
.slide .info-block {
    background: linear-gradient(135deg, var(--accent-color), #00cec9);
    color: white;
    padding: 1.5rem;
    border-radius: 1rem;
    margin: 1rem 0;
    box-shadow: 0 10px 25px rgba(155, 89, 182, 0.3);
    transform: perspective(1000px) rotateY(5deg);
}

.slide .warning-block {
    background: linear-gradient(135deg, var(--secondary-color), #fdcb6e);
    color: white;
    padding: 1.5rem;
    border-radius: 1rem;
    margin: 1rem 0;
    box-shadow: 0 10px 25px rgba(243, 156, 18, 0.3);
    transform: perspective(1000px) rotateY(-5deg);
}

/* Creative Tables */
.slide table {
    border-collapse: separate;
    border-spacing: 0;
    border-radius: 1rem;
    overflow: hidden;
    box-shadow: 0 15px 35px rgba(0,0,0,0.1);
}

.slide table th {
    background: linear-gradient(45deg, var(--primary-color), var(--accent-color));
    color: white;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 1px;
}

.slide table td {
    background: rgba(255,255,255,0.9);
    transition: background-color 0.3s ease;
}

.slide table tr:hover td {
    background: rgba(155, 89, 182, 0.1);
}

/* Creative Code */
.slide pre {
    background: linear-gradient(135deg, #2d3436, #636e72);
    color: #ddd;
    border-radius: 1rem;
    border: none;
    box-shadow: 0 10px 25px rgba(0,0,0,0.2);
}

/* Interactive Elements */
.slide .interactive-element {
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    cursor: pointer;
}

.slide .interactive-element:hover {
    transform: scale(1.05) rotate(2deg);
    box-shadow: 0 15px 30px rgba(0,0,0,0.2);
}
`

const minimalThemeCSS = `/* Minimal Clean Theme */

/* Slide Layouts */
.slide {
    background: var(--background-color);
    border: none;
    box-shadow: none;
}

.slide h1 {
    color: var(--primary-color);
    font-weight: 300;
    font-size: 2rem;
    text-align: left;
    margin-bottom: 2rem;
    letter-spacing: -0.02em;
}

.slide h2 {
    color: var(--primary-color);
    font-weight: 400;
    font-size: 1.5rem;
    margin-bottom: 1.5rem;
    letter-spacing: -0.01em;
}

.slide h3 {
    color: var(--accent-color);
    font-weight: 400;
    font-size: 1.2rem;
    margin-bottom: 1rem;
}

/* Minimal Lists */
.slide ul {
    list-style: none;
    padding-left: 0;
}

.slide ul li {
    position: relative;
    padding-left: 1rem;
    margin-bottom: 0.75rem;
    color: var(--text-color);
}

.slide ul li::before {
    content: "–";
    color: var(--accent-color);
    position: absolute;
    left: 0;
    font-weight: 300;
}

.slide ol {
    list-style: decimal;
    padding-left: 1.5rem;
    color: var(--text-color);
}

/* Minimal Blocks */
.slide .info-block {
    background: none;
    border-left: 3px solid var(--accent-color);
    padding: 1rem 1.5rem;
    margin: 1.5rem 0;
    color: var(--text-color);
}

.slide .warning-block {
    background: none;
    border-left: 3px solid #999;
    padding: 1rem 1.5rem;
    margin: 1.5rem 0;
    color: var(--text-color);
}

/* Minimal Tables */
.slide table {
    border-collapse: collapse;
    border: none;
    margin: 1.5rem 0;
}

.slide table th {
    background: none;
    color: var(--primary-color);
    font-weight: 400;
    text-align: left;
    border-bottom: 1px solid var(--accent-color);
    padding: 0.75rem 1rem;
}

.slide table td {
    border: none;
    border-bottom: 1px solid #eee;
    padding: 0.75rem 1rem;
    color: var(--text-color);
}

.slide table tr:hover td {
    background: none;
}

/* Minimal Code */
.slide pre {
    background: #fafafa;
    border: 1px solid #eee;
    border-radius: var(--border-radius);
    font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
    font-size: 0.9rem;
}

.slide code {
    background: #f5f5f5;
    padding: 0.2rem 0.4rem;
    border-radius: 0.2rem;
    font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
    font-size: 0.9em;
}

/* Minimal Images */
.slide img {
    max-width: 100%;
    height: auto;
    border: none;
    border-radius: 0;
    box-shadow: none;
}

/* Clean spacing */
.slide p {
    margin-bottom: 1rem;
    color: var(--text-color);
    line-height: var(--line-height-base);
}

.slide .spacer {
    height: 2rem;
}
`
