package main

import (
	"embed"
	"encoding/json"
	"image"
	_ "image/png"
	"log"
)

// Known-good spawn zone reference tiles, embedded at compile time.
// Captured from a live, freshly-created character spawn (map 26 in the
// reference test data, 2026-08-25) with no client-side rendering
// artifacts. Confirmed by the person who was standing in it that this
// specific layout is the exact, unvarying spawn template -- not just
// one instance among many random layouts.
//
// This lets a small map be recognized as a spawn zone on its own, even
// if it's the only instance of its particular seed currently in the
// data (the peer-to-peer matching in findDuplicateMaps only groups
// maps that match *each other*, so a genuinely unique/unmatched spawn
// instance would otherwise only ever land in the weaker
// no-entrance-marker bucket instead of being confidently identified).
//
//go:embed spawnref/tiles/*.png spawnref/coords.json
var spawnRefFS embed.FS

type spawnRefCoord struct {
	ID string `json:"id"`
	X  int    `json:"x"`
	Y  int    `json:"y"`
}

// spawnReferenceLandmarks is loaded once at startup -- same landmark
// tile signatures buildLandmarkTiles produces for a real map (see
// tileSignature's comment for why raw pixel/byte comparison doesn't
// work: Haven & Hearth re-rolls each grass tile's texture variant per
// capture, so two renders of the identical template never hash or
// pixel-match on their grass background). Goes through the exact same
// landmarkSetSimilarity comparison as any other map. coords.json's X/Y
// fields aren't used for the comparison itself -- kept as a record of
// what was actually captured.
var spawnReferenceLandmarks []landmarkTile

func init() {
	raw, err := spawnRefFS.ReadFile("spawnref/coords.json")
	if err != nil {
		log.Printf("spawnref: failed to load coords.json: %v", err)
		return
	}
	var coords []spawnRefCoord
	if err := json.Unmarshal(raw, &coords); err != nil {
		log.Printf("spawnref: failed to parse coords.json: %v", err)
		return
	}

	var tiles []landmarkTile
	for _, c := range coords {
		f, err := spawnRefFS.Open("spawnref/tiles/" + c.ID + ".png")
		if err != nil {
			log.Printf("spawnref: missing tile %s: %v", c.ID, err)
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			log.Printf("spawnref: failed to decode tile %s: %v", c.ID, err)
			continue
		}
		sig := tileSignature(img)
		if nonGrassCount(sig) >= minLandmarkCells {
			tiles = append(tiles, landmarkTile{ID: c.ID, Sig: sig})
		}
	}
	spawnReferenceLandmarks = tiles
	log.Printf("spawnref: loaded %d landmark tiles", len(tiles))
}
