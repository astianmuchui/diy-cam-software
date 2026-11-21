
(function () {
    'use strict';

    let map;
    let densityLayer;
    let confidenceLayer;
    let sessionCoverageLayer;
    let geocodeMarker;
    let opencageKey = '';
    let layerControl;

    const SESSION_COLORS = [
        '#3b82f6', '#ef4444', '#10b981', '#f97316', '#8b5cf6',
        '#06b6d4', '#ec4899', '#eab308', '#14b8a6', '#f43f5e',
    ];

    const tileLayers = {
        'Google Maps Style': L.tileLayer(
            'https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}{r}.png',
            {
                attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
                subdomains: 'abcd',
                maxZoom: 19,
            }
        ),
        'Satellite': L.tileLayer(
            'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
            {
                attribution: 'Tiles &copy; Esri',
                maxZoom: 18,
            }
        ),
        'Dark': L.tileLayer(
            'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
            {
                attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
                subdomains: 'abcd',
                maxZoom: 19,
            }
        ),
        'Terrain': L.tileLayer(
            'https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png',
            {
                attribution: 'Map data: &copy; OSM contributors | OpenTopoMap',
                subdomains: 'abc',
                maxZoom: 17,
            }
        ),
        'OpenStreetMap': L.tileLayer(
            'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
            {
                attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
                maxZoom: 19,
            }
        ),
    };

    function rebuildLayerControl() {
        if (layerControl) layerControl.remove();
        const overlays = {};
        if (densityLayer)         overlays['Density Circles']      = densityLayer;
        if (confidenceLayer)      overlays['Confidence Hotspots']  = confidenceLayer;
        if (sessionCoverageLayer) overlays['Session Areas']        = sessionCoverageLayer;
        layerControl = L.control.layers(tileLayers, overlays, { position: 'topright' }).addTo(map);
    }

    function _cross(O, A, B) {
        return (A[0] - O[0]) * (B[1] - O[1]) - (A[1] - O[1]) * (B[0] - O[0]);
    }
    function convexHull(points) {
        if (points.length < 3) return points;
        const pts = points.slice().sort(function (a, b) {
            return a[0] !== b[0] ? a[0] - b[0] : a[1] - b[1];
        });
        const n = pts.length;
        const hull = [];
        for (let i = 0; i < n; i++) {
            while (hull.length >= 2 && _cross(hull[hull.length - 2], hull[hull.length - 1], pts[i]) <= 0)
                hull.pop();
            hull.push(pts[i]);
        }
        const lo = hull.length + 1;
        for (let i = n - 2; i >= 0; i--) {
            while (hull.length >= lo && _cross(hull[hull.length - 2], hull[hull.length - 1], pts[i]) <= 0)
                hull.pop();
            hull.push(pts[i]);
        }
        hull.pop();
        return hull;
    }

    function densityColor(count) {
        if (count > 50) return '#db2777';
        if (count > 20) return '#7c3aed';
        if (count > 10) return '#2563eb';
        if (count > 3)  return '#0891b2';
        return '#6366f1';
    }

    function densityRadius(totalFish) {
        const base = Math.sqrt(totalFish || 1) * 3000;
        return Math.max(1500, Math.min(base, 25000));
    }

    function confidenceHotspotColor(conf) {
        if (conf >= 0.75) return '#ef4444';
        if (conf >= 0.55) return '#f97316';
        if (conf >= 0.35) return '#eab308';
        return '#10b981';
    }

    function loadDensity(days) {
        const statusEl = document.getElementById('map-status');
        if (statusEl) statusEl.textContent = 'Loading data…';

        if (densityLayer)    map.removeLayer(densityLayer);
        if (confidenceLayer) map.removeLayer(confidenceLayer);

        densityLayer    = L.layerGroup();
        confidenceLayer = L.layerGroup();

        fetch('/api/v1/analytics/density-map?days=' + days)
            .then(function (r) { return r.json(); })
            .then(function (data) {
                if (!Array.isArray(data) || data.length === 0) {
                    if (statusEl) statusEl.textContent = 'No detection data for this period.';
                    rebuildLayerControl();
                    return;
                }

                data.forEach(function (pt) {
                    const conf = pt.avg_confidence || 0;
                    const popupHtml =
                        '<div style="font-family:Inter,sans-serif;font-size:13px;min-width:170px">' +
                        '<b style="color:#1e293b">Signal Hotspot</b>' +
                        '<hr style="margin:6px 0;border-color:#e2e8f0">' +
                        '<table style="width:100%;border-collapse:collapse">' +
                        '<tr><td style="color:#64748b;padding:2px 0">Lat/Lon</td><td style="text-align:right">' + pt.lat.toFixed(4) + ', ' + pt.lon.toFixed(4) + '</td></tr>' +
                        '<tr><td style="color:#64748b;padding:2px 0">Detections</td><td style="text-align:right;font-weight:700;color:#2563eb">' + pt.count + '</td></tr>' +
                        '<tr><td style="color:#64748b;padding:2px 0">Total fish</td><td style="text-align:right;font-weight:700;color:#db2777">' + pt.total_fish + '</td></tr>' +
                        '<tr><td style="color:#64748b;padding:2px 0">Avg confidence</td><td style="text-align:right;font-weight:700;color:' + confidenceHotspotColor(conf) + '">' + (conf * 100).toFixed(1) + '%</td></tr>' +
                        '</table></div>';

                    L.circle([pt.lat, pt.lon], {
                        color: densityColor(pt.count),
                        fillColor: densityColor(pt.count),
                        fillOpacity: 0.45,
                        weight: 1.5,
                        radius: densityRadius(pt.total_fish),
                    }).bindPopup(popupHtml).addTo(densityLayer);

                    L.circleMarker([pt.lat, pt.lon], {
                        radius: 8 + Math.round(conf * 10),
                        fillColor: confidenceHotspotColor(conf),
                        color: '#fff',
                        weight: 2,
                        fillOpacity: 0.85,
                    }).bindPopup(popupHtml).addTo(confidenceLayer);
                });

                densityLayer.addTo(map);
                rebuildLayerControl();
                if (statusEl) statusEl.textContent = data.length + ' hotspot(s) loaded.';
            })
            .catch(function () {
                if (statusEl) statusEl.textContent = 'Failed to load data.';
            });
    }

    function loadSessionCoverage() {
        if (sessionCoverageLayer) map.removeLayer(sessionCoverageLayer);
        sessionCoverageLayer = L.layerGroup();

        fetch('/api/v1/sessions/map-coverage')
            .then(function (r) { return r.json(); })
            .then(function (json) {
                const sessions = json.data || [];
                if (sessions.length === 0) { rebuildLayerControl(); return; }

                sessions.forEach(function (s, idx) {
                    if (!s.points || s.points.length === 0) return;
                    const color = SESSION_COLORS[idx % SESSION_COLORS.length];
                    const isActive = s.status === 'active';
                    const popupHtml =
                        '<div style="font-family:Inter,sans-serif;font-size:13px;min-width:180px">' +
                        '<b style="color:#1e293b">' + s.name + '</b>' +
                        (isActive ? ' <span style="color:#10b981;font-size:11px">● live</span>' : '') +
                        '<hr style="margin:5px 0;border-color:#e2e8f0">' +
                        '<table style="width:100%;border-collapse:collapse">' +
                        '<tr><td style="color:#64748b;padding:2px 0">Device</td><td style="text-align:right;font-family:monospace;font-size:11px">' + s.device_id + '</td></tr>' +
                        '<tr><td style="color:#64748b;padding:2px 0">Status</td><td style="text-align:right">' + s.status + '</td></tr>' +
                        '<tr><td style="color:#64748b;padding:2px 0">Readings</td><td style="text-align:right;font-weight:600">' + s.reading_count + '</td></tr>' +
                        '</table>' +
                        '<a href="/admin/sessions/' + s.id + '" style="display:block;margin-top:8px;text-align:center;padding:4px;background:#eff6ff;border-radius:6px;color:#2563eb;font-size:12px;font-weight:600;text-decoration:none">View Analysis →</a>' +
                        '</div>';

                    if (s.points.length >= 3) {
                        const hull = convexHull(s.points);
                        if (hull.length >= 3) {
                            L.polygon(hull, {
                                color: color,
                                fillColor: color,
                                fillOpacity: isActive ? 0.12 : 0.07,
                                weight: isActive ? 2.5 : 1.5,
                                dashArray: isActive ? null : '7 5',
                            }).bindPopup(popupHtml).addTo(sessionCoverageLayer);
                            return;
                        }
                    }
                    L.circleMarker(s.points[0], {
                        radius: 8, color: color, fillColor: color,
                        fillOpacity: 0.6, weight: 2,
                    }).bindPopup(popupHtml).addTo(sessionCoverageLayer);
                });

                sessionCoverageLayer.addTo(map);
                rebuildLayerControl();
            })
            .catch(function (err) {
                console.warn('session coverage load failed:', err);
                rebuildLayerControl();
            });
    }

    function geocodeSearch(query) {
        if (!query.trim()) return;
        if (!opencageKey) {
            alert('OpenCage API key not configured (OPENCAGE_API_KEY env var).');
            return;
        }

        fetch(
            'https://api.opencagedata.com/geocode/v1/json?q=' +
            encodeURIComponent(query) + '&key=' + opencageKey + '&limit=1&no_annotations=1'
        )
            .then(function (r) { return r.json(); })
            .then(function (data) {
                if (!data.results || data.results.length === 0) {
                    alert('Location not found.');
                    return;
                }
                const loc   = data.results[0].geometry;
                const label = data.results[0].formatted;

                if (geocodeMarker) map.removeLayer(geocodeMarker);
                geocodeMarker = L.marker([loc.lat, loc.lng], {
                    icon: L.divIcon({
                        className: '',
                        html: '<div style="background:linear-gradient(135deg,#2563eb,#db2777);width:28px;height:28px;border-radius:50% 50% 50% 0;transform:rotate(-45deg);border:3px solid #fff;box-shadow:0 2px 8px rgba(0,0,0,.3)"></div>',
                        iconSize: [28, 28],
                        iconAnchor: [14, 28],
                    }),
                }).addTo(map);
                geocodeMarker.bindPopup('<b>' + label + '</b>').openPopup();
                map.setView([loc.lat, loc.lng], 12, { animate: true });
            })
            .catch(function () { alert('Geocoding request failed.'); });
    }

    function onMapClick(e) {
        if (!opencageKey) return;
        fetch(
            'https://api.opencagedata.com/geocode/v1/json?q=' +
            e.latlng.lat + '+' + e.latlng.lng + '&key=' + opencageKey + '&limit=1&no_annotations=1'
        )
            .then(function (r) { return r.json(); })
            .then(function (data) {
                const label = (data.results && data.results[0])
                    ? data.results[0].formatted
                    : 'Unknown location';
                L.popup()
                    .setLatLng(e.latlng)
                    .setContent(
                        '<div style="font-family:Inter,sans-serif;font-size:12px">' +
                        '<b>' + label + '</b><br>' +
                        '<span style="color:#64748b">' + e.latlng.lat.toFixed(6) + ', ' + e.latlng.lng.toFixed(6) + '</span>' +
                        '</div>'
                    )
                    .openOn(map);
            })
            .catch(function () { });
    }

    window.initMap = function (key) {
        opencageKey = key || '';

        map = L.map('map', {
            center: [-1.0, 33.0],
            zoom: 8,
            layers: [tileLayers['Google Maps Style']],
            zoomControl: true,
        });

        layerControl = L.control.layers(tileLayers, null, { position: 'topright' }).addTo(map);
        map.on('click', onMapClick);

        const daysFilter = document.getElementById('days-filter');

        loadDensity(daysFilter ? daysFilter.value : 30);
        loadSessionCoverage();

        const reloadBtn = document.getElementById('reload-btn');
        if (reloadBtn) {
            reloadBtn.addEventListener('click', function () {
                loadDensity(daysFilter ? daysFilter.value : 30);
                loadSessionCoverage();
            });
        }

        if (daysFilter) {
            daysFilter.addEventListener('change', function () {
                loadDensity(this.value);
            });
        }

        const geocodeInput = document.getElementById('geocode-input');
        if (geocodeInput) {
            geocodeInput.addEventListener('keydown', function (e) {
                if (e.key === 'Enter') geocodeSearch(this.value);
            });
        }
    };
})();
