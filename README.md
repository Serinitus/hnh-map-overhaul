# HavenMap

Fork of [Cediner/hnh-map-vuetify](https://github.com/Cediner/hnh-map-vuetify), an automapper server for Haven & Hearth.
Subsequently forked from [Aritain/hnh-map-updated](https://github.com/Aritain/hnh-map-updated).

All changes on top of upstream are vibe-coded via Claude.

## Setup

    docker build -t "your_tag" .
    docker run -v /srv/hnh-map:/map -p 80:8080 "your_tag"

Listens internally on port 8080, expects `/map` mounted as a volume (database and images stored here). Put it behind whatever reverse proxy you like and point an auto-mapping-capable client at it.

Login as `admin`/`admin`, go to the admin portal, add your first user with all roles toggled on (you'll need `admin` at least). You'll be logged out and the `admin` account removed — log back in as your new user. Add accounts for everyone else from there, then generate upload tokens.

Set the prefix (prepended to tokens, e.g. `http://example.com`) to make client configuration easier.

The first client to connect sets the 0,0 grid. Wipe data in the admin portal to reset it.

### Roles

- `map`: view the map
- `upload`: send character, marker, and tile data
- `admin`: modify server settings, manage users, wipe data

## Changelog

### Aritain's changes

Changes on top of the upstream fork:

- Roads: draw and label named roads between two points on the map
- Custom markers: user-placed markers in 8 colors, independent of client uploads
- Map pings: ctrl+click broadcasts a ping (sound + snackbar + temp marker) to everyone with the map open, over the existing SSE update channel; admin can upload/select the ping sound
- Cave-entrance icons now render on the map (I'm still not sure if that's something ND fixed on client side or my changes made this work)
- Fixed a marker-upload bug where a non-JSON hex `id` field from some clients silently dropped the whole marker batch
- Fixed broken tile zoom resolution (`_getZoomForUrl`) in the tile layer
- Player markers: pulsing indicator icon, always-on nametag tooltip
- Thingwall triangulation and general marker/tooltip behavior improvements
- Dark theme by default
- Markers and quest tooltips visible by default
- Closer zoom
- Better icon handling while zooming

### Serinitus's changes

Changes on top of `Aritain/hnh-map-updated` (full technical detail --
files touched, root causes, verification notes -- in `CHANGES.md`):

**Bug fixes**

- Minehole/ladder markers no longer cause issues regardless of client
  (Kami, Hurricane) — recognized and rendered correctly regardless of
  which client uploaded them. **Warning:** the Nurgling-side crash bug
  itself is still unfixed upstream and will still crash any non-Nurgling
  client until that's addressed there.
- Fixed Clutter/system markers leaking into the Natural Resources panel
- New mine layers now align to their entrance point instead of resetting
  to (0,0)
- Kami's "Export 2 Mapper" bulk upload now actually works (run it twice
  — the second pass captures far more than the first)
- Turning off Players now actually stays off (was silently reappearing
  every 2-second poll)

**Public access & permissions**

- Per-account control over which map sections (Natural Resources,
  Thingwalls, Roads, etc.) are visible
- New Public-access mode — share a read-only map with no login
  required, one master switch
- Per-map "Required Tier" locking

**Personal settings**

- New `/settings` page: Players Names, Hide Character Names, and a
  Thingwall Icon Scale slider (0.25x–4x) — personal preferences instead
  of admin-managed drawer toggles

**Drawer redesign**

- Every toggle is now a green/red icon instead of a switch; Thingwalls/
  Quest Givers/Players get an independent show/hide-names icon
- One unified search bar replaces four separate Thingwall/NPC/Marker/
  Player dropdowns — also fixes re-selecting the same result doing
  nothing
- Natural Resources (renamed from Natural Markers) and Clutter get
  per-category toggle panels instead of one blanket switch
- Mineholes and Caves split out into their own standalone toggles
- "Draw Road" button replaced with an inline Edit icon next to Roads
- Overlay Map gets its own independent visibility toggle
- Thingwall icons now scale with zoom to match the live server, instead
  of staying a fixed size
- Layout fixes: app bar spans the full width, drawer no longer overlaps
  it, row/label alignment cleaned up throughout
- Only Thingwalls and Vortexes are visible by default now
