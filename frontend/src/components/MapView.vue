<template>
  <div>
    <v-app-bar app dense height="56" color="#1a1a1a" dark style="z-index: 1001;">
      <img src="haven.jpg" height="36" style="border-radius:4px;object-fit:cover;margin-right:10px;"/>
      <span style="font-size:1.25rem;"><span style="color:#FF9800;">Haven</span><span style="color:#fff;">Map</span></span>
      <v-spacer></v-spacer>
      <v-btn text href="/map" class="mr-2">Map</v-btn>
      <v-btn v-if="!isPublic" text href="/" class="mr-4">Tokens</v-btn>
      <v-btn v-if="isPublic" text href="/login">
        Login
      </v-btn>
      <v-menu v-else open-on-hover offset-y>
        <template v-slot:activator="{ on, attrs }">
          <v-btn text v-bind="attrs" v-on="on">
            {{ username }}
            <v-icon right>mdi-menu-down</v-icon>
          </v-btn>
        </template>
        <v-list>
          <v-list-item v-if="auths.includes('admin')" href="/admin">
            <v-list-item-title>Admin</v-list-item-title>
          </v-list-item>
          <v-list-item href="/settings">
            <v-list-item-title>Settings</v-list-item-title>
          </v-list-item>
          <v-list-item href="/logout">
            <v-list-item-title>Logout</v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>
    </v-app-bar>

    <v-navigation-drawer
        :mini-variant.sync="mini"
        :mini-variant-width="36"
        app
        style="z-index: 1000"
        prominent>
      <v-list-item>
        <v-btn
            icon
            @click.stop="mini = !mini"
            style="/*margin-left: -6px;*/"
        >
          <v-icon>mdi-arrow-expand</v-icon>
        </v-btn>
      </v-list-item>

      <v-divider></v-divider>

      <v-list dense v-if="!mini">
        <!-- Unified search across Thingwalls/Quest Givers/Custom Markers/
             Players -- replaces four separate dropdowns. Selection is
             handled entirely through @change, not a v-model watcher, so
             re-selecting the same single result (e.g. only one player
             online) always re-homes instead of silently no-oping because
             Vue sees no value change. -->
        <v-list-item>
          <v-list-item-content>
            <v-list-item-title>
              <v-autocomplete return-object outlined dense clearable
                              :items="searchItems" item-text="name"
                              v-model="searchSelection" @change="jumpToSearchResult"
                              placeholder="Search">
                <template v-slot:item="data">
                  <div style="display:flex;align-items:center;width:100%;overflow:hidden;">
                    <img v-if="data.item.icon" class="mr-2" style="width:24px;height:24px;flex-shrink:0;" :src="data.item.icon"/>
                    <v-icon v-else class="mr-2">mdi-account</v-icon>
                    <span style="flex-grow:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">{{ data.item.name }}</span>
                    <span style="opacity:0.6;font-size:0.75em;margin-left:8px;flex-shrink:0;">{{ data.item.typeLabel }}</span>
                  </div>
                </template>
              </v-autocomplete>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <!-- HIDE GRID -->
        <v-list-item>
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showGridCoordinates" title="Show/hide"></icon-toggle>
              </div>
              <span>Grid Coordinates</span>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <!-- Test row for the dual-icon pattern: eye controls visibility,
             the comment-bubble icon controls whether names stay pinned
             above each marker. -->
        <v-list-item v-if="canSeeSection('thingwalls')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showThingwalls" class="mr-2" title="Show/hide"></icon-toggle>
                <icon-toggle v-model="showThingwallNames" title="Show/hide names"
                             on-icon="mdi-comment-check-outline" off-icon="mdi-comment-remove-outline"></icon-toggle>
              </div>
              <span>Thingwalls</span>
              <v-spacer></v-spacer>
              <div class="row-marker-icons">
                <img src="gfx/terobjs/mm/thingwall.png" style="width:20px;height:20px;"/>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item v-if="canSeeSection('vortexes')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showVortexes" title="Show/hide"></icon-toggle>
              </div>
              <span>Vortexes</span>
              <v-spacer></v-spacer>
              <div class="row-marker-icons">
                <img src="gfx/terobjs/mm/watervortex.png" style="width:20px;height:20px;"/>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <!-- MAP (Jump To Map / Overlay Map) -- same expand-chevron pattern
             as Natural Resources/Other/Clutter, grouping the two map-select
             controls that used to sit bare at the top of the drawer.
             Framed with top/bottom border lines (and margin outside them,
             so the lines read as separation from neighboring rows rather
             than sitting flush against them) to set it apart as its own
             region, left-aligned like every other row. -->
        <v-list-item style="border-top:1px solid white;border-bottom:1px solid white;margin:10px 0;padding-top:6px !important;padding-bottom:6px !important;">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <!-- Narrower than the standard 64px .row-icons gutter --
                   Map has no eye toggle of its own, so this lines its
                   label up with where a row's *second* icon (e.g. the
                   show/hide-names icon on Thingwalls) would sit, one
                   icon-width in, rather than the full two-icon width. -->
              <div style="width:32px;flex-shrink:0;"></div>
              <div style="display:flex;align-items:center;flex-grow:1;cursor:pointer;"
                   @click="mapSectionExpanded = !mapSectionExpanded">
                <span style="font-size:15px;">Map</span>
                <v-spacer></v-spacer>
                <v-icon title="Expand/collapse">
                  {{ mapSectionExpanded ? 'mdi-chevron-up' : 'mdi-chevron-down' }}
                </v-icon>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item v-if="mapSectionExpanded">
          <v-list-item-content>
            <v-list-item-title>
              <label class="title">Jump To Map</label>
              <v-autocomplete return-object outlined dense :items="maps" v-model="selectedMap"
                              placeholder="Select Map"></v-autocomplete>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item v-if="mapSectionExpanded">
          <v-list-item-content>
            <v-list-item-title>
              <label class="title" style="display:flex;align-items:center;">
                <!-- Independent of which map is picked below: toggling
                     this off hides the overlay (overlayLayer.map = -1)
                     without clearing the dropdown selection, so turning
                     it back on restores the same overlay instantly. -->
                <div class="row-icons">
                  <icon-toggle v-model="showOverlayMap" title="Show/hide"></icon-toggle>
                </div>
                <span>Overlay Map</span>
              </label>
              <v-autocomplete return-object outlined dense :items="maps" v-model="overlayMap"
                              placeholder="Select Map"></v-autocomplete>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <!-- HIDE MARKER -->
        <v-list-item v-if="canSeeSection('naturalMarkers')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showMarkers" title="Show/hide"></icon-toggle>
              </div>
              <!-- Clickable from the first letter of the label all the way
                   to the arrow, not just the tiny chevron itself. -->
              <div style="display:flex;align-items:center;flex-grow:1;cursor:pointer;"
                   @click="naturalMarkersExpanded = !naturalMarkersExpanded">
                <span>Natural Resources</span>
                <v-spacer></v-spacer>
                <v-icon small title="Expand/collapse">
                  {{ naturalMarkersExpanded ? 'mdi-chevron-up' : 'mdi-chevron-down' }}
                </v-icon>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <!-- NATURAL MARKER CATEGORIES (per-type toggle, e.g. Mineshaft, Cave, Burrow) --
             Expand state is independent of the eye toggle above: showMarkers
             controls whether these markers render on the map, while
             naturalMarkersExpanded only controls whether this list is open
             in the drawer -- collapsing it doesn't hide anything, and
             hiding the section doesn't force it closed. -->
        <v-list-item v-if="canSeeSection('naturalMarkers') && naturalMarkersExpanded">
          <v-list-item-content>
            <v-list-item-title>
              <div style="max-height:260px;overflow-y:auto;">
                <div v-for="cat in marksCategories" :key="cat"
                     style="display:flex;align-items:center;justify-content:space-between;">
                  <span style="display:flex;align-items:center;overflow:hidden;">
                    <img v-if="resourceCategoryIcons[cat]" class="mr-2"
                         style="width:20px;height:20px;flex-shrink:0;"
                         :src="resourceCategoryIcons[cat] + '.png'"/>
                    <span style="text-overflow:ellipsis;overflow:hidden;white-space:nowrap;">{{ cat }}</span>
                  </span>
                  <v-switch dense hide-details class="mt-0 pt-0" style="flex-shrink:0;"
                            :input-value="selectedResourceTypes.includes(cat)"
                            @change="toggleResourceType(cat)"></v-switch>
                </div>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <!-- MINEHOLES / CAVES -- split out of Natural Resources into their
             own standalone toggles (two separate rows, not one combined
             toggle). Same auth gate as Natural Resources since it's still
             that same underlying data, just broken out for clarity. -->
        <v-list-item v-if="canSeeSection('naturalMarkers')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showMineholes" title="Show/hide"></icon-toggle>
              </div>
              <span>Mineholes</span>
              <v-spacer></v-spacer>
              <div class="row-marker-icons">
                <img src="mm/down.png" style="width:20px;height:20px;"/>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item v-if="canSeeSection('naturalMarkers')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showCaves" title="Show/hide"></icon-toggle>
              </div>
              <span>Caves</span>
              <v-spacer></v-spacer>
              <div class="row-marker-icons">
                <img src="gfx/hud/mmap/cave.png" style="width:20px;height:20px;"/>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item v-if="canSeeSection('questGivers')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showQuests" class="mr-2" title="Show/hide"></icon-toggle>
                <icon-toggle v-model="showQuestNames" title="Show/hide names"
                             on-icon="mdi-comment-check-outline" off-icon="mdi-comment-remove-outline"></icon-toggle>
              </div>
              <span>Quest Givers</span>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <!-- Gated by "point" (the Characters role) directly, not
             canSeeSection -- getChars() already enforces that auth
             server-side, so a separate sec_players checkbox would just be
             a UI switch that has to agree with the real data-access one.
             The names icon duplicates the /settings page's Players Names
             preference -- kept in sync via the same showPlayerTooltips
             model, so either control works and they can't drift apart. -->
        <v-list-item v-if="auths.includes('point')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showPlayers" class="mr-2" title="Show/hide"></icon-toggle>
                <icon-toggle v-model="showPlayerTooltips" title="Show/hide names"
                             on-icon="mdi-comment-check-outline" off-icon="mdi-comment-remove-outline"></icon-toggle>
              </div>
              <span>Players</span>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item v-if="canSeeSection('roads')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showRoads" class="mr-2" title="Show/hide"></icon-toggle>
                <!-- Replaces the old full-width "Draw Road"/"Cancel Draw
                     Road" button -- same drawingRoad state and
                     toggleDrawRoad cleanup (clears roadPoints/temp
                     marker), just as an inline icon like the other rows.
                     :value/@input instead of v-model since toggleDrawRoad
                     flips the state itself rather than taking the new
                     value as a parameter. -->
                <icon-toggle v-if="auths.includes('admin') || auths.includes('writer')"
                             :value="drawingRoad" @input="toggleDrawRoad" title="Edit"
                             on-icon="mdi-note-edit" off-icon="mdi-note-edit-outline"
                             on-color="primary" off-color="green"></icon-toggle>
              </div>
              <span>Roads</span>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item v-if="canSeeSection('customMarkers')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showCustomMarkers" title="Show/hide"></icon-toggle>
              </div>
              <span>Custom Markers</span>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <!-- HIDE CLUTTER -->
        <v-list-item v-if="canSeeSection('clutter')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showClutter" title="Show/hide"></icon-toggle>
              </div>
              <div style="display:flex;align-items:center;flex-grow:1;cursor:pointer;"
                   @click="clutterExpanded = !clutterExpanded">
                <span>Clutter</span>
                <v-spacer></v-spacer>
                <v-icon small title="Expand/collapse">
                  {{ clutterExpanded ? 'mdi-chevron-up' : 'mdi-chevron-down' }}
                </v-icon>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <!-- CLUTTER CATEGORIES (burrow, clamreef, flintwash, spawningbed) --
             turning Clutter on no longer means all of it; each type is its
             own switch, same pattern as Natural Markers/Other. Expand
             state is independent of the eye, same reasoning as those two. -->
        <v-list-item v-if="canSeeSection('clutter') && clutterExpanded">
          <v-list-item-content>
            <v-list-item-title>
              <div style="max-height:260px;overflow-y:auto;">
                <div v-for="cat in clutterCategories" :key="cat"
                     style="display:flex;align-items:center;justify-content:space-between;">
                  <span style="display:flex;align-items:center;overflow:hidden;">
                    <img v-if="clutterCategoryIcons[cat]" class="mr-2"
                         style="width:20px;height:20px;flex-shrink:0;"
                         :src="clutterCategoryIcons[cat] + '.png'"/>
                    <span style="text-overflow:ellipsis;overflow:hidden;white-space:nowrap;">{{ cat }}</span>
                  </span>
                  <v-switch dense hide-details class="mt-0 pt-0" style="flex-shrink:0;"
                            :input-value="selectedClutterTypes.includes(cat)"
                            @change="toggleClutterType(cat)"></v-switch>
                </div>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <!-- UNKNOWN/UNVETTED MARKER TYPES (admin-only; anything not in this
             Mapper's known-resource allowlist -- see KNOWN_RESOURCE_TYPES --
             lands here instead of cluttering the Natural Resources panel
             above with one-off categories from upload noise) -->
        <v-list-item v-if="canSeeSection('other')">
          <v-list-item-content>
            <v-list-item-title style="display:flex;align-items:center;">
              <div class="row-icons">
                <icon-toggle v-model="showUnknownMarkers" title="Show/hide"></icon-toggle>
              </div>
              <div style="display:flex;align-items:center;flex-grow:1;cursor:pointer;"
                   @click="otherExpanded = !otherExpanded">
                <span>Other (Admin)</span>
                <v-spacer></v-spacer>
                <v-icon small title="Expand/collapse">
                  {{ otherExpanded ? 'mdi-chevron-up' : 'mdi-chevron-down' }}
                </v-icon>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

        <v-list-item v-if="canSeeSection('other') && otherExpanded">
          <v-list-item-content>
            <v-list-item-title>
              <div style="max-height:260px;overflow-y:auto;">
                <div v-for="cat in unknownCategories" :key="cat"
                     style="display:flex;align-items:center;justify-content:space-between;">
                  <span style="display:flex;align-items:center;overflow:hidden;">
                    <img v-if="unknownCategoryIcons[cat]" class="mr-2"
                         style="width:20px;height:20px;flex-shrink:0;"
                         :src="unknownCategoryIcons[cat] + '.png'"/>
                    <span style="text-overflow:ellipsis;overflow:hidden;white-space:nowrap;">{{ cat }}</span>
                  </span>
                  <v-switch dense hide-details class="mt-0 pt-0" style="flex-shrink:0;"
                            :input-value="selectedUnknownTypes.includes(cat)"
                            @change="toggleUnknownType(cat)"></v-switch>
                </div>
              </div>
            </v-list-item-title>
          </v-list-item-content>
        </v-list-item>

      </v-list>


      <!--        <div style="position: absolute;"><button class="btn btn-primary btn-sm" v-on:click="expandControlPanel = !expandControlPanel">-</button></div>-->
      <!--        <div class="card-body" v-bind:class="{ hidden: !expandControlPanel }">-->
      <!--          <div class="form-group">-->
      <!--            <div class="form-check">-->
      <!--              <input type="checkbox" class="form-check-input" id="check-grid-coords"-->
      <!--                     v-model="showGridCoordinates">-->
      <!--              <label class="form-check-label" for="check-grid-coords">Show grid coordinates</label>-->
      <!--            </div>-->
      <!--            <div class="form-check">-->
      <!--              <input type="checkbox" class="form-check-input" id="check-hide-markers"-->
      <!--                     v-model="hideMarkers">-->
      <!--              <label class="form-check-label" for="check-hide-markers">Hide Markers</label>-->
      <!--            </div>-->
      <!--            <button type="button" class="btn btn-secondary" style="margin-top: 10px;" v-on:click="zoomOut">Zoom-->
      <!--              out-->
      <!--            </button>-->
      <!--          </div>-->
      <!--          <div class="form-group">-->
      <!--            <label>Jump to Map</label>-->
      <!--            <v-autocomplete :items="maps" v-model="selectedMap" placeholder="Select Map"></v-autocomplete>-->
      <!--          </div>-->
      <!--          <div class="form-group">-->
      <!--            <label>Overlay Map</label>-->
      <!--            <v-autocomplete :items="maps" v-model="overlayMap" placeholder="Select Map"></v-autocomplete>-->
      <!--          </div>-->
      <!--          <div class="form-group">-->
      <!--            <label>Jump to Any Marker</label>-->
      <!--            <v-autocomplete :items="allMarks" v-model="selectedMarker"-->
      <!--                            placeholder="Select Marker"></v-autocomplete>-->
      <!--          </div>-->
      <!--          <div class="form-group">-->
      <!--            <label>Jump to Player</label>-->
      <!--            <v-autocomplete :items="players" v-model="selectedPlayer" placeholder="Select Player"></v-autocomplete>-->
      <!--          </div>-->
      <!--        </div>-->
    </v-navigation-drawer>

    <v-main>
      <v-container>
        <div ref="map" class="map"></div>
        <div class="control-panel card">

        </div>

        <vue-context ref="menu">
          <template slot-scope="tile" v-if="tile.data">
            <li>
              <a @click.prevent="wipeTile(tile.data)">Wipe tile {{ tile.data.coords.x }}, {{ tile.data.coords.y }}</a>
            </li>
            <li>
              <a @click.prevent="queryCoordSet(tile.data)">Rewrite tile coords for {{ tile.data.coords.x }},
                {{ tile.data.coords.y }}</a>
            </li>
            <li>
              <a @click.prevent="promptCustomMarker(tile.data)">Add marker here</a>
            </li>
          </template>
        </vue-context>

        <vue-context ref="markermenu">
          <template slot-scope="data" v-if="data.data">
            <li>
              <a @click.prevent="hideMarker(data.data)">Hide marker {{ data.data.name }}</a>
            </li>
          </template>
        </vue-context>

        <vue-context ref="roadmenu">
          <template slot-scope="data" v-if="data.data">
            <li>
              <a @click.prevent="deleteRoad(data.data)">Delete road {{ data.data.name }}</a>
            </li>
          </template>
        </vue-context>

        <vue-context ref="custommarkermenu">
          <template slot-scope="data" v-if="data.data">
            <li>
              <a @click.prevent="deleteCustomMarker(data.data)">Delete marker {{ data.data.name }}</a>
            </li>
          </template>
        </vue-context>

        <modal name="coordSet">
          <form class="pa-4" v-on:submit.prevent="setCoords(form)">
            <v-text-field light v-model="coordSet.x" label="X" placeholder="0"></v-text-field>
            <v-text-field light v-model="coordSet.y" label="Y" placeholder="0"></v-text-field>
            <v-btn color="primary" type="submit">Submit</v-btn>
          </form>
        </modal>

        <modal name="roadName">
          <form class="pa-4" v-on:submit.prevent="saveRoad">
            <v-text-field light v-model="roadName" label="Road name" autofocus></v-text-field>
            <v-btn color="primary" type="submit">Submit</v-btn>
          </form>
        </modal>

        <modal name="customMarkerName">
          <form class="pa-4" v-on:submit.prevent="saveCustomMarker">
            <v-text-field light v-model="customMarkerName" label="Marker name" autofocus></v-text-field>
            <div class="color-swatches">
              <img v-for="c in customMarkerColors" :key="c"
                   class="color-swatch" :class="{selected: customMarkerColor === c}"
                   :src="`gfx/terobjs/mm/custom_pin_${c}.png`"
                   :title="c"
                   @click="customMarkerColor = c">
            </div>
            <v-btn color="primary" type="submit">Submit</v-btn>
          </form>
        </modal>
      </v-container>
    </v-main>
    <v-snackbar v-model="pingSnackbar.show" :timeout="6000">
      {{ pingSnackbar.text }}
      <template v-slot:action="{ attrs }">
        <v-btn text v-bind="attrs" @click="jumpToPing">Jump to ping</v-btn>
      </template>
    </v-snackbar>
  </div>
