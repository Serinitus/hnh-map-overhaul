import {HnHMaxZoom, PlayerPulseIcon} from "../utils/LeafletCustomTypes";
import * as L from "leaflet";

export class Character {
    constructor(characterData) {
        this.name = characterData.name;
        this.position = characterData.position;
        this.type = characterData.type;
        this.id = characterData.id;
        this.map = characterData.map;
        this.marker = false;
        this.text = this.name;
        this.value = this.id;
        this.onClick = null;
        this.tstate = false;
        this.hideCharacterNames = false;
    }

    getId() {
        return `${this.name}`;
    }

    displayName() {
        return this.hideCharacterNames ? this.name.replace(/\s*\([^)]*\)\s*$/, '') : this.name;
    }

    remove(mapview) {
        if (this.marker) {
            this.marker.unbindTooltip();
            mapview.map.removeLayer(this.marker);
            this.marker.remove();
            this.marker = null;
        }
    }

    add(mapview) {
        if (this.map === mapview.mapid) {
            let position = mapview.map.unproject([this.position.x, this.position.y], HnHMaxZoom);
            this.marker = L.marker(position, {icon: PlayerPulseIcon, riseOnHover: true/*title: this.name*/});
            this.marker.marker = this;
            this.marker.bindPopup(this.displayName());
            this.marker.bindTooltip("<div style='color:#48fd00;'><b>" + this.displayName() + "</b></div>", {
                permanent: true,
                direction: 'top',
                sticky: false,
                opacity: 1,
                offset: [0, 0]
            });
            // this.marker.on('mouseover', function(ev) {
            //     ev.target.openPopup();
            // });
            // this.marker.on('mouseout', function(ev) {
            //     ev.target.closePopup();
            // });
            this.marker.on("click", this.callCallback.bind(this));
            this.marker.addTo(mapview.map);
            this.tooltip(this.tstate);
        }
    }

    update(mapview, updated, visible) {
        if (this.map !== updated.map) {
            this.remove(mapview);
        }
        this.map = updated.map;
        this.position = updated.position;
        if (!this.marker && visible && this.map === mapview.mapid) {
            this.add(mapview);
        }
        if (this.marker) {
            let position = mapview.map.unproject([updated.position.x, updated.position.y], HnHMaxZoom);
            this.marker.setLatLng(position);
        }
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

    setHideCharacterNames(value) {
        this.hideCharacterNames = value;
        if (this.marker) {
            this.marker.unbindPopup();
            this.marker.bindPopup(this.displayName());
            this.marker.unbindTooltip();
            this.marker.bindTooltip("<div style='color:#48fd00;'><b>" + this.displayName() + "</b></div>", {
                permanent: true,
                direction: 'top',
                sticky: false,
                opacity: 1,
                offset: [0, 0]
            });
            this.tooltip(this.tstate);
        }
    }

    tooltip(value) {
        if (value)
            this.bindTooltip();
        else
            this.unbindTooltip();
    }

    setClickCallback(callback) {
        this.onClick = callback;
    }

    callCallback(e) {
        if (this.onClick != null) {
            this.onClick(e);
        }
    }
}