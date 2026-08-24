// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

/**
 * Charts Module for SlideLang
 * Event-driven module that follows SlideLang standard patterns
 * Only renders charts when their slide is visible
 */

// Global chart registry to track active charts
const chartRegistry = new Map();

// Default color palette for charts
const defaultColors = [
    '#3B82F6', '#10B981', '#F59E0B', '#EF4444',
    '#06B6D4', '#8B5CF6', '#F97316', '#EC4899'
];

// getCategoricalPalette / categoricalColor (motor-temas-v2.md §2.2): un
// tema puede sobreescribir defaultColors índice por índice vía sus tokens
// chart-cat-1..8, ya resueltos a literales server-side
// (themes.ResolveThemeTokens) — Chart.js's fillStyle de canvas no acepta
// var(). chart-cat-* es un set CON ORDEN (identidad de serie legible bajo
// daltonismo): se preserva el MISMO wrap por módulo que este módulo ya
// usaba con defaultColors, nunca se genera un matiz nuevo para una serie
// N+1. Un tema que no declara chart-cat-* (todo tema del repo hoy) deja
// defaultColors sin tocar.
function getCategoricalPalette() {
    const metadata = (typeof SlideLang !== 'undefined' && SlideLang.metadata) || {};
    const tokens = metadata.themeTokens;
    if (tokens && Array.isArray(tokens.chartCategorical) && tokens.chartCategorical.length > 0) {
        return tokens.chartCategorical;
    }
    return defaultColors;
}

function categoricalColor(index) {
    const palette = getCategoricalPalette();
    return palette[index % palette.length];
}

// withAlpha (motor-temas-v2.md §2.2): categoricalColor() can now return a
// theme's chart-cat-* token instead of one of the hardcoded hex
// defaultColors, and a token isn't guaranteed to be #RRGGBB — it can be
// any CSS color IsValidMapColor-class validation doesn't gate (rgb(),
// hsl(), a 3-digit hex, a bare name like "red"). Appending the literal
// string '80' — the old fill-alpha trick — only produces a valid color for
// exactly the 6-digit-hex case; for anything else it silently produces an
// invalid CSS color string ("red80", "rgb(...)80") that Chart.js's canvas
// fillStyle then just ignores. ALPHA_FRACTION = 0x80/255, chosen so the
// hex branch below reproduces the exact byte sequence ("...80") the old
// code always produced — this is a behavior-preserving fix for
// defaultColors, not a visual change.
const ALPHA_FRACTION = 128 / 255;

function withAlpha(color, alphaFraction) {
    if (typeof color !== 'string') {
        return color;
    }
    const hexMatch = color.match(/^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/);
    if (hexMatch) {
        let hex = hexMatch[1];
        if (hex.length === 3) {
            hex = hex.split('').map((c) => c + c).join('');
        } else if (hex.length === 8) {
            hex = hex.slice(0, 6);
        }
        const alphaHex = Math.round(alphaFraction * 255).toString(16).padStart(2, '0');
        return '#' + hex + alphaHex;
    }
    const rgbMatch = color.match(/^rgba?\(\s*([\d.]+%?)\s*,\s*([\d.]+%?)\s*,\s*([\d.]+%?)\s*(?:,\s*[\d.]+\s*)?\)$/i);
    if (rgbMatch) {
        return 'rgba(' + rgbMatch[1] + ', ' + rgbMatch[2] + ', ' + rgbMatch[3] + ', ' + alphaFraction + ')';
    }
    const hslMatch = color.match(/^hsla?\(\s*([\d.]+)\s*,\s*([\d.]+%)\s*,\s*([\d.]+%)\s*(?:,\s*[\d.]+\s*)?\)$/i);
    if (hslMatch) {
        return 'hsla(' + hslMatch[1] + ', ' + hslMatch[2] + ', ' + hslMatch[3] + ', ' + alphaFraction + ')';
    }
    // Named color or anything else Chart.js/canvas would otherwise accept
    // as-is: color-mix is the only format-agnostic way to apply alpha
    // without a full CSS color parser, and has been supported in every
    // Chart.js-capable browser (and the Chromium version this toolchain's
    // offline/PDF path embeds) since 2023.
    return 'color-mix(in srgb, ' + color + ' ' + Math.round(alphaFraction * 100) + '%, transparent)';
}