</template>

<script>
import ModelSelect from 'vue-search-select'
import {GridCoordLayer, HnHCRS, HnHMaxZoom, HnHMinZoom, TileSize, CustomMarkerColors} from "../utils/LeafletCustomTypes";
import {SmartTileLayer} from "../utils/SmartTileLayer";
import * as L from "leaflet";
import {API_ENDPOINT} from "../main";
import {Marker} from "../data/Marker";
import {Road} from "../data/Road";
import {CustomMarker} from "../data/CustomMarker";
import {UniqueList} from "../data/UniqueList";
import {Character} from "../data/Character";
import VueContext from 'vue-context';
import IconToggle from './IconToggle.vue';

const CLUTTER_TYPES = ["burrow", "clamreef", "flintwash", "spawningbed"];

// Vetted natural-resource/auto-mark categories, cross-checked against
// another public mapper's own marker legend (public.hearthworld.com).
// A marker whose type isn't in this list is either a real resource we
// haven't catalogued yet or upload noise from a client bug -- either way
// it's routed to the admin-only "Other" section (see KNOWN_RESOURCE_TYPES
// usage in updateMarkers) instead of cluttering the normal resource panel
// with one-off categories.
const KNOWN_RESOURCE_TYPES = [
  // "mineshaft"/"cave" (merged categories, see MINESHAFT_IMAGES/CAVE_IMAGES
  // in Marker.js) are intentionally not here -- they're routed to their
  // own standalone Mineholes & Caves toggle, not this section, see the
  // it.type === "Mineshaft" || it.type === "Cave" branch in updateMarkers.
  "abyssalchasm", "amberwash", "windthrow", "batguano", "caveorgan",
  "clamreef", "claypit", "coralreef", "driftkelp", "fairystone", "flintwash",
  "geyser", "headwaters", "woodheart", "icespire", "jotunmussel", "lilypadlotus",
  "rockcrystal", "saltbasin", "tarpit",
  "irminsul", "monolith", "allies", "enemys", "quality",
];

