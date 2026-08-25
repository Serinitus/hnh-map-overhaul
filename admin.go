package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

// TierOption is one choice in the "required tier" dropdowns for both map
// layers and settings-drawer sections -- both reuse the same Auths values
// (see AUTH_GROUP1-5 etc in main.go) as the tier vocabulary.
type TierOption struct {
	Value string
	Label string
}

var tierOptions = []TierOption{
	{"", "None"},
	{AUTH_GROUP1, "Group 1"},
	{AUTH_GROUP2, "Group 2"},
	{AUTH_GROUP3, "Group 3"},
	{AUTH_GROUP4, "Group 4"},
	{AUTH_GROUP5, "Group 5"},
	{"writer", "Redactor"},
	{AUTH_ADMIN, "Admin"},
}

// SectionOption is one Markers-panel section that can be granted per
// account, the same way Roles (Map/Markers/Admin/etc) already are. Key
// must match the section key used in MapView.vue's canSeeSection() calls;
// the checkbox itself is stored as an ordinary Auths entry "sec_<Key>", so
// no separate storage/admin plumbing is needed beyond this list -- adding
// a new section here is enough to make it a new per-account toggle.
type SectionOption struct {
	Key   string
	Label string
}

// Players/Characters is deliberately absent here: unlike every other
// section, it's already gated by a real, server-enforced auth (AUTH_POINTER
// aka "Characters" in Roles, see getChars in map.go) -- a second sec_players
// checkbox would just be a UI switch that has to agree with a data-access
// switch to do anything, so MapView.vue's Players toggle checks
// auths.includes("point") directly instead of going through this list.
var sectionOptions = []SectionOption{
	{"naturalMarkers", "Natural Resources"},
	{"customMarkers", "Custom Markers"},
	{"clutter", "Clutter"},
	{"thingwalls", "Thingwalls"},
	{"vortexes", "Vortexes"},
	{"questGivers", "Quest Givers"},
	{"roads", "Roads"},
	{"other", "Other"},
}

func (m *Map) admin(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	users := []string{}
	prefix := ""
	maps := []MapInfo{}
	defaultHide := false
	pingSound := ""
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return nil
		}
		config := tx.Bucket([]byte("config"))
		if config != nil {
			prefix = string(config.Get([]byte("prefix")))
			defaultHide = config.Get([]byte("defaultHide")) != nil
			pingSound = string(config.Get([]byte("pingSound")))
		}
		mapB := tx.Bucket([]byte("maps"))
		if mapB != nil {
			mapB.ForEach(func(k, v []byte) error {
				mi := MapInfo{}
				json.Unmarshal(v, &mi)
				maps = append(maps, mi)
				return nil
			})
		}
		return b.ForEach(func(k, v []byte) error {
			users = append(users, string(k))
			return nil
		})
	})

	pingSounds := []string{}
	entries, err := os.ReadDir(filepath.Join(m.gridStorage, "sounds"))
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				pingSounds = append(pingSounds, e.Name())
			}
		}
	}

	m.ExecuteTemplate(rw, filepath.FromSlash("admin/index.tmpl"), struct {
		Page          Page
		Session       *Session
		Users         []string
		Prefix        string
		DefaultHide   bool
		Maps          []MapInfo
		PingSounds    []string
		PingSound     string
		TierOptions   []TierOption
		PublicEnabled bool
	}{
		Page:          m.getPage(req),
		Session:       s,
		Users:         users,
		Prefix:        prefix,
		DefaultHide:   defaultHide,
		Maps:          maps,
		PingSounds:    pingSounds,
		PingSound:     pingSound,
		TierOptions:   tierOptions,
		PublicEnabled: m.getPublicConfig().Enabled,
	})
}

func (m *Map) adminUser(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	if req.Method == "POST" {
		req.ParseForm()
		username := req.FormValue("user")
		password := req.FormValue("pass")
		auths := req.Form["auths"]
		tempAdmin := false
		m.db.Update(func(tx *bbolt.Tx) error {
			users, err := tx.CreateBucketIfNotExists([]byte("users"))
			if err != nil {
				return err
			}
			if s.Username == "admin" && users.Get([]byte("admin")) == nil {
				tempAdmin = true
			}
			u := User{}
			raw := users.Get([]byte(username))
			if raw != nil {
				json.Unmarshal(raw, &u)
			}
			if password != "" {
				u.Pass, _ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			}
			u.Auths = auths
			raw, _ = json.Marshal(u)
			users.Put([]byte(username), raw)
			return nil
		})
		if username == s.Username {
			s.Auths = auths
		}
		if tempAdmin {
			m.deleteSession(s)
		}
		http.Redirect(rw, req, "/admin", 302)
		return
	}

	user := req.FormValue("user")
	u := User{}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return nil
		}
		userRaw := b.Get([]byte(user))
		if userRaw == nil {
			return nil
		}
		return json.Unmarshal(userRaw, &u)
	})

	m.ExecuteTemplate(rw, filepath.FromSlash("admin/user.tmpl"), struct {
		Page           Page
		Session        *Session
		User           User
		Username       string
		SectionOptions []SectionOption
	}{
		Page:           m.getPage(req),
		Session:        s,
		User:           u,
		Username:       user,
		SectionOptions: sectionOptions,
	})
}

// adminPublic edits the singleton "Public" settings (see PublicConfig in
// main.go) -- not a row in the users bucket, so it has no username,
// password, or delete action, just an Enabled switch plus the same
// Roles/Markers checkboxes a normal account gets.
func (m *Map) adminPublic(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	if req.Method == "POST" {
		req.ParseForm()
		cfg := PublicConfig{
			Enabled: req.FormValue("enabled") != "",
			Auths:   req.Form["auths"],
		}
		m.db.Update(func(tx *bbolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte("config"))
			if err != nil {
				return err
			}
			raw, err := json.Marshal(cfg)
			if err != nil {
				return err
			}
			return b.Put([]byte("public"), raw)
		})
		http.Redirect(rw, req, "/admin", 302)
		return
	}

	m.ExecuteTemplate(rw, filepath.FromSlash("admin/public.tmpl"), struct {
		Page           Page
		Session        *Session
		Config         PublicConfig
		SectionOptions []SectionOption
	}{
		Page:           m.getPage(req),
		Session:        s,
		Config:         m.getPublicConfig(),
		SectionOptions: sectionOptions,
	})
}

