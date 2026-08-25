import {HnHMaxZoom, ImageIcon, getCustomMarkerIcon} from "../utils/LeafletCustomTypes";
import * as L from "leaflet";

// Thingwall icons shrink as you zoom out instead of staying a fixed size
// regardless of zoom -- ported from the live server's own scaling formula.
// At full zoom-in (levelsOut=0) this is 1 (no change); each zoom level out
// shrinks it by another 25%, floored at 35% so icons never vanish.
function iconScale(zoom) {
    let levelsOut = Math.max(0, HnHMaxZoom - zoom);
    return Math.max(0.35, Math.pow(0.75, levelsOut));
}

// mm/down and mm/up are Nurgling-local icon overrides with no live-server
// equivalent; newer markers report the real gob resource names instead.
// Both are recognized here so markers from either convention group into
// one "Mineshaft" category rather than showing as two separate types.
const MINESHAFT_IMAGES = ["mm/down", "mm/up", "gfx/terobjs/minehole", "gfx/terobjs/ladder"];
const MINESHAFT_ICON = {
    "mm/down": "mm/down", "gfx/terobjs/minehole": "mm/down",
    "mm/up": "mm/up", "gfx/terobjs/ladder": "mm/up",
};

// gfx/hud/mmap/cave is the pre-existing cave-entrance marker (predates any
// Kami auto-mark support). gfx/tiles/ridges/cavein(2)/caveout are Kami's
// own cave-in/cave-out auto-marks. All represent the same feature, so they
// share one "Cave" category and one icon, the same way Mineshaft merges
// minehole+ladder.
const CAVE_IMAGES = ["gfx/hud/mmap/cave", "gfx/tiles/ridges/cavein", "gfx/tiles/ridges/cavein2", "gfx/tiles/ridges/caveout"];

function detectType(name) {
    if (name === "gfx/invobjs/small/bush" || name === "gfx/invobjs/small/bumling" || name === "gfx/terobjs/mm/gianttoad") return "quest";
    if (name === "gfx/terobjs/mm/thingwall") return "thingwall";
    if (MINESHAFT_IMAGES.includes(name)) return "Mineshaft";
    if (CAVE_IMAGES.includes(name)) return "Cave";
    if (name === "custom") return "custom";
    let idx = name.lastIndexOf("/");
    return idx === -1 ? name : name.substring(name.lastIndexOf("/") + 1);
}

export class Marker {
    constructor(markerData) {
        this.id = markerData.id;
        this.position = markerData.position;
        this.name = markerData.name;
        this.image = markerData.image;
        this.type = detectType(this.image);
        this.marker = false;
        this.text = this.name;
        this.value = this.id;
        this.hidden = markerData.hidden;
        this.map = markerData.map;
        this.onClick = null;
        this.onContext = null;
        this.onHoverEnter = null;
        this.onHoverLeave = null;
        this.tstate = false;
        this.view = false;
    }

    remove(mapview) {
        if (this.marker) {
            this.marker.unbindTooltip();
            mapview.map.removeLayer(this.marker);
            this.marker.remove();
            this.marker = null;
        }
        this.view = false;
    }

    // thingwallScale is the account's personal multiplier from /settings
    // (see MapView.vue's thingwallScale, default 1 = unmodified live
    // behavior) -- applied on top of the zoom-based iconScale, only for
    // Thingwalls. Every other marker type's sizing is unchanged/static,
    // same as before.
    buildIcon(zoom, thingwallScale) {
        let isCustom = this.image === "gfx/terobjs/mm/custom";
        let isCave = CAVE_IMAGES.includes(this.image) || this.name.toLowerCase() === "cave";
        let isThingwall = this.image === "gfx/terobjs/mm/thingwall";
        let isMineshaft = MINESHAFT_IMAGES.includes(this.image);

        if (isCustom && !isCave) {
            return getCustomMarkerIcon('white');
        }
        let hsz = isThingwall ? Math.round(45 * iconScale(zoom) * (thingwallScale || 1)) : isMineshaft ? 13.5 : 18;
        let url = isMineshaft ? `${MINESHAFT_ICON[this.image]}.png` : `${this.image}.png`;
        if (isCave)
            url = 'gfx/hud/mmap/cave.png';
        return new ImageIcon({iconUrl: url, iconSize: [hsz * 2, hsz * 2], iconAnchor: [hsz, hsz]});
    }