export default {
  name: "MapView",
  components: {
    ModelSelect,
    VueContext,
    IconToggle,
  },
  data: function () {
    return {
      mini: true,
      showGridCoordinates: false,
      // Thingwalls/Vortexes are the only marker types visible by default
      // -- every other type's eye toggle starts off, left for the user
      // to turn on. Name/tooltip preferences (showQuestNames etc.) are a
      // separate, orthogonal setting from visibility and aren't touched
      // by this -- they only matter once their section's eye is on.
      showMarkers: false,
      showQuests: false,
      showQuestNames: true,
      showThingwalls: true,
      showThingwallNames: true,
      showVortexes: true,
      showPlayers: false,
      showPlayerTooltips: true,
      showRoads: false,
      showCustomMarkers: false,
      showClutter: false,
      selectedClutterTypes: [],
      clutterExpanded: false,
      clutterCategories: [],
      selectedResourceTypes: [],
      naturalMarkersExpanded: false,
      // Mineholes and Caves used to be lumped into Natural Resources'
      // per-category panel alongside every other resource type -- split
      // into their own simple on/off rows (same pattern as
      // Vortexes/Custom Markers), two separate toggles rather than one
      // combined switch.
      showMineholes: false,
      showCaves: false,
      showUnknownMarkers: false,
      selectedUnknownTypes: [],
      otherExpanded: false,
      hideCharacterNames: true,
      expandControlPanel: true,

      drawingRoad: false,
      roadPoints: [],
      roadTempMarker: null,
      roadName: '',
      pingMarker: null,

      pendingMarkerPoint: null,
      customMarkerName: '',
      customMarkerColor: 'white',
      customMarkerColors: CustomMarkerColors,

      trackingCharacterId: -1,
      autoMode: false,
      polling: null,
      zz: false,
      // markersCache: [],
      allMarks: [],
      otherMarks: [],
      otherMarksAdmin: [],
      customMarks: [],
      marksCategories: [],
      unknownCategories: [],
      thingMarks: [],
      vortexMarks: [],
      mineholeMarks: [],
      caveMarks: [],
      questMarks: [],
      clutterMarks: [],
      players: [],
      maps: [],
      selectedMap: null,
      searchSelection: null,
      overlayMap: {value: false},
      showOverlayMap: true,
      thingwallScale: 1,
      mapSectionExpanded: true,
      auths: [],
      username: '',
      isPublic: false,
      mapid: 0,
      coordSetFrom: {x: 0, y: 0},
      coordSet: {
        x: 0,
        y: 0
      },
      pingSnackbar: {show: false, text: '', map: 0, x: 0, y: 0}
    }
  },
  watch: {
    showGridCoordinates(value) {
      console.log("showGridCoordinates", value);
      if (value) {
        this.coordLayer.setOpacity(1);
      } else {
        this.coordLayer.setOpacity(0);
      }
    },
    showMarkers(value) {
      console.log("showMarkers", value);
      if (!value) {
        this.otherMarks.forEach(it => it.remove(this));
      } else {
        this.otherMarks.filter(it => (it.map === this.mapid || it.map === this.overlayLayer.map) && this.selectedResourceTypes.includes(it.type)).forEach(it => it.add(this));
      }
    },
    selectedResourceTypes() {
      console.log("selectedResourceTypes", this.selectedResourceTypes);
      this.otherMarks.forEach(it => it.remove(this));
      if (this.showMarkers) {
        this.otherMarks.filter(it => (it.map === this.mapid || it.map === this.overlayLayer.map) && this.selectedResourceTypes.includes(it.type)).forEach(it => it.add(this));
      }
    },
    showUnknownMarkers(value) {
      console.log("showUnknownMarkers", value);
      if (!value) {
        this.otherMarksAdmin.forEach(it => it.remove(this));
      } else {
        this.otherMarksAdmin.filter(it => (it.map === this.mapid || it.map === this.overlayLayer.map) && this.selectedUnknownTypes.includes(it.type)).forEach(it => it.add(this));
      }
    },
    selectedUnknownTypes() {
      console.log("selectedUnknownTypes", this.selectedUnknownTypes);
      this.otherMarksAdmin.forEach(it => it.remove(this));
      if (this.showUnknownMarkers) {
        this.otherMarksAdmin.filter(it => (it.map === this.mapid || it.map === this.overlayLayer.map) && this.selectedUnknownTypes.includes(it.type)).forEach(it => it.add(this));
      }
    },
    showThingwalls(value) {
      console.log("showThingwalls", value);
      if (!value) {
        this.thingMarks.forEach(it => it.remove(this));
      } else {
        this.thingMarks.filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => {
          it.add(this);
          it.tooltip(this.showThingwallNames);
        });
      }
    },
    showThingwallNames(value) {
      console.log("showThingwallNames", value);
      this.thingMarks.forEach(it => it.tooltip(value));
    },
    showVortexes(value) {
      console.log("showVortexes", value);
      if (!value) {
        this.vortexMarks.forEach(it => it.remove(this));
      } else {
        this.vortexMarks.filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => it.add(this));
      }
    },
    showMineholes(value) {
      console.log("showMineholes", value);
      if (!value) {
        this.mineholeMarks.forEach(it => it.remove(this));
      } else {
        this.mineholeMarks.filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => it.add(this));
      }
    },
    showCaves(value) {
      console.log("showCaves", value);
      if (!value) {
        this.caveMarks.forEach(it => it.remove(this));
      } else {
        this.caveMarks.filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => it.add(this));
      }
    },
    showQuests(value) {
      console.log("showQuests", value);
      if (!value) {
        this.questMarks.forEach(it => it.remove(this));
      } else {
        this.questMarks.filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => {
          it.add(this);
          it.tooltip(this.showQuestNames);
        });
      }
    },
    showQuestNames(value) {
      console.log("showQuestNames", value);
      this.questMarks.forEach(it => it.tooltip(value));
    },
    showClutter(value) {
      console.log("showClutter", value);
      if (!value) {
        this.clutterMarks.forEach(it => it.remove(this));
      } else {
        this.clutterMarks.filter(it => (it.map === this.mapid || it.map === this.overlayLayer.map) && this.selectedClutterTypes.includes(it.type)).forEach(it => it.add(this));
      }
    },
    selectedClutterTypes() {
      console.log("selectedClutterTypes", this.selectedClutterTypes);
      this.clutterMarks.forEach(it => it.remove(this));
      if (this.showClutter) {
        this.clutterMarks.filter(it => (it.map === this.mapid || it.map === this.overlayLayer.map) && this.selectedClutterTypes.includes(it.type)).forEach(it => it.add(this));
      }
    },
    showPlayers(value) {
      console.log("showPlayers", value);
      if (!value) {
        this.characters.getElements().forEach(it => it.remove(this));
      } else {
        this.characters.getElements().filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => {
          it.add(this);
          it.tooltip(this.showPlayers);
        });
      }
    },
    showRoads(value) {
      console.log("showRoads", value);
      if (!value) {
        this.roads.getElements().forEach(it => it.remove(this));
      } else {
        this.roads.getElements().filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => it.add(this));
      }
    },
    showCustomMarkers(value) {
      console.log("showCustomMarkers", value);
      if (!value) {
        this.customMarkers.getElements().forEach(it => it.remove(this));
      } else {
        this.customMarkers.getElements().filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => it.add(this));
      }
    },
    showPlayerTooltips(value) {
      console.log("showPlayerTooltips", value);
      this.characters.getElements().forEach(it => it.tooltip(value));
    },
    hideCharacterNames(value) {
      console.log("hideCharacterNames", value);
      this.characters.getElements().forEach(it => it.setHideCharacterNames(value));
    },
    trackingCharacterId(value) {
      if (value !== -1) {
        let character = this.characters.byId(value);
        if (character) {
          this.jumpToCharacter(value);
        } else {
          this.map.setView([0, 0], HnHMinZoom);
          let mapid = this.maps[0].ID;
          this.$router.replace({path: `/grid/${mapid}/0/0/${HnHMinZoom}`}).catch(() => {});
          this.trackingCharacterId = -1;
        }
      }
    },
    selectedMap(value) {
      console.log('selectedMap', value)
      if (value) {
        this.changeMap(value.ID);
        let zoom = this.map.getZoom();
        this.map.setView([0, 0], zoom);

        this.$router.replace({path: `/grid/${this.mapid}/0/0/${zoom}`}).catch(() => {});
        this.trackingCharacterId = -1;
      }
    },
    // Both the selected overlay map and its independent visibility toggle
    // funnel through the same applyOverlayMap() -- see there for why.
    overlayMap() {
      this.applyOverlayMap();
    },
    showOverlayMap() {
      this.applyOverlayMap();
    },
  },
  mounted() {
    let chars = this.$http.get(`${API_ENDPOINT}/v1/characters`)
    let maps = this.$http.get(`${API_ENDPOINT}/maps`)

    Promise.all([chars, maps]).then(values => {
      this.setupMap(values[0].body, values[1].body);
    }, () => this.$emit("error"));
  },
  beforeDestroy: function () {
    clearInterval(this.intervalId)
  },
  computed: {
    resourceCategoryIcons() {
      let icons = {};
      this.otherMarks.forEach(it => {
        if (!(it.type in icons)) icons[it.type] = it.image;
      });
      return icons;
    },
    unknownCategoryIcons() {
      let icons = {};
      this.otherMarksAdmin.forEach(it => {
        if (!(it.type in icons)) icons[it.type] = it.image;
      });
      return icons;
    },
    clutterCategoryIcons() {
      let icons = {};
      this.clutterMarks.forEach(it => {
        if (!(it.type in icons)) icons[it.type] = it.image;
      });
      return icons;
    },
    // Unified list backing the single search bar that replaced the four
    // separate Thingwall/NPC/Marker/Player dropdowns.
    searchItems() {
      let items = [];
      this.thingMarks.forEach(it => items.push({searchType: 'thingwall', name: it.name, icon: it.image + '.png', typeLabel: 'Thingwall', raw: it}));
      this.questMarks.forEach(it => items.push({searchType: 'quest', name: it.name, icon: it.image + '.png', typeLabel: 'Quest', raw: it}));
      this.customMarks.forEach(it => items.push({searchType: 'marker', name: it.name, icon: 'gfx/terobjs/mm/custom_pin_' + it.color + '.png', typeLabel: 'Marker', raw: it}));
      this.players.forEach(it => items.push({searchType: 'player', name: it.name, icon: null, typeLabel: 'Player', raw: it}));
      return items;
    }
  },
  methods: {
    // Shared "home in" logic for Thingwalls, Quest Givers, and Custom
    // Markers -- identical body previously duplicated across three watchers.
    jumpToMapMarker(value) {
      let markerMapId = value.map;
      this.maps.forEach((map) => {
        if (markerMapId === map.ID) {
          if (this.mapid !== map.ID)
            this.changeMap(map.ID);
          if (!value.marker) value.add(this);
          this.map.setView(value.marker.getLatLng(), HnHMaxZoom);
          this.trackingCharacterId = -1;
        }
      });
    },
    // Shared "home in" logic for players -- also called directly by
    // jumpToSearchResult (not just the trackingCharacterId watcher) so
    // re-selecting the same player in the search bar always re-centers,
    // even when trackingCharacterId's value doesn't actually change.
    jumpToCharacter(id) {
      let character = this.characters.byId(id);
      if (!character) return;
      this.changeMap(character.map);
      let latlng = this.map.unproject([character.position.x, character.position.y], HnHMaxZoom);
      this.map.setView(latlng, HnHMaxZoom);
      this.$router.push({path: `/character/${id}`});
      this.autoMode = true;
    },
    // Fires on every search-bar selection via @change (not a v-model
    // watcher), so re-picking the same single result always re-homes
    // instead of silently no-oping when Vue sees no value change.
    jumpToSearchResult(item) {
      if (!item) return;
      if (item.searchType === 'player') {
        this.trackingCharacterId = item.raw.id;
        this.jumpToCharacter(item.raw.id);
      } else {
        this.jumpToMapMarker(item.raw);
      }
      this.$nextTick(() => {
        this.searchSelection = null;
      });
    },
    toggleResourceType(cat) {
      let idx = this.selectedResourceTypes.indexOf(cat);
      if (idx === -1) {
        this.selectedResourceTypes.push(cat);
      } else {
        this.selectedResourceTypes.splice(idx, 1);
      }
    },
    toggleUnknownType(cat) {
      let idx = this.selectedUnknownTypes.indexOf(cat);
      if (idx === -1) {
        this.selectedUnknownTypes.push(cat);
      } else {
        this.selectedUnknownTypes.splice(idx, 1);
      }
    },
    toggleClutterType(cat) {
      let idx = this.selectedClutterTypes.indexOf(cat);
      if (idx === -1) {
        this.selectedClutterTypes.push(cat);
      } else {
        this.selectedClutterTypes.splice(idx, 1);
      }
    },
    // Extracted from the old overlayMap watcher body (unchanged internals)
    // so both picking an overlay map and toggling its visibility eye run
    // the exact same refresh -- turning visibility off temporarily hides
    // the overlay layer/markers (overlayLayer.map = -1) without clearing
    // the dropdown selection, and turning it back on restores from
    // whatever's currently selected.
    applyOverlayMap() {
      let value = (this.showOverlayMap && this.overlayMap) ? this.overlayMap : null;
      console.log("overlayMap");
      if (value) {
        this.overlayLayer.map = value.ID;
        this.overlayLayer.redraw();
        if (this.showMarkers) {
          this.otherMarks.forEach(it => it.remove(this));
          this.otherMarks.filter(it => (it.map === this.mapid || it.map === this.overlayLayer.map) && this.selectedResourceTypes.includes(it.type)).forEach(it => it.add(this));
        }
        if (this.showUnknownMarkers) {
          this.otherMarksAdmin.forEach(it => it.remove(this));
          this.otherMarksAdmin.filter(it => (it.map === this.mapid || it.map === this.overlayLayer.map) && this.selectedUnknownTypes.includes(it.type)).forEach(it => it.add(this));
        }
        if (this.showThingwalls) {
          this.thingMarks.forEach(it => it.remove(this));
          this.thingMarks.filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => {
            it.add(this);
            it.tooltip(this.showThingwallNames);
          });
        }
        if (this.showVortexes) {
          this.vortexMarks.forEach(it => it.remove(this));
          this.vortexMarks.filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => it.add(this));
        }
        if (this.showMineholes) {
          this.mineholeMarks.forEach(it => it.remove(this));
          this.mineholeMarks.filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => it.add(this));
        }
        if (this.showCaves) {
          this.caveMarks.forEach(it => it.remove(this));
          this.caveMarks.filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => it.add(this));
        }
        if (this.showQuests) {
          this.questMarks.forEach(it => it.remove(this));
          this.questMarks.filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => {
            it.add(this);
            it.tooltip(this.showQuestNames);
          });
        }
        if (this.showClutter) {
          this.clutterMarks.forEach(it => it.remove(this));
          this.clutterMarks.filter(it => (it.map === this.mapid || it.map === this.overlayLayer.map) && this.selectedClutterTypes.includes(it.type)).forEach(it => it.add(this));
        }
        if (this.showPlayers) {
          this.characters.getElements().forEach(it => it.remove(this));
          this.characters.getElements().filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => {
            it.add(this);
            it.tooltip(this.showPlayerTooltips);
          });
        }
        if (this.showRoads) {
          this.roads.getElements().forEach(it => it.remove(this));
          this.roads.getElements().filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => it.add(this));
        }
        if (this.showCustomMarkers) {
          this.customMarkers.getElements().forEach(it => it.remove(this));
          this.customMarkers.getElements().filter(it => it.map === this.mapid || it.map === this.overlayLayer.map).forEach(it => it.add(this));
        }
      } else {
        this.overlayLayer.map = -1;
        this.overlayLayer.redraw();
        if (this.showMarkers) {
          this.otherMarks.forEach(it => it.remove(this));
          this.otherMarks.filter(it => it.map === this.mapid && this.selectedResourceTypes.includes(it.type)).forEach(it => it.add(this));
        }
        if (this.showUnknownMarkers) {
          this.otherMarksAdmin.forEach(it => it.remove(this));
          this.otherMarksAdmin.filter(it => it.map === this.mapid && this.selectedUnknownTypes.includes(it.type)).forEach(it => it.add(this));
        }
        if (this.showThingwalls) {
          this.thingMarks.forEach(it => it.remove(this));
          this.thingMarks.filter(it => it.map === this.mapid).forEach(it => {
            it.add(this);
            it.tooltip(this.showThingwallNames);
          });
        }
        if (this.showVortexes) {
          this.vortexMarks.forEach(it => it.remove(this));
          this.vortexMarks.filter(it => it.map === this.mapid).forEach(it => it.add(this));
        }
        if (this.showMineholes) {
          this.mineholeMarks.forEach(it => it.remove(this));
          this.mineholeMarks.filter(it => it.map === this.mapid).forEach(it => it.add(this));
        }
        if (this.showCaves) {
          this.caveMarks.forEach(it => it.remove(this));
          this.caveMarks.filter(it => it.map === this.mapid).forEach(it => it.add(this));
        }
        if (this.showQuests) {
          this.questMarks.forEach(it => it.remove(this));
          this.questMarks.filter(it => it.map === this.mapid).forEach(it => {
            it.add(this);
            it.tooltip(this.showQuestNames);
          });
        }
        if (this.showClutter) {
          this.clutterMarks.forEach(it => it.remove(this));
          this.clutterMarks.filter(it => it.map === this.mapid && this.selectedClutterTypes.includes(it.type)).forEach(it => it.add(this));
        }
        if (this.showPlayers) {
          this.characters.getElements().forEach(it => it.remove(this));
          this.characters.getElements().filter(it => it.map === this.mapid).forEach(it => {
            it.add(this);
            it.tooltip(this.showPlayerTooltips);
          });
        }
        if (this.showRoads) {
          this.roads.getElements().forEach(it => it.remove(this));
          this.roads.getElements().filter(it => it.map === this.mapid).forEach(it => it.add(this));
        }
        if (this.showCustomMarkers) {
          this.customMarkers.getElements().forEach(it => it.remove(this));
          this.customMarkers.getElements().filter(it => it.map === this.mapid).forEach(it => it.add(this));
        }
      }
    },
    setupMap(characters, maps) {
      this.$http.get(`${API_ENDPOINT}/config`).then(response => {
        this.processConfig(response.body);
      }, () => this.$emit("error"));
      // Create map and layer
      this.map = L.map(this.$refs.map, {
        // Map setup
        minZoom: HnHMinZoom,
        maxZoom: HnHMaxZoom + 2,
        crs: HnHCRS,

        // Disable all visuals
        attributionControl: false,
        inertia: false,
        zoomAnimation: false,
        fadeAnimation: false,
        markerZoomAnimation: false
      });

      for (let id in maps) {
        let map = maps[id];
        map.text = map.Name;
        map.value = map.ID;
        this.maps.push(map);
      }
      this.maps.sort((a, b) => {
        return a.size < b.size;
      });

      // Update url on manual drag, zoom
      this.map.on("drag", () => {
        let point = this.map.project(this.map.getCenter(), this.map.getZoom());
        let coordinate = {x: ~~(point.x / TileSize), y: ~~(point.y / TileSize), z: this.map.getZoom()};
        this.$router.replace({path: `/grid/${this.mapid}/${coordinate.x}/${coordinate.y}/${coordinate.z}`}).catch(() => {});
        this.trackingCharacterId = -1;
      });
      this.map.on("zoom", () => {
        if (this.autoMode) {
          this.autoMode = false;
        } else {
          let point = this.map.project(this.map.getCenter(), this.map.getZoom());
          let coordinate = {
            x: Math.floor(point.x / TileSize),
            y: Math.floor(point.y / TileSize),
            z: this.map.getZoom()
          };
          this.$router.replace({path: `/grid/${this.mapid}/${coordinate.x}/${coordinate.y}/${coordinate.z}`}).catch(() => {});
          this.trackingCharacterId = -1;
        }
        // Thingwall icons scale with zoom (see Marker.js buildIcon) --
        // re-apply on every zoom change so they actually shrink/grow
        // live instead of only sizing correctly at the zoom level they
        // happened to be added at. Scoped to Thingwalls only, matching
        // the ask -- other marker types keep their static sizing.
        this.thingMarks.forEach(it => it.rescale(this));
      });

      this.layer = new SmartTileLayer('grids/{map}/{z}/{x}_{y}.png?{cache}', {
        minZoom: HnHMinZoom,
        maxZoom: HnHMaxZoom + 2,
        maxNativeZoom: HnHMaxZoom,
        zoomOffset: 0,
        zoomReverse: true,
        tileSize: TileSize
      });
      this.layer.invalidTile = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=';
      this.layer.addTo(this.map);

      this.overlayLayer = new SmartTileLayer('grids/{map}/{z}/{x}_{y}.png?{cache}', {
        minZoom: HnHMinZoom,
        maxZoom: HnHMaxZoom + 2,
        maxNativeZoom: HnHMaxZoom,
        zoomOffset: 0,
        zoomReverse: true,
        tileSize: TileSize,
        opacity: 0.6
      });
      this.overlayLayer.invalidTile = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=';
      this.overlayLayer.addTo(this.map);

      this.coordLayer = new GridCoordLayer({tileSize: TileSize, opacity: 0});
      this.coordLayer.addTo(this.map);

      this.markerLayer = L.layerGroup();
      this.markerLayer.addTo(this.map);

      this.roadLayer = L.layerGroup();
      this.roadLayer.addTo(this.map);

      this.thingwallConnectionsLayer = L.layerGroup();
      this.thingwallConnectionsLayer.addTo(this.map);

      /*this.map.on('mousemove', (mev) => {
          coords = this.map.project(mev.latlng, this.map.getZoom());
      })*/

      this.map.on('contextmenu', ((mev) => {
        if (this.drawingRoad) {
          return;
        }
        if (this.auths.includes('admin') || this.auths.includes('writer')) {
          let point = this.map.project(mev.latlng, this.map.getZoom());
          let coords = {x: Math.floor(point.x / TileSize), y: Math.floor(point.y / TileSize)};
          let markerPoint = this.map.project(mev.latlng, HnHMaxZoom);
          this.$refs.menu.open(mev.originalEvent, {
            coords: coords,
            point: {x: Math.round(markerPoint.x), y: Math.round(markerPoint.y)}
          });
        }
      }).bind(this));

      this.map.on('click', ((mev) => {
        if (mev.originalEvent.ctrlKey) {
          this.sendPing(mev.latlng);
          return;
        }
        if (!this.drawingRoad) {
          return;
        }
        let point = this.map.project(mev.latlng, HnHMaxZoom);
        this.roadPoints.push({x: Math.round(point.x), y: Math.round(point.y)});
        if (this.roadPoints.length === 1) {
          this.roadTempMarker = L.circleMarker(mev.latlng, {radius: 5, color: '#FDB800'}).addTo(this.map);
        } else if (this.roadPoints.length === 2) {
          this.drawingRoad = false;
          if (this.roadTempMarker) {
            this.map.removeLayer(this.roadTempMarker);
            this.roadTempMarker = null;
          }
          this.roadName = '';
          this.$modal.show('roadName');
        }
      }).bind(this));

      this.source = new EventSource("updates");
      this.source.onmessage = (function (event) {
        var updates = JSON.parse(event.data);
        for (var update of updates) {
          var key = update['M'] + ':' + update['X'] + ':' + update['Y'] + ':' + update['Z'];
          this.layer.cache[key] = update['T'];
          if (this.layer.map === update['M']) {
            this.layer.refresh(update['X'], update['Y'], update['Z']);
          }
        }
      }).bind(this);

      this.source.addEventListener('merge', ((e) => {
        var merge = JSON.parse(e.data);
        if (this.mapid === merge['From']) {
          let mapTo = merge['To'];
          let point = this.map.project(this.map.getCenter(), this.map.getZoom());
          let coordinate = {
            x: Math.floor(point.x / TileSize),
            y: Math.floor(point.y / TileSize),
            z: this.map.getZoom()
          };
          coordinate.x += merge['Shift'].x;
          coordinate.y += merge['Shift'].y;
          this.$router.replace({path: `/grid/${mapTo}/${coordinate.x}/${coordinate.y}/${coordinate.z}`}).catch(() => {});

          let latLng = this.toLatLng(coordinate.x * 100, coordinate.y * 100);

          this.changeMap(mapTo);
          this.$http.get(`${API_ENDPOINT}/v1/markers`).then(response => {
            this.updateMarkers(response.body);
          }, () => {
            this.$emit("error")
          });
          this.map.setView(latLng, this.map.getZoom());
        }
      }).bind(this));

      this.source.addEventListener('ping', ((e) => {
        let ping = JSON.parse(e.data);
        this.playPingSound();
        this.pingSnackbar = {
          show: true,
          text: `${ping.Username} pinged the map`,
          map: ping.Map,
          x: ping.X,
          y: ping.Y
        };
        if (this.mapid === ping.Map) {
          this.showPingMarker(this.toLatLng(ping.X, ping.Y));
        }
      }).bind(this));

      this.markers = new UniqueList();
      this.characters = new UniqueList();
      this.roads = new UniqueList();
      this.customMarkers = new UniqueList();

      // Create markers
      this.updateCharacters(characters);

      // Check parameters
      if (this.$route.params.characterId) { // Navigate to character
        this.trackingCharacterId = +this.$route.params.characterId;
      } else if (this.$route.params.gridX && this.$route.params.gridY && this.$route.params.zoom) { // Navigate to specific grid
        let latLng = this.toLatLng(this.$route.params.gridX * 100, this.$route.params.gridY * 100);

        if (this.mapid !== +this.$route.params.map) {
          this.changeMap(+this.$route.params.map);
        }

        this.map.setView(latLng, this.$route.params.zoom);
      } else { // Just show a map
        if (this.maps.length > 0) {
          this.changeMap(this.maps[0].ID);
        }
        this.map.setView([0, 0], HnHMinZoom);
      }

      this.intervalId = setInterval(() => {
        this.$http.get(`${API_ENDPOINT}/v1/characters`).then(response => {
          this.updateCharacters(response.body);
        }, () => {
          clearInterval(this.intervalId);
          this.$emit("error")
        });
      }, 2000);
      // Request markers
      this.$http.get(`${API_ENDPOINT}/v1/markers`).then(response => {
        this.updateMarkers(response.body);
      }, () => {
        this.$emit("error")
      });
      // Request roads
      this.$http.get(`${API_ENDPOINT}/v1/roads`).then(response => {
        this.updateRoads(response.body);
      }, () => {
        this.$emit("error")
      });
      // Request custom markers
      this.$http.get(`${API_ENDPOINT}/v1/customMarkers`).then(response => {
        this.updateCustomMarkers(response.body);
      }, () => {
        this.$emit("error")
      });
    },
    updateMarkers(markersData) {
      this.markers.update(markersData.map(it => {
            let m = new Marker(it);
            if (m.type === "thingwall")
              m.tstate = this.showThingwallNames;
            else if (m.type === "quest")
              m.tstate = this.showQuestNames;
            else
              m.tstate = false;
            return m;
          }),
          (marker) => { // Add
            let visible = marker.type === "thingwall" ? this.showThingwalls
                : marker.type === "watervortex" ? this.showVortexes
                : marker.type === "Mineshaft" ? this.showMineholes
                : marker.type === "Cave" ? this.showCaves
                : marker.type === "quest" ? this.showQuests
                : CLUTTER_TYPES.includes(marker.type) ? this.showClutter && this.selectedClutterTypes.includes(marker.type)
                : KNOWN_RESOURCE_TYPES.includes(marker.type.toLowerCase()) ? this.showMarkers && this.selectedResourceTypes.includes(marker.type)
                : this.showUnknownMarkers && this.selectedUnknownTypes.includes(marker.type);
            if (visible && (marker.map === this.mapid || marker.map === this.overlayLayer.map)) {
              marker.add(this);
            }
            marker.setClickCallback(() => {
              this.map.setView(marker.marker.getLatLng(), this.map.getZoom());
            });
            marker.setContextMenu((mev) => {
              if (this.auths.includes('admin') || this.auths.includes('writer')) {
                this.$refs.markermenu.open(mev.originalEvent, {name: marker.name, id: marker.id});
              }
            });
            if (marker.type === "thingwall") {
              marker.setHoverCallback(
                  (m) => this.showThingwallConnections(m),
                  () => this.clearThingwallConnections()
              );
            }
          },
          (marker) => { // Remove
            marker.remove(this);
          },
          (marker, updated) => { // Update
            marker.update(this, updated);
          });
      // this.markersCache.length = 0;
      // this.markers.getElements().forEach(it => this.markersCache.push(it));
      /*this.markersCache.sort((a, b) => {
        let im = a.image.localeCompare(b.image);
        return im === 0 ? a.name.localeCompare(b.name) : im;
      });*/

      this.allMarks.length = 0;
      this.otherMarks.length = 0;
      this.otherMarksAdmin.length = 0;
      this.thingMarks.length = 0;
      this.vortexMarks.length = 0;
      this.mineholeMarks.length = 0;
      this.caveMarks.length = 0;
      this.questMarks.length = 0;
      this.clutterMarks.length = 0;
      this.clutterCategories.length = 0;
      this.markers.getElements().filter(it => it.name != null && it.name.length > 0 && !it.hidden).sort((a, b) => {
        let im = a.image.localeCompare(b.image);
        return im === 0 ? a.name.localeCompare(b.name) : im;
      }).forEach(it => {
        // gfx/terobjs/mm/custom covers two very different things: the
        // dedicated admin-placed Custom Markers feature (its own separate
        // system entirely, not touched here) and this Mapper's own
        // auto-generated "entrance marker" / last-known-position markers
        // dropped whenever a new map layer is discovered. The latter are
        // meta/navigational, not real game content, so they're excluded
        // from every visible bucket entirely rather than folded into
        // Clutter (which should stay actual game-world resources) or the
        // resource panel.
        if (it.image === "gfx/terobjs/mm/custom") {
          return;
        }
        this.allMarks.push(it);
        if (it.type === "thingwall")
          this.thingMarks.push(it);
        else if (it.type === "watervortex")
          this.vortexMarks.push(it);
        else if (it.type === "Mineshaft")
          this.mineholeMarks.push(it);
        else if (it.type === "Cave")
          this.caveMarks.push(it);
        else if (it.type === "quest")
          this.questMarks.push(it);
        else if (CLUTTER_TYPES.includes(it.type)) {
          this.clutterMarks.push(it);
          if (!this.clutterCategories.includes(it.type))
            this.clutterCategories.push(it.type);
        }
        else if (KNOWN_RESOURCE_TYPES.includes(it.type.toLowerCase())) {
          this.otherMarks.push(it);
          if (!this.marksCategories.includes(it.type))
            this.marksCategories.push(it.type);
        } else {
          this.otherMarksAdmin.push(it);
          if (!this.unknownCategories.includes(it.type))
            this.unknownCategories.push(it.type);
        }
      });
    },
    showThingwallConnections(marker) {
      this.thingwallConnectionsLayer.clearLayers();
      let neighbors = this.thingMarks
          .filter(it => it !== marker && it.map === marker.map && it.marker)
          .map(it => ({
            marker: it,
            dist: Math.hypot(it.position.x - marker.position.x, it.position.y - marker.position.y)
          }))
          .sort((a, b) => a.dist - b.dist)
          .slice(0, 3);
      let from = marker.marker.getLatLng();
      neighbors.forEach(n => {
        L.polyline([from, n.marker.marker.getLatLng()], {
          color: '#00cffd',
          weight: 2,
          opacity: 0.8,
          dashArray: '6, 6'
        }).addTo(this.thingwallConnectionsLayer);
      });
    },
    clearThingwallConnections() {
      this.thingwallConnectionsLayer.clearLayers();
    },
    updateRoads(roadsData) {
      this.roads.update(roadsData.map(it => new Road(it)),
          (road) => { // Add
            if (this.showRoads && (road.map === this.mapid || road.map === this.overlayLayer.map)) {
              road.add(this);
            }
            road.setContextMenu((mev) => {
              if (this.auths.includes('admin') || this.auths.includes('writer')) {
                this.$refs.roadmenu.open(mev.originalEvent, {name: road.name, id: road.id});
              }
            });
          },
          (road) => { // Remove
            road.remove(this);
          }
      );
    },
    toggleDrawRoad() {
      this.drawingRoad = !this.drawingRoad;
      this.roadPoints = [];
      if (this.roadTempMarker) {
        this.map.removeLayer(this.roadTempMarker);
        this.roadTempMarker = null;
      }
    },
    saveRoad() {
      if (!this.roadName || this.roadPoints.length !== 2) {
        return;
      }
      this.$http.get(`${API_ENDPOINT}/admin/addRoad`, {
        params: {
          map: this.mapid,
          ax: this.roadPoints[0].x,
          ay: this.roadPoints[0].y,
          bx: this.roadPoints[1].x,
          by: this.roadPoints[1].y,
          name: this.roadName
        }
      }).then(response => {
        this.updateRoads([...this.roads.getElements(), response.body]);
        this.$modal.hide('roadName');
        this.roadPoints = [];
      }, () => this.$emit("error"));
    },
    deleteRoad(data) {
      this.$http.get(`${API_ENDPOINT}/admin/deleteRoad`, {params: {id: data.id}});
      let road = this.roads.byId(data.id);
      if (road) {
        road.remove(this);
        delete this.roads.elements[data.id];
      }
    },
    updateCustomMarkers(markersData) {
      this.customMarkers.update(markersData.map(it => new CustomMarker(it)),
          (marker) => { // Add
            if (this.showCustomMarkers && (marker.map === this.mapid || marker.map === this.overlayLayer.map)) {
              marker.add(this);
            }
            marker.setContextMenu((mev) => {
              if (this.auths.includes('admin') || this.auths.includes('writer')) {
                this.$refs.custommarkermenu.open(mev.originalEvent, {name: marker.name, id: marker.id});
              }
            });
          },
          (marker) => { // Remove
            marker.remove(this);
          }
      );
      this.customMarks.length = 0;
      this.customMarkers.getElements().filter(it => it.name != null && it.name.length > 0).sort((a, b) => {
        return a.name.localeCompare(b.name);
      }).forEach(it => this.customMarks.push(it));
    },
    promptCustomMarker(data) {
      this.pendingMarkerPoint = data.point;
      this.customMarkerName = '';
      this.customMarkerColor = 'white';
      this.$modal.show('customMarkerName');
    },
    saveCustomMarker() {
      if (!this.customMarkerName || !this.pendingMarkerPoint) {
        return;
      }
      this.$http.get(`${API_ENDPOINT}/admin/addCustomMarker`, {
        params: {
          map: this.mapid,
          x: this.pendingMarkerPoint.x,
          y: this.pendingMarkerPoint.y,
          name: this.customMarkerName,
          color: this.customMarkerColor
        }
      }).then(response => {
        this.updateCustomMarkers([...this.customMarkers.getElements(), response.body]);
        this.$modal.hide('customMarkerName');
        this.pendingMarkerPoint = null;
      }, () => this.$emit("error"));
    },
    deleteCustomMarker(data) {
      this.$http.get(`${API_ENDPOINT}/admin/deleteCustomMarker`, {params: {id: data.id}});
      let marker = this.customMarkers.byId(data.id);
      if (marker) {
        marker.remove(this);
        delete this.customMarkers.elements[data.id];
      }
    },
    updateCharacters(charactersData) {
      this.characters.update(charactersData.map(it => {
            let ch = new Character(it);
            ch.tstate = this.showPlayerTooltips;
            ch.hideCharacterNames = this.hideCharacterNames;
            return ch;
          }),
          (character) => { // Add
            if (this.showPlayers) {
              character.add(this);
            }
            character.setClickCallback(() => { // Zoom to character on marker click
              this.trackingCharacterId = character.id;
            });
          },
          (character) => { // Remove
            character.remove(this);
          },
          (character, updated) => { // Update
            if (this.trackingCharacterId === updated.id) {
              if (this.mapid !== updated.map) {
                this.changeMap(updated.map);
              }
              let latlng = this.map.unproject([updated.position.x, updated.position.y], HnHMaxZoom);
              this.map.setView(latlng, this.map.getZoom());
            }
            character.update(this, updated, this.showPlayers);
          }
      );
      this.players.length = 0;
      this.characters.getElements().forEach(it => this.players.push(it));
    },
    processConfig(config) {
      document.title = config.title;
      this.auths = config.auths;
      this.username = config.username;
      this.isPublic = !!config.public;
      // Personal preferences from the account's own /settings page --
      // omitted by the server for an account that's never visited it, so
      // keep this component's own default (true) in that case instead of
      // falling back to JS's undefined-is-falsy.
      if (config.showPlayerNames !== undefined) this.showPlayerTooltips = config.showPlayerNames;
      if (config.hideCharacterNames !== undefined) this.hideCharacterNames = config.hideCharacterNames;
      if (config.thingwallScale !== undefined) this.thingwallScale = config.thingwallScale;
      // A section this account isn't granted (see canSeeSection/v-if in
      // the template) isn't just hidden from the settings drawer -- its
      // underlying data is forced off too, so it can't show just because
      // a toggle happened to default to "on".
      if (!this.canSeeSection('naturalMarkers')) this.showMarkers = false;
      if (!this.canSeeSection('naturalMarkers')) this.showMineholes = false;
      if (!this.canSeeSection('naturalMarkers')) this.showCaves = false;
      if (!this.canSeeSection('other')) this.showUnknownMarkers = false;
      if (!this.canSeeSection('thingwalls')) this.showThingwalls = false;
      if (!this.canSeeSection('vortexes')) this.showVortexes = false;
      if (!this.canSeeSection('questGivers')) this.showQuests = false;
      if (!this.auths.includes('point')) this.showPlayers = false;
      if (!this.canSeeSection('roads')) this.showRoads = false;
      if (!this.canSeeSection('customMarkers')) this.showCustomMarkers = false;
      if (!this.canSeeSection('clutter')) this.showClutter = false;
    },
    // Each Markers section is granted per-account as an ordinary auth
    // string "sec_<key>" (see SectionOptions in admin.go / the Markers
    // checkbox group in user.tmpl) -- same mechanism as Map/Writer/Admin.
    canSeeSection(key) {
      return this.auths.includes('sec_' + key);
    },
    toLatLng(x, y) {
      return this.map.unproject([x, y], HnHMaxZoom);
    },
    sendPing(latlng) {
      let point = this.map.project(latlng, HnHMaxZoom);
      this.$http.get(`${API_ENDPOINT}/v1/ping`, {
        params: {map: this.mapid, x: Math.round(point.x), y: Math.round(point.y)}
      });
    },
    jumpToPing() {
      this.pingSnackbar.show = false;
      if (this.mapid !== this.pingSnackbar.map) {
        this.changeMap(this.pingSnackbar.map);
      }
      let latLng = this.toLatLng(this.pingSnackbar.x, this.pingSnackbar.y);
      this.map.setView(latLng, HnHMaxZoom);
      this.showPingMarker(latLng);
    },
    showPingMarker(latlng) {
      if (this.pingMarker) {
        this.map.removeLayer(this.pingMarker);
        this.pingMarker = null;
      }
      this.pingMarker = L.circleMarker(latlng, {radius: 12, color: '#FF0000', weight: 3, fillColor: '#FF0000', fillOpacity: 0.3}).addTo(this.map);
      setTimeout(() => {
        if (this.pingMarker) {
          this.map.removeLayer(this.pingMarker);
          this.pingMarker = null;
        }
      }, 8000);
    },
    playPingSound() {
      let audio = new Audio('/map/api/v1/pingSound?t=' + Date.now());
      audio.addEventListener('error', () => this.playDefaultPingSound());
      audio.play().catch(() => this.playDefaultPingSound());
    },
    playDefaultPingSound() {
      let ctx = new (window.AudioContext || window.webkitAudioContext)();
      let osc = ctx.createOscillator();
      let gain = ctx.createGain();
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.frequency.value = 880;
      gain.gain.setValueAtTime(0.2, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.4);
      osc.start();
      osc.stop(ctx.currentTime + 0.4);
    },
    wipeTile(data) {
      this.$http.get(`${API_ENDPOINT}/admin/wipeTile`, {params: {...data.coords, map: this.mapid}});
    },
    hideMarker(data) {
      this.$http.get(`${API_ENDPOINT}/admin/hideMarker`, {params: {id: data.id}});
      this.markers.byId(data.id).remove(this);
    },
    queryCoordSet(data) {
      this.coordSetFrom = data.coords;
      this.$modal.show('coordSet');
    },
    setCoords(form) {
      this.$http.get(`${API_ENDPOINT}/admin/setCoords`, {
        params: {
          map: this.mapid,
          fx: this.coordSetFrom.x,
          fy: this.coordSetFrom.y,
          tx: this.coordSet.x,
          ty: this.coordSet.y,
        }
      });
    },
    changeMap(mapid) {
      if (mapid !== this.mapid) {
        this.mapid = mapid;
        this.layer.map = this.mapid;
        this.layer.redraw();
        this.overlayLayer.map = -1;
        this.overlayLayer.redraw();
        if (this.showMarkers) {
          this.otherMarks.forEach(it => it.remove(this));
          this.otherMarks.filter(it => it.map === this.mapid && this.selectedResourceTypes.includes(it.type)).forEach(it => {
            it.add(this);
            it.tooltip(false);
          });
        }
        if (this.showUnknownMarkers) {
          this.otherMarksAdmin.forEach(it => it.remove(this));
          this.otherMarksAdmin.filter(it => it.map === this.mapid && this.selectedUnknownTypes.includes(it.type)).forEach(it => {
            it.add(this);
            it.tooltip(false);
          });
        }
        if (this.showThingwalls) {
          this.thingMarks.forEach(it => it.remove(this));
          this.thingMarks.filter(it => it.map === this.mapid).forEach(it => {
            it.add(this);
            it.tooltip(this.showThingwallNames);
          });
        }
        if (this.showVortexes) {
          this.vortexMarks.forEach(it => it.remove(this));
          this.vortexMarks.filter(it => it.map === this.mapid).forEach(it => it.add(this));
        }
        if (this.showMineholes) {
          this.mineholeMarks.forEach(it => it.remove(this));
          this.mineholeMarks.filter(it => it.map === this.mapid).forEach(it => it.add(this));
        }
        if (this.showCaves) {
          this.caveMarks.forEach(it => it.remove(this));
          this.caveMarks.filter(it => it.map === this.mapid).forEach(it => it.add(this));
        }
        if (this.showQuests) {
          this.questMarks.forEach(it => it.remove(this));
          this.questMarks.filter(it => it.map === this.mapid).forEach(it => {
            it.add(this);
            it.tooltip(this.showQuestNames);
          });
        }
        if (this.showClutter) {
          this.clutterMarks.forEach(it => it.remove(this));
          this.clutterMarks.filter(it => it.map === this.mapid && this.selectedClutterTypes.includes(it.type)).forEach(it => it.add(this));
        }
        if (this.showPlayers) {
          this.characters.getElements().forEach(it => it.remove(this));
          this.characters.getElements().filter(it => it.map === this.mapid).forEach(it => {
            it.add(this);
            it.tooltip(this.showPlayerTooltips);
          });
        }
        if (this.showRoads) {
          this.roads.getElements().forEach(it => it.remove(this));
          this.roads.getElements().filter(it => it.map === this.mapid).forEach(it => it.add(this));
        }
        if (this.showCustomMarkers) {
          this.customMarkers.getElements().forEach(it => it.remove(this));
          this.customMarkers.getElements().filter(it => it.map === this.mapid).forEach(it => it.add(this));
        }
      }
    }
  }
}
</script>