// applyExtensionChartColors (motor-temas-v2.md §2.2): superpone los
// tokens chart-grid/-axis/-label/-tooltip-bg sobre lo que buildConfig() o
// la config JSON del autor ya hayan producido, sin pisar nunca un valor
// que la config ya trae puesto. chart-surface NO se maneja acá: a
// diferencia de estos (dibujados sobre canvas, solo alcanzables vía
// opciones de Chart.js), es un fondo plano detrás del canvas y se
// propaga por CSS normal (ver charts.css) — GenerateThemeCSS ya emite
// CUALQUIER variable que un tema declare en :root, así que un segundo
// camino Go/JS para ese único token sería duplicar un mecanismo que ya
// funciona.
// CARTESIAN_CHART_TYPES are the Chart.js types that get an implicit
// x/y scale pair when options.scales omits one — bar/line/scatter/bubble,
// and (via buildConfig's originalType==='combo' -> chartType 'bar' mapping)
// combo. pie/doughnut have no cartesian scales at all (applyThemeColors
// deletes options.scales for them above), so they're deliberately excluded
// here rather than gaining a spurious x/y.
const CARTESIAN_CHART_TYPES = ['bar', 'line', 'scatter', 'bubble'];

function applyExtensionChartColors(themedConfig) {
    const metadata = (typeof SlideLang !== 'undefined' && SlideLang.metadata) || {};
    const tokens = (metadata.themeTokens && metadata.themeTokens.chart) || null;
    if (!tokens) {
        return;
    }
    if (!themedConfig.options) {
        themedConfig.options = {};
    }

    if (tokens['chart-grid'] || tokens['chart-axis']) {
        // buildConfig() always declares scales.y explicitly but leaves x
        // implicit (Chart.js creates it at chart-construction time from
        // its own defaults) — so "scales exists" isn't enough to know
        // every axis is present. Materialize x/y here, before iterating,
        // whenever they're missing on a cartesian chart; this mirrors what
        // Chart.js would do internally anyway, so it changes coloring only,
        // never chart behavior.
        if (CARTESIAN_CHART_TYPES.includes(themedConfig.type)) {
            themedConfig.options.scales = themedConfig.options.scales || {};
            if (!themedConfig.options.scales.x) {
                themedConfig.options.scales.x = {};
            }
            if (!themedConfig.options.scales.y) {
                themedConfig.options.scales.y = {};
            }
        }
        const scales = themedConfig.options.scales;
        if (scales && typeof scales === 'object') {
            Object.keys(scales).forEach((key) => {
                const scale = scales[key];
                if (!scale || typeof scale !== 'object') {
                    return;
                }
                if (tokens['chart-grid']) {
                    scale.grid = scale.grid || {};
                    if (scale.grid.color === undefined) {
                        scale.grid.color = tokens['chart-grid'];
                    }
                }
                if (tokens['chart-axis']) {
                    scale.ticks = scale.ticks || {};
                    if (scale.ticks.color === undefined) {
                        scale.ticks.color = tokens['chart-axis'];
                    }
                }
            });
        }
    }

    if (tokens['chart-label']) {
        themedConfig.options.plugins = themedConfig.options.plugins || {};
        themedConfig.options.plugins.legend = themedConfig.options.plugins.legend || {};
        themedConfig.options.plugins.legend.labels = themedConfig.options.plugins.legend.labels || {};
        if (themedConfig.options.plugins.legend.labels.color === undefined) {
            themedConfig.options.plugins.legend.labels.color = tokens['chart-label'];
        }
    }

    if (tokens['chart-tooltip-bg']) {
        themedConfig.options.plugins = themedConfig.options.plugins || {};
        themedConfig.options.plugins.tooltip = themedConfig.options.plugins.tooltip || {};
        if (themedConfig.options.plugins.tooltip.backgroundColor === undefined) {
            themedConfig.options.plugins.tooltip.backgroundColor = tokens['chart-tooltip-bg'];
        }
    }
}