    // Re-applies the icon for the current zoom/scale without recreating
    // the marker (so tooltip-open state, event listeners, etc. all stay
    // put) -- called on every "zoom" event, see MapView.vue.
    rescale(mapview) {
        if (this.marker) {
            this.marker.setIcon(this.buildIcon(mapview.map.getZoom(), mapview.thingwallScale));
        }
    }

    add(mapview) {
        this.view = mapview.map;
        if (!this.hidden) {
            let icon = this.buildIcon(this.view.getZoom(), mapview.thingwallScale);

            let position = this.view.unproject([this.position.x, this.position.y], HnHMaxZoom);
            this.marker = L.marker(position, {icon: icon, riseOnHover: true/*, title: this.name*/});
            let col = "#FFF";
            if (this.type === "quest") {
                col = "#00cffd";
            } else if (this.type === "thingwall") {
                col = "#00cffd";
            }
            this.marker.marker = this;
            this.marker.bindTooltip("<div style='color:" + col + ";'><b>" + this.name + "</b></div>", {
                permanent: true,
                direction: 'top',
                sticky: false,
                opacity: 0.9
            });
            this.marker.on('mouseover', function (ev) {
                ev.target.openTooltip();
            });
            this.marker.on('mouseout', function (ev) {
                if (!ev.target.marker.tstate) {
                    ev.target.closeTooltip();
                }
            });
            // this.marker.bindPopup(this.name);
            // this.marker.on('mouseover', function(ev) {
            //     ev.target.openPopup();
            // });
            // this.marker.on('mouseout', function(ev) {
            //     ev.target.closePopup();
            // });
            this.marker.addTo(mapview.markerLayer);
            if (!this.tstate) {
                this.marker.closeTooltip();
            }
            this.marker.on("click", this.callClickCallback.bind(this));
            this.marker.on("contextmenu", this.callContextCallback.bind(this));
            this.marker.on("mouseover", () => {
                if (this.onHoverEnter) this.onHoverEnter(this);
            });
            this.marker.on("mouseout", () => {
                if (this.onHoverLeave) this.onHoverLeave(this);
            });
        }
    }

    update(mapview, updated) {
        let mapChanged = this.map !== updated.map;
        let imageChanged = this.image !== updated.image;
        this.name = updated.name;
        this.text = this.name;
        this.image = updated.image;
        this.type = detectType(this.image);
        this.hidden = updated.hidden;
        this.map = updated.map;
        this.position = updated.position;

        if (mapChanged || imageChanged || this.hidden) {
            this.remove(mapview);
        }
        if (!this.marker && !this.hidden && (this.map === mapview.mapid || this.map === mapview.overlayLayer.map)) {
            this.add(mapview);
        }
        if (this.marker) {
            let position = mapview.map.unproject([this.position.x, this.position.y], HnHMaxZoom);
            this.marker.setLatLng(position);
        }
    }

    tooltipState(value) {
        this.tstate = value;
    }

    bindTooltip() {
        this.tstate = true;
        if (this.marker) {
            this.marker.openTooltip();
        }
    }

    unbindTooltip() {
        this.tstate = false;
        if (this.marker) {
            this.marker.closeTooltip();
        }
    }

    tooltip(value) {
        try {
            console.log(this.name + " " + value);
            if (value)
                this.bindTooltip();
            else
                this.unbindTooltip();
        } catch (e) {
            console.log(e);
        }
    }

    /**
     * Перемещение к какому-либо маркеру
     * @param map
     */
    jumpTo(map) {
        if (this.marker) {
            let position = map.unproject([this.position.x, this.position.y], HnHMaxZoom);
            this.marker.setLatLng(position);
        }
    }

    setClickCallback(callback) {
        this.onClick = callback;
    }

    callClickCallback(e) {
        if (this.onClick != null) {
            this.onClick(e);
        }
    }

    setContextMenu(callback) {
        this.onContext = callback;
    }

    callContextCallback(e) {
        if (this.onContext != null) {
            this.onContext(e);
        }
    }

    setHoverCallback(onEnter, onLeave) {
        this.onHoverEnter = onEnter;
        this.onHoverLeave = onLeave;
    }
}