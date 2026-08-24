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
// defaultColors, and a token isn't guaranteed to be #RRGGBB or opaque — a
// theme author can declare chart-cat-1 as #ff000020, rgba(255,0,0,0.1), or
// any other alpha-carrying form for a deliberate translucent fill.
//
// A code-review-flagged regression: an earlier version of this function
// always overwrote alpha with ALPHA_FRACTION regardless of whether the
// color already declared its own — #ff000020 became #ff000080,
// rgba(r,g,b,0.1) became rgba(r,g,b,0.5019…). Worse, it silently
// contradicted an invariant native_chart.go's chartColorFromCSS documents
// explicitly (established by a PR #224 review finding): a chart-cat-*
// color's alpha "is meaningful and preserved as-is" in the native
// (PDF/PPTX) render path. Overwriting it here reintroduced exactly the
// browser-vs-native divergence that finding closed.
//
// Fixed rule: NEVER compose, NEVER overwrite. A color that already
// declares alpha (4/8-digit hex, an explicit 4th rgb()/rgba() or
// hsl()/hsla() component, or anything using the modern '/' alpha syntax)
// is returned untouched. Only an opaque color gets ALPHA_FRACTION applied
// — which is exactly what defaultColors always was, so this is still
// byte-for-byte behavior-preserving for every theme that declares no
// chart-cat-* tokens (i.e. every theme in the repo today).
const ALPHA_FRACTION = 128 / 255;

