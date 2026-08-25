package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"

	"go.etcd.io/bbolt"
)

type Config struct {
	Title    string   `json:"title"`
	Auths    []string `json:"auths"`
	Username string   `json:"username"`
	// Public is true when this request has no real session and is running
	// on the PublicConfig fallback auths (see getSessionOrPublic) -- the
	// frontend uses this to show a "Login" link instead of a username.
	Public bool `json:"public"`
	// ShowPlayerNames/HideCharacterNames are personal preferences from the
	// account's own User record (see settings.go) -- omitted entirely for
	// an account that's never visited /settings, or for a Public session
	// (no account to read), so the frontend keeps its own default instead
	// of every account silently starting from Go's zero value.
	ShowPlayerNames    *bool    `json:"showPlayerNames,omitempty"`
	HideCharacterNames *bool    `json:"hideCharacterNames,omitempty"`
	ThingwallScale     *float64 `json:"thingwallScale,omitempty"`
}

func (m *Map) getChars(rw http.ResponseWriter, req *http.Request) {
	s := m.getSessionOrPublic(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !s.Auths.Has(AUTH_POINTER) {
		json.NewEncoder(rw).Encode([]interface{}{})
		return
	}
	groups := groupArr(s.Auths)
	chars := []Character{}
	m.chmu.RLock()
	defer m.chmu.RUnlock()
	for _, v := range m.characters {
		b := hasCommonElement(groups, v.group)
		if b {
			chars = append(chars, v)
		}
		//log.Printf("Auth: %v, User: %v, Char: %v, %t\n", s.Auths, groups, v.group, b)
	}
	json.NewEncoder(rw).Encode(chars)
}

func (m *Map) sendPing(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	mapid, err := strconv.Atoi(req.FormValue("map"))
	if err != nil {
		http.Error(rw, "map parse failed", http.StatusBadRequest)
		return
	}
	x, err := strconv.Atoi(req.FormValue("x"))
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
		return
	}
	y, err := strconv.Atoi(req.FormValue("y"))
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
		return
	}
	m.pingUpdates.send(&Ping{
		Map:      mapid,
		X:        x,
		Y:        y,
		Username: s.Username,
	})
}

func (m *Map) getPingSound(rw http.ResponseWriter, req *http.Request) {
	s := m.getSessionOrPublic(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	name := ""
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		if b == nil {
			return nil
		}
		name = string(b.Get([]byte("pingSound")))
		return nil
	})
	if name == "" {
		rw.WriteHeader(http.StatusNoContent)
		return
	}
	rw.Header().Set("Cache-Control", "no-store")
	http.ServeFile(rw, req, filepath.Join(m.gridStorage, "sounds", name))
}

func (m *Map) getMarkers(rw http.ResponseWriter, req *http.Request) {
	s := m.getSessionOrPublic(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !s.Auths.Has(AUTH_MARKERS) {
		json.NewEncoder(rw).Encode([]interface{}{})
		return
	}
	markers := []FrontendMarker{}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("markers"))
		if b == nil {
			return nil
		}
		grid := b.Bucket([]byte("grid"))
		if grid == nil {
			return nil
		}
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		return grid.ForEach(func(k, v []byte) error {
			m := Marker{}
			json.Unmarshal(v, &m)
			graw := grids.Get([]byte(m.GridID))
			if graw == nil {
				// log.Printf("DEBUG getMarkers: DROPPED id=%d name=%q grid=%s image=%q hidden=%v: grid not registered\n",
				// 	m.ID, m.Name, m.GridID, m.Image, m.Hidden)
				return nil
			}
			g := GridData{}
			json.Unmarshal(graw, &g)
			// log.Printf("DEBUG getMarkers: SERVED id=%d name=%q image=%q hidden=%v map=%d worldPos=%d,%d\n",
			// 	m.ID, m.Name, m.Image, m.Hidden, g.Map, m.Position.X+g.Coord.X*100, m.Position.Y+g.Coord.Y*100)
			markers = append(markers, FrontendMarker{
				Image:  m.Image,
				Hidden: m.Hidden,
				ID:     m.ID,
				Name:   m.Name,
				Map:    g.Map,
				Position: Position{
					X: m.Position.X + g.Coord.X*100,
					Y: m.Position.Y + g.Coord.Y*100,
				},
			})
			return nil
		})
	})
	json.NewEncoder(rw).Encode(markers)
}

func (m *Map) getMaps(rw http.ResponseWriter, req *http.Request) {
	s := m.getSessionOrPublic(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	maps := map[int]*MapInfo{}
	m.db.View(func(tx *bbolt.Tx) error {
		mapB := tx.Bucket([]byte("maps"))
		if mapB == nil {
			return nil
		}
		return mapB.ForEach(func(k, v []byte) error {
			mapid, err := strconv.Atoi(string(k))
			if err != nil {
				return nil
			}
			mi := &MapInfo{}
			json.Unmarshal(v, &mi)
			// The bucket key is authoritative -- a record saved by an
			// older, buggy admin handler could have an explicit
			// stale/zero ID stored in it (see admin.go's adminMap for
			// the fix), which would otherwise send the frontend's map
			// switcher a wrong/colliding map ID.
			mi.ID = mapid
			if mi.Hidden {
				return nil
			}
			if mi.RequiredAuth != "" && !s.Auths.Has(mi.RequiredAuth) {
				return nil
			}
			maps[mapid] = mi
			return nil
		})
	})
	json.NewEncoder(rw).Encode(maps)
}

func (m *Map) config(rw http.ResponseWriter, req *http.Request) {
	s := m.getSessionOrPublic(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	config := Config{
		Auths:    s.Auths,
		Username: s.Username,
		Public:   s.Username == "",
	}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		if b != nil {
			title := b.Get([]byte("title"))
			config.Title = string(title)
		}
		if s.Username != "" {
			if users := tx.Bucket([]byte("users")); users != nil {
				if raw := users.Get([]byte(s.Username)); raw != nil {
					u := User{}
					if json.Unmarshal(raw, &u) == nil {
						config.ShowPlayerNames = u.ShowPlayerNames
						config.HideCharacterNames = u.HideCharacterNames
						config.ThingwallScale = u.ThingwallScale
					}
				}
			}
		}
		return nil
	})
	json.NewEncoder(rw).Encode(config)
}

func groupArr(a Auths) []int {
	groups := []int{}
	if a.Has(AUTH_GROUP1) {
		groups = append(groups, 1)
	}
	if a.Has(AUTH_GROUP2) {
		groups = append(groups, 2)
	}
	if a.Has(AUTH_GROUP3) {
		groups = append(groups, 3)
	}
	if a.Has(AUTH_GROUP4) {
		groups = append(groups, 4)
	}
	if a.Has(AUTH_GROUP5) {
		groups = append(groups, 5)
	}

	return groups
}

func hasCommonElement(arr1, arr2 []int) bool {
	if len(arr2) == 0 {
		return true
	}
	if len(arr1) == 0 {
		return false
	}
	for _, num1 := range arr1 {
		for _, num2 := range arr2 {
			if num1 == num2 {
				return true
			}
		}
	}

	return false
}