func (m *Map) wipe(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	err := m.db.Update(func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte("grids")) != nil {
			err := tx.DeleteBucket([]byte("grids"))
			if err != nil {
				return err
			}
		}
		if tx.Bucket([]byte("markers")) != nil {
			err := tx.DeleteBucket([]byte("markers"))
			if err != nil {
				return err
			}
		}
		if tx.Bucket([]byte("tiles")) != nil {
			err := tx.DeleteBucket([]byte("tiles"))
			if err != nil {
				return err
			}
		}
		if tx.Bucket([]byte("maps")) != nil {
			err := tx.DeleteBucket([]byte("maps"))
			if err != nil {
				return err
			}
		}
		if tx.Bucket([]byte("roads")) != nil {
			err := tx.DeleteBucket([]byte("roads"))
			if err != nil {
				return err
			}
		}
		if tx.Bucket([]byte("customMarkers")) != nil {
			err := tx.DeleteBucket([]byte("customMarkers"))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Println(err)
	}
	/*for z := 1; z <= 6; z++ {
		os.RemoveAll(fmt.Sprintf("%s/%d", m.gridStorage, z))
	}*/
	http.Redirect(rw, req, "/admin/", 302)
}

func (m *Map) setPrefix(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		return b.Put([]byte("prefix"), []byte(req.FormValue("prefix")))
	})
	http.Redirect(rw, req, "/admin/", 302)
}

func (m *Map) setDefaultHide(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		if req.FormValue("defaultHide") != "" {
			return b.Put([]byte("defaultHide"), []byte(req.FormValue("defaultHide")))
		} else {
			return b.Delete([]byte("defaultHide"))
		}
	})
	http.Redirect(rw, req, "/admin/", 302)
}

func (m *Map) uploadPingSound(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	err := req.ParseMultipartForm(1024 * 1024 * 20)
	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	f, hdr, err := req.FormFile("sound")
	if err != nil {
		log.Println(err)
		http.Error(rw, "request error", http.StatusBadRequest)
		return
	}
	defer f.Close()

	err = os.MkdirAll(filepath.Join(m.gridStorage, "sounds"), 0777)
	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	out, err := os.Create(filepath.Join(m.gridStorage, "sounds", filepath.Base(hdr.Filename)))
	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	defer out.Close()
	io.Copy(out, f)

	http.Redirect(rw, req, "/admin/", 302)
}

func (m *Map) setPingSound(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	name := filepath.Base(req.FormValue("sound"))
	m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		return b.Put([]byte("pingSound"), []byte(name))
	})
	http.Redirect(rw, req, "/admin/", 302)
}

func (m *Map) setTitle(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		return b.Put([]byte("title"), []byte(req.FormValue("title")))
	})
	http.Redirect(rw, req, "/admin/", 302)
}

type zoomproc struct {
	c Coord
	m int
}

func (m *Map) rebuildZooms(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	needProcess := map[zoomproc]struct{}{}
	saveGrid := map[zoomproc]string{}

	noGrids := false
	m.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("grids"))
		if b == nil {
			noGrids = true
			return nil
		}
		b.ForEach(func(k, v []byte) error {
			grid := GridData{}
			json.Unmarshal(v, &grid)
			needProcess[zoomproc{grid.Coord.Parent(), grid.Map}] = struct{}{}
			saveGrid[zoomproc{grid.Coord, grid.Map}] = grid.ID
			return nil
		})
		tx.DeleteBucket([]byte("tiles"))
		return nil
	})

	if noGrids {
		return
	}
	go func() {
		log.Println("Rebuild Zooms...")
		log.Println("Rebuild Zooms Saving...")
		for g, id := range saveGrid {
			f := fmt.Sprintf("%s/grids/%s.png", m.gridStorage, id)
			if _, err := os.Stat(f); err != nil {
				continue
			}
			//log.Println("Rebuild Zooms: Save: " + f)
			m.SaveTile(g.m, g.c, 0, fmt.Sprintf("grids/%s.png", id), time.Now().UnixNano())
		}
		for z := 1; z <= 6; z++ {
			log.Printf("Rebuild Zooms: %d", z)
			process := needProcess
			needProcess = map[zoomproc]struct{}{}
			for p := range process {
				//log.Printf("Update Zooms: %d:%d", p.m, p.c)
				m.updateZoomLevel(p.m, p.c, z)
				needProcess[zoomproc{p.c.Parent(), p.m}] = struct{}{}
			}
		}
		log.Println("Rebuild Zooms Finish!")
	}()
	http.Redirect(rw, req, "/admin/", 302)
}

func (m *Map) deleteUser(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	username := req.FormValue("user")
	m.db.Update(func(tx *bbolt.Tx) error {
		users, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return err
		}
		u := User{}
		raw := users.Get([]byte(username))
		if raw != nil {
			json.Unmarshal(raw, &u)
		}
		tokens, err := tx.CreateBucketIfNotExists([]byte("tokens"))
		if err != nil {
			return err
		}
		for _, tok := range u.Tokens {
			err = tokens.Delete([]byte(tok))
			if err != nil {
				return err
			}
		}
		err = users.Delete([]byte(username))
		if err != nil {
			return err
		}
		return nil
	})
	if username == s.Username {
		m.deleteSession(s)
	}
	http.Redirect(rw, req, "/admin", 302)
	return
}

var errFound = errors.New("found tile")

