# Changes from stock Aritain/hnh-map-updated

This is a fresh clone of `Aritain/hnh-map-updated` (`master`, commit
`c8bfe9c`) with the following changes applied on top. Each item covers
what changed, which files, why, and anything worth knowing before relying
on it further.

## 1. Minehole/Ladder markers now recognized and displayed correctly across Nurgling and Kami, with proper icons

**What it does:** Minehole/ladder markers previously either crashed
non-Nurgling clients outright or landed as unlabeled generic markers,
depending on which client uploaded them. Both are now recognized and
rendered with the correct icon regardless of source client.

**Why:** Nurgling's own minehole/ladder icon overrides (`mm/down`/`mm/up`)
are local to Nurgling's client jar -- they don't exist on the live game
server. Nurgling was uploading those short names directly as a marker's
`image`, so any other mapv4 client (Kami, Hurricane) that downloaded and
tried to render one hit an uncaught `Resource$NoSuchResourceException` and
crashed. Separately, Kami has its own auto-marking feature
(`AutoMarkers`/`automark.json5`) for minehole, ladder, tarpit, amber wash,
flint wash, cave-in, and cave-out -- but Kami's own upload code
(`MappingClient.ProcessMapper`) never sets the `image`/`type` fields for
that marker class, only for its other two marker types. Those markers
still upload, just blank, and previously landed as unlabeled generic
custom markers.

**Files:** `client.go` (`kamiAutoMarkNames`, in `uploadMarkers`),
`frontend/src/data/Marker.js` (`detectType`/icon selection),
`frontend/public/mm/{down,up}.png` (new icon assets -- these didn't exist
in this repo before and are what the icon URLs actually resolve to)

**How it works:** The actual fix for Nurgling's crash is client-side (in
Nurgling's own `mapv4/Requestor.java`, not part of this repo) -- it now
translates `mm/down`/`mm/up` to the real, universally-resolvable gob
resources `gfx/terobjs/minehole`/`gfx/terobjs/ladder` (verified live,
HTTP 200) at upload time. This repo's `Marker.js` recognizes both the old
and new names so already-stored markers keep working. For Kami's blank
uploads, `uploadMarkers`'s "default to generic custom marker" fallback now
first checks the marker's `name` against Kami's own `automark.json5`
display names ("Minehole", "Ladder", "Tarpit", "Amber Wash", "Flint Wash",
"Cave", "Exit") and recovers the correct image from that instead.

**Potential complications:**
- Name-matching for the Kami recovery is a heuristic, not a real
  identifier -- a marker manually named exactly "Ladder" or "Cave" for
  unrelated reasons would be mis-recognized. Low-stakes (worst case is a
  cosmetically wrong icon), but not a precise match.
- Only fixes *new* uploads. Markers already stored under the old
  `mm/down`/`mm/up` names, or already landed as generic custom markers,
  stay that way -- the upload handler dedupes by exact grid+position and
  skips writing if something already exists there.
- Verified against the live production database (audited separately, not
  part of this repo) that every historical minehole/ladder marker came
  from Nurgling's old convention -- Kami's `radar/*` naming has never
  actually appeared in real data, so this is aimed at correctness/future
  use more than a fix for an observed live problem.

## 2. Natural Resources: per-category toggle sliders added

**What it does:** The "Natural Markers" section now has an individual
toggle per resource type (Mineshaft, Burrow, Clam Reef, whatever else
appears in the data), each with its own icon, instead of one blanket
show/hide switch for everything.

**Why:** Stock Aritain already tracked `marksCategories` (a list of
distinct marker types) but never consumed it for anything -- there was no
UI built on top of it, so it was dead data. This adds that UI.

**Files:** `frontend/src/components/MapView.vue`

**How it works:** New `selectedResourceTypes` array (which categories are
currently checked) and `resourceCategoryIcons` computed property (one
representative icon per category, hardcoded to always use the minehole
icon for the merged "Mineshaft" category regardless of which marker
populated it first). A `toggleResourceType(cat)` method flips a category
in/out of `selectedResourceTypes`. All five places that filter/re-render
resource markers (initial load, map change, overlay toggle, the
`showMarkers` toggle, and the new `selectedResourceTypes` watcher) now
check `selectedResourceTypes.includes(marker.type)` in addition to the
existing checks.

**Potential complications:**
- All categories start **unchecked** by default -- nothing shows until a
  category is explicitly toggled on. This matches the reference UX this
  was modeled after, but is a real behavior difference from stock
  Aritain's "everything shows unless the blanket toggle is off" default.
  Worth confirming that's the intended default before relying on it.

## 3. Fixed resource panel leaking Clutter/system markers into the wrong section

**What it does:** Fixes a bug this repo's own change (item 2) exposed --
Burrow and other `CLUTTER_TYPES` markers, along with this Mapper's
auto-generated entrance/last-known-position markers, were showing up in
the new per-category resource panel instead of staying in their own
sections.

**Why:** `marksCategories.push(it.type)` was unconditional in stock
Aritain's `updateMarkers()` -- outside the if/else chain that routes
markers into `thingMarks`/`questMarks`/`clutterMarks`/`otherMarks`. It
collected the type of *every* marker regardless of which bucket it
actually went into. This had zero visible effect before, since nothing
ever read `marksCategories` (see item 2) -- it only became a real bug once
something finally consumed that list.

**Files:** `frontend/src/components/MapView.vue` (`updateMarkers`)

**How it works:** Moved the `marksCategories` push inside the `else`
branch, so only markers actually routed to `otherMarks` (real resource
types) populate it. Separately, entrance/last-known-position markers
(`image === "gfx/terobjs/mm/custom"`, generated by this Mapper itself when
a new map layer is discovered -- see item 4) are now excluded from every
visible bucket entirely, rather than being folded into Clutter (which
should stay actual game-world resources) or showing as their own
resource-panel category.

**Potential complications:**
- The dedicated, admin-placed "Custom Markers" feature (a completely
  separate system -- its own backend bucket, its own API, its own
  `showCustomMarkers` toggle) is untouched by this. The exclusion here
  only affects auto-generated markers that happen to share the same
  generic image string, not that feature.

## 4. New mine layers now align to their entry point instead of resetting to {0,0}

**What it does:** When a client reports grids for a brand-new,
previously-unregistered map (e.g. descending into an unmapped mineshaft),
the first grid's coordinate is now seeded from the coordinate of the grid
the player was standing on immediately before the transition, instead of
always starting at `{0, 0}`. Mine layers connected by a ladder/minehole
now tend to line up spatially instead of each resetting its own
coordinate frame. As a side effect, a marker labeled with the new layer's
name gets dropped at the entry point on the layer the player came from, so
the connection is visible from either side.

**Files:** `main.go` (`LastPosition` type, `Map.lastPos` field),
`client.go` (`updatePositions`, `gridUpdate`)

**How it works:** `updatePositions` now records each player's last known
position (map, grid, local offset, timestamp) keyed by username, pulled
from the request context that was already wired through this repo's
`client()` dispatcher for other handlers. `gridUpdate`'s new-map-creation
path checks for a recent (<30s) last position on a *different* map; if
found, it seeds the new map's coordinate origin from that position's grid
coordinate and drops the entrance marker.

**Potential complications:**
- Known limitation: teleports (not physical ladder/minehole transitions)
  will seed an arbitrary/wrong origin on first discovery of a location
  reached that way. Self-corrects the next time that location is reached
  via its actual physical connection, since the grids will already be
  registered by then.
- New per-username in-memory state (`lastPos`) that didn't exist before --
  small footprint, never persisted, cleared implicitly by the 30-second
  staleness check but never actively evicted otherwise. Not a real problem
  in practice (bounded by distinct usernames ever seen), just worth
  knowing it's there.

## 5. Compatibility with Kami's "Export 2 Mapper (V1)" **(needs to run twice, but does function)**

**What it does:** Kami's map window has an "Export 2 Mapper (V1)" button
(`MapFile.exportToMapper`) that bulk-uploads a player's entire locally
cached map history in one pass, separate from the normal per-login
upload. It was calling three routes (`gridCacheUpdate`, `gridCacheUpload`,
`gridCacheOverlayUpload`) this repo never implemented at all, so every
request 404'd silently and the button has likely never worked against any
modern mapv4 server.

**Files:** `client.go`

**How it works:** Added the three routes to the `/client/{token}/{action}`
dispatcher. `gridCacheUpdate` registers an entire segment's grids at once
(the payload already carries relative x/y per grid, unlike the
position-tracking-oriented `gridUpdate`), reusing an existing map if any
given grid is already known, or creating a new one otherwise.
`gridCacheUpload` aliases the existing `gridUpload` handler directly,
since the multipart payload shape already matches. `gridCacheOverlayUpload`
accepts and discards -- overlay tiles aren't rendered anywhere in this
codebase.

