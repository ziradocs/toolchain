// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package layouts

// InfographicsCSS provides CSS for infographic and data visualization layouts
const InfographicsCSS = `/* === INFOGRAPHIC LAYOUTS === */

/* Process Flow Layout */
.slide.process-layout {
    padding: 3rem;
}

.slide.process-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.process-layout .process-flow {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    max-width: 1000px;
    margin: 0 auto;
}

.slide.process-layout .process-step {
    flex: 1;
    text-align: center;
    position: relative;
}

.slide.process-layout .process-step:not(:last-child)::after {
    content: '→';
    position: absolute;
    right: -1rem;
    top: 50%;
    transform: translateY(-50%);
    font-size: 2rem;
    color: var(--primary-color);
    z-index: 1;
}

.slide.process-layout .process-icon {
    width: 80px;
    height: 80px;
    background: var(--primary-color);
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 0 auto 1rem;
    color: var(--text-on-primary);
    font-size: 2rem;
    font-weight: bold;
    box-shadow: var(--shadow-main);
}

.slide.process-layout .process-title {
    font-size: 1.2rem;
    color: var(--secondary-color);
    margin-bottom: 0.5rem;
    font-weight: 600;
}

.slide.process-layout .process-description {
    color: var(--text-color);
    font-size: 0.95rem;
    line-height: 1.4;
}

/* Statistics Layout */
.slide.statistics-layout {
    padding: 3rem;
    background: linear-gradient(135deg, var(--bg-light), var(--bg-white));
}

.slide.statistics-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.statistics-layout .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 2rem;
    max-width: 1000px;
    margin: 0 auto;
}

.slide.statistics-layout .stat-card {
    background: var(--bg-white);
    border-radius: var(--border-radius-lg);
    padding: 2rem;
    text-align: center;
    box-shadow: var(--shadow-main);
    transition: var(--transition);
    position: relative;
    overflow: hidden;
}

.slide.statistics-layout .stat-card:hover {
    transform: translateY(-5px);
    box-shadow: var(--shadow-lg);
}

.slide.statistics-layout .stat-card::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 4px;
    background: var(--primary-color);
}

.slide.statistics-layout .stat-number {
    font-size: 3.5rem;
    font-weight: 700;
    color: var(--primary-color);
    line-height: 1;
    margin-bottom: 0.5rem;
}

.slide.statistics-layout .stat-label {
    font-size: 1.1rem;
    color: var(--secondary-color);
    font-weight: 600;
    margin-bottom: 0.5rem;
}

.slide.statistics-layout .stat-description {
    font-size: 0.9rem;
    color: var(--text-light);
    line-height: 1.4;
}

/* Hierarchy Layout */
.slide.hierarchy-layout {
    padding: 3rem;
}

.slide.hierarchy-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.hierarchy-layout .hierarchy-container {
    max-width: 800px;
    margin: 0 auto;
}

.slide.hierarchy-layout .hierarchy-level {
    margin-bottom: 2rem;
}

.slide.hierarchy-layout .hierarchy-level-1 {
    text-align: center;
}

.slide.hierarchy-layout .hierarchy-level-1 .hierarchy-item {
    background: var(--primary-color);
    color: var(--text-on-primary);
    padding: 1.5rem 3rem;
    border-radius: var(--border-radius-lg);
    font-size: 1.3rem;
    font-weight: 600;
    margin: 0 auto;
    max-width: 400px;
    box-shadow: var(--shadow-main);
}

.slide.hierarchy-layout .hierarchy-level-2 {
    display: flex;
    justify-content: center;
    gap: 2rem;
    margin-top: 2rem;
}

.slide.hierarchy-layout .hierarchy-level-2 .hierarchy-item {
    background: var(--secondary-color);
    color: var(--text-on-primary);
    padding: 1rem 2rem;
    border-radius: var(--border-radius);
    font-size: 1.1rem;
    font-weight: 500;
    flex: 1;
    max-width: 180px;
    text-align: center;
    box-shadow: var(--shadow-main);
}

.slide.hierarchy-layout .hierarchy-level-3 {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    margin-top: 2rem;
}

.slide.hierarchy-layout .hierarchy-level-3 .hierarchy-item {
    background: var(--bg-white);
    color: var(--text-color);
    padding: 1rem;
    border-radius: var(--border-radius);
    border: 2px solid var(--primary-color);
    font-size: 1rem;
    text-align: center;
    flex: 1;
    box-shadow: var(--shadow-main);
}

/* Connection lines for hierarchy */
.slide.hierarchy-layout .hierarchy-level-1::after {
    content: '';
    display: block;
    width: 2px;
    height: 30px;
    background: var(--primary-color);
    margin: 1rem auto;
}

.slide.hierarchy-layout .hierarchy-level-2::before {
    content: '';
    display: block;
    height: 2px;
    background: var(--primary-color);
    margin-bottom: 1rem;
    width: 60%;
    margin-left: auto;
    margin-right: auto;
}

/* Flow Diagram Layout */
.slide.flow-layout {
    padding: 3rem;
}

.slide.flow-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.flow-layout .flow-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2rem;
    max-width: 600px;
    margin: 0 auto;
}

.slide.flow-layout .flow-step {
    background: var(--bg-white);
    border: 2px solid var(--primary-color);
    border-radius: var(--border-radius-lg);
    padding: 1.5rem 2rem;
    text-align: center;
    width: 100%;
    box-shadow: var(--shadow-main);
    transition: var(--transition);
    position: relative;
}

.slide.flow-layout .flow-step:hover {
    transform: scale(1.02);
    box-shadow: var(--shadow-lg);
}

.slide.flow-layout .flow-step:not(:last-child)::after {
    content: '↓';
    position: absolute;
    bottom: -25px;
    left: 50%;
    transform: translateX(-50%);
    font-size: 1.5rem;
    color: var(--primary-color);
    background: var(--bg-white);
    padding: 0 0.5rem;
}

.slide.flow-layout .flow-step.decision {
    background: var(--warning-color);
    color: var(--text-on-accent);
    border-color: var(--warning-color);
    transform: rotate(45deg);
    margin: 1rem 0;
}

.slide.flow-layout .flow-step.decision .flow-content {
    transform: rotate(-45deg);
}

.slide.flow-layout .flow-title {
    font-size: 1.2rem;
    font-weight: 600;
    color: var(--secondary-color);
    margin-bottom: 0.5rem;
}

.slide.flow-layout .flow-step.decision .flow-title {
    color: var(--text-on-accent);
}

.slide.flow-layout .flow-description {
    font-size: 1rem;
    color: var(--text-color);
    line-height: 1.4;
}

.slide.flow-layout .flow-step.decision .flow-description {
    color: var(--text-on-accent);
}

/* Map Visualization Layout */
.slide.map-layout {
    padding: 3rem;
}

.slide.map-layout h1 {
    text-align: center;
    margin-bottom: 2rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.map-layout .map-container {
    display: flex;
    gap: 2rem;
    height: 70vh;
}

.slide.map-layout .map-visual {
    flex: 2;
    background: var(--bg-light);
    border-radius: var(--border-radius-lg);
    position: relative;
    overflow: hidden;
    box-shadow: var(--shadow-main);
}

.slide.map-layout .map-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 1rem;
}

.slide.map-layout .map-legend {
    background: var(--bg-white);
    border-radius: var(--border-radius);
    padding: 1.5rem;
    box-shadow: var(--shadow-main);
}

.slide.map-layout .map-legend h3 {
    color: var(--secondary-color);
    margin-bottom: 1rem;
    font-size: 1.2rem;
}

.slide.map-layout .legend-item {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    margin-bottom: 0.8rem;
}

.slide.map-layout .legend-color {
    width: 20px;
    height: 20px;
    border-radius: 4px;
    flex-shrink: 0;
}

.slide.map-layout .legend-label {
    color: var(--text-color);
    font-size: 0.95rem;
}

/* Chart Container Layout */
.slide.chart-layout {
    padding: 3rem;
}

.slide.chart-layout h1 {
    text-align: center;
    margin-bottom: 2rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.chart-layout .chart-container {
    background: var(--bg-white);
    border-radius: var(--border-radius-lg);
    padding: 2rem;
    box-shadow: var(--shadow-main);
    height: 70vh;
    display: flex;
    flex-direction: column;
}

.slide.chart-layout .chart-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2rem;
    padding-bottom: 1rem;
    border-bottom: 2px solid var(--bg-light);
}

.slide.chart-layout .chart-title {
    font-size: 1.5rem;
    color: var(--secondary-color);
    font-weight: 600;
}

.slide.chart-layout .chart-subtitle {
    color: var(--text-light);
    font-size: 1rem;
}

.slide.chart-layout .chart-content {
    flex: 1;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
}

.slide.chart-layout .chart-placeholder {
    width: 100%;
    height: 100%;
    border: 2px dashed var(--primary-color);
    border-radius: var(--border-radius);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--primary-color);
    font-size: 1.2rem;
    background: var(--bg-placeholder);
}

/* Diagram Layout */
.slide.diagram-layout {
    padding: 3rem;
}

.slide.diagram-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.diagram-layout .diagram-container {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    grid-template-rows: repeat(3, 1fr);
    gap: 2rem;
    height: 60vh;
    max-width: 800px;
    margin: 0 auto;
}

.slide.diagram-layout .diagram-node {
    background: var(--bg-white);
    border-radius: var(--border-radius-lg);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    box-shadow: var(--shadow-main);
    transition: var(--transition);
    position: relative;
}

.slide.diagram-layout .diagram-node:hover {
    transform: scale(1.05);
    box-shadow: var(--shadow-lg);
}

.slide.diagram-layout .diagram-node.primary {
    background: var(--primary-color);
    color: var(--text-on-primary);
    grid-column: 2;
    grid-row: 2;
}

.slide.diagram-layout .diagram-node.secondary {
    border: 2px solid var(--primary-color);
    color: var(--secondary-color);
}

.slide.diagram-layout .node-icon {
    font-size: 2rem;
    margin-bottom: 0.5rem;
}

.slide.diagram-layout .node-title {
    font-size: 1.1rem;
    font-weight: 600;
    margin-bottom: 0.5rem;
}

.slide.diagram-layout .node-description {
    font-size: 0.9rem;
    line-height: 1.3;
}

/* Metrics Dashboard Layout */
.slide.metrics-layout {
    padding: 3rem;
    background: var(--bg-light);
}

.slide.metrics-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.metrics-layout .metrics-dashboard {
    display: grid;
    grid-template-columns: 2fr 1fr;
    grid-template-rows: auto 1fr;
    gap: 2rem;
    height: 70vh;
}

.slide.metrics-layout .main-metric {
    grid-column: 1;
    grid-row: 1 / -1;
    background: var(--bg-white);
    border-radius: var(--border-radius-lg);
    padding: 3rem;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    text-align: center;
    box-shadow: var(--shadow-main);
}

.slide.metrics-layout .main-metric-number {
    font-size: 5rem;
    font-weight: 700;
    color: var(--primary-color);
    line-height: 1;
    margin-bottom: 1rem;
}

.slide.metrics-layout .main-metric-label {
    font-size: 1.5rem;
    color: var(--secondary-color);
    font-weight: 600;
    margin-bottom: 1rem;
}

.slide.metrics-layout .main-metric-change {
    font-size: 1.1rem;
    font-weight: 500;
}

.slide.metrics-layout .main-metric-change.positive {
    color: var(--success-color);
}

.slide.metrics-layout .main-metric-change.negative {
    color: var(--danger-color);
}

.slide.metrics-layout .side-metrics {
    grid-column: 2;
    display: flex;
    flex-direction: column;
    gap: 1rem;
}

.slide.metrics-layout .side-metric {
    background: var(--bg-white);
    border-radius: var(--border-radius);
    padding: 1.5rem;
    box-shadow: var(--shadow-main);
    text-align: center;
}

.slide.metrics-layout .side-metric-number {
    font-size: 2rem;
    font-weight: 700;
    color: var(--primary-color);
    margin-bottom: 0.5rem;
}

.slide.metrics-layout .side-metric-label {
    font-size: 0.9rem;
    color: var(--text-color);
    font-weight: 500;
}

/* Responsive adjustments for infographic layouts */
@media (max-width: 768px) {
    .slide.process-layout .process-flow {
        flex-direction: column;
        gap: 2rem;
    }
    
    .slide.process-layout .process-step:not(:last-child)::after {
        content: '↓';
        right: auto;
        top: auto;
        bottom: -1.5rem;
        left: 50%;
        transform: translateX(-50%);
    }
    
    .slide.hierarchy-layout .hierarchy-level-2,
    .slide.hierarchy-layout .hierarchy-level-3 {
        flex-direction: column;
        align-items: center;
    }
    
    .slide.map-layout .map-container {
        flex-direction: column;
        height: auto;
    }
    
    .slide.metrics-layout .metrics-dashboard {
        grid-template-columns: 1fr;
        grid-template-rows: auto auto;
    }
    
    .slide.metrics-layout .main-metric {
        grid-column: 1;
        grid-row: 1;
    }
    
    .slide.diagram-layout .diagram-container {
        grid-template-columns: 1fr;
        grid-template-rows: auto;
        height: auto;
    }
    
    .slide.diagram-layout .diagram-node.primary {
        grid-column: 1;
        grid-row: auto;
        order: -1;
    }
}
`
