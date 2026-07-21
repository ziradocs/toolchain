// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"embed"
	"log"
)

//go:embed assets/js/modules/maps.js
var mapsModuleFS embed.FS

// GetMapsJS retorna el módulo JavaScript para Leaflet
// Ahora carga desde el archivo externo con fallback para compatibilidad
func GetMapsJS() string {
	// Intentar cargar el módulo externo
	if content, err := mapsModuleFS.ReadFile("assets/js/modules/maps.js"); err == nil {
		return string(content)
	} else {
		log.Printf("Warning: Could not load external maps module, using fallback: %v", err)
	}

	// Fallback para compatibilidad si el archivo externo no se puede cargar
	return `// === MAPS MODULE (FALLBACK) ===
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
        
        if (typeof console !== 'undefined' && console.log) {
            console.log('[SlideLang.maps] Module initialized and subscribed to navigation events');
        }
    },
    
    subscribeToEvents: function() {
        // Listen to the standard navigation event
        document.addEventListener('slidelang:slideChanged', (event) => {
            this.handleSlideChange(event.detail);
        });
    },
    
    handleSlideChange: function(slideInfo) {
        if (typeof console !== 'undefined' && console.log) {
            console.log('[SlideLang.maps] Slide changed, processing maps for slide:', slideInfo.slideId);
        }
        
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
    
    // Helper function to decode HTML entities
    decodeHTMLEntities: function(text) {
        if (!text) return text;
        
        // Use DOMParser for safer HTML entity decoding
        const doc = new DOMParser().parseFromString(text, 'text/html');
        const decoded = doc.documentElement.textContent;
        
        // If DOMParser fails, fallback to textarea method
        if (!decoded) {
            const textArea = document.createElement('textarea');
            textArea.innerHTML = text;
            return textArea.value;
        }
        
        return decoded;
    },
    
    createMap: function(container, index) {
        if (typeof console !== 'undefined' && console.log) {
            console.log('[SlideLang.maps] Creating map for container:', container.id);
        }
        
        const mapType = container.getAttribute('data-map-type');
        const zoom = parseInt(container.getAttribute('data-map-zoom')) || 2;
        const markersAttr = container.getAttribute('data-map-markers');
        
        let markers = [];
        try {
            // Decode HTML entities before parsing JSON
            const decodedMarkersAttr = this.decodeHTMLEntities(markersAttr);
            markers = JSON.parse(decodedMarkersAttr || '[]');
        } catch (e) {
            if (typeof console !== 'undefined' && console.warn) {
                console.warn('[SlideLang.maps] Error parsing markers, using fallback:', e);
            }
            // Marcadores de fallback
            markers = [
                {lat: 40.7128, lng: -74.0060, label: "New York"},
                {lat: 51.5074, lng: -0.1278, label: "London"}
            ];
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
        
        if (typeof console !== 'undefined' && console.log) {
            console.log('[SlideLang.maps] Map created successfully:', container.id);
        }
        
        // Force a resize after a short delay to ensure proper rendering
        setTimeout(() => {
            map.invalidateSize();
        }, 100);
    },
    
    calculateCenter: function(markers) {
        let centerLat = 20, centerLng = 0;
        
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
        
        markers.forEach(marker => {
            const lat = marker.lat || marker.Lat;
            const lng = marker.lng || marker.Lng;
            const label = marker.label || marker.Label;
            const value = marker.value || marker.Value;
            const color = marker.color || marker.Color;
            const size = marker.size || marker.Size;
            const details = marker.details || marker.Details;
            const icon = marker.icon || marker.Icon;
            
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
                
                // Crear popup con información completa
                if (label || details || value) {
                    let popupContent = '';
                    
                    if (label) {
                        popupContent += '<h4 style="margin: 0 0 8px 0; color: ' + (color || '#333') + ';">' + label + '</h4>';
                    }
                    
                    if (value) {
                        popupContent += '<div style="margin: 4px 0; font-weight: bold; font-size: 16px;">Valor: ' + this.formatValue(value) + '</div>';
                    }
                    
                    if (details) {
                        popupContent += '<div style="margin: 4px 0; color: #666; font-size: 14px;">' + details + '</div>';
                    }
                    
                    leafletMarker.bindPopup(popupContent);
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
        if (typeof console !== 'undefined' && console.log) {
            console.log('[SlideLang.maps] Refreshing', this.mapInstances.length, 'map instances');
        }
        
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
        if (typeof console !== 'undefined' && console.log) {
            console.log('[SlideLang.maps] Refreshing map for slide:', slideId);
        }
        
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
                    
                    if (typeof console !== 'undefined' && console.log) {
                        console.log('[SlideLang.maps] Map refreshed successfully:', mapId);
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
`
}
