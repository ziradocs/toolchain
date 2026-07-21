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
                            defaultColors[segmentIndex % defaultColors.length] + '80'
                        );
                    }
                    if (!dataset.borderColor) {
                        dataset.borderColor = dataset.data.map((_, segmentIndex) => 
                            defaultColors[segmentIndex % defaultColors.length]
                        );
                    }
                } else {
                    // Para otros tipos de charts, usar un color por dataset
                    if (!dataset.backgroundColor) {
                        dataset.backgroundColor = defaultColors[index % defaultColors.length] + '80';
                    }
                    if (!dataset.borderColor) {
                        dataset.borderColor = defaultColors[index % defaultColors.length];
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
                    backgroundColor: defaultColors[index % defaultColors.length] + '80',
                    borderColor: defaultColors[index % defaultColors.length],
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
                backgroundColor: defaultColors[0] + '80',
                borderColor: defaultColors[0],
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