**Potential complications -- confirmed by testing, not theoretical:**
- Grid *registration* reaches ~100% of the reported total in a single
  pass every time (it's pure metadata, always available).
- Tile *image* uploads are bottlenecked client-side, not server-side:
  Kami re-renders each tile from local cache before uploading, and a grid
  whose data isn't currently loaded produces no image and no request at
  all -- not a rejection, just nothing sent. A first run typically
  captures a small fraction of the total. Running the export a second
  time (from the same location, without even moving) captures dramatically
  more -- most likely because the first pass's render attempts trigger
  background data loading that the second pass then benefits from. In one
  test: run 1 landed 155/16,738 tiles; run 2 (same spot, no movement)
  reached 419+ and climbing. A separate earlier test on another instance
  went from 150 to 16,632 between runs. **Practical takeaway: always run
  the export at least twice before concluding it under-performed.**

## 6. Natural Resources panel filtered to known types, with an admin-only "Other" section for anything unrecognized

**What it does:** The per-category resource toggle panel (item 2) now only
lists types this Mapper actually recognizes as real game resources. Any
auto-marked type that isn't recognized is routed to a separate "Other
(Admin)" section instead of adding a new row to the main panel -- visible
only to admin accounts, off by default.

**Why:** Item 2 made every distinct marker type spawn its own toggle row,
including malformed/garbage types from client upload bugs -- exactly the
kind of "random broken resource fragments" the per-category panel was
supposed to clean up in the first place. Cross-checking against another
public Haven & Hearth mapper's own marker legend gave a concrete list of
real, vetted resource categories to allowlist against.

**Files:** `frontend/src/components/MapView.vue` (`KNOWN_RESOURCE_TYPES`,
`otherMarksAdmin`/`unknownCategories`/`selectedUnknownTypes` state, the
routing split in `updateMarkers`, the "Other (Admin)" panel)

**How it works:** A marker's type (already computed the same way as before
-- see item 1/2) is checked against `KNOWN_RESOURCE_TYPES`. Known types
route into the existing `otherMarks`/`marksCategories` pair unchanged.
Unknown types route into a parallel `otherMarksAdmin`/`unknownCategories`
pair, rendered as its own toggle list gated behind `auths.includes('admin')`
in the template, structured identically to the main panel. This is purely
a display-side filter -- unrecognized markers are still stored and still
uploaded normally, just not shown to regular players until an admin
reviews the "Other" section and (if it's a real resource) the allowlist
gets updated to include it.

**Potential complications:**
- The allowlist is a fixed list of type strings, not derived from live
  data, so a resource that exists in-game but wasn't present in the data
  used to build/verify this list could initially land in "Other" until
  someone notices and adds it. Confirmed and fixed once already during
  testing (`lilypad`/`saltbash`/`ancientwindthrow`/`heartwood` guesses
  didn't match the real type strings -- corrected to
  `lilypadlotus`/`saltbasin`/`windthrow`/`woodheart` after checking against
  actual uploaded data). This is self-correcting by design: nothing is
  ever dropped, just temporarily miscategorized.
- "Other (Admin)" only appears for accounts with the `admin` auth --
  `writer` accounts do not see it, unlike some other admin/writer-shared
  controls elsewhere in this panel.

## 7. Per-account control over which Markers sections are visible, plus a dedicated Public-access settings section

**What it does:** Each user account now has a "Markers" checkbox group
(Natural Resources, Custom Markers, Clutter, Players, Thingwalls, Vortexes,
Quest Givers, Roads, Other) alongside its existing Roles checkboxes,
controlling which settings-drawer sections that account sees on the map.
Separately, per-map "Required Tier" locking was added to the existing
Hidden/Priority map-edit page. And a new "Public" link next to "Add user"
opens a dedicated settings page -- not a user account, no username or
password -- with one master "Enable Public Access" switch plus the same
Roles/Markers checkboxes, controlling exactly what a visitor with no
login at all can see. When enabled, the site's home page (`/`) serves the
map directly instead of the login screen for anyone without a session,
and the map's account menu shows a plain "Login" link instead of a
username.

**Why:** Two earlier iterations landed here. The first was a single global
"required tier" dropdown per section, edited in one shared admin card --
replaced because it didn't match how every other permission in this app
already works (one checkbox list per account, edited on that account's own
page). The second represented "no login" as a literal fake user account
(`Not Logged In`) that anonymous requests resolved to -- workable, but
confusing: it showed up in the Users table like a real account, could be
edited/deleted the same way, and had no separate on/off switch, so its
mere existence silently turned public access on. Splitting it into its own
non-user settings section with an explicit Enabled toggle makes the
public/private state a single, deliberate switch instead of an implicit
side effect of whether a specific row exists in the Users table.

**Files:** `admin.go` (`SectionOption`/`sectionOptions`, `TierOption`/
`tierOptions`, `adminPublic`), `templates/admin/user.tmpl` (Markers
checkbox group, looped from `SectionOptions`), `templates/admin/
public.tmpl` (new -- Enable switch + Roles + Markers, no username/
password/delete), `templates/admin/map.tmpl` (Required Tier select),
`templates/admin/index.tmpl` (Public link, lit green when enabled),
`main.go` (`MapInfo.RequiredAuth`, `PublicConfig`, `getPublicConfig`,
`getSessionOrPublic`), `manage.go` (`index` redirects anonymous visitors
to `/map/` when Public is enabled), `map.go` (`Config.Public`, `getMaps`
tier filtering; `config`, `getChars`, `getMarkers`, `getPingSound`
switched to `getSessionOrPublic`), `roads.go` (`getRoads`),
`custommarkers.go` (`getCustomMarkers`), `tile.go` (`gridTile`,
`watchGridUpdates`), `frontend/src/components/MapView.vue`
(`canSeeSection`, `isPublic`/Login link, `showVortexes`/`vortexMarks` as a
new dedicated top-level bucket alongside Thingwalls/Quest Givers, since
Vortexes needed its own toggle rather than living inside the Natural
Resources per-category panel)

**How it works:** `PublicConfig{Enabled bool, Auths Auths}` is stored as
one JSON blob under the `config` bucket's `"public"` key -- not a row in
`users` at all. `getSessionOrPublic` returns the real session if one
exists; otherwise, only if `Enabled` is true, it returns a synthetic
`Session` (empty `Username`, `Auths` from `PublicConfig`) standing in for
the request. That's used only in read-only, map-viewing handlers (listed
above), never in any write or `/admin/*` handler, which all still call
`getSession` directly and redirect to `/login` on `nil` regardless of the
Public setting. `Config.Public` (`s.Username == ""`) tells the frontend
this is an anonymous session so it can swap the account menu for a
"Login" link and hide the "Tokens" shortcut. `canSeeSection(key)` is just
`auths.includes('sec_' + key)`, so it works identically whether `auths`
came from a real login or the Public fallback. Separately, `MapInfo.
RequiredAuth` extends the pre-existing `Hidden` flag with the same
tier-dropdown mechanism (`getMaps` now checks it exactly like `Hidden`) --
this part is independent of the Public/per-account work and addresses a
different scope (whole map layers, not settings-drawer sections).

**Potential complications:**
- This is a real, if narrow, security-relevant change: it's the first
  code path in this codebase that serves data to a request with no
  authenticated session. It's scoped tightly (read-only GET handlers only,
  off by default, one explicit Enabled switch) but should get a second
  look before relying on it in production.
- Markers-section defaults are deny, same as every other Roles checkbox
  already worked -- including for the `admin` account itself, which needed
  its own Markers boxes checked after this shipped or its own map view
  would show nothing. Confirmed and fixed during testing on this instance;
  worth remembering before deploying this change anywhere with existing
  accounts.
- Public access is one fixed set of auths shared by every visitor with no
  cookie -- there's no way to give different anonymous visitors different
  access, and it's a single global switch, not per-map or per-IP.
- Verified end-to-end on this instance: a real credential-less `fetch`
  correctly received only the configured auths, `/` correctly redirected
  to `/map/`, `/admin/*` correctly stayed blocked, and the drawer correctly
  showed only the granted sections (plus the always-ungated Grid
  Coordinates/Players Names/Hide Character Names) with nothing else
  leaking through.
- `logout` also redirects to `/map/` instead of `/login` when Public
  access is enabled, for the same reason `index` does -- logging out of a
  real account should land you in the public experience, not a login wall,
  on an instance that has one.
- "Players" is intentionally not one of the Markers checkboxes (see the
  comment on `sectionOptions` in admin.go). It's the one section that
  already had a real, server-enforced auth doing this exact job (`point`,
  labeled "Characters" in Roles, checked in `getChars`) -- a second
  `sec_players` checkbox would only have been a UI switch that had to
  agree with the actual data-access switch to do anything. The Players
  toggle in `MapView.vue` checks `auths.includes('point')` directly
  instead. Every other section (Natural Resources, Clutter, Thingwalls,
  etc.) has no such pre-existing per-category auth -- `getMarkers()` gates
  all of them behind one blanket `markers` role -- so `sec_*` is the only
  granularity that exists for those and isn't redundant.

## 8. Self-service /settings page for personal character-display preferences

**What it does:** Players Names and Hide Character Names are no longer
admin-managed drawer toggles -- they're personal preferences on a new
`/settings` page any logged-in account can reach from the account menu
(next to Admin/Logout), and they now persist per-account instead of
resetting to on every page load.

**Why:** Unlike every other drawer toggle, these two aren't about what
data an account is allowed to see -- they're purely about how that one
account wants character names displayed to them. That's a personal
preference, not something that belongs in an admin-configured visibility
system, and it's the kind of setting a user should be able to change for
themselves without asking an admin to edit their account.

**Files:** `settings.go` (new), `templates/settings.tmpl` (new),
`templates/navbar.tmpl` (Settings link), `main.go` (`User.
ShowPlayerNames`/`HideCharacterNames` as `*bool`, `/settings` route),
`map.go` (`Config.ShowPlayerNames`/`HideCharacterNames`, populated from
the account's stored `User` record in `config`),
`frontend/src/components/MapView.vue` (drawer rows removed, `processConfig`
reads the persisted values, account-menu Settings link)

**How it works:** Stored as `*bool` on `User`, not plain `bool` -- an
account that's never visited `/settings` has both fields `nil`, which
`config()` omits from the JSON response entirely (`omitempty`) rather than
serializing as `false`. The frontend only overrides its own default
(`true`) when the field is actually present, so no existing account's
behavior changed the moment this shipped. The settings page itself still
hides both checkboxes for an account without `point`, matching the
existing Players/Characters reasoning in item 7.

## 9. Fixed a pre-existing bug: turning off "Players" didn't stick

**What it does:** Unchecking Players (or Players Names/Hide Character
Names being off) used to only last until the next character-position poll
(every 2 seconds, sometimes sooner if a request was already in flight) --
the player markers would silently reappear on their own.

**Why:** This predates every change in this document -- confirmed via
`git log` that `frontend/src/data/Character.js` and the polling loop in
`MapView.vue` were untouched by any of our own work before this fix.
`Character.update()` (called every time the 2-second poll in `setupMap`
re-fetches character positions) had `if (!this.marker && this.map ===
mapview.mapid) { this.add(mapview); }` -- it only knew "do I have a
marker," not "is Players even enabled." Turning Players off removes every
marker (`this.marker = null`), and the very next poll cycle saw no marker
and silently added it right back, with no idea the removal was
deliberate.

**Files:** `frontend/src/data/Character.js` (`update` now takes a
`visible` parameter), `frontend/src/components/MapView.vue`
(`updateCharacters`'s Add callback now checks `this.showPlayers`; its
Update callback passes `this.showPlayers` through to `character.update`)

**How it works:** `Character.update(mapview, updated, visible)` only
auto-re-adds a missing marker when `visible` is true, and both call sites
in `updateCharacters` now pass `this.showPlayers` for that argument --
the same gate `updateMarkers`'s equivalent callback already had for every
other marker type. Turning Players off now stays off across polls, same
as every other toggle in the drawer.

## 10. Drawer redesign: left-side icon toggles, and name-visibility as its own control on Thingwalls/Quest Givers/Players

**What it does:** Every drawer row's on/off switch is now a green/red eye
icon on the left of the label (green `mdi-eye` = shown, red `mdi-eye-off`
= hidden) instead of the old full-width switch button. Thingwalls, Quest
Givers, and Players additionally get a second icon -- a green
checkmark/red-X speech-bubble (`mdi-comment-check-outline`/`mdi-comment-
remove-outline`) -- controlling whether that marker type's name tooltip
stays pinned open, independent of the eye. Natural Markers and Other
(Admin) keep their expandable per-category sub-panel, now opened with an
explicit chevron on the row's right edge instead of automatically
following the eye -- collapsing that panel no longer hides the markers,
and hiding the markers no longer force-collapses the panel.

**Why:** Thingwalls and Quest Givers previously had their tooltips
hardcoded permanently on (`it.tooltip(true)`) with no way to turn them
off at all. Players already had a names toggle, but it lived on the
`/settings` page (item 8) -- putting an equivalent control back inline
next to the Players eye makes that page redundant for this one case,
which is fine since nothing else needs the same treatment yet. Decoupling
the sub-panel's open/closed state from the eye toggle was a deliberate
UX call: whether a section's markers show on the map and whether its
category list is open in the drawer are different questions, and forcing
them together meant you couldn't glance at (or edit) Natural Resources
categories while that section was hidden.

**Files:** `frontend/src/components/IconToggle.vue` (new, replaces the
deleted `ToggleButton.vue`), `frontend/src/components/MapView.vue` (every
drawer row, `showThingwallNames`/`showQuestNames`/`naturalMarkersExpanded`/
`otherExpanded` state, tooltip wiring throughout `showThingwalls`/
`showQuests`/the overlay and map-change handlers/`updateMarkers`)

**How it works:** `IconToggle` is a generic `v-icon` wrapper taking
`value`/`onIcon`/`offIcon`/`onColor`/`offColor` props and emitting
`input` on click -- the eye pair is its default, the comment-bubble pair
is passed explicitly for the three name-toggle rows. `showThingwallNames`/
`showQuestNames` replace every hardcoded `it.tooltip(true)` for those two
marker types (six call sites: the `showThingwalls`/`showQuests` watchers,
both branches of the `overlayMap` watcher, `changeMap`, and the initial
`tstate` assignment in `updateMarkers`) plus new watchers that reapply
`.tooltip(value)` across already-rendered markers when toggled, the same
pattern used for `showPlayerTooltips`/`showUnknownMarkers`/etc. The
Natural Markers/Other sub-panels swapped their `v-if` from `&&
showMarkers`/`&& showUnknownMarkers` to `&& naturalMarkersExpanded`/`&&
otherExpanded`, toggled only by the new chevron, with no other change to
what the eye itself controls.

**Potential complications:**
- The Players names icon and the `/settings` page's Players Names
  checkbox are two controls over the same `showPlayerTooltips` model by
  design -- confirmed intentional (this item exists partly because the
  user pointed out the drawer icon makes the settings-page version
  redundant for this one row). Not a bug; both stay in sync since they
  share the same underlying value.
- Vortexes, Roads, Custom Markers, Clutter, Grid Coordinates don't get a
  names icon -- there's no per-item name/tooltip concept for those marker
  types to toggle. Thingwalls/Quest Givers/Players were called out
  explicitly as the starting set to test this pattern on; extending it
  further (per the original ask, Natural Resources/Clutter/Roads/etc.)
  would need real per-marker-type tooltip support added first, not just a
  UI toggle.
- Verified interactively end-to-end after deploying: eye click flips
  icon+color and actually hides/shows markers on the map; name-bubble
  click flips independently; expanding Natural Markers' category list,
  then turning its eye off, leaves the list open (previously it would
  have force-collapsed).

## 11. Unified search bar replaces the four separate Thingwall/NPC/Marker/Player dropdowns, and fixes a real reselect bug in the process

**What it does:** One search field now covers Thingwalls, Quest Givers,
Custom Markers, and Players together (icon + name + type label per
result), replacing four separate `v-autocomplete` dropdowns. Selecting a
result homes the map in on it exactly like the old dropdowns did. Along
the way, this fixes a genuine bug: re-selecting the same single result
twice in a row (e.g. only one player online) previously did nothing the
second time, requiring a full page reload to "unstick" it.

**Why:** The four-dropdown layout was already earmarked for a rebuild
(unifying them into one search-as-you-type control), so the fix went into
the new mechanism directly rather than patching the code being replaced.
The reselect bug's root cause was structural, not a one-line typo: each
dropdown drove its jump-to logic through a Vue `watch` on its `v-model`,
and Vue only re-fires a watcher when the value actually *changes* --
picking the same object out of a list with only one entry assigns the
same reference back to itself, so nothing fires and nothing re-centers.
Any fix scoped to "patch the watcher" would've had to fight that
same-value problem four separate times, once per dropdown.

**Files:** `frontend/src/components/MapView.vue` (`searchItems` computed,
`searchSelection` state, `jumpToSearchResult`/`jumpToMapMarker`/
`jumpToCharacter` methods, `trackingCharacterId` watcher simplified to
call `jumpToCharacter`, the four old `selectedThing`/`selectedQuest`/
`selectedCustomMarker`/`selectedPlayer` watchers and data fields removed)

**How it works:** `searchItems` normalizes `thingMarks`/`questMarks`/
`customMarks`/`players` (four differently-shaped arrays) into one flat
list of `{searchType, name, icon, typeLabel, raw}`, which the single
`v-autocomplete` filters by `name` as you type. The fix for the reselect
bug: selection is handled by `@change="jumpToSearchResult"` -- a real DOM
event Vuetify fires on every selection, including picking the same item
twice -- instead of a `watch` on the bound value. `jumpToSearchResult`
dispatches on `item.searchType`: players go through `jumpToCharacter`
(extracted from the old `trackingCharacterId` watcher body, now called
directly so it runs even when `trackingCharacterId`'s value doesn't
change), everything else through `jumpToMapMarker` (the identical body
that used to be duplicated across three separate watchers). The search
field clears itself via `$nextTick` after each jump, ready for the next
search.

**Potential complications:**
- Verified directly: selected the same Thingwall twice with the map
  panned away in between (`map.getCenter()` checked before/after) --
  both selections correctly re-centered on it. Couldn't verify the player
  path the same way in this session (no live character was online in the
  test data at verification time), but it runs through the exact same
  `jumpToSearchResult` → direct-method-call path, not a watcher, so the
  same fix applies.
- The four old dropdowns' per-type item templates (thingwall/quest icons
  from `.image`, custom marker icons from `.color`) are preserved as
  `icon`/`typeLabel` in the normalized list; players have no icon image
  today and fall back to a generic `mdi-account` glyph in the results
  list.