<style>
.container {
  max-width: 100%;
  padding: 0px !important;
}

.map {
  width: 100vw;
  height: calc(100vh - 56px);
}

.leaflet-container {
  background: #000;
}

.player-pulse-icon {
  background: transparent;
  border: none;
}

.custom-marker-icon {
  background: transparent;
  border: none;
}

.color-swatches {
  display: flex;
  gap: 8px;
  margin: 10px 0;
}

.color-swatch {
  display: inline-block;
  width: 26px;
  height: 36px;
  cursor: pointer;
  padding: 3px;
  border-radius: 4px;
}

.color-swatch.selected {
  outline: 2px solid #FF9800;
  background: rgba(255, 152, 0, 0.15);
}

.player-pulse {
  position: relative;
  width: 24px;
  height: 24px;
}

.player-pulse-dot {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 15px;
  height: 15px;
  margin: -7.5px 0 0 -7.5px;
  background: #ff0000;
  border-radius: 50%;
  box-shadow: 0 0 4px rgba(255, 0, 0, 0.9);
}

.player-pulse-ring {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 15px;
  height: 15px;
  margin: -7.5px 0 0 -7.5px;
  border-radius: 50%;
  background: rgba(255, 0, 0, 0.7);
  animation: player-pulse-anim 1.5s ease-out infinite;
}