/**
 * Charts Module - Following SlideLang Standard Pattern
 */
const SlideLangCharts = {
    initialized: false,
    
    init: function() {
        if (this.initialized) {
            return;
        }
        
        // Verificar dependencias externas
        if (typeof Chart === 'undefined') {
            console.error('[Charts] Chart.js is not loaded!');
            return;
        }
        
        this.initialized = true;
        this.setupEventListeners();
        this.processInitialSlide();
    },
    
    // Sistema de eventos estándar
    setupEventListeners: function() {
        document.addEventListener('slidelang:slideChanged', (event) => {
            const { slideElement, currentSlide, previousSlide } = event.detail;
            this.handleSlideChange(slideElement);
        });
    },
    
    processInitialSlide: function() {
        const activeSlide = document.querySelector('.slidelang-slide.slidelang-active');
        if (activeSlide) {
            // Check if the active slide has any charts before processing
            const hasCharts = this.slideHasCharts(activeSlide);
            if (hasCharts) {
                this.handleSlideChange(activeSlide);
            }
        }
    },
    
    slideHasCharts: function(slideElement) {
        // Check for charts in metadata first
        const metadata = SlideLang.metadata || {};
        if (metadata.charts && metadata.charts.length > 0) {
            // Check if any chart belongs to this slide
            const slideCharts = metadata.charts.filter(chartConfig => {
                const canvas = document.getElementById(chartConfig.id);
                return canvas && slideElement.contains(canvas);
            });
            if (slideCharts.length > 0) {
                return true;
            }
        }
        
        // Fallback: check for canvas elements with chart attributes
        const canvasElements = slideElement.querySelectorAll('canvas.slidelang-chart-canvas');
        return canvasElements.length > 0;
    },
    
    handleSlideChange: function(slideElement) {
        // Destruir charts no visibles
        this.destroyChartsNotInSlide(slideElement);
        
        // Procesar charts en slide actual
        this.processElementsInSlide(slideElement);
    },
    
    processElementsInSlide: function(slideElement) {
        const processedCharts = new Set();
        
        // Priority 1: Process charts from metadata
        this.processChartsFromMetadata(processedCharts);
        
        // Priority 2: Fallback to legacy data attributes for unprocessed charts
        this.processChartsFromAttributes(slideElement, processedCharts);
    },
    
    processChartsFromMetadata: function(processedCharts) {
        const metadata = SlideLang.metadata || {};
        if (!metadata.charts || metadata.charts.length === 0) {
            return;
        }
        
        metadata.charts.forEach(chartConfig => {
            const chartId = chartConfig.id;
            const canvas = document.getElementById(chartId);
            
            if (!canvas) {
                console.warn(`[Charts] Canvas with ID ${chartId} not found`);
                return;
            }
            
            if (chartRegistry.has(chartId)) {
                return;
            }
            
            // Only process if canvas is in current slide
            const slideElement = canvas.closest('.slidelang-slide.slidelang-active');
            if (!slideElement) {
                return;
            }
            
            const config = chartConfig.config;

            const chart = this.createChart(canvas, config);
            if (chart) {
                chartRegistry.set(chartId, chart);
                processedCharts.add(chartId);
            }
        });
    },
    
    processChartsFromAttributes: function(slideElement, processedCharts) {
        const canvasElements = slideElement.querySelectorAll('canvas.slidelang-chart-canvas');
        const unprocessedCanvases = Array.from(canvasElements).filter(canvas => 
            canvas.id && !processedCharts.has(canvas.id)
        );
        
        if (unprocessedCanvases.length === 0) {
            return;
        }
        
        unprocessedCanvases.forEach(canvas => {
            const chartId = canvas.id;
            
            if (chartRegistry.has(chartId)) {
                return;
            }
            
            const config = this.createConfigFromAttributes(canvas);
            if (config) {
                const chart = this.createChart(canvas, config);
                if (chart) {
                    chartRegistry.set(chartId, chart);
                }
            }
        });
    },
      createChart: function(canvas, config) {
        // Validar que el canvas existe y es válido
        if (!canvas) {
            console.error('[Charts] Canvas element not provided');
            return null;
        }

        // Si es string, buscar el elemento
        if (typeof canvas === 'string') {
            canvas = document.getElementById(canvas);
            if (!canvas) {
                console.error('[Charts] Canvas element not found:', canvas);
                return null;
            }
        }

        // Si el elemento es un DIV, buscar canvas dentro
        if (canvas.tagName === 'DIV') {
            const canvasElement = canvas.querySelector('canvas');
            if (!canvasElement) {
                console.error('[Charts] Canvas element not found inside DIV:', canvas);
                return null;
            }
            canvas = canvasElement;
        }

        // Verificar que es un canvas válido
        if (!canvas.tagName || canvas.tagName !== 'CANVAS') {
            console.error('[Charts] Element is not a canvas:', canvas.tagName, canvas);
            return null;
        }

        try {
            const ctx = canvas.getContext('2d');
            if (!ctx) {
                console.error('[Charts] Could not get 2D context for canvas:', canvas.id);
                return null;
            }

            // DEBUG: Log completo de la configuración
            // (Removed verbose debug logging)

            // Apply theme colors to config
            const themedConfig = this.applyThemeColors(config);
            
            // Process JavaScript functions in callbacks
            if (themedConfig.options && themedConfig.options.plugins && 
                themedConfig.options.plugins.tooltip && 
                themedConfig.options.plugins.tooltip.callbacks) {
                const callbacks = themedConfig.options.plugins.tooltip.callbacks;
                
                // DEBUG: Log callbacks antes del procesamiento
                // (Removed verbose debug logging)
                
                for (const [key, value] of Object.entries(callbacks)) {
                    // (Removed verbose debug logging)
                    
                    if (value && typeof value === 'object' && value._function === true && value.body) {
                        // Convert function string to actual function
                        try {
                            themedConfig.options.plugins.tooltip.callbacks[key] = new Function('context', value.body);
                        } catch (error) {
                            if (typeof console !== 'undefined' && console.warn) {
                                console.warn('[Charts] Error creating callback function:', error);
                            }
                        }
                    }
                }
            }

            // Create Chart.js instance
            return new Chart(ctx, themedConfig);
            
        } catch (error) {
            console.error(`[Charts] Error creating chart for canvas ${canvas.id}:`, error);
            return null;
        }
    },
    
    applyThemeColors: function(config) {
        const themedConfig = JSON.parse(JSON.stringify(config)); // Deep clone
        
        // Apply colors to datasets if not already set
        if (themedConfig.data && themedConfig.data.datasets) {
            themedConfig.data.datasets.forEach((dataset, index) => {
                // Para pie/doughnut charts, aplicar colores a cada segmento
                if (config.type === 'pie' || config.type === 'doughnut') {
                    if (!dataset.backgroundColor) {
                        // Asignar un color diferente a cada segmento
                        dataset.backgroundColor = dataset.data.map((_, segmentIndex) =>
                            withAlpha(categoricalColor(segmentIndex), ALPHA_FRACTION)
                        );
                    }
                    if (!dataset.borderColor) {
                        dataset.borderColor = dataset.data.map((_, segmentIndex) => 
                            categoricalColor(segmentIndex)
                        );
                    }
                } else {
                    // Para otros tipos de charts, usar un color por dataset
                    if (!dataset.backgroundColor) {
                        dataset.backgroundColor = withAlpha(categoricalColor(index), ALPHA_FRACTION);
                    }
                    if (!dataset.borderColor) {
                        dataset.borderColor = categoricalColor(index);
                    }
                }
                if (!dataset.borderWidth) {
                    dataset.borderWidth = 2;
                }
            });
        }
        
        // Ensure responsive options
        if (!themedConfig.options) {
            themedConfig.options = {};
        }
        
        themedConfig.options.responsive = true;
        themedConfig.options.maintainAspectRatio = false;
        
        // Para pie/doughnut charts, configurar opciones específicas
        if (config.type === 'pie' || config.type === 'doughnut') {
            // Remover escalas Y (no aplicables a pie/doughnut)
            if (themedConfig.options.scales) {
                delete themedConfig.options.scales;
            }
            
            // Configurar plugins por defecto
            if (!themedConfig.options.plugins) {
                themedConfig.options.plugins = {};
            }
            
            // Configurar legend
            if (!themedConfig.options.plugins.legend) {
                themedConfig.options.plugins.legend = {
                    position: 'right'
                };
            }
            
            // Configurar tooltip con porcentajes
            if (!themedConfig.options.plugins.tooltip) {
                themedConfig.options.plugins.tooltip = {
                    callbacks: {
                        label: function(context) {
                            const total = context.dataset.data.reduce((a, b) => a + b, 0);
                            const percentage = Math.round((context.parsed / total) * 100);
                            return context.label + ': $' + context.parsed + 'K (' + percentage + '%)';
                        }
                    }
                };
            }
        }

        applyExtensionChartColors(themedConfig);

        return themedConfig;
    },

    createConfigFromAttributes: function(canvas) {
        const chartType = canvas.getAttribute('data-chart-type') || 'bar';
        const originalType = canvas.getAttribute('data-chart-original-type') || chartType;
        const chartDataAttr = canvas.getAttribute('data-chart-data');
        const chartSeriesAttr = canvas.getAttribute('data-chart-series');
        const chartLabelsAttr = canvas.getAttribute('data-chart-labels');
        const seriesTypesAttr = canvas.getAttribute('data-chart-series-types');
        const rawChartDataAttr = canvas.getAttribute('data-chart-raw');
        
        // Si hay datos JSON raw, usarlos directamente
        if (rawChartDataAttr) {
            try {
                const decodedJSON = this.decodeHTMLEntities(rawChartDataAttr);
                return JSON.parse(decodedJSON);
            } catch (error) {
                console.error('[Charts] Error parsing raw chart JSON:', error);
            }
        }
        
        // Procesar datos tradicionales
        try {
            const chartData = JSON.parse(chartDataAttr || '[]');
            const chartSeries = JSON.parse(chartSeriesAttr || '[]');
            const chartLabels = JSON.parse(chartLabelsAttr || '[]');
            const seriesTypes = JSON.parse(seriesTypesAttr || '[]');
            
            return this.buildConfig(chartType, originalType, chartData, chartSeries, chartLabels, seriesTypes);
        } catch (error) {
            console.error('[Charts] Error parsing chart data attributes:', error);
            return null;
        }
    },
    
    buildConfig: function(chartType, originalType, data, series, labels, seriesTypes) {
        // Process data for Chart.js
        const chartLabels = labels.length > 0 ? labels : (data.length > 0 ? data.map(row => row[0]) : []);
        const datasets = this.buildDatasets(data, series, seriesTypes, originalType);
        
        // For combo charts, use 'bar' as base type but allow specific types per dataset
        const finalChartType = originalType === 'combo' ? 'bar' : chartType;
        
        // Configure scales - for combo charts we need multiple scales
        const scales = {
            y: {
                beginAtZero: true,
                position: 'left'
            }
        };
        
        // If combo chart with multiple series, add secondary Y scale
        if (originalType === 'combo' && series.length > 1) {
            scales.y1 = {
                type: 'linear',
                display: true,
                position: 'right',
                beginAtZero: true,
                grid: {
                    drawOnChartArea: false
                }
            };
        }
        
        return {
            type: finalChartType,
            data: {
                labels: chartLabels,
                datasets: datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: scales,
                plugins: {
                    legend: {
                        position: 'top',
                    },
                    tooltip: {
                        mode: 'index',
                        intersect: false,
                    }
                }
            }
        };
    },
    
    buildDatasets: function(data, series, seriesTypes, originalType) {
        const datasets = [];
        
        if (series.length > 0) {
            series.forEach((seriesName, index) => {
                const seriesData = data.map(row => row[index + 1] || 0);
                
                const dataset = {
                    label: seriesName,
                    data: seriesData,
                    backgroundColor: withAlpha(categoricalColor(index), ALPHA_FRACTION),
                    borderColor: categoricalColor(index),
                    borderWidth: 2,
                    tension: 0.1
                };
                
                // For combo charts, add specific type and Y scale
                if (originalType === 'combo' && seriesTypes.length > 0) {
                    dataset.type = seriesTypes[index];
                    // Assign second Y scale to datasets that are not the first
                    if (index > 0) {
                        dataset.yAxisID = 'y1';
                    }
                }
                
                datasets.push(dataset);
            });
        } else {
            // Default dataset
            datasets.push({
                label: 'Dataset 1',
                data: data.map(row => row[1] || 0),
                backgroundColor: withAlpha(categoricalColor(0), ALPHA_FRACTION),
                borderColor: categoricalColor(0),
                borderWidth: 2,
                tension: 0.1
            });
        }
        
        return datasets;
    },
    
    destroyChartsNotInSlide: function(activeSlideElement) {
        const destroyList = [];
        
        chartRegistry.forEach((chart, chartId) => {
            const canvas = document.getElementById(chartId);
            if (!canvas || !activeSlideElement.contains(canvas)) {
                destroyList.push(chartId);
            }
        });
        
        destroyList.forEach(chartId => {
            const chart = chartRegistry.get(chartId);
            if (chart) {
                try {
                    chart.destroy();
                    chartRegistry.delete(chartId);
                } catch (error) {
                    console.error(`[Charts] Error destroying chart ${chartId}:`, error);
                }
            }
        });
    },
    
    decodeHTMLEntities: function(str) {
        const entities = {
            '&#34;': '"', '&quot;': '"', '&#39;': "'", '&apos;': "'",
            '&lt;': '<', '&gt;': '>', '&amp;': '&'
        };
        
        let decoded = str;
        for (let entity in entities) {
            decoded = decoded.replace(new RegExp(entity, 'g'), entities[entity]);
        }
        return decoded;
    },
    
    // Public API methods
    getActiveChartsCount: function() {
        return chartRegistry.size;
    },
    
    destroyAllCharts: function() {
        const chartIds = Array.from(chartRegistry.keys());
        chartIds.forEach(chartId => {
            const chart = chartRegistry.get(chartId);
            if (chart) {
                chart.destroy();
            }
        });
        chartRegistry.clear();
    }
};

// Register with SlideLang namespace
if (typeof window !== 'undefined') {
    // Ensure SlideLang namespace exists
    if (!window.SlideLang) {
        window.SlideLang = {};
    }
}

// Auto-registro siguiendo el patrón estándar
(function() {
    function registerModule() {
        if (typeof window !== 'undefined' && window.SlideLang) {
            // Register as charts module in SlideLang system
            SlideLang.registerModule('charts', SlideLangCharts);
            
            // Only initialize if not already done
            if (!SlideLangCharts.initialized) {
                SlideLangCharts.init();
            }
        } else {
            setTimeout(registerModule, 50);
        }
    }
    
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', registerModule);
    } else {
        registerModule();
    }
})();

// Export for global access
window.SlideLangCharts = SlideLangCharts;