## 12. Clutter gets the same per-category toggle panel as Natural Markers/Other

**What it does:** Clutter is no longer one blanket on/off switch for
burrow, clamreef, flintwash, and spawningbed all at once -- expanding it
(same chevron pattern as Natural Markers/Other) shows one switch per
type, so turning Clutter on doesn't mean showing all of it.

**Why:** Directly requested: enabling Clutter previously meant every
clutter type appeared together with no way to see just one kind (e.g.
burrows without also getting spawning beds).

**Files:** `frontend/src/components/MapView.vue`
(`selectedClutterTypes`/`clutterExpanded`/`clutterCategories`/
`clutterCategoryIcons`/`toggleClutterType`, every `clutterMarks` filter
site, the per-marker visibility ternary in `updateMarkers`)

**How it works:** Identical structure to Natural Markers/Other's existing
per-category panels (`selectedResourceTypes`/`marksCategories`/etc.) --
`clutterCategories` is populated the same way during the marker-routing
pass in `updateMarkers`, and every place that already filtered on
`showClutter` now also checks `selectedClutterTypes.includes(it.type)`.

**Potential complications:**
- Verified live: with `showClutter` on and only `burrow` selected, 218
  burrow markers rendered and zero markers of the other three clutter
  types did, confirming the filter is exclusive rather than additive-only.

## 13. Replaced the "Draw Road" button with an inline Edit icon on Roads; drawer section reorder; a real app-bar/drawer layout bug fix; Overlay Map gets its own visibility toggle

**What it does:** Four unrelated-but-simultaneous changes to the drawer:
(1) the old full-width "Draw Road"/"Cancel Draw Road" button is gone,
replaced by a green/blue pencil-note icon next to the Roads eye (same
`IconToggle` component as everywhere else, labeled "Edit"); (2) drawer
sections were reordered (Search, Grid Coordinates, Thingwalls, Vortexes,
a new collapsible "Map" group, Natural Markers, Quest Givers, Players,
Roads, Custom Markers, Clutter, Other (Admin)); (3) Jump To Map and
Overlay Map are now grouped under that "Map" section with the same
expand-chevron pattern as Natural Markers/Clutter/Other, instead of
sitting bare at the top of the drawer; (4) Overlay Map gets its own eye
icon, toggling the overlay's visibility independent of which map is
selected -- turning it off no longer clears the dropdown, turning it back
on instantly restores the same overlay.

**Why:** All requested together as one pass at getting the drawer closer
to a final design (a reference mockup was provided). The Edit icon
replaces a control that was visually and functionally inconsistent with
the rest of the redesigned drawer (item 10) -- a large standalone button
next to a list of compact icon rows. The reorder and Map group match the
supplied mockup directly. The separate app-bar layout bug (below) was
found and fixed while verifying the reorder, not requested on its own.

**Files:** `frontend/src/components/MapView.vue` (drawer template
reorder; Roads row's Edit `icon-toggle`; `mapSectionExpanded`/
`showOverlayMap` state; `applyOverlayMap` method; `.v-navigation-drawer`/
`.v-app-bar` CSS; `:mini-variant-width="36"`; `.draw-road-btn` CSS removed
along with the button)

**How it works:**
- The Edit icon binds `:value="drawingRoad"` / `@input="toggleDrawRoad"`
  rather than `v-model`, since `toggleDrawRoad` flips the state itself
  (and clears `roadPoints`/a temp marker) rather than accepting the new
  value as a parameter -- `v-model` would have bypassed that cleanup.
- The overlay visibility toggle required extracting the old `overlayMap`
  watcher's entire body (previously ~100 lines duplicated across two
  branches, one per selected/cleared state) into a new `applyOverlayMap()`
  method, unchanged internally, that now computes its effective "is there
  an overlay to show" value from `showOverlayMap && overlayMap` instead of
  just `overlayMap`. Both `overlayMap` and `showOverlayMap` are now thin
  watchers that just call it. Toggling visibility off sets
  `overlayLayer.map = -1` (exactly what clearing the dropdown already
  did) without touching the `overlayMap` selection itself.