@keyframes player-pulse-anim {
  0% {
    transform: scale(1);
    opacity: 0.8;
  }
  100% {
    transform: scale(3.5);
    opacity: 0;
  }
}

.leaflet-control {
  margin: auto !important;
}

.map-tile {
  border-bottom: 1px solid #404040;
  border-right: 1px solid #404040;
  color: #404040;
  font-size: 12px;
}

.map-tile-text {
  position: absolute;
  left: 2px;
  top: 2px;
  color: #FDB800;
  font-size: 10px;
  text-shadow: -1px -1px #000, 1px 1px #000, -1px 1px #000, 1px -1px #000;
}

.control-panel {
  position: absolute;
  top: 10%;
  left: 10px;
  z-index: 502;
}

.v-list-item {
  padding: 0px !important;
  min-height: 0px !important;
  margin-left: 0 !important;
  height: auto !important;
}

.v-btn {
  padding: 0px !important;
}

.short-btn {
  min-height: auto !important;
  height: auto !important;
  text-transform: none !important;
}

.v-list {
  padding: 5px !important;
  height: auto !important;
  min-height: 0px !important;
}

.v-navigation-drawer__content {
  padding: 0px !important;
}

.v-navigation-drawer {
  width: auto !important;
  /* Vuetify's automatic app-bar/drawer offset coordination isn't kicking
     in here (drawer was rendering at top:0, directly under the 56px
     dense app-bar instead of below it) -- pin it explicitly rather than
     keep fighting whatever's suppressing the automatic calculation. */
  top: 56px !important;
}

