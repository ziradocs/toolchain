// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// === MAPS MODULE ===
// SlideLang Maps Module - External JavaScript Implementation
// This module renders maps exclusively from metadata JSON, following the unified architecture pattern

SlideLang.maps = {
    initialized: false,
    mapInstances: [],
    processedMaps: new Set(),
    
    init: function() {
        if (typeof L === 'undefined') {
            if (typeof console !== 'undefined' && console.warn) {
                console.warn('[SlideLang.maps] Leaflet library not found');
            }
            return;
        }
        
        if (this.initialized) {
            return;
        }
        
        this.initialized = true;
        
        // Subscribe to navigation events
        this.subscribeToEvents();
        
        // Process maps on current slide
        this.processCurrentSlide();
    },
    
    subscribeToEvents: function() {
        // Listen to the standard navigation event
        document.addEventListener('slidelang:slideChanged', (event) => {
            this.handleSlideChange(event.detail);
        });
    },
    
    handleSlideChange: function(slideInfo) {
        // Process maps on the new active slide
        this.processCurrentSlide();
        
        // Refresh existing maps to ensure proper rendering
        this.refreshMaps();
    },
    
    processCurrentSlide: function() {
        // Find the current active slide
        const activeSlide = document.querySelector('.slidelang-slide.slidelang-active');
        if (!activeSlide) {
            return;
        }
        
        // Find map elements in the active slide
        const mapContainers = activeSlide.querySelectorAll('.slidelang-map-container');
        
        if (mapContainers.length === 0) {
            return;
        }
        
        // Process each map container
        mapContainers.forEach((container, index) => {
            const mapId = container.id || activeSlide.id + '-map-' + index;
            
            // Skip if already processed
            if (this.processedMaps.has(mapId)) {
                return;
            }
            
            container.id = mapId;
            this.createMap(container, index);
            this.processedMaps.add(mapId);
        });
    },
    
    createMap: function(container, index) {
        // Read from metadata JSON only
        const metadata = SlideLang.metadata || {};
        if (!metadata.maps) {
            if (typeof console !== 'undefined' && console.warn) {
                console.warn('[SlideLang.maps] No maps metadata found');
            }
            return;
        }

        // Find map data for this container
        const mapData = metadata.maps.find(map => map.id === container.id);
        if (!mapData) {
            if (typeof console !== 'undefined' && console.warn) {
                console.warn('[SlideLang.maps] No map data found for container:', container.id);
            }
            return;
        }

        const mapType = mapData.mapType || 'world';
        const zoom = mapData.zoom || 2;
        let markers = mapData.markers || [];

        // Parse markers if they come as JSON string (from Go template)
        if (typeof markers === 'string') {
            try {
                markers = JSON.parse(markers);
            } catch (e) {
                console.warn('[SlideLang.maps] Failed to parse markers JSON:', e);
                markers = [];
            }
        }

        // Calcular centro del mapa basado en marcadores
        const center = this.calculateCenter(markers);
        
        const map = L.map(container.id).setView([center.lat, center.lng], zoom);
        
        // Agregar tile layer
        this.addTileLayer(map, mapType);
        
        // Agregar marcadores
        const leafletMarkers = this.addMarkers(map, markers);
        
        // Ajustar vista si hay múltiples marcadores
        if (markers.length > 1 && leafletMarkers.length > 1) {
            const group = new L.featureGroup(leafletMarkers);
            map.fitBounds(group.getBounds().pad(0.1));
        }
        
        // Guardar instancia del mapa
        this.mapInstances.push({
            id: container.id,
            map: map,
            markers: leafletMarkers
        });
        
        // Force a resize after a short delay to ensure proper rendering
        setTimeout(() => {
            map.invalidateSize();
        }, 100);
    },
    
    calculateCenter: function(markers) {
        let centerLat = 20, centerLng = 0;
        
        // Ensure markers is an array
        if (!Array.isArray(markers)) {
            console.warn('[SlideLang.maps] Markers is not an array:', markers);
            return { lat: centerLat, lng: centerLng };
        }
        
        if (markers.length > 0) {
            centerLat = markers.reduce((sum, m) => sum + (m.lat || m.Lat), 0) / markers.length;
            centerLng = markers.reduce((sum, m) => sum + (m.lng || m.Lng), 0) / markers.length;
        }
        
        return { lat: centerLat, lng: centerLng };
    },
    
    addTileLayer: function(map, mapType) {
        let tileLayerUrl = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
        let attribution = '© OpenStreetMap contributors';
        
        switch (mapType) {
            case 'satellite':
                tileLayerUrl = 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}';
                attribution = '© Esri';
                break;
            case 'terrain':
                tileLayerUrl = 'https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png';
                attribution = '© OpenTopoMap contributors';
                break;
            case 'dark':
                tileLayerUrl = 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png';
                attribution = '© CARTO';
                break;
            default:
                // Usar OpenStreetMap por defecto
                break;
        }
        
        L.tileLayer(tileLayerUrl, {
            attribution: attribution,
            maxZoom: 18
        }).addTo(map);
    },
    
    addMarkers: function(map, markers) {
        const leafletMarkers = [];
        
        // Ensure markers is an array
        if (!Array.isArray(markers)) {
            console.warn('[SlideLang.maps] Markers is not an array, skipping marker addition');
            return leafletMarkers;
        }
        
        // isValidColor: allowlist estricta de hex (#rgb..#rrggbbaa) o nombre de
        // color CSS conocido (debe reflejar exactamente cssNamedColors en
        // slidelang-core/renderer/sanitizer.go). El servidor ya valida/escapa
        // label/details/color (renderer.SanitizeColor, renderer.EscapeHTML),
        // pero el color se revalida aquí porque se interpola en un atributo
        // style vía divIcon (ver docs/SECURITY_AUDIT_2026-07.md, AL-7).
        const hexColorPattern = /^#[0-9a-fA-F]{3,8}$/;
        const cssNamedColors = new Set([
            'black', 'silver', 'gray', 'white', 'maroon',
            'red', 'purple', 'fuchsia', 'green', 'lime',
            'olive', 'yellow', 'navy', 'blue', 'teal',
            'aqua', 'orange', 'pink', 'brown', 'cyan',
            'magenta', 'gold', 'indigo', 'violet', 'coral',
            'salmon', 'khaki', 'crimson', 'turquoise', 'orchid',
            'tomato', 'chocolate', 'darkgreen', 'darkblue',
            'darkred', 'lightblue', 'lightgreen', 'lightgray',
            'lightgrey', 'darkgray', 'darkgrey', 'transparent',
        ]);
        const isValidColor = (c) => typeof c === 'string' &&
            (hexColorPattern.test(c) || cssNamedColors.has(c.toLowerCase()));

        markers.forEach(marker => {
            const lat = marker.lat || marker.Lat;
            const lng = marker.lng || marker.Lng;
            const label = marker.label || marker.Label;
            const value = marker.value || marker.Value;
            const rawColor = marker.color || marker.Color;
            const size = marker.size || marker.Size;
            const details = marker.details || marker.Details;
            const icon = marker.icon || marker.Icon;
            const color = isValidColor(rawColor) ? rawColor : null;

            if (lat && lng) {
                let leafletMarker;

                // Crear icono personalizado basado en color y tamaño
                if (color || size) {
                    const iconSize = this.getIconSize(size);
                    const iconColor = color || '#3388ff';

                    const customIcon = L.divIcon({
                        className: 'custom-marker',
                        html: '<div style="' +
                            'background-color: ' + iconColor + ';' +
                            'width: ' + iconSize + 'px;' +
                            'height: ' + iconSize + 'px;' +
                            'border-radius: 50%;' +
                            'border: 3px solid white;' +
                            'box-shadow: 0 2px 6px rgba(0,0,0,0.3);' +
                            'display: flex;' +
                            'align-items: center;' +
                            'justify-content: center;' +
                            'color: white;' +
                            'font-weight: bold;' +
                            'font-size: ' + (iconSize * 0.3) + 'px;' +
                        '">' + (value ? this.formatValue(value) : '') + '</div>',
                        iconSize: [iconSize, iconSize],
                        iconAnchor: [iconSize/2, iconSize/2],
                        popupAnchor: [0, -iconSize/2]
                    });
                    leafletMarker = L.marker([lat, lng], {icon: customIcon}).addTo(map);
                } else if (icon) {
                    // Custom icon from URL
                    const customIcon = L.icon({
                        iconUrl: icon,
                        iconSize: [32, 32],
                        iconAnchor: [16, 32],
                        popupAnchor: [0, -32]
                    });
                    leafletMarker = L.marker([lat, lng], {icon: customIcon}).addTo(map);
                } else {
                    // Default marker
                    leafletMarker = L.marker([lat, lng]).addTo(map);
                }

                // Crear popup con información completa. Se construye con nodos
                // DOM (textContent) en vez de concatenar HTML: aunque el
                // servidor ya escapa label/details, esto evita depender de esa
                // única capa de defensa (ver AL-7).
                if (label || details || value) {
                    const popupContainer = document.createElement('div');

                    if (label) {
                        const labelEl = document.createElement('h4');
                        labelEl.style.margin = '0 0 8px 0';
                        labelEl.style.color = color || '#333';
                        labelEl.textContent = label;
                        popupContainer.appendChild(labelEl);
                    }

                    if (value) {
                        const valueEl = document.createElement('div');
                        valueEl.style.margin = '4px 0';
                        valueEl.style.fontWeight = 'bold';
                        valueEl.style.fontSize = '16px';
                        valueEl.textContent = 'Valor: ' + this.formatValue(value);
                        popupContainer.appendChild(valueEl);
                    }

                    if (details) {
                        const detailsEl = document.createElement('div');
                        detailsEl.style.margin = '4px 0';
                        detailsEl.style.color = '#666';
                        detailsEl.style.fontSize = '14px';
                        detailsEl.textContent = details;
                        popupContainer.appendChild(detailsEl);
                    }

                    leafletMarker.bindPopup(popupContainer);
                }

                leafletMarkers.push(leafletMarker);
            }
        });
        
        return leafletMarkers;
    },
    
    getIconSize: function(size) {
        switch(size) {
            case 'small': return 20;
            case 'medium': return 30;
            case 'large': return 40;
            case 'xlarge': return 50;
            default: return 25;
        }
    },
    
    formatValue: function(value) {
        if (value >= 1000) {
            return (value / 1000).toFixed(1) + 'K';
        }
        return value.toString();
    },
    
    // Método para refrescar mapas cuando cambia de slide
    refreshMaps: function() {
        this.mapInstances.forEach(instance => {
            if (instance.map) {
                instance.map.invalidateSize();
                // Force marker re-positioning by removing and re-adding them
                instance.markers.forEach(marker => {
                    const latLng = marker.getLatLng();
                    marker.setLatLng(latLng);
                });
            }
        });
    },
    
    // Method to refresh specific map when slide becomes visible
    refreshMapOnSlideChange: function(slideId) {
        const slide = document.getElementById(slideId);
        if (!slide) return;
        
        const mapContainer = slide.querySelector('.slidelang-map-container');
        if (!mapContainer) return;
        
        const mapId = mapContainer.id;
        const mapInstance = this.mapInstances.find(inst => inst.id === mapId);
        
        if (mapInstance && mapInstance.map) {
            // Force map to recalculate size and re-render
            setTimeout(() => {
                try {
                    mapInstance.map.invalidateSize(true);
                    
                    // Force marker re-positioning
                    mapInstance.markers.forEach((marker, index) => {
                        const latLng = marker.getLatLng();
                        marker.setLatLng(latLng);
                    });
                    
                    // Re-fit bounds if multiple markers
                    if (mapInstance.markers.length > 1) {
                        const group = new L.featureGroup(mapInstance.markers);
                        mapInstance.map.fitBounds(group.getBounds().pad(0.1));
                    }
                } catch (e) {
                    if (typeof console !== 'undefined' && console.error) {
                        console.error('[SlideLang.maps] Error refreshing map:', e);
                    }
                }
            }, 100);
        }
    }
};

// Auto-register maps module
// This ensures the module is automatically registered and initialized when loaded
(function() {
    function registerMaps() {
        if (typeof SlideLang !== 'undefined' && SlideLang.registerModule) {
            SlideLang.registerModule('maps', SlideLang.maps);
            SlideLang.maps.init();
        } else {
            setTimeout(registerMaps, 50);
        }
    }
    
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', registerMaps);
    } else {
        registerMaps();
    }
})();