function withAlpha(color, alphaFraction) {
    if (typeof color !== 'string') {
        return color;
    }
    const trimmed = color.trim();

    const hexMatch = trimmed.match(/^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/);
    if (hexMatch) {
        const hex = hexMatch[1];
        if (hex.length === 4 || hex.length === 8) {
            return trimmed; // already carries its own alpha nibble/byte
        }
        const expanded = hex.length === 3 ? hex.split('').map((c) => c + c).join('') : hex;
        const alphaHex = Math.round(alphaFraction * 255).toString(16).padStart(2, '0');
        return '#' + expanded + alphaHex;
    }

    // Component regexes deliberately don't validate each component's own
    // syntax (that's IsValidMapColor/IsValidMermaidColor's job upstream,
    // for the token families that go through one) — only arity, to detect
    // whether a 4th (alpha) component is already present.
    const rgbMatch = trimmed.match(/^rgba?\(\s*([^,]+?)\s*,\s*([^,]+?)\s*,\s*([^,]+?)\s*(,\s*[^,]+?\s*)?\)$/i);
    if (rgbMatch) {
        if (rgbMatch[4] !== undefined) {
            return trimmed; // rgb()/rgba() with 4 components already has alpha
        }
        return 'rgba(' + rgbMatch[1] + ', ' + rgbMatch[2] + ', ' + rgbMatch[3] + ', ' + alphaFraction + ')';
    }

    const hslMatch = trimmed.match(/^hsla?\(\s*([^,]+?)\s*,\s*([^,]+?)\s*,\s*([^,]+?)\s*(,\s*[^,]+?\s*)?\)$/i);
    if (hslMatch) {
        if (hslMatch[4] !== undefined) {
            return trimmed;
        }
        return 'hsla(' + hslMatch[1] + ', ' + hslMatch[2] + ', ' + hslMatch[3] + ', ' + alphaFraction + ')';
    }

    if (trimmed.indexOf('/') !== -1) {
        // Modern space-separated syntax (e.g. "rgb(255 0 0 / 50%)") can
        // carry its own alpha after '/' — never assume it's opaque.
        return trimmed;
    }

    // Named color or anything else Chart.js/canvas would otherwise accept
    // as-is: color-mix is the only format-agnostic way to apply a DEFAULT
    // alpha without a full CSS color parser, and has been supported in
    // every Chart.js-capable browser (and the Chromium version this
    // toolchain's offline/PDF path embeds) since 2023.
    return 'color-mix(in srgb, ' + trimmed + ' ' + Math.round(alphaFraction * 100) + '%, transparent)';
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

// CHART_SCALE_DIMENSIONS (H4/H5 fix, a code-review-flagged gap): the set
// of scale dimensions Chart.js itself creates implicitly for a chart type
// when options.scales doesn't declare them — bar/line/scatter/bubble (and,
// via buildConfig's originalType==='combo' -> chartType 'bar' mapping,
// combo) get 'x'+'y'; radar/polarArea get the radial 'r' scale. Before
// this fix only the cartesian set was covered, so a <<chart: radar>> never
// received chart-grid/chart-axis at all. A type absent from this map
// (pie, doughnut, treemap, or anything unrecognized) gets NO
// materialization — correct for the first two (applyThemeColors deletes
// options.scales for them above) and the safe default for anything else:
// not applying a token is the harmless pre-existing behavior, a spurious
// axis Chart.js wouldn't have drawn is a visual regression.
const CHART_SCALE_DIMENSIONS = {
    bar: ['x', 'y'],
    line: ['x', 'y'],
    scatter: ['x', 'y'],
    bubble: ['x', 'y'],
    radar: ['r'],
    polarArea: ['r'],
};

// requiredScaleIds returns the literal scale keys Chart.js itself will
// require (and, if missing, silently create with its OWN untamed
// defaults) for themedConfig — the ONLY keys ensureThemeableScales may
// materialize.
//
// A code-review-flagged correctness bug lived in an earlier version of
// this function: it computed a scale's "dimension" by reading its
// declared `axis` property (or a dataset's xAxisID/yAxisID reference) and
// treated any EXISTING scale matching a required dimension as already
// "covering" it — so a custom-named scale like "revenue": {axis:'y'}
// was treated as satisfying the chart's 'y' requirement, and no literal
// 'y' scale got materialized. That is NOT how Chart.js 4.x's own
// mergeScaleConfig (core.controller.js) actually decides which scales to
// create: it materializes a default scale under the LITERAL id 'x'/'y'
// for every cartesian dataset unless that SPECIFIC dataset overrides it
// via xAxisID/yAxisID — a differently-named scale elsewhere in options,
// even with a matching `axis`, does not suppress that. Confirmed
// empirically against Chart.js 4.5.1 (a code-review finding): a bar chart
// with `scales: {revenue: {axis:'y'}, x: {...}}` and a dataset with no
// yAxisID still gets Chart.js's own default 'y' scale rendered ALONGSIDE
// "revenue" — untamed, with Chart.js's own default grid/tick colors,
// because nothing told Chart.js to skip it.
//
// So the correct rule mirrors Chart.js's own per-dataset computation
// directly: for cartesian types, the required id is
// `dataset.xAxisID || 'x'` / `dataset.yAxisID || 'y'`, evaluated PER
// DATASET (not deduplicated by "dimension" across the whole config). For
// radial types (radar/polarArea) Chart.js has no per-dataset override —
// the radial scale is always keyed literally 'r'.
function requiredScaleIds(themedConfig) {
    const dimensions = CHART_SCALE_DIMENSIONS[themedConfig.type];
    if (!dimensions || !dimensions.length) {
        return [];
    }
    if (dimensions.length === 1) {
        return dimensions.slice(); // radial: no per-dataset override, always literally 'r'
    }

    const datasets = (themedConfig.data && themedConfig.data.datasets) || [];
    const ids = new Set();
    if (!datasets.length) {
        dimensions.forEach((d) => ids.add(d));
        return Array.from(ids);
    }
    datasets.forEach((dataset) => {
        if (!dataset || typeof dataset !== 'object') {
            dimensions.forEach((d) => ids.add(d));
            return;
        }
        ids.add(typeof dataset.xAxisID === 'string' && dataset.xAxisID ? dataset.xAxisID : 'x');
        ids.add(typeof dataset.yAxisID === 'string' && dataset.yAxisID ? dataset.yAxisID : 'y');
    });
    return Array.from(ids);
}

// ensureThemeableScales materializes any of requiredScaleIds' keys that
// options.scales doesn't already declare — mirroring exactly what
// Chart.js would create by default, so that scale gets themed too instead
// of rendering with Chart.js's own untamed defaults. Existing scales
// (required or not — e.g. an author's own custom-named "revenue") are
// left as-is here; the caller's coloring loop themes every key present,
// not just the ones this function adds.
function ensureThemeableScales(themedConfig) {
    const requiredIds = requiredScaleIds(themedConfig);
    if (!requiredIds.length) {
        return themedConfig.options.scales || null;
    }

    const existing = themedConfig.options.scales || {};
    let changed = false;
    requiredIds.forEach((id) => {
        if (!existing[id]) {
            existing[id] = {};
            changed = true;
        }
    });
    if (changed) {
        themedConfig.options.scales = existing;
    }
    return existing;
}

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
        const scales = ensureThemeableScales(themedConfig);
        // Radial charts (radar/polarArea) have exactly one scale, always
        // keyed literally 'r' — Chart.js has no per-dataset override for
        // it, unlike x/y. isRadial is computed once from the chart TYPE,
        // not by inspecting each scale key, so it can't be fooled by a
        // cartesian chart that happens to name a scale "r" for unrelated
        // reasons.
        const radialDimensions = CHART_SCALE_DIMENSIONS[themedConfig.type];
        const isRadial = !!radialDimensions && radialDimensions.length === 1 && radialDimensions[0] === 'r';
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
                // Radial scales draw two more theme-relevant elements a
                // cartesian scale doesn't have: the spokes (angleLines)
                // and the category labels around the circle
                // (pointLabels). Without these a radar chart's
                // concentric-circle grid and numeric ticks get themed but
                // the spokes and category labels stay Chart.js-default —
                // visibly half-done.
                if (isRadial && key === 'r') {
                    if (tokens['chart-grid']) {
                        scale.angleLines = scale.angleLines || {};
                        if (scale.angleLines.color === undefined) {
                            scale.angleLines.color = tokens['chart-grid'];
                        }
                    }
                    if (tokens['chart-axis']) {
                        scale.pointLabels = scale.pointLabels || {};
                        if (scale.pointLabels.color === undefined) {
                            scale.pointLabels.color = tokens['chart-axis'];
                        }
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