.v-app-bar {
  /* By default Vuetify shrinks the app-bar to make room for the drawer
     beside it (matching :mini-variant-width, 36px collapsed). The design
     wants the bar spanning the full width edge-to-edge instead, with the
     drawer sitting entirely below it -- overriding the horizontal
     coordination while leaving the vertical (drawer top:56px above)
     alone. */
  left: 0 !important;
  width: 100% !important;
}

.v-text-field__details {
  min-height: 0px !important;
  margin: 0px !important;
}

.v-messages {
  min-height: 0px !important;
}

.v-list-item__content {
  padding: 0px !important;
}

.row-icons {
  /* Fixed-width gutter for each drawer row's icon-toggle(s) -- rows with
     one icon (Grid Coordinates) and rows with two (Thingwalls, Quest
     Givers, Players) previously pushed their label to different X
     offsets since the icons sat directly inline before it. Reserving the
     same width regardless of how many icons a row actually has keeps
     every label starting at the same position. Sized for the widest
     case (two icons: ~24px + 8px margin each). */
  display: flex;
  align-items: center;
  width: 64px;
  flex-shrink: 0;
}

.row-marker-icons {
  /* Trailing, right-aligned counterpart to .row-icons -- shows the
     actual in-game marker image(s) for a row (Thingwalls, Vortexes,
     Mineholes & Caves), aligned among themselves via a shared fixed
     width, same reasoning as .row-icons but for decorative reference
     images instead of functional toggles. Sized for the widest case
     (Mineholes & Caves: two images side by side). */
  display: flex;
  align-items: center;
  justify-content: flex-end;
  width: 48px;
  flex-shrink: 0;
}

.v-input__slot {
  padding: 0px 5px !important;
}

.leaflet-tooltip {
  background-color: transparent !important;
  border: none !important;
  box-shadow: none !important;
  color: #FDB800 !important;
  font-size: 10px !important;
  text-shadow: -1px -1px #000, 1px 1px #000, -1px 1px #000, 1px -1px #000 !important;
}

.leaflet-tooltip-top:before,
.leaflet-tooltip-bottom:before,
.leaflet-tooltip-left:before,
.leaflet-tooltip-right:before {
  border: none !important;
}

.leaflet-popup-content-wrapper {
  background: transparent !important;
  box-shadow: none !important;
}

.leaflet-popup-content {
  font-size: 13px !important;
  color: #ffffff !important;
  text-shadow: -1px -1px #000, 1px 1px #000, -1px 1px #000, 1px -1px #000 !important;
  text-align: center !important;
}

.leaflet-popup-tip {
  display: none !important;
}

.hidden {
  display: none;
}

@import '~vue-context/dist/css/vue-context.css';
</style>