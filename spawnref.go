package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

// spawnReferenceTileSet is loaded once at startup, same
// {"relX,relY": tileHash} shape buildTileSet produces for a real map --
// same key format (fmt.Sprintf("%d,%d", ...)) and same hash source (raw
// file bytes, not a decode/re-encode round trip, which would never
// match buildTileSet's hashes for identical-looking tiles) so it can go
// through the exact same tileSetSimilarity comparison.
var spawnReferenceTileSet map[string]string

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

	minX, minY := coords[0].X, coords[0].Y
	for _, c := range coords {
		if c.X < minX {
			minX = c.X
		}
		if c.Y < minY {
			minY = c.Y
		}
	}

	tiles := map[string]string{}
	for _, c := range coords {
		f, err := spawnRefFS.Open("spawnref/tiles/" + c.ID + ".png")
		if err != nil {
			log.Printf("spawnref: missing tile %s: %v", c.ID, err)
			continue
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		f.Close()
		if copyErr != nil {
			log.Printf("spawnref: failed to read tile %s: %v", c.ID, copyErr)
			continue
		}
		key := fmt.Sprintf("%d,%d", c.X-minX, c.Y-minY)
		tiles[key] = hex.EncodeToString(h.Sum(nil))
	}
	spawnReferenceTileSet = tiles
	log.Printf("spawnref: loaded %d reference tiles", len(tiles))
}