- **The app-bar/drawer layout bug (found via direct measurement, not
  visual inspection alone -- `getBoundingClientRect()` on both elements):**
  the navigation drawer was rendering at `top:0`, directly under the
  56px-tall app-bar instead of below it, for reasons that weren't fully
  traceable to a single Vuetify setting (its automatic `app`-prop
  coordination simply wasn't kicking in here). Fixed by pinning
  `.v-navigation-drawer { top: 56px !important }` directly. That fix then
  exposed a second, related mismatch: Vuetify's internal layout
  bookkeeping was reserving its own default 56px for the collapsed
  (mini-variant) drawer width, while a separate pre-existing rule
  (`.v-navigation-drawer { width: auto !important }`, not something this
  session added) was visually shrinking it to 36px to fit its actual
  content (just the expand-arrow icon) -- the 20px gap between what
  Vuetify thought it reserved and what was actually there was pushing the
  app-bar and the map's Leaflet zoom control 56px right of where they
  should have been. Fixed by adding `:mini-variant-width="36"` so
  Vuetify's bookkeeping matches the real rendered width. Finally, by
  default Vuetify's `app`-aware app-bar shrinks itself to make room
  beside the drawer -- the design wants it spanning the full width
  edge-to-edge instead with the drawer entirely below it, so `.v-app-bar`
  now force-overrides `left: 0` / `width: 100%`.

**Potential complications:**
- Verified all three layout numbers directly via `getBoundingClientRect()`
  before and after: app-bar now spans the full viewport width at
  `left:0`, the drawer starts at `top:56` with no vertical overlap, and
  the Leaflet zoom control sits cleanly at the collapsed drawer's right
  edge (`left:36`) instead of 56px further right than it should have
  been.
- Verified the overlay toggle's actual effect on `overlayLayer.map`
  directly: selecting a map set it to that map's ID, toggling visibility
  off set it to `-1`, toggling back on restored the original ID without
  needing to reselect anything.
- The drawer's expanded width (228px) wasn't changed in this pass --
  worth confirming with the mockup once the layout bug fix is visible,
  since some "sidebar too wide" perception may have been the overlap bug
  itself rather than the width.

## 14. Thingwall icons scale with zoom (matching live), plus a personal scale slider on /settings

**What it does:** Thingwall icons now shrink as you zoom out instead of
staying a fixed 90x90px at every zoom level -- matching how the live
server (per a separate unmerged fork) already renders them. On top of
that live-matching default, `/settings` gets a "Thingwall Icon Scale"
slider (0.25x-2x, default 1x = unmodified live behavior) so an account
can size them smaller or larger to taste.

**Why:** Directly requested, with an explicit goal of matching live
before considering sharing this branch with Aritain. The fixed-size
icons were the one visibly "unclean" difference from live; the slider is
the "give people the option" half of the request, layered on top rather
than replacing the live default.

**Files:** `frontend/src/data/Marker.js` (`iconScale`, `buildIcon`,
`rescale`), `frontend/src/components/MapView.vue` (`thingwallScale`
state read in `processConfig`, `rescale` wired into the existing `"zoom"`
map event), `settings.go`/`templates/settings.tmpl` (slider UI, `0.25`-`2`
clamped and defaulted to `1` server-side), `main.go` (`User.
ThingwallScale *float64`), `map.go` (`Config.ThingwallScale`)

**How it works:** `iconScale(zoom) = max(0.35, 0.75^levelsOut)` where
`levelsOut` is how many zoom levels below `HnHMaxZoom` the map currently
is -- 1 (full size) at max zoom, shrinking 25% per level out, floored at
35% so icons never disappear. Thingwalls' half-size becomes `round(45 *
iconScale(zoom) * thingwallScale)` (every other marker type's sizing is
untouched, unchanged from before). Icon *building* was extracted from
`add()` into `buildIcon(zoom, thingwallScale)` so a new `rescale(mapview)`
method can call `marker.setIcon(...)` with a freshly computed size on
every zoom change, without recreating the marker (tooltip-open state,
listeners, etc. all survive) -- wired into the map's existing `"zoom"`
event handler, scoped to `thingMarks` only. The slider itself follows the
exact same nil-means-unset persistence pattern as items 8/9's Players
Names/Hide Character Names (`*float64` on `User`, omitted from `Config`
JSON when never set, frontend keeps its own default of `1` in that case).

**Potential complications:**
- Verified live end-to-end, not just read from code: set the slider to
  1.5x, saved, confirmed `/map/api/config` returned `1.5`, then read a
  real Thingwall marker's actual rendered Leaflet icon size at two
  different zoom levels -- 48px at zoom 1 and 136px at max zoom 9 --
  both matching the formula's predicted output exactly
  (`round(45*0.35*1.5)*2=48`, `round(45*1*1.5)*2=136`), confirming the
  live rescale-on-zoom wiring actually fires, not just the initial
  render.
- The live-server "default" is based on formula/values found in a
  separate unmerged fork repo on this machine (`HnH Mapper - Serin
  Fork`), not independently verified against the actual live site itself
  -- worth a visual side-by-side once this deploys, in case that fork
  had since drifted from what's really live.

## 15. Thingwall scale slider raised to 4x max; drawer row labels aligned

**What it does:** Two small follow-ups to items 13/14: the Thingwall
Icon Scale slider's max is now 4x instead of 2x (both the HTML `max`
attribute and the server-side clamp in `settings.go`). Separately, every
drawer row's label now starts at the same horizontal position regardless
of how many icons precede it -- previously a two-icon row (Thingwalls,
Quest Givers, Players) pushed its label further right than a one-icon
row (Grid Coordinates, Vortexes, etc.), since the icons sat directly
inline before the label with no reserved space.

**Files:** `templates/settings.tmpl`/`settings.go` (slider max),
`frontend/src/components/MapView.vue` (`.row-icons` CSS class, applied to
every drawer row's icon-toggle(s))

**How it works:** `.row-icons` is a fixed 64px-wide flex container
(sized for the widest case, two icons) wrapping each row's icon-toggle(s)
-- including an empty one on the "Map" section header, which has no icon
at all, so its label lines up too. Labels sit outside that container, so
they always start at the same X offset whether the row has zero, one, or
two icons inside it.

**Potential complications:**
- Verified directly via `getBoundingClientRect()` on all eleven section
  labels (Grid Coordinates through Other (Admin)) -- every one starts at
  exactly the same `left` pixel value now.

## 16. Mineholes & Caves split into their own toggle (Natural Resources renamed); Map section framed and right-aligned; reference marker icons added to Thingwalls/Vortexes/Mineholes & Caves

**What it does:** Three more drawer changes. (1) "Natural Markers" is
renamed "Natural Resources", and Mineholes/Caves -- previously two
entries mixed into that section's per-category panel alongside every
other resource type -- get pulled out into their own simple standalone
toggle row directly below it (no per-category breakdown, just one on/off
switch for both, same pattern as Vortexes/Custom Markers). (2) The "Map"
section header gets a white top/bottom border and its label right-aligned
next to the chevron instead of following the standard left-aligned
pattern, visually setting it apart as its own framed region. (3)
Thingwalls, Vortexes, and the new Mineholes & Caves row each show their
actual in-game marker icon(s) on the right edge, aligned among
themselves via a new `.row-marker-icons` class (mirroring `.row-icons`
from item 15, but right-aligned and for reference images instead of
functional toggles).

**Why:** All directly requested. Splitting Mineholes/Caves out mirrors
the same reasoning as item 12's Clutter split (a blanket toggle covering
unrelated categories) but scoped to just this one pair rather than a
full per-category panel, since two merged-icon types don't need one.

**Files:** `frontend/src/components/MapView.vue` (`showMineCave`/
`mineCaveMarks` state, watcher, `applyOverlayMap` branches, `changeMap`
branch, `updateMarkers` routing + visibility ternary, force-off list,
`.row-marker-icons` CSS, template changes for all three items;
`KNOWN_RESOURCE_TYPES` no longer lists `mineshaft`/`cave`;
`resourceCategoryIcons`' now-unreachable Mineshaft/Cave special-casing
removed)

**How it works:**
- Mineholes & Caves is structurally a straight copy of the Vortexes
  pattern (`showVortexes`/`vortexMarks`) applied to a new
  `showMineCave`/`mineCaveMarks` pair, gated by the same
  `canSeeSection('naturalMarkers')` as Natural Resources since it's the
  same underlying data/permission, just a second row. The
  `updateMarkers` routing pass now checks `it.type === "Mineshaft" ||
  it.type === "Cave"` *before* the general `KNOWN_RESOURCE_TYPES` branch,
  so those two types never reach the old bucket at all.
- The Map header's border/right-alignment is plain CSS/template
  reordering on that one row -- no functional change, `v-spacer` now
  comes before the label instead of after.
- The marker-icon images are static -- Thingwalls/Vortexes have one
  fixed icon each (`gfx/terobjs/mm/thingwall.png`/`watervortex.png`),
  Mineholes & Caves shows both merged-category icons side by side
  (`mm/down.png`, `gfx/hud/mmap/cave.png`), matching the same icons
  already used elsewhere (e.g. the old `resourceCategoryIcons`
  special-casing this replaces).

**Potential complications:**
- Verified live: `marksCategories` (Natural Resources' per-category list)
  no longer contains "Mineshaft"/"Cave"; `mineCaveMarks` correctly holds
  both types (11 markers in the test data); toggling the new row's eye
  off actually removed all 11 from the map and back on restored them;
  all three rows' `.row-marker-icons` containers right-align to the exact
  same pixel edge, and every image (`naturalWidth > 0`) loaded
  successfully rather than 404ing.

## 17. Corrections to item 16: Mineholes/Caves as two separate toggles, Map back to left-aligned with more separation, and click-to-expand on the label text

**What it does:** Three corrections after review of item 16. (1)
Mineholes and Caves are now two fully independent standalone toggle rows
(`showMineholes`/`mineholeMarks` and `showCaves`/`caveMarks`), not one
combined switch controlling both -- each keeps its own reference icon on
the right. (2) The "Map" section reverts to left-aligned (matching every
other row) instead of right-aligned, keeps its white top/bottom border,
and gets 10px of margin outside those borders so they read as visible
separation from the rows above/below instead of sitting flush against
them. (3) Every expandable section (Map, Natural Resources, Clutter,
Other (Admin)) is now clickable across its entire label-to-arrow span,
not just the small chevron icon itself.

**Why:** Direct correction of a misread ("right aligned" was a mistake,
left-aligned was intended) and a design call reversed after seeing it
live (two independent toggles instead of one combined). The click-target
expansion was a usability ask on top of the existing expand/collapse
sections from earlier items.

**Files:** `frontend/src/components/MapView.vue` (state/watcher/routing/
filter/reset/force-off split into `showMineholes`+`showCaves` pairs
throughout, mirroring the same locations item 16 touched; Map row
template/CSS; a wrapping `<div @click>` around label+spacer+chevron on
all four expandable sections)

**How it works:** The Mineholes/Caves split is a mechanical duplication
of every place item 16 touched (watcher, both `applyOverlayMap` branches,
`changeMap`, the `updateMarkers` routing push and visibility ternary,
reset, force-off) -- same structure as the Vortexes pattern, just twice.
The click-to-expand fix wraps `<span>Label</span><v-spacer/><v-icon
chevron></v-icon>` in a `<div style="flex-grow:1;cursor:pointer"
@click="...Expanded = !...Expanded">`, moving the click handler off the
tiny icon and onto that whole flex region -- deliberately stops short of
the `.row-icons` eye-toggle gutter on the left, so clicking the
visibility icon still only toggles visibility, not both.

**Potential complications:**
- Verified live: turning Mineholes off while Caves stayed on removed
  exactly the 5 mineholes from the map and left all 6 caves showing
  (independent state, not coupled); clicking the "Natural Resources" text
  itself (not the chevron) toggled `naturalMarkersExpanded` from false to
  true; the Map row's label now left-aligns at the same x-position
  (69px) as every other row, with the border still present and 10px
  margin confirmed via computed style.

## 18. Map row sizing polish; only Thingwalls/Vortexes visible by default

**What it does:** Two more small fixes. (1) The "Map" section header's
label is 2px larger (13px to 15px) and the row gets 6px of padding above
and below its content, so the top/bottom border lines (item 17) don't sit
flush against the text -- it was reading as squished. (2) On a fresh page
load, only Thingwalls and Vortexes are visible by default; every other
marker-visibility toggle (Natural Resources, Mineholes, Caves, Quest
Givers, Players, Roads, Custom Markers) now starts off, left for the
user to turn on. Name/tooltip preferences (Quest/Thingwall/Player names)
are untouched by this -- those are a separate "how it's labeled" setting
from "is it shown at all," and don't matter until their section's eye is
turned on anyway.

**Files:** `frontend/src/components/MapView.vue` (Map row inline style,
`show*` defaults in `data()`)

**How it works:** The padding fix needed `!important` on the inline
style -- a global `.v-list-item { padding: 0px !important; }` rule
elsewhere in this file was silently winning over a plain inline
`padding-top/bottom` without it. The visibility defaults are a pure
`data()` change; none of these are persisted server-side (unlike e.g.
Players Names in /settings), so every account gets the same fresh-load
defaults.

**Potential complications:**
- Verified live: computed padding on the Map row is now `6px/6px`
  (previously silently `0px/0px` despite the inline style being present
  in the DOM, confirming the `!important` conflict), row height grew from
  18px to 38px; on a fresh page load `showThingwalls`/`showVortexes` are
  `true` and every other visibility toggle (`showMarkers`, `showQuests`,
  `showPlayers`, `showRoads`, `showCustomMarkers`, `showClutter`,
  `showMineholes`, `showCaves`, `showUnknownMarkers`,
  `showGridCoordinates`) reads `false`.

## 19. Map label repositioned to align with the show/hide-names icon column

**What it does:** "Map"'s label now starts one icon-width in from the
row's left edge (aligned with where a two-icon row's second icon --
e.g. Thingwalls' show/hide-names toggle -- sits) instead of the full
64px `.row-icons` width every other section's label starts at.

**Why:** Directly requested -- the Map row has no eye toggle of its own,
so the standard 64px gutter left it looking mis-aligned relative to
the rows around it; a narrower 32px gutter positions it against the
second icon column instead.

**Files:** `frontend/src/components/MapView.vue` (Map row's leading
spacer div, `width:32px` instead of the shared `.row-icons` class)

**How it works:** A plain `width:32px` div (not the `.row-icons` class,
which is fixed at 64px for two icons) replaces the empty gutter used in
item 17.

**Potential complications:**
- Verified live: the Map label and Thingwalls' show/hide-names icon
  both measure `left: 37px` -- exact pixel alignment, not approximate.

## 20. Admin: delete a map layer entirely; Users/Maps sections made collapsible

**What it does:** Two admin-page changes. (1) Each map's Edit page gets a
"Delete map" danger-zone action (same confirm-modal pattern as the
existing "Wipe all data") that permanently removes that one map layer and
everything scoped to it -- grids, tiles, markers, roads, custom markers --
without touching any other map. A matching "Delete" link was also added
to each row on the admin index's map list. (2) The Users and Maps
sections on the admin index are now collapsible (open by default) instead
of two bare tables sitting in a long flat page.

**Why:** Directly requested -- there was previously no way to remove a
map layer at all, only Hide it (which just stops it from being listed;
the data stays). The collapsible sections were the first requested step
("at the bare minimum to start") toward cleaning up what was described as
a chaotic-looking admin page; the other cards further down (Prefix,
Title, Ping sound, Wipe, etc.) are unchanged for now.

**Files:** `admin.go` (`deleteMap`), `main.go` (route registration),
`templates/admin/map.tmpl` (Delete button + modal), `templates/admin/
index.tmpl` (Delete link per map row, `<details>`-based collapsible
sections + CSS)

**How it works:** Storage for each data type turned out to need a
different deletion strategy, found by reading the actual write paths
rather than assumed: `grids` is a flat bucket keyed by grid ID with a
`Map` field per record, so it's scanned and filtered. `tiles` is already
nested as `tiles/{mapID}/{zoom}/{coord}` (see `SaveTile` in tile.go), so
deleting a map's tiles is one `DeleteBucket` call, no scanning needed.
`markers` was the trickiest -- actual marker records live under
`markers/grid` keyed by `"{gridID}_{x}_{y}"` with no `Map` field of their
own (see `markerUpdate` in client.go), so deleting them requires first
collecting which grid IDs belong to the map being deleted, then matching
marker keys by that grid-ID prefix; `markers/id` is a separate
id-to-gridkey index for admin lookups by numeric ID that also needs its
matching stale entries removed, or lookups by ID would resolve to a
grid-key that no longer exists. `roads` and `customMarkers` are flat
buckets with their own `Map` field, same pattern as `grids`. All of this
runs inside one `bbolt` transaction, matching the existing `wipe()`
handler's approach just scoped to one map ID instead of every bucket
outright. The collapsible sections are plain HTML `<details>`/`<summary>`
-- no JS dependency, since this page already only uses Materialize CSS
plus a little intercooler.js for the Hide/Show AJAX toggle, not a
reactive framework.

**Potential complications:**
- This is genuinely irreversible, more surgically than "Wipe all data"
  (which nukes everything) -- there's no soft-delete or backup step, by
  design matching how Wipe already works.
- Verified live with real data, not just reasoning about the code: added
  a temporary debug endpoint (removed again before finishing) that
  counted grids/markers/roads/customMarkers/tiles-exist/map-exists for a
  given map ID. Picked a small test map (9 grids, 3 markers -- enough to
  exercise the marker-cascade path, unlike some other test maps in this
  data with 0 markers) rather than the main overworld map (16,643 grids,
  1,180 markers) to keep the blast radius small. Before: `grids:9,
  markers:3, tilesExist:true, mapExists:true`. After deleteMap: `grids:0,
  markers:0, tilesExist:false, mapExists:false` -- and re-checked the
  main overworld map immediately after to confirm its own counts (16,643
  grids, 1,180 markers) were completely unaffected, i.e. the deletion
  was correctly scoped to only the targeted map.
- Also verified the actual UI path, not just the raw endpoint: clicked
  the real "Delete!" button on a map's edit page and confirmed the modal
  opens with that map's name/ID in the confirmation text before either
  Cancel or DELETE is pressed.
- The collapsible sections' open/closed toggle was verified via a real
  click on each `<summary>`, confirming both start open and correctly
  collapse.

## 21. Admin polish: sections closed by default, Public moved above Users, Maps table gets icon buttons

**What it does:** Three follow-ups to item 20. (1) Users/Maps now start
collapsed instead of open. (2) The "Public" access link moves above the
Users section entirely, instead of sitting inside it next to "Add user"
-- it's a site-wide setting, not a user account. (3) Each map row's
Edit/Hide/Delete actions are now small circular icon buttons (pencil,
eye/eye-off, red circle with an X) instead of three separate text
buttons, tighter together.

**Why:** Directly requested, after seeing item 20 live.

**Files:** `templates/admin/index.tmpl` (icon-btn CSS, Public moved,
`open` removed from both `<details>`, Maps table body swapped to icon
buttons)

**How it works:** The Hide/Show toggle keeps the same intercooler
`ic-post-to` AJAX wiring as before (item 20/pre-existing) -- only the
block's rendered content changed, from "Show"/"Hide" text to a
`visibility`/`visibility_off` Material icon, so the partial swap on
click still works the same way. Edit and Delete are plain links styled
as circles via `.icon-btn`; Delete gets `.icon-btn-delete` for the red
background, matching the "red background, black X" ask.

**Potential complications:**
- Verified live: both sections read `open: false` on load; the Public
  link's DOM position now precedes the Users `<details>`
  (`compareDocumentPosition` check); the Hide/Show icon button still
  correctly flips between `visibility`/`visibility_off` on click, same
  live AJAX behavior as before, just re-skinned.

## 22. Admin: "Duplicates Detected" report -- flags likely spawn zones, dungeon instances, and disconnected caves for review (experimental)

**What it does:** A new `/admin/duplicateMaps` page (linked from the Maps
section) surfaces small map layers (under 20 grids) that look like
junk -- spawn zones, dungeon instances, house interiors, or other
generated/orphaned layers -- using two independent signals, combined
into one report with a confidence split:

- **High confidence:** maps that both match another map's *exact* tile
  layout *and* have no Mineshaft/Cave-type marker anywhere on them.
- **Lower confidence (two sub-sections):** maps that match another
  map's layout but do have an entrance marker, and maps with no
  entrance marker that don't match any other map's layout.

Nothing is deleted automatically. Each entry has its own Edit/Delete
icon buttons, matching the Maps table.

**Why:** Directly requested, to find and clean up spawn zones and
similar junk layers accumulating in the map list. Marked experimental
per your request, since the detection is real but imperfect --
verified against labeled ground truth this session (see below) rather
than left as an unvalidated guess.

**Files:** `admin.go` (`DuplicateGroup`, `maxSpawnZoneGrids`,
`entryMarkerImages`, `findSmallMapCandidates`, `computeHasEntry`,
`findDuplicateMaps`, `findMapsWithoutEntry`, `adminDuplicateMaps`),
`main.go` (route registration), `templates/admin/duplicates.tmpl` (new),
`templates/admin/index.tmpl` ("Find duplicate small maps" link)

**How it works:**
- **Layout-match signal:** every grid tile under 20-grid maps gets
  SHA-256 hashed by file content, normalized to offsets relative to
  that map's own bounding box (so the same template matches regardless
  of which absolute coordinates the mapper happened to assign it), then
  all of a map's (relative-x, relative-y, tile-hash) triples get hashed
  together into one fingerprint. Maps sharing a fingerprint are grouped.
  Maps over 20 grids are excluded entirely, both because a real
  explored area won't match a small template and to keep this cheap --
  the main overworld alone can have tens of thousands of grids.
- **Entrance-marker signal:** checks every marker on a small map against
  `entryMarkerImages`, the same Mineshaft/Cave image set Marker.js uses
  (`mm/down`, `mm/up`, `gfx/terobjs/minehole`, `gfx/terobjs/ladder`,
  `gfx/hud/mmap/cave`, `gfx/tiles/ridges/cavein`/`cavein2`/`caveout`) --
  deliberately *not* including `gfx/terobjs/mm/custom` (the invisible
  entrance/last-known-position marker from item 3), since the ask was
  specifically about real, player-visible cave/minehole markers, not
  the mapper's own internal tracking markers.
- Both signals are computed from one shared `findSmallMapCandidates()`
  pass over the `grids` bucket, so the data is only scanned once
  regardless of how many maps qualify.

**Ground-truth validation this session (real data, not synthetic):**
Ran this against `reference-test-data` and got labels for every small
map from the person who actually knows what they are:
- The high-confidence list flagged exactly `5, 6, 7, 11, 12, 13, 14, 15`
  as four template-matched pairs -- confirmed as **8/8 real spawn
  zones**, zero false positives.
- Map 9 (a real mine with a genuine entrance) was correctly excluded
  from every list -- confirms the entrance-marker signal recognizes a
  real positive, not just flagging everything.
- Map 18 (a valid spawn not yet stitched into a known region) was also
  correctly excluded.
- The lower-confidence "no entrance marker" list additionally caught
  4 (Fox Burrow, a real mini dungeon), and 16/17/19 (real cave systems)
  -- confirmed as real content whose entrance marker simply never got
  uploaded (older client, or icons not yet toggled on, per the map 16
  case specifically). These aren't wrong to flag -- they're genuinely
  small, orphaned, and hard to coordinate-anchor -- but they're real
  data, which is exactly why this stays a review list, never
  auto-delete.
- This validation is also what "tuned" the report structure: the two
  raw signals started as two separate lists with overlapping entries
  (a map showing up twice); per your request they're now one combined
  report, split into the high/lower-confidence sections above so a map
  only appears once, in its strongest-evidence bucket.

**Potential complications:**
- Exact-hash tile matching could still miss a true template duplicate
  if the client renders the same tile with pixel-level differences
  between sessions (lighting, foliage frame) -- not hit in this
  session's testing, but a known theoretical gap; a perceptual/fuzzy
  hash would be more robust if this turns out to matter in practice.
- The "no entrance marker" signal, even split into its own
  lower-confidence section, will keep surfacing real dungeons/caves
  whose upload simply predates marker support or icon toggling -- this
  is a genuine data-completeness gap, not something fixable by
  widening the marker-image list (both real false-positive maps found
  this session had *zero* markers of any kind, not markers of some
  other unrecognized type).
- Houses weren't tested this session (real cave/mine data was generated
  and checked instead) -- flagged as a known risk earlier: a house
  interior could plausibly template-match another house if both stay
  unfurnished/unexplored past their shared starting layout, which would
  be a real false positive on the high-confidence list specifically,
  unlike the spawn-zone case. Worth checking before trusting that
  section fully for houses.

## 23. Admin UI fixes: inline delete confirmation (no detour through Edit), and a cursor bug

**What it does:** Two small fixes on top of items 20-22. (1) The Delete
icon on both the Maps table and the new Duplicates report now opens its
confirmation modal directly on the same page, instead of linking to the
map's Edit page first just to reach the same modal there. (2) The
Show/Hide eye icon no longer shows a text-selection (I-beam) cursor on
hover -- it was missing `cursor: pointer` entirely; every other icon
button had it implicitly from other styling, this one didn't.

**Why:** Directly requested -- the Edit-page detour for Delete was
redundant now that the confirmation modal itself doesn't need anything
from that page, and the cursor was a real (if minor) usability
nuisance, even though the button worked correctly.

**Files:** `templates/admin/index.tmpl` (per-row delete modal, `.icon-btn`
CSS), `templates/admin/duplicates.tmpl` (same per-row modal pattern via
a new `admin/duplicates.tmpl:maptable` sub-template, reused across all
three report sections instead of duplicating the row markup three times)

**How it works:** Each map row now carries its own
`<div id="deleteMap-{ID}" class="modal">`, matching the exact confirm
text/buttons already used on the Edit page's delete modal (item 20),
just triggered inline (`href="#deleteMap-{ID}" class="... modal-trigger"`)
instead of navigating there. `map.tmpl`'s own Delete button/modal is
unchanged -- Edit still has its own working delete path, this just stops
forcing a detour through it from the list views.

**Potential complications:**
- Verified live: clicking Delete on both the Maps table and the
  Duplicates report opens the modal without a page navigation
  (`location.pathname` unchanged), with the correct map name/ID in the
  confirmation text; the eye icon's computed `cursor` now reads
  `pointer`.

## 24. Duplicate-map layout matching loosened from exact to 90% similarity

**What it does:** The high/lower-confidence "shared layout" groups in
item 22's Duplicates report now group maps whose tiles are at least 90%
identical (same relative position, same tile-image content), instead of
requiring every single tile to match exactly.

**Why:** Directly requested -- exact-hash matching (item 22's original
approach) is fragile to any single pixel-level rendering difference
between two otherwise-identical spawn instances (lighting, foliage
frame), which would silently prevent two real duplicates from ever
grouping together. This was already flagged as a theoretical risk in
item 22; 90% tolerates that kind of noise while still requiring the
layouts to be genuinely the same shape.

**Files:** `admin.go` (`layoutSimilarityThreshold`, `buildTileSet`,
`tileSetSimilarity`, `findDuplicateMaps` rewritten to pairwise-compare
and union-find instead of exact-fingerprint grouping),
`templates/admin/duplicates.tmpl` (wording updated from "exact tile
layout" to "at least 90%"; "~N grids each" instead of a bare count,
since group members can now have slightly different grid counts)

**How it works:** Each map's tiles are normalized into a
`{"relX,relY": tileHash}` map (same normalization as before, just kept
as a lookup instead of concatenated into one combined hash). Every pair
of small maps gets compared via `tileSetSimilarity` -- matching
(position, hash) pairs divided by the union of all positions either map
has, so it penalizes both mismatched tile content at a shared position
and a differently-shaped map. Pairs at or above 90% get merged via a
union-find, so if A-B and B-C are each individually similar enough (even
when A-C alone wouldn't clear the threshold), all three still land in
one group instead of two separate pairs.

**Potential complications:**
- Verified live against the same `reference-test-data`: the exact-match
  version had grouped the 8 confirmed spawn zones into four 2-map pairs
  (5+7, 6+13, 11+14, 12+15). At 90%, three of those pairs (6+13, 11+14,
  and implicitly whatever connected them) merged into one 4-map group
  (6, 11, 13, 14), while 5+7 and 12+15 stayed as their own separate
  pairs -- same 8 maps total, better grouped, and consistent with there
  being a few genuinely different spawn-seed variants rather than one
  single template.
- O(n^2) pairwise comparison across all small-map candidates -- fine at
  the scale tested (dozens of maps, each tile-set capped at 20 entries
  by `maxSpawnZoneGrids`), but worth knowing if an instance ever
  accumulates hundreds of small maps.

## 25. Similarity threshold lowered to 75%; map preview stitching; embedded known-spawn-zone reference

**What it does:** Two additions on top of items 20-24, plus a threshold change.

1. **Layout-match threshold lowered from 90% to 75%** (`layoutSimilarityThreshold`), to see how much further loosening it catches without pulling in unrelated maps -- lower this further only if it's still missing real duplicates.

2. **Map preview stitching**: a map's Edit page now shows every one of its grid tiles composited into a single image, for maps at or under 20 grids. A quick "look at this whole seed at once" view instead of paging through the interactive Leaflet map one grid at a time.

3. **Embedded known-spawn-zone reference** (`spawnref.go`, `findConfirmedSpawnZones`): a known-good spawn-zone layout captured from a live character spawn, embedded in the binary at compile time, matched against every small candidate map. This was initially wired up as its own "Confirmed" tier at the top of the Duplicates report -- reverted before this session ended (see item 26) since the explicit direction was to keep refining one matching method, not ship a separate report tier around it. `findConfirmedSpawnZones` and the embedded reference itself stay in the codebase, unused by the report handler, available for the next round of tuning.

**Why:** All directly requested, in the course of validating item 22 against real, freshly-generated data (a live character spawn walked through during this session).

**Files:** `admin.go` (`layoutSimilarityThreshold` now 0.75, `stitchMap`, `adminMapPreview`, `adminMap` now computes `ShowPreview`/`MaxPreviewGrids`), `main.go` (route registrations), `templates/admin/map.tmpl` (Preview card), `spawnref.go` (new -- loads the embedded reference at startup), `spawnref/tiles/*.png` + `spawnref/coords.json` (new -- the actual reference tile images and their coordinates)

**How it works:**
- **Preview stitching** (`stitchMap`) reads every grid a map has, computes its bounding box, decodes each tile PNG once up front (both to determine the real per-tile pixel size before allocating the canvas, rather than guessing and risking early/late placements disagreeing, and so one missing/corrupt file just leaves that cell blank instead of failing the whole image), then composites everything onto one `image.RGBA` canvas and serves it as JPEG. Capped at `maxSpawnZoneGrids`, same reasoning as the Duplicates report -- this is for eyeballing small candidates, never intended to run against the overworld.
- **Embedded reference**: the tile PNGs for a specific, confirmed-clean spawn instance (map 26 in this session's `reference-test-data`, chosen over its otherwise-identical pair map 25 specifically because it had no client-side rendering artifacts) were copied into `spawnref/tiles/`, with their relative coordinates recorded in `spawnref/coords.json`. `spawnref.go` uses `go:embed` to bundle both into the compiled binary. Its comparison logic was superseded in item 26.

**Potential complications:**
- The hardcoded "90%" text left over in three `duplicates.tmpl` description paragraphs after this item lowered the actual threshold to 75% -- caught and fixed in item 26, alongside the bigger matching-method rewrite.

## 26. Duplicate/spawn-zone matching rewritten from tile-content hashing to terrain-feature classification

**What it does:** Replaces how two map tiles get compared for the Duplicates report and the embedded spawn reference. Previously (items 22-25) a tile counted as "the same" as another only if its raw file bytes hashed identically. Now each tile gets downsampled into a 10x10 grid, every cell classified by hue into a small terrain bucket (grass, water, dirt/orange, red, black/unexplored, neutral), and two tiles are compared only over the cells where at least one of them has a non-grass classification. A map's landmark tiles (the ones with enough non-grass content to be worth matching on) are then greedily one-to-one matched against another map's landmark tiles, and the pair's overall score is the fraction of landmarks that found a good match. Also fixes the leftover hardcoded "90%" text in `duplicates.tmpl` (now reads `{{.ThresholdPct}}`, matching whatever `layoutSimilarityThreshold` actually is), and reverts the "Confirmed" report tier from item 25 back to `findConfirmedSpawnZones` being available but unused by the report -- explicit direction was to keep refining one matching method rather than carve out a separate always-on tier around it.

**Why:** Real ground-truth data broke content hashing outright, not just at the edges. Two maps the user identified by eye as literally the same spawn template (maps 5 and 6) scored 0% under every version of the content-hash approach, including the position-independent rewrite from earlier in this session. Direct pixel comparison of two tiles that looked identical in screenshots (map 5's grid at (9,-10) vs map 6's at (-11,-10), both showing the same river bend) showed why: 95.6% of pixels differed, some by up to ~10% of the full channel range. That's Haven & Hearth re-rolling each tile's grass texture variant independently per capture -- the river/dirt shapes render in the same spot every time, but the surrounding grass pixels never match byte-for-byte or even closely pixel-for-pixel across two different captures of the same template. No amount of hash-similarity-threshold tuning can see past that; the comparison has to ignore grass noise structurally instead.

**Files:** `admin.go` (new `classifyPixel`, `tileSignature`, `isSignalClass`, `nonGrassCount`, `tileSimilarity`, `landmarkTile` type, `buildLandmarkTiles` replacing `buildTileSet`, `landmarkSetSimilarity` replacing `tileSetSimilarity`, `sigCells`/`minLandmarkCells`/`tileMatchThreshold` constants, `findDuplicateMaps` and `debugSimilarityMatrix` updated to the new types; `crypto/sha256`/`encoding/hex` imports dropped, `math` added), `spawnref.go` (rebuilt to decode + classify the embedded reference tiles into `spawnReferenceLandmarks []landmarkTile` instead of hashing raw bytes into a `map[string]bool`), `templates/admin/duplicates.tmpl` (Confirmed section removed, hardcoded 90% text fixed), `main.go` (`/admin/debugMapShape` was written in an earlier session but never actually registered -- added the missing route while re-verifying this fix, since without it the endpoint silently fell through to the admin index page instead of erroring).

**How it works:**
- `classifyPixel` converts RGB to HSV and buckets by hue/saturation/value rather than raw color, so different grass texture rolls (still green, still mid-saturation) land in the same 'G' bucket regardless of their exact shade.
- `tileSignature` downsamples a tile to a 10x10 grid, taking the majority classification per cell -- coarse enough to smooth over per-pixel dithering noise, fine enough to keep the shape of a river bend or a lake's outline.
- `tileSimilarity` compares two signatures only over cells where at least one side is non-grass (`isSignalClass`) -- two tiles that are both plain grass in some cell get no credit for "agreeing" there, since that's not distinguishing information.
- `buildLandmarkTiles` keeps only tiles whose signature has at least `minLandmarkCells` (5) non-grass cells -- the rest are near-pure grass and would just be noise in the matching.
- `landmarkSetSimilarity` greedily matches each of map A's landmark tiles to its best-scoring available tile in map B (`tileMatchThreshold` 0.6 minimum), then scores the pair as the Dice coefficient of matched pairs over both maps' total landmark counts -- tolerant of one capture simply including a couple more/fewer grid squares than the other.

**Potential complications:**
- Verified against the full real ground-truth set from this session (maps 5, 6, 7, 11, 12, 13, 14, 15, 25, 26 -- all independently identified by the user as real spawn zones or spawn-zone captures). Every pair now scores at or above 89% (most at 100%), all comfortably above the 75% threshold, and the live Duplicates report groups all ten into one cluster. Under the old content-hash approach these split into four separate, non-overlapping clusters plus one isolated 0%-similarity pair (5-6) that should have matched.
- A second, unrelated group also surfaced under the new matching (maps 3, 19, 24, ~9 grids each) that wasn't part of the original ground-truth set -- not yet visually verified against real gameplay the way the spawn-zone set was, worth a manual look before trusting it the same way.
- `sigCells`/`minLandmarkCells`/`tileMatchThreshold` are new tunable knobs on top of the existing `layoutSimilarityThreshold` -- all four were chosen from this session's validation run, not derived analytically, so they're reasonable starting points rather than settled values if future ground truth disagrees.
- The hue-bucket boundaries in `classifyPixel` were picked by eye against this session's tile screenshots, not against Haven & Hearth's actual terrain color palette -- a terrain type whose color falls near a bucket boundary (e.g. a yellowish dirt vs. orange dirt) could get misclassified; nothing in this session's data hit that case, but it's untested.
- The maps 3/19/24 group turned out to be exactly the case above: all three are real cave/mine layers (mostly-unmined cave with a small amount of tunnel dug, natural or mined), so they legitimately share a lot of generic unmined-cave terrain without being the same specific dig. At 75%, map 3 separately scored 80% against both 19 and 24, transitively grouping all three even though 19 and 24 only matched *each other* at 33% -- a misleading result caught by checking the pair directly with `debugSimilarityMatrix?ids=3,19,24`. Fixed in item 27.

## 27. Higher similarity threshold for maps with a cave/minehole/ladder marker

**What it does:** Adds `entryMarkerSimilarityThreshold` (0.90), used instead of `layoutSimilarityThreshold` (0.75) whenever either map in a compared pair has a cave, minehole, or ladder marker on it (`hasEntry`, already computed for the no-entry-marker signal). Spawn zones never carry one of these markers, so they keep matching at 75%; real cave/mine layers now need 90%.

**Why:** Item 26's real cave/mine layers (maps 3, 19, 24) exposed a case the flat 75% threshold handled badly: two genuinely different digs into the same underlying cave template can share a lot of generic unmined-cave terrain without being duplicates of each other. A single bridging map (3) scoring 80% against both 19 and 24 was enough to transitively group all three, even though 19 and 24 only matched each other at 33%. Spawn zones don't have this problem -- they're either the exact same fixed template or they're not -- so tightening the threshold only for entry-marked maps fixes the false grouping without touching the (already-validated) spawn-zone clustering.

**Files:** `admin.go` (`entryMarkerSimilarityThreshold` constant; `findDuplicateMaps`'s union-find loop now picks the threshold per pair; `debugSimilarityMatrix` gained an optional `?ids=` query param to check an arbitrary set of maps against each other instead of the hardcoded spawn-zone list, which is what surfaced the 3/19/24 problem in the first place)

**How it works:** In `findDuplicateMaps`, before comparing two candidate maps, checks `hasEntry` for either one; if either has an entry marker, the pair needs `landmarkSetSimilarity` >= 0.90 to union, otherwise 0.75.

**Potential complications:**
- Verified live: with the 90% threshold, maps 3/19/24 no longer group at all (their 80%/80%/33% scores all now fall below the bar), while the 10-map spawn-zone cluster from item 26 is completely unaffected, since none of those maps have an entry marker. Both expected outcomes confirmed against the live report, not just the raw similarity numbers.
- 90% for entry-marked maps is a first guess based on one real example (3/19/24), not a swept/tuned value -- if a genuine cave/mine duplicate later fails to group at 90%, that's the number to revisit first.

## 28. Dungeon/house interior detection -- a fourth, completely different signal

**What it does:** Adds a new "Likely dungeon/house interior" section to the Duplicates report, based on a signal unrelated to template matching: how much of a map's own tiles render as almost entirely black. `findInteriorMaps` flags a map when at least 90% of its tiles (`darkMapThreshold`) are each at least 90% black pixels (`darkTileThreshold`), pixels counted as "dark" below a per-channel value of 30 (`darkPixelValue`). Flagged maps are pulled out of the template-match groups and the no-entrance solo list wherever they'd otherwise also appear, same one-map-one-section rule as item 26's interiors-vs-groups dedup.

**Why:** Real ground truth (maps 4, 8, 21, 22, 23, identified by eye as dungeons/interiors) exposed a real gap: none of them were showing up anywhere in the report. They weren't matching each other or the spawn reference under `landmarkSetSimilarity` -- decoding their tiles showed why: these maps render as almost solid black (0,0,0), the same exact color used for ordinary unexplored space at the edge of a spawn zone capture (confirmed byte-identical via direct pixel sampling), with only the occasional thin corridor/wall line in a distinguishing color. That line is real signal (it does classify into the existing 'O' orange bucket), but at only a few pixels wide against a 100x100 tile downsampled into 10x10 cells, it almost never reaches even one cell's majority vote, let alone `minLandmarkCells`. So per-tile terrain classification alone can't see these maps at all -- confirmed by testing: `debugSimilarityMatrix?ids=4,8,21,22,23` returned every pair as `-1` (no landmark tiles produced for any of them). Rather than trying to make the sparse-line signal work, the fix uses the one thing that *is* abundant and reliable in these maps: almost the entire tile is black, which open terrain (even at the unexplored edge of a spawn/cave capture) never is across anywhere near 90% of its tiles. Per explicit direction, this doesn't try to sub-cluster interiors by which specific dungeon/instance template they are -- everything gets flagged into one bucket for manual review.

**Files:** `admin.go` (`darkPixelValue`/`darkTileThreshold`/`darkMapThreshold` constants, `tileDarkFraction`, `findInteriorMaps`, `adminDuplicateMaps` now also computes and dedupes `Interiors`, temporary `debugInteriors` endpoint), `main.go` (route registration), `templates/admin/duplicates.tmpl` (new section)

**How it works:** `tileDarkFraction` decodes a tile and reports what fraction of its pixels are near-black. `findInteriorMaps` runs this over every tile of every small candidate map and flags the map if at least 90% of its own tiles individually cleared the 90% dark-pixel bar -- two thresholds because a single wrongly-dark or wrongly-lit tile shouldn't tip a whole map either way.

**Potential complications:**
- Verified against real data: maps 4, 8, 21, 22, 23 all measured 99.8-100% average dark-pixel fraction with every single tile clearing 90%, while the confirmed spawn zones (5, 25) sat at 30-34% and the real cave layers (3, 19, 24) sat at 1.5-5.3% -- a very clean separation, not a borderline call.
- Both threshold pairs (90%/90%) came from one round of real data with a wide margin either side, not a tuned boundary -- if a legitimately-open map with an unusually large unexplored black area ever gets flagged, or a genuine interior with more lit floor space slips through, these are the first numbers to revisit.
- Decodes every tile of every small candidate map to compute this, on top of `buildLandmarkTiles` already decoding the same tiles for template matching -- redundant work, acceptable for now given the size cap (`maxSpawnZoneGrids`) keeps this cheap regardless, but worth merging into one pass if the Duplicates report ever needs to run somewhere performance-sensitive.

## 29. Interior detection split in two: no-minimap player structures vs. real dungeon/basement instances

**What it does:** Splits item 28's single "Likely dungeon/house interior" section into two, using a new `pureBlackThreshold` (0.999) alongside the existing `darkTileThreshold`: **"Player structure -- no minimap ever rendered"** for maps where every single tile is essentially 100.000% black with zero exceptions, and **"Dungeon/basement instance -- partial minimap content"** for maps that are still almost entirely black but have at least one tile with a sliver of real content. `findInteriorMaps` now returns both lists (`noRender`, `partial`) instead of one.

**Why:** Directly requested after checking maps 8, 21, and 23 (all in item 28's original bucket) against each other -- despite different grid counts (9, 12, 9), every single tile across all three measured exactly 1.0 dark fraction with zero exceptions, while maps 4 and 22 (also in that bucket) each had a few tiles just under 1.0 (0.99-0.998). Real-world identification confirmed why: 8/21/23 are a house, a stonestead, and its upper floor -- player-built structures that have no in-game minimap at all, so there's genuinely nothing to render. 4 is the Fox Burrow dungeon and 22 is a basement, both of which do get a real (if small) minimap. That's a materially different, more actionable signal than "mostly black" alone -- a map with *provably zero* rendered content across every tile is a much safer deletion candidate than one with actual explored content on it.

**Files:** `admin.go` (`pureBlackThreshold` constant, `findInteriorMaps` now returns two slices instead of one, `adminDuplicateMaps` and `debugInteriors` updated to match), `templates/admin/duplicates.tmpl` (one section split into two)

**How it works:** `findInteriorMaps` now tracks two counters per map while scanning its tiles: `darkTiles` (unchanged, >= `darkTileThreshold`, decides whether the map qualifies as an interior at all) and `pureBlackTiles` (>= `pureBlackThreshold`). A qualifying map goes to `noRender` if literally every one of its tiles was pure-black, otherwise `partial`.

**Potential complications:**
- Verified live against the real ground truth that prompted this: maps 8, 21, 23 -> `noRender`; maps 4, 22 -> `partial`, exactly matching the manual identification.
- `pureBlackThreshold` at 0.999 rather than a literal 1.0 leaves a hair of tolerance for compression/rendering noise -- untested whether that tolerance is too generous or too strict beyond this session's five real examples.

## 30. Bulk delete via checkboxes on the Duplicates report; delete no longer bounces to the main admin page; a real interior-detection bug fixed along the way

**What it does:** Three related changes to how deleting from the Duplicates report works.

1. **Checkbox bulk delete**: every `maptable` in the report (each interior/group/solo section) is now its own `<form>` with a checkbox per row, a header "select all" checkbox, and a "Delete Selected" button, posting to a new `/admin/deleteMaps` endpoint that removes every selected map in one request/one transaction.
2. **Delete no longer always redirects to `/admin`**: both the single-map delete (existing per-row modal) and the new bulk delete now redirect back to wherever the request came from (`redirectTarget`, an optional `redirect` form value defaulting to `/admin`) -- deleting from the Duplicates report now lands back on the Duplicates report, not the main admin page.
3. **Interior detection bug fix**: found while verifying (2) against a map the user flagged as unexpectedly missing from the interiors report. `findInteriorMaps` was dividing by every grid a map is *registered* as having, but only counting dark-fraction results for tiles whose PNG file actually opened and decoded -- a map missing even a few tile files on disk got its ratio unfairly dragged down by files that were never actually there to judge. Real example: map 33, 9 registered grids but only 6 tile files present, all 6 fully dark -- 6/9 = 67%, under the 90% bar, so it never showed up anywhere in the report despite being a real interior.

**Why:** Directly requested -- deleting one map at a time from a long filtered list, with a full page reload back to the unrelated main admin page after every single one, made reviewing a list of a dozen+ flagged maps tedious. The interior-detection bug was caught as a side effect of testing this against real data.

**Files:** `admin.go` (`redirectTarget`, `deleteMapTx` extracted from the old `deleteMap` body so both `deleteMap` and the new `deleteMaps` share it, new `deleteMaps` handler, `findInteriorMaps` now tracks a separate `decoded` count instead of using `len(entries)` as the denominator, new temporary `debugMapMeta` endpoint used to diagnose the map-33 case), `main.go` (route registration), `templates/admin/duplicates.tmpl` (`maptable` wrapped in a form with checkboxes, select-all, and a Delete Selected button; single-row delete link now passes `redirect=/admin/duplicateMaps`)

**How it works:**
- `deleteMapTx(tx, mapid)` holds the actual bucket-by-bucket deletion logic (grids, markers, roads, custom markers, tiles, the map entry itself) against an already-open transaction. `deleteMap` opens one transaction and calls it once; `deleteMaps` parses however many `map` values were submitted, opens one transaction, and calls it in a loop -- so a batch delete is atomic (all removed or none, if something fails partway).
- `redirectTarget(req)` reads the `redirect` form value and validates it's an internal path (starts with a single `/`, not `//`) before use, since it flows into `http.Redirect`; falls back to `/admin` if absent or invalid.
- The bulk-delete form's submit handler counts checked boxes client-side, blocks submission with an alert if none are selected, and otherwise shows a native `confirm()` naming the count before submitting -- same "this cannot be undone" framing as the existing single-delete modal.

**Potential complications:**
- Verified the redirect fix and checkbox wiring live (DOM inspection: 26 row checkboxes, 4 select-all checkboxes, 4 forms posting to `/admin/deleteMaps`; single-row delete links confirmed carrying `redirect=/admin/duplicateMaps`) but did not execute an actual delete against the test data during verification.
- Bulk delete has no per-map error reporting -- if one map ID in a batch fails partway through (e.g. a concurrent modification), the whole transaction rolls back and the admin just sees an internal error page with no indication of which map or how many succeeded. Fine for now given this is a low-traffic admin-only tool, but worth revisiting if batches get large or this becomes more heavily used.
- The interior-detection denominator fix changes real report output -- any map with missing tile files that was previously silently excluded from the interiors report may now appear there (confirmed: map 33 now correctly appears under "partial minimap content").

## 31. Two Duplicates-report fixes: invisible checkboxes, and a Preview icon

**What it does:** Two small fixes on top of item 30.

1. **Checkboxes were invisible.** Materialize's global CSS hides raw `<input type="checkbox">` (opacity 0, taken out of flow) and expects it wrapped in `<label><input/><span></span></label>` to render its custom checkbox box via the `<span>`. Item 30's checkboxes were dropped in bare, so they existed and worked in the DOM but were never visible on screen. Wrapped both the per-row and header "select all" checkboxes in that structure.
2. **Preview icon**: each row's Edit/Delete icon pair now has a third "Preview" icon (Material icon `image`) between them, opening the same stitched-tile-image modal pattern already used for delete-confirm -- an `<img>` pointing at the existing `/admin/mapPreview?map=X` endpoint from item 25, no new backend needed since every map in this report is already under the 20-grid cap that endpoint requires.

**Why:** (1) directly reported after item 30 shipped -- confirmed via computed style that the checkboxes had `opacity:0` and were absolutely positioned. (2) directly requested, to let an admin eyeball what a flagged map actually looks like without leaving the report to open its Edit page.

**Files:** `templates/admin/duplicates.tmpl` (`maptable` sub-template: checkbox markup, new preview icon + modal)

**How it works:** No backend changes -- both fixes are template-only. The preview modal reuses the exact same `<img src="/admin/mapPreview?map={{.ID}}">` pattern item 25 already put on the Edit page, just inside the report's existing modal-trigger/`.modal` convention instead of an inline card.

**Potential complications:**
- Verified live: computed style on a checkbox's `::before` now shows an 18x18px box with a visible border (previously invisible); clicking a Preview icon opens the modal (`class="modal open"`) and the image loads successfully (confirmed 300x400px for map 21).

## 32. Adjustable map order in the frontend dropdown, plus a real ID-corruption bug fixed along the way

**What it does:** Adds `MapInfo.SortOrder int`, editable from two places: an inline "Order" column + save button on the main admin Maps table (`/admin/setMapOrder`, new), and a "Sort Order" field on the full Edit-map page (existing `adminMap` POST handler). The frontend map-switcher dropdown (`MapView.vue`) now sorts by this value ascending, ties broken alphabetically by name -- replacing a sort comparator that referenced a `.size` field that doesn't exist on `MapInfo` and so silently did nothing, leaving the dropdown in whatever order the maps object happened to iterate in.

While verifying this, found and fixed a real, independent bug: four separate places that load or save a `MapInfo` (`adminICMap`'s toggle-hidden, the new `setMapOrder`, `adminMap`'s GET and POST, `findSmallMapCandidates`, and `getMaps` in map.go) trusted whatever `ID` value was stored in the record's own JSON instead of the bucket key it was actually stored/looked-up under. For a map that had never had an explicit `maps` bucket entry written (confirmed live: map 5 in this session's test data), that meant the record's `ID` field defaulted to 0 -- and saving it back (e.g. from the Edit page, or this session's own `setMapOrder` test call) would silently persist `"ID":0` into the record permanently, corrupting it for every future read until fixed.

**Why:** The ordering was directly requested. The ID bug was caught mid-verification: after testing `setMapOrder` against map 5 (which turned out to have no prior `maps` bucket entry), `/map/api/maps` came back with `"ID":0` for it instead of 5 -- exactly the kind of corruption that would send the frontend's map switcher the wrong map ID when that entry was selected, or silently misdirect an Edit-page save to bucket key "0" instead of the map actually being edited.

**Files:** `main.go` (`MapInfo.SortOrder`), `admin.go` (`setMapOrder` new handler + route, `adminMap` GET/POST and `adminICMap` and `findSmallMapCandidates` all now re-apply `mapinfo.ID = mapid`/`mi.ID = mapid` after unmarshal), `map.go` (`getMaps` same fix), `templates/admin/index.tmpl` (Order column), `templates/admin/map.tmpl` (Sort Order field), `frontend/src/components/MapView.vue` (fixed sort comparator)

**How it works:** Every place that loads a `MapInfo` by its bucket key now treats that key as authoritative for the `ID` field, re-assigning it *after* `json.Unmarshal` (not just as a struct default before it) -- unconditional, so it also self-heals any record that already got corrupted by the old code path, without needing a data migration. The frontend sort is a plain two-key comparator: `SortOrder` ascending, then `Name.localeCompare` for anything tied (which is every map that's never had an order set, since the field defaults to 0).

**Potential complications:**
- Verified live end-to-end: set map 5's `SortOrder` to -100 via `/admin/setMapOrder`, confirmed it sorted first ahead of three real named maps (`Overworld`, `Edgeville Mine - Layer 1`/`Layer 2`, which already had SortOrder 0/1/2 from prior real use) once the standard sort comparator was applied to the live `/map/api/maps` response; also confirmed the ID-corruption fix self-healed map 5's record (`ID:0` before the fix, `ID:5` after, despite the stale value already being stored from testing) and reverted the test SortOrder value back to 0 afterward.
- The admin Maps table's inline Order input has no client-side validation beyond `type="number"` -- an empty submission saves as 0 (same default new maps already start with), not an error.
- Did not audit every other place `MapInfo` gets read (only the ones actually involved in saving/serving it to a client) -- if another code path also assumes a stored `ID` is trustworthy, it wasn't touched here.

## 33. Client token prefix now auto-resolved from the request instead of requiring manual admin setup

**What it does:** The copy/paste-ready client token shown on the Tokens page (`http://.../client/{token}`) previously always needed an admin to manually type the server's public address into "Set prefix for tokens" first -- defaulting to the literal placeholder `http://example.com` if they never did. Now it's resolved automatically from the request that's loading the page itself (`requestOrigin`, new): whatever scheme+host a browser used to reach this page is necessarily valid to paste back into Kami/Nurgling. The admin setting still exists but is now an optional override, only used when explicitly set to a non-empty value -- for the rare case a server sits behind something where the incoming request's own host doesn't match the address clients actually need (e.g. an internal hostname).

**Why:** Directly requested -- there's no reason the server needs to be manually told its own address for something it can derive from every request it already receives, and a forgotten/never-set prefix was previously an easy way to silently hand out a broken, unusable `http://example.com` token.

**Files:** `manage.go` (`requestOrigin` new helper, `index` handler default changed from a hardcoded string to `requestOrigin(req)`, stored config only overrides when non-empty), `templates/admin/index.tmpl` (label/description updated to describe it as an override, placeholder text added)

**How it works:** `requestOrigin` checks `X-Forwarded-Proto`/`X-Forwarded-Host` first (same reasoning as the existing `clientIP` helper's `X-Forwarded-For` check -- correct behind a reverse proxy like nginx terminating TLS in front of this server), falling back to `req.TLS`/`req.Host` for a direct connection.

**Potential complications:**
- Verified live: loading the Tokens page at `http://localhost:8083/` now shows the token as `http://localhost:8083/client/{token}` with no prefix ever configured, instead of the old `http://example.com/client/{token}` default.
- If this server is ever put behind a reverse proxy that doesn't set `X-Forwarded-Proto`/`X-Forwarded-Host`, the scheme could resolve to `http` even when the public-facing connection is actually `https` -- the manual override in admin settings still covers that case, just no longer needed for the common direct-or-standard-proxy setup.

## 34. Redundant nav self-links removed; Tokens page opened up to public visitors with a locked shared token; personal prefix override restored

**What it does:** Three related navigation/access changes.

1. **No more self-referential nav buttons.** The map view's own app-bar always showed a "Map" button even though it's the only page that app-bar ever renders on -- removed outright. The Go-template navbar (Tokens/Admin/Settings/Password/Login pages) always showed a "Tokens" link even while already on the Tokens page -- now hidden via a new `Page.CurrentPath` field, same pattern as before but generalized.
2. **Tokens page now reachable by public visitors.** Previously `/` unconditionally redirected anyone without a real login straight to `/map/`, so there was no way to see anything token-related without an account. Now: the map view's "Tokens" button (visible to everyone, not just logged-in users, via a new `/?tokens` link) takes a public visitor to a locked, read-only view showing one admin-designated shared token instead of a personal list -- no Generate/Delete/Change-Password controls, just the token line. Typing the bare domain directly (no `?tokens`) still goes straight to the map, unchanged -- only the explicit Tokens link opens the locked view, per the fact that most visits are just "go look at the map."
3. **Personal prefix override, back but relocated.** Item 33 moved the token-URL prefix from a manually-configured admin setting to auto-detection from the request. This adds a lightweight per-visitor safety valve right on the Tokens page itself: an "Address override" text field, pre-filled with the resolved prefix, that live-edits the displayed token line(s) in the browser (plain JS, nothing sent to the server, nothing saved) -- for the case someone's own network path to the server doesn't match what the server sees. Shown to every visitor, including the locked public view.

**Why:** All directly requested. The public-token piece needed a real "shared token" concept that didn't exist before -- `PublicConfig.Token`, a real token from some account's Tokens page (with Upload permission) that the admin explicitly designates for public advertising; upload permission still comes entirely from whichever real account generated it, this field is purely what gets displayed to anonymous visitors.

**Files:** `main.go` (`Page.CurrentPath`, `PublicConfig.Token`), `admin.go` (`adminPublic` saves the new `token` form field), `manage.go` (`index` handler: public+no-session case now serves a locked view gated behind `?tokens` instead of always redirecting), `templates/navbar.tmpl` (Tokens link hidden on `CurrentPath == "/"`), `templates/index.tmpl` (`Locked` mode hides Change Password/Generate Token, adds the token-line data attributes + override input/script), `templates/admin/public.tmpl` (new Token field), `frontend/src/components/MapView.vue` (Map button removed, Tokens button un-gated from `isPublic` and pointed at `/?tokens`)

**How it works:**
- `Page.CurrentPath` is set from `req.URL.Path` in `getPage`, `json:"-"` since it's render-only, never persisted.
- The locked Tokens view builds a `Session{Auths: pub.Auths}` (empty Username, matching the existing `getSessionOrPublic` convention) so the same `{{if .Session.Auths.Has "upload"}}` gate index.tmpl already had for real users applies identically here -- the admin has to have checked "Upload" under Public's Roles for the token section to render at all, same requirement as any other Public-auth-gated feature.
- The override input's `input` listener rewrites every `.token-line` element's text using its `data-token` attribute plus whatever the field currently holds, trimming trailing slashes -- purely cosmetic/local, the actual `{{$.Prefix}}` server-rendered value is untouched and is what a fresh page load always shows again.

**Potential complications:**
- Verified the entire flow live: navbar Tokens link disappears while already on `/`; admin's own token page unaffected; after enabling Public with Upload checked and setting a real generated token as the shared Token, logging out and visiting bare `/` redirected to `/map/#/` as before, the map app-bar showed only Tokens (no Map) + Login, and following Tokens (`/?tokens`) rendered the locked page with the correct token line, no Change Password/Generate Token controls, and the override field live-updating the displayed line on input.
- The shared public token is exactly as sensitive as any other upload token -- anyone who can view the public Tokens page can now also see and use it. That's the intended behavior (it's meant to be shared), but worth remembering it's not scoped or rate-limited differently from a personal token just because it's labeled "public".

## 35. Item 34 follow-up fixes: overlapping label, blank navbar for public visitors, cave icon size

**What it does:** Three small fixes caught while using item 34 live.

1. **Overlapping text**: the Tokens page's new "Address override" field had a long, sentence-length `<label>` -- Materialize only reserves one line of space above an input for its shrunk/active label, so the wrapped second line visually collided with the input's value below it. Shortened the label to two words and moved the explanation to a plain paragraph above the field instead.
2. **Blank navbar for public visitors**: the account-menu trigger in the top-right of every Go-template page shows `{{.Session.Username}}` -- blank for an anonymous public visitor (empty string), rather than something indicating you're not logged in. Now shows a plain "Login" link instead of an empty dropdown trigger when there's no real username, mirroring the pattern the map view's own app-bar already used.
3. **Cave icons now match Mineshaft's smaller size**: `buildIcon` in `Marker.js` sized Mineshaft (minehole/ladder) markers at a fixed 13.5, smaller than every other marker type's 18 default -- Cave markers were still falling into that 18 default. Now sized the same as Mineshaft.

**Why:** All three caught/requested directly while testing item 34 and the map view.

**Files:** `templates/index.tmpl` (label fix), `templates/navbar.tmpl` (Login link fix), `frontend/src/data/Marker.js` (`hsz` ternary now includes `isCave` alongside `isMineshaft`)

**How it works:** No new mechanisms -- (1) and (3) are template/formula tweaks; (2) wraps both the account-menu `<li>` and the entire `#navbar-user-dropdown` list in `{{if .Session.Username}}`, since an anonymous session has nothing sensible to put in Admin/Settings/Logout anyway.

**Potential complications:**
- Verified live: the Tokens page label no longer wraps into the input; the public locked Tokens page's navbar now shows "Login" instead of a blank dropdown trigger. Cave icon sizing verified by code review (mirrors the existing, already-shipped Mineshaft path exactly) but not visually confirmed against a real cave marker in this session's test data.