func (m *Map) wipeTile(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !(s.Auths.Has(AUTH_ADMIN) || s.Auths.Has(AUTH_WRITER)) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	mraw := req.FormValue("map")
	mapid, err := strconv.Atoi(mraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	xraw := req.FormValue("x")
	x, err := strconv.Atoi(xraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	yraw := req.FormValue("y")
	y, err := strconv.Atoi(yraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	c := Coord{
		X: x,
		Y: y,
	}

	m.db.Update(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		ids := [][]byte{}
		err := grids.ForEach(func(k, v []byte) error {
			g := GridData{}
			err := json.Unmarshal(v, &g)
			if err != nil {
				return err
			}
			if g.Coord == c && g.Map == mapid {
				ids = append(ids, k)
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, id := range ids {
			grids.Delete(id)
		}

		return nil
	})

	m.SaveTile(mapid, c, 0, "", -1)
	for z := 1; z <= 6; z++ {
		c = c.Parent()
		m.updateZoomLevel(mapid, c, z)
	}
	rw.WriteHeader(200)
}

func (m *Map) setCoords(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !(s.Auths.Has(AUTH_ADMIN) || s.Auths.Has(AUTH_WRITER)) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	mraw := req.FormValue("map")
	mapid, err := strconv.Atoi(mraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	fxraw := req.FormValue("fx")
	fx, err := strconv.Atoi(fxraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	fyraw := req.FormValue("fy")
	fy, err := strconv.Atoi(fyraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	fc := Coord{
		X: fx,
		Y: fy,
	}

	txraw := req.FormValue("tx")
	tx, err := strconv.Atoi(txraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	tyraw := req.FormValue("ty")
	ty, err := strconv.Atoi(tyraw)
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
	}
	tc := Coord{
		X: tx,
		Y: ty,
	}

	diff := Coord{
		X: tc.X - fc.X,
		Y: tc.Y - fc.Y,
	}
	tds := []*TileData{}
	m.db.Update(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		tiles := tx.Bucket([]byte("tiles"))
		if tiles == nil {
			return nil
		}
		mapZooms := tiles.Bucket([]byte(strconv.Itoa(mapid)))
		if mapZooms == nil {
			return nil
		}
		mapTiles := mapZooms.Bucket([]byte("0"))
		err := grids.ForEach(func(k, v []byte) error {
			g := GridData{}
			err := json.Unmarshal(v, &g)
			if err != nil {
				return err
			}
			if g.Map == mapid {
				g.Coord.X += diff.X
				g.Coord.Y += diff.Y
				raw, _ := json.Marshal(g)
				grids.Put(k, raw)
			}
			return nil
		})
		if err != nil {
			return err
		}
		pixelDiff := Position{X: diff.X * 100, Y: diff.Y * 100}
		if roads := tx.Bucket([]byte("roads")); roads != nil {
			err = roads.ForEach(func(k, v []byte) error {
				r := Road{}
				err := json.Unmarshal(v, &r)
				if err != nil {
					return err
				}
				if r.Map == mapid {
					r.PointA.X += pixelDiff.X
					r.PointA.Y += pixelDiff.Y
					r.PointB.X += pixelDiff.X
					r.PointB.Y += pixelDiff.Y
					raw, _ := json.Marshal(r)
					roads.Put(k, raw)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		if customMarkers := tx.Bucket([]byte("customMarkers")); customMarkers != nil {
			err = customMarkers.ForEach(func(k, v []byte) error {
				cm := CustomMarker{}
				err := json.Unmarshal(v, &cm)
				if err != nil {
					return err
				}
				if cm.Map == mapid {
					cm.X += pixelDiff.X
					cm.Y += pixelDiff.Y
					raw, _ := json.Marshal(cm)
					customMarkers.Put(k, raw)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		err = mapTiles.ForEach(func(k, v []byte) error {
			td := &TileData{}
			err := json.Unmarshal(v, &td)
			if err != nil {
				return err
			}
			td.Coord.X += diff.X
			td.Coord.Y += diff.Y
			tds = append(tds, td)
			return nil
		})
		if err != nil {
			return err
		}
		err = tiles.DeleteBucket([]byte(strconv.Itoa(mapid)))
		if err != nil {
			return err
		}
		return nil
	})
	needProcess := map[zoomproc]struct{}{}
	for _, td := range tds {
		m.SaveTile(td.MapID, td.Coord, td.Zoom, td.File, time.Now().UnixNano())
		needProcess[zoomproc{c: Coord{X: td.Coord.X, Y: td.Coord.Y}.Parent(), m: td.MapID}] = struct{}{}
	}
	for z := 1; z <= 6; z++ {
		process := needProcess
		needProcess = map[zoomproc]struct{}{}
		for p := range process {
			m.updateZoomLevel(p.m, p.c, z)
			needProcess[zoomproc{p.c.Parent(), p.m}] = struct{}{}
		}
	}
	rw.WriteHeader(200)
}

func (m *Map) backup(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}
	rw.Header().Set("Content-Type", "application/zip")
	rw.Header().Set("Content-Disposition", "attachment; filename=\"backup.zip\"")

	zw := zip.NewWriter(rw)
	defer zw.Close()

	err := m.db.Update(func(tx *bbolt.Tx) error {
		w, err := zw.Create("grids.db")
		if err != nil {
			return err
		}
		err = tx.Copy(w)
		if err != nil {
			return err
		}

		tiles := tx.Bucket([]byte("tiles"))
		if tiles == nil {
			return nil
		}
		zoom := tiles.Bucket([]byte("0"))
		if zoom == nil {
			return nil
		}
		return zoom.ForEach(func(k, v []byte) error {
			td := TileData{}
			json.Unmarshal(v, &td)
			if td.File == "" {
				return nil
			}
			f, err := os.Open(m.gridStorage + "/" + td.File)
			if err != nil {
				return nil
			}
			defer f.Close()
			w, err := zw.Create(td.File)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, f)
			return err
		})
	})
	if err != nil {
		log.Println(err)
	}

}

type mapData struct {
	Grids   map[string]string
	Markers map[string][]Marker
}

func (m *Map) export(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Error(rw, "Unauthorized", http.StatusUnauthorized)
		return
	}
	rw.Header().Set("Content-Type", "application/zip")
	rw.Header().Set("Content-Disposition", "attachment; filename=\"griddata.zip\"")

	zw := zip.NewWriter(rw)
	defer zw.Close()

	err := m.db.Update(func(tx *bbolt.Tx) error {
		maps := map[int]mapData{}
		gridMap := map[string]int{}

		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		tiles := tx.Bucket([]byte("tiles"))
		if tiles == nil {
			return nil
		}

		err := grids.ForEach(func(k, v []byte) error {
			gd := GridData{}
			err := json.Unmarshal(v, &gd)
			if err != nil {
				return err
			}
			md, ok := maps[gd.Map]
			if !ok {
				md = mapData{
					Grids:   map[string]string{},
					Markers: map[string][]Marker{},
				}
				maps[gd.Map] = md
			}
			md.Grids[gd.Coord.Name()] = gd.ID
			gridMap[gd.ID] = gd.Map
			mapb := tiles.Bucket([]byte(strconv.Itoa(gd.Map)))
			if mapb == nil {
				return nil
			}
			zoom := mapb.Bucket([]byte("0"))
			if zoom == nil {
				return nil
			}
			tdraw := zoom.Get([]byte(gd.Coord.Name()))
			if tdraw == nil {
				return nil
			}
			td := TileData{}
			err = json.Unmarshal(tdraw, &td)
			if err != nil {
				return err
			}
			w, err := zw.Create(fmt.Sprintf("%d/%s.png", gd.Map, gd.ID))
			if err != nil {
				return err
			}
			f, err := os.Open(filepath.Join(m.gridStorage, td.File))
			if err != nil {
				return err
			}
			defer f.Close()
			io.Copy(w, f)
			return nil
		})
		if err != nil {
			return err
		}

		err = func() error {
			markersb := tx.Bucket([]byte("markers"))
			if markersb == nil {
				return nil
			}
			markersgrid := markersb.Bucket([]byte("grid"))
			if markersgrid == nil {
				return nil
			}
			return markersgrid.ForEach(func(k, v []byte) error {
				m := Marker{}
				err := json.Unmarshal(v, &m)
				if err != nil {
					return nil
				}
				if _, ok := maps[gridMap[m.GridID]]; ok {
					maps[gridMap[m.GridID]].Markers[m.GridID] = append(maps[gridMap[m.GridID]].Markers[m.GridID], m)
				}
				return nil
			})
		}()
		if err != nil {
			return err
		}

		for mapid, mapdata := range maps {
			w, err := zw.Create(fmt.Sprintf("%d/grids.json", mapid))
			if err != nil {
				return err
			}
			json.NewEncoder(w).Encode(mapdata)
		}
		return nil
	})
	if err != nil {
		log.Println(err)
	}

}

func (m *Map) hideMarker(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !(s.Auths.Has(AUTH_ADMIN) || s.Auths.Has(AUTH_WRITER)) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	err := m.db.Update(func(tx *bbolt.Tx) error {
		mb, err := tx.CreateBucketIfNotExists([]byte("markers"))
		if err != nil {
			return err
		}
		grid, err := mb.CreateBucketIfNotExists([]byte("grid"))
		if err != nil {
			return err
		}
		idB, err := mb.CreateBucketIfNotExists([]byte("id"))
		if err != nil {
			return err
		}
		key := idB.Get([]byte(req.FormValue("id")))
		if key == nil {
			return fmt.Errorf("Could not find key %s", req.FormValue("id"))
		}
		raw := grid.Get(key)
		if raw == nil {
			return fmt.Errorf("Could not find key %s", string(key))
		}
		m := Marker{}
		json.Unmarshal(raw, &m)
		m.Hidden = true
		raw, _ = json.Marshal(m)
		grid.Put(key, raw)
		return nil
	})
	if err != nil {
		log.Println(err)
	}
	return
}

func (m *Map) merge(rw http.ResponseWriter, req *http.Request) {
	err := req.ParseMultipartForm(1024 * 1024 * 500)
	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	mergef, hdr, err := req.FormFile("merge")
	if err != nil {
		log.Println(err)
		http.Error(rw, "request error", http.StatusBadRequest)
		return
	}
	zr, err := zip.NewReader(mergef, hdr.Size)
	if err != nil {
		log.Println(err)
		http.Error(rw, "request error", http.StatusBadRequest)
		return
	}

	ops := []struct {
		mapid int
		x, y  int
		f     string
	}{}
	newTiles := map[string]struct{}{}

	err = m.db.Update(func(tx *bbolt.Tx) error {
		grids, err := tx.CreateBucketIfNotExists([]byte("grids"))
		if err != nil {
			return err
		}
		tiles, err := tx.CreateBucketIfNotExists([]byte("tiles"))
		if err != nil {
			return err
		}
		mb, err := tx.CreateBucketIfNotExists([]byte("markers"))
		if err != nil {
			return err
		}
		mgrid, err := mb.CreateBucketIfNotExists([]byte("grid"))
		if err != nil {
			return err
		}
		idB, err := mb.CreateBucketIfNotExists([]byte("id"))
		if err != nil {
			return err
		}
		configb, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		for _, fhdr := range zr.File {
			if strings.HasSuffix(fhdr.Name, ".json") {
				f, err := fhdr.Open()
				if err != nil {
					return err
				}
				md := mapData{}
				err = json.NewDecoder(f).Decode(&md)
				if err != nil {
					return err
				}

				for _, ms := range md.Markers {
					for _, mraw := range ms {
						key := []byte(fmt.Sprintf("%s_%d_%d", mraw.GridID, mraw.Position.X, mraw.Position.Y))
						if mgrid.Get(key) != nil {
							continue
						}
						if mraw.Image == "" {
							mraw.Image = "gfx/terobjs/mm/custom"
						}
						id, err := idB.NextSequence()
						if err != nil {
							return err
						}
						idKey := []byte(strconv.Itoa(int(id)))
						m := Marker{
							Name:   mraw.Name,
							ID:     int(id),
							GridID: mraw.GridID,
							Position: Position{
								X: mraw.Position.X,
								Y: mraw.Position.Y,
							},
							Image: mraw.Image,
						}
						raw, _ := json.Marshal(m)
						mgrid.Put(key, raw)
						idB.Put(idKey, key)
					}
				}

				mapB, err := tx.CreateBucketIfNotExists([]byte("maps"))
				if err != nil {
					return err
				}

				newGrids := map[Coord]string{}
				maps := map[int]struct{ X, Y int }{}
				for k, v := range md.Grids {
					c := Coord{}
					_, err := fmt.Sscanf(k, "%d_%d", &c.X, &c.Y)
					if err != nil {
						return err
					}
					newGrids[c] = v
					gridRaw := grids.Get([]byte(v))
					if gridRaw != nil {
						gd := GridData{}
						json.Unmarshal(gridRaw, &gd)
						maps[gd.Map] = struct{ X, Y int }{gd.Coord.X - c.X, gd.Coord.Y - c.Y}
					}
				}
				if len(maps) == 0 {
					seq, err := mapB.NextSequence()
					if err != nil {
						return err
					}
					mi := MapInfo{
						ID:     int(seq),
						Name:   strconv.Itoa(int(seq)),
						Hidden: configb.Get([]byte("defaultHide")) != nil,
					}
					raw, _ := json.Marshal(mi)
					err = mapB.Put([]byte(strconv.Itoa(int(seq))), raw)
					if err != nil {
						return err
					}
					for c, grid := range newGrids {
						cur := GridData{}
						cur.ID = grid
						cur.Map = int(seq)
						cur.Coord = c

						raw, err := json.Marshal(cur)
						if err != nil {
							return err
						}
						grids.Put([]byte(grid), raw)
					}
					continue
				}

				mapid := -1
				offset := struct{ X, Y int }{}
				for id, off := range maps {
					mi := MapInfo{}
					mraw := mapB.Get([]byte(strconv.Itoa(id)))
					if mraw != nil {
						json.Unmarshal(mraw, &mi)
					}
					if mi.Priority {
						mapid = id
						offset = off
						break
					}
					if id < mapid || mapid == -1 {
						mapid = id
						offset = off
					}
				}

				for c, grid := range newGrids {
					cur := GridData{}
					if curRaw := grids.Get([]byte(grid)); curRaw != nil {
						continue
					}

					cur.ID = grid
					cur.Map = mapid
					cur.Coord.X = c.X + offset.X
					cur.Coord.Y = c.Y + offset.Y
					raw, err := json.Marshal(cur)
					if err != nil {
						return err
					}
					grids.Put([]byte(grid), raw)
				}
				if len(maps) > 1 {
					grids.ForEach(func(k, v []byte) error {
						gd := GridData{}
						json.Unmarshal(v, &gd)
						if gd.Map == mapid {
							return nil
						}
						if merge, ok := maps[gd.Map]; ok {
							var td *TileData
							mapb, err := tiles.CreateBucketIfNotExists([]byte(strconv.Itoa(gd.Map)))
							if err != nil {
								return err
							}
							zoom, err := mapb.CreateBucketIfNotExists([]byte(strconv.Itoa(0)))
							if err != nil {
								return err
							}
							tileraw := zoom.Get([]byte(gd.Coord.Name()))
							if tileraw != nil {
								json.Unmarshal(tileraw, &td)
							}

							gd.Map = mapid
							gd.Coord.X += offset.X - merge.X
							gd.Coord.Y += offset.Y - merge.Y
							raw, _ := json.Marshal(gd)
							if td != nil {
								ops = append(ops, struct {
									mapid int
									x     int
									y     int
									f     string
								}{
									mapid: mapid,
									x:     gd.Coord.X,
									y:     gd.Coord.Y,
									f:     td.File,
								})
							}
							grids.Put(k, raw)
						}
						return nil
					})
				}
				for mergeid, merge := range maps {
					if mapid == mergeid {
						continue
					}
					mapB.Delete([]byte(strconv.Itoa(mergeid)))
					log.Println("Reporting merge", mergeid, mapid)
					m.reportMerge(mergeid, mapid, Coord{X: offset.X - merge.X, Y: offset.Y - merge.Y})
				}

			} else if strings.HasSuffix(fhdr.Name, ".png") {
				os.MkdirAll(filepath.Join(m.gridStorage, "grids"), 0777)
				f, err := os.Create(filepath.Join(m.gridStorage, "grids", filepath.Base(fhdr.Name)))
				if err != nil {
					return err
				}
				r, err := fhdr.Open()
				if err != nil {
					f.Close()
					return err
				}
				io.Copy(f, r)
				r.Close()
				f.Close()
				newTiles[strings.TrimSuffix(filepath.Base(fhdr.Name), ".png")] = struct{}{}
			}
		}

		for gid := range newTiles {
			gridRaw := grids.Get([]byte(gid))
			if gridRaw != nil {
				gd := GridData{}
				json.Unmarshal(gridRaw, &gd)
				ops = append(ops, struct {
					mapid int
					x     int
					y     int
					f     string
				}{
					mapid: gd.Map,
					x:     gd.Coord.X,
					y:     gd.Coord.Y,
					f:     filepath.Join("grids", gid+".png"),
				})
			}
		}
		return nil
	})

	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}

	for _, op := range ops {
		m.SaveTile(op.mapid, Coord{X: op.x, Y: op.y}, 0, op.f, time.Now().UnixNano())
	}
	m.rebuildZooms(rw, req)
}

func (m *Map) adminICMap(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	mraw := req.FormValue("map")
	mapid, err := strconv.Atoi(mraw)
	if err != nil {
		http.Error(rw, "map parse failed", http.StatusBadRequest)
		return
	}

	action := req.FormValue("action")

	m.db.Update(func(tx *bbolt.Tx) error {
		maps, err := tx.CreateBucketIfNotExists([]byte("maps"))
		if err != nil {
			return err
		}
		rawmap := maps.Get([]byte(strconv.Itoa(mapid)))
		mapinfo := MapInfo{}
		if rawmap != nil {
			json.Unmarshal(rawmap, &mapinfo)
		}
		switch action {
		case "toggle-hidden":
			mapinfo.Hidden = !mapinfo.Hidden
			m.ExecuteTemplate(rw, filepath.FromSlash("admin/index.tmpl:toggle-hidden"), mapinfo)
		}
		rawmap, err = json.Marshal(mapinfo)
		if err != nil {
			return err
		}
		return maps.Put([]byte(strconv.Itoa(mapid)), rawmap)
	})
}

func (m *Map) adminMap(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	mraw := req.FormValue("map")
	mapid, err := strconv.Atoi(mraw)
	if err != nil {
		http.Error(rw, "map parse failed", http.StatusBadRequest)
		return
	}

	if req.Method == "POST" {
		req.ParseForm()

		name := req.FormValue("name")
		hidden := !(req.FormValue("hidden") == "")
		priority := !(req.FormValue("priority") == "")
		requiredAuth := req.FormValue("requiredAuth")

		m.db.Update(func(tx *bbolt.Tx) error {
			maps, err := tx.CreateBucketIfNotExists([]byte("maps"))
			if err != nil {
				return err
			}
			rawmap := maps.Get([]byte(strconv.Itoa(mapid)))
			mapinfo := MapInfo{}
			if rawmap != nil {
				json.Unmarshal(rawmap, &mapinfo)
			}
			mapinfo.Name = name
			mapinfo.Hidden = hidden
			mapinfo.Priority = priority
			mapinfo.RequiredAuth = requiredAuth
			rawmap, err = json.Marshal(mapinfo)
			if err != nil {
				return err
			}
			return maps.Put([]byte(strconv.Itoa(mapid)), rawmap)
		})

		http.Redirect(rw, req, "/admin", 302)
		return
	}
	mi := MapInfo{}
	m.db.View(func(tx *bbolt.Tx) error {
		mapB := tx.Bucket([]byte("maps"))
		if mapB == nil {
			return nil
		}
		mraw := mapB.Get([]byte(strconv.Itoa(mapid)))
		return json.Unmarshal(mraw, &mi)
	})

	m.ExecuteTemplate(rw, filepath.FromSlash("admin/map.tmpl"), struct {
		Page        Page
		Session     *Session
		MapInfo     MapInfo
		TierOptions []TierOption
	}{
		Page:        m.getPage(req),
		Session:     s,
		MapInfo:     mi,
		TierOptions: tierOptions,
	})
}

// deleteMap permanently removes a map layer and everything scoped to it --
// grids, tiles, markers, roads, and custom markers -- unlike Hide (map.go's
// toggle-hidden), which only stops it from being listed. Irreversible, no
// backing up/soft-delete; mirrors the existing "Wipe all data" pattern
// (see wipe() above) but scoped to one map instead of every map.
func (m *Map) deleteMap(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}

	mapid, err := strconv.Atoi(req.FormValue("map"))
	if err != nil {
		http.Error(rw, "map parse failed", http.StatusBadRequest)
		return
	}
	mapKey := []byte(strconv.Itoa(mapid))

	err = m.db.Update(func(tx *bbolt.Tx) error {
		// Grids: flat bucket keyed by grid ID, each value carrying which
		// map it belongs to -- collect the grid IDs for this map first,
		// both to delete them and because markers below are keyed by
		// grid ID, not map ID, so we need that set to find them.
		deletedGridIDs := map[string]bool{}
		if grids := tx.Bucket([]byte("grids")); grids != nil {
			c := grids.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				gd := GridData{}
				if json.Unmarshal(v, &gd) == nil && gd.Map == mapid {
					deletedGridIDs[string(k)] = true
				}
			}
			for gid := range deletedGridIDs {
				if err := grids.Delete([]byte(gid)); err != nil {
					return err
				}
			}
		}

		// Markers: stored under markers/grid, keyed by "{gridID}_{x}_{y}"
		// (see markerUpdate in client.go), with markers/id as a separate
		// id->gridkey index for admin lookups by numeric ID. Delete both
		// sides for every marker whose grid ID is one we just removed.
		if mb := tx.Bucket([]byte("markers")); mb != nil {
			deletedGridKeys := map[string]bool{}
			if grid := mb.Bucket([]byte("grid")); grid != nil {
				c := grid.Cursor()
				for k, _ := c.First(); k != nil; k, _ = c.Next() {
					gridID := strings.SplitN(string(k), "_", 2)[0]
					if deletedGridIDs[gridID] {
						deletedGridKeys[string(k)] = true
					}
				}
				for gk := range deletedGridKeys {
					if err := grid.Delete([]byte(gk)); err != nil {
						return err
					}
				}
			}
			if idB := mb.Bucket([]byte("id")); idB != nil {
				staleIDs := [][]byte{}
				c := idB.Cursor()
				for k, v := c.First(); k != nil; k, v = c.Next() {
					if deletedGridKeys[string(v)] {
						staleIDs = append(staleIDs, append([]byte{}, k...))
					}
				}
				for _, k := range staleIDs {
					if err := idB.Delete(k); err != nil {
						return err
					}
				}
			}
		}

		// Roads and Custom Markers: flat buckets keyed by their own
		// numeric ID, each value carrying a Map field directly.
		if roads := tx.Bucket([]byte("roads")); roads != nil {
			stale := [][]byte{}
			c := roads.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				r := Road{}
				if json.Unmarshal(v, &r) == nil && r.Map == mapid {
					stale = append(stale, append([]byte{}, k...))
				}
			}
			for _, k := range stale {
				if err := roads.Delete(k); err != nil {
					return err
				}
			}
		}
		if cm := tx.Bucket([]byte("customMarkers")); cm != nil {
			stale := [][]byte{}
			c := cm.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				marker := CustomMarker{}
				if json.Unmarshal(v, &marker) == nil && marker.Map == mapid {
					stale = append(stale, append([]byte{}, k...))
				}
			}
			for _, k := range stale {
				if err := cm.Delete(k); err != nil {
					return err
				}
			}
		}

		// Tiles: already nested as tiles/{mapID}/{zoom}/{coord} (see
		// SaveTile in tile.go), so this is the one bucket that doesn't
		// need scanning -- just drop its whole sub-bucket.
		if tiles := tx.Bucket([]byte("tiles")); tiles != nil {
			if tiles.Bucket(mapKey) != nil {
				if err := tiles.DeleteBucket(mapKey); err != nil {
					return err
				}
			}
		}

		maps, err := tx.CreateBucketIfNotExists([]byte("maps"))
		if err != nil {
			return err
		}
		return maps.Delete(mapKey)
	})
	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(rw, req, "/admin", 302)
}

// DuplicateGroup is a set of maps that appear to be the same templated
// layout -- e.g. spawn zones, and likely dungeons/house interiors too,
// since Haven & Hearth generates all of these from a fixed set of
// templates rather than exploring real terrain. Detection only, no
// auto-delete -- deleteMap above handles removal once a human confirms
// a group is actually junk.
type DuplicateGroup struct {
	Fingerprint string
	Maps        []MapInfo
	GridCount   int
	// NoneHaveEntry is true when every map in this group also has zero
	// Mineshaft/Cave-type markers -- both signals agreeing is a stronger
	// hint than either alone (confirmed against real spawn-zone data:
	// every spawn zone found this way hit both signals at once).
	NoneHaveEntry bool
}

// maxSpawnZoneGrids is the size cutoff below which a map is even
// considered as a duplicate-template candidate -- real explored areas
// (however small) grow past this quickly, but a generated
// spawn/dungeon/interior template stays small. Below this, maps are
// fingerprinted and compared; anything larger is skipped entirely, both
// because it won't be a template match and to keep this cheap (the main
// overworld alone can have tens of thousands of grids).
const maxSpawnZoneGrids = 20

// Same image sets Marker.js uses for MINESHAFT_IMAGES/CAVE_IMAGES --
// kept in sync manually since this is Go, not shared code. A real
// mine/cave layer gets an entrance marker dropped on it when it's
// physically connected to another map (see the entry-point marker in
// item 4/gridUpdate); a map with none of these is either not actually
// a mine/cave, or is disconnected -- e.g. a spawn zone, reached by
// teleport rather than a physical ladder/cave-in.
var entryMarkerImages = map[string]bool{
	"mm/down": true, "mm/up": true, "gfx/terobjs/minehole": true, "gfx/terobjs/ladder": true,
	"gfx/hud/mmap/cave": true, "gfx/tiles/ridges/cavein": true, "gfx/tiles/ridges/cavein2": true, "gfx/tiles/ridges/caveout": true,
}

// findSmallMapCandidates returns, for every map at or under
// maxSpawnZoneGrids, its grid count and grid entries (id/coord) --
// shared groundwork for both findDuplicateMaps and
// findMapsWithoutEntry so the grids bucket is only scanned once.
func (m *Map) findSmallMapCandidates() (gridCounts map[int]int, candidates map[int][]struct {
	id   string
	x, y int
}, mapInfos map[int]MapInfo, err error) {
	gridCounts = map[int]int{}
	err = m.db.View(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids == nil {
			return nil
		}
		return grids.ForEach(func(k, v []byte) error {
			gd := GridData{}
			if json.Unmarshal(v, &gd) == nil {
				gridCounts[gd.Map]++
			}
			return nil
		})
	})
	if err != nil {
		return
	}

	candidates = map[int][]struct {
		id   string
		x, y int
	}{}
	mapInfos = map[int]MapInfo{}
	err = m.db.View(func(tx *bbolt.Tx) error {
		grids := tx.Bucket([]byte("grids"))
		if grids != nil {
			ferr := grids.ForEach(func(k, v []byte) error {
				gd := GridData{}
				if json.Unmarshal(v, &gd) != nil {
					return nil
				}
				if count := gridCounts[gd.Map]; count > 0 && count <= maxSpawnZoneGrids {
					candidates[gd.Map] = append(candidates[gd.Map], struct {
						id   string
						x, y int
					}{id: gd.ID, x: gd.Coord.X, y: gd.Coord.Y})
				}
				return nil
			})
			if ferr != nil {
				return ferr
			}
		}
		maps := tx.Bucket([]byte("maps"))
		if maps != nil {
			for mapid := range candidates {
				raw := maps.Get([]byte(strconv.Itoa(mapid)))
				mi := MapInfo{ID: mapid}
				if raw != nil {
					json.Unmarshal(raw, &mi)
				}
				mapInfos[mapid] = mi
			}
		}
		return nil
	})
	return
}

// findMapsWithoutEntry returns every small candidate map (same size
// cutoff as findDuplicateMaps) that has zero Mineshaft/Cave-type
// markers anywhere on it -- a second, independent signal for the same
// kind of junk map, since a map could fail to template-match anything
// (e.g. it's the only instance of its kind currently in the data) but
// still be an orphaned zone with no real entrance.
// computeHasEntry checks, for every map in candidates, whether it has at
// least one Mineshaft/Cave-type marker on it -- shared by
// findDuplicateMaps (to flag when a whole group also lacks any entry,
// the strongest combined signal) and findMapsWithoutEntry.
func (m *Map) computeHasEntry(candidates map[int][]struct {
	id   string
	x, y int
}) (map[int]bool, error) {
	gridToMap := map[string]int{}
	for mapid, entries := range candidates {
		for _, e := range entries {
			gridToMap[e.id] = mapid
		}
	}

	hasEntry := map[int]bool{}
	err := m.db.View(func(tx *bbolt.Tx) error {
		mb := tx.Bucket([]byte("markers"))
		if mb == nil {
			return nil
		}
		grid := mb.Bucket([]byte("grid"))
		if grid == nil {
			return nil
		}
		return grid.ForEach(func(k, v []byte) error {
			gridID := strings.SplitN(string(k), "_", 2)[0]
			mapid, ok := gridToMap[gridID]
			if !ok {
				return nil
			}
			marker := Marker{}
			if json.Unmarshal(v, &marker) == nil && entryMarkerImages[marker.Image] {
				hasEntry[mapid] = true
			}
			return nil
		})
	})
	return hasEntry, err
}

func (m *Map) findMapsWithoutEntry() ([]MapInfo, error) {
	_, candidates, mapInfos, err := m.findSmallMapCandidates()
	if err != nil {
		return nil, err
	}
	hasEntry, err := m.computeHasEntry(candidates)
	if err != nil {
		return nil, err
	}

	result := []MapInfo{}
	for mapid := range candidates {
		if !hasEntry[mapid] {
			result = append(result, mapInfos[mapid])
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// layoutSimilarityThreshold is how much of two maps' tiles have to line
// up (same relative position, same tile-image content hash) for them to
// count as the same template. Started at an exact 100% match, but a
// single client-side rendering difference (lighting, foliage frame)
// between two otherwise-identical spawn instances would silently break
// that -- 90% tolerates a handful of mismatched tiles while still
// requiring the layouts to be genuinely the same shape.
const layoutSimilarityThreshold = 0.90

// mapTileSet is a map's grids normalized to (relative x, relative y) ->
// tile content hash, relative to that map's own bounding box so the
// same template matches regardless of which absolute coordinates the
// mapper assigned it.
func (m *Map) buildTileSet(entries []struct {
	id   string
	x, y int
}) map[string]string {
	minX, minY := entries[0].x, entries[0].y
	for _, e := range entries {
		if e.x < minX {
			minX = e.x
		}
		if e.y < minY {
			minY = e.y
		}
	}
	tiles := map[string]string{}
	for _, e := range entries {
		f, err := os.Open(filepath.Join(m.gridStorage, "grids", e.id+".png"))
		if err != nil {
			// A grid this map thinks it has, but whose tile image is
			// missing on disk -- skip that one tile rather than failing
			// this map's whole comparison (and definitely rather than
			// crashing the report over one bad file).
			continue
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		f.Close()
		if copyErr != nil {
			continue
		}
		key := fmt.Sprintf("%d,%d", e.x-minX, e.y-minY)
		tiles[key] = hex.EncodeToString(h.Sum(nil))
	}
	return tiles
}

// tileSetSimilarity is the fraction of (position, tile-hash) pairs the
// two sets agree on, out of every position either one has -- so it
// penalizes both mismatched tile content at a shared position and a
// differently-shaped map (extra/missing positions), not just one or
// the other.
func tileSetSimilarity(a, b map[string]string) float64 {
	union := map[string]bool{}
	matches := 0
	for pos, hashA := range a {
		union[pos] = true
		if hashB, ok := b[pos]; ok && hashB == hashA {
			matches++
		}
	}
	for pos := range b {
		union[pos] = true
	}
	if len(union) == 0 {
		return 0
	}
	return float64(matches) / float64(len(union))
}

func (m *Map) findDuplicateMaps() ([]DuplicateGroup, error) {
	gridCounts, candidates, mapInfos, err := m.findSmallMapCandidates()
	if err != nil {
		return nil, err
	}
	hasEntry, err := m.computeHasEntry(candidates)
	if err != nil {
		return nil, err
	}

	mapids := make([]int, 0, len(candidates))
	tileSets := map[int]map[string]string{}
	for mapid, entries := range candidates {
		mapids = append(mapids, mapid)
		tileSets[mapid] = m.buildTileSet(entries)
	}
	sort.Ints(mapids)

	// Union-find over the >=90%-similar pairs, so A-B and B-C both being
	// similar enough (even if A-C alone wouldn't clear the threshold)
	// still lands all three in one group.
	parent := map[int]int{}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for _, id := range mapids {
		parent[id] = id
	}
	for i := 0; i < len(mapids); i++ {
		for j := i + 1; j < len(mapids); j++ {
			if tileSetSimilarity(tileSets[mapids[i]], tileSets[mapids[j]]) >= layoutSimilarityThreshold {
				union(mapids[i], mapids[j])
			}
		}
	}

	byRoot := map[int][]int{}
	for _, id := range mapids {
		root := find(id)
		byRoot[root] = append(byRoot[root], id)
	}

	result := []DuplicateGroup{}
	for root, members := range byRoot {
		if len(members) < 2 {
			continue
		}
		dg := DuplicateGroup{Fingerprint: fmt.Sprintf("group-%d", root)}
		dg.NoneHaveEntry = true
		minGrids := gridCounts[members[0]]
		for _, id := range members {
			dg.Maps = append(dg.Maps, mapInfos[id])
			if hasEntry[id] {
				dg.NoneHaveEntry = false
			}
			if gridCounts[id] < minGrids {
				minGrids = gridCounts[id]
			}
		}
		dg.GridCount = minGrids
		sort.Slice(dg.Maps, func(i, j int) bool { return dg.Maps[i].ID < dg.Maps[j].ID })
		result = append(result, dg)
	}
	sort.Slice(result, func(i, j int) bool { return len(result[i].Maps) > len(result[j].Maps) })
	return result, nil
}

func (m *Map) adminDuplicateMaps(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_ADMIN) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	groups, err := m.findDuplicateMaps()
	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	noEntry, err := m.findMapsWithoutEntry()
	if err != nil {
		log.Println(err)
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	// Solo is the no-entry list minus anything already shown in a
	// template-match group above -- keeps this one combined report
	// instead of listing the same map twice under two headings.
	inGroup := map[int]bool{}
	for _, g := range groups {
		for _, mi := range g.Maps {
			inGroup[mi.ID] = true
		}
	}
	solo := []MapInfo{}
	for _, mi := range noEntry {
		if !inGroup[mi.ID] {
			solo = append(solo, mi)
		}
	}
	m.ExecuteTemplate(rw, filepath.FromSlash("admin/duplicates.tmpl"), struct {
		Page    Page
		Session *Session
		Groups  []DuplicateGroup
		Solo    []MapInfo
	}{
		Page:    m.getPage(req),
		Session: s,
		Groups:  groups,
		Solo:    solo,
	})
}

