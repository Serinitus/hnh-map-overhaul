package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cediner/hnh-map-vuetify/webapp"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

type Map struct {
	gridStorage string
	db          *bbolt.DB

	characters map[string]Character
	chmu       sync.RWMutex

	// Last known position of each user's player character, kept in the same
	// grid-local coordinate form used for marker storage (GridID + local
	// offset) so a new-map creation event can be linked back to it without
	// any coordinate math.
	lastPos map[string]LastPosition

	*webapp.WebApp

	gridUpdates  topic
	mergeUpdates mergeTopic
	pingUpdates  pingTopic
}

type LastPosition struct {
	Map     int
	GridID  string
	X, Y    int
	Updated time.Time
}

type Session struct {
	ID        string
	Username  string
	Auths     Auths `json:"-"`
	TempAdmin bool
}

var (
	gridStorage = flag.String("grids", "grids", "directory to store grids in")
	port        = flag.Int("port", func() int {
		if port, ok := os.LookupEnv("HNHMAP_PORT"); ok {
			p, err := strconv.Atoi(port)
			if err != nil {
				log.Fatal(err)
			}
			return p
		}
		return 8080
	}(), "Port to listen on")
)

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "public/favicon.ico")
}

func main() {
	flag.Parse()

	db, err := bbolt.Open(*gridStorage+"/grids.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	m := Map{
		gridStorage: *gridStorage,
		db:          db,

		characters: map[string]Character{},
		lastPos:    map[string]LastPosition{},

		WebApp: webapp.Must(webapp.New().LoadTemplates("./templates/")),
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		vraw := b.Get([]byte("version"))
		v, _ := strconv.Atoi(string(vraw))
		if v < len(migrations) {
			for _, f := range migrations[v:] {
				err = f(tx)
				if err != nil {
					return err
				}
			}
		}
		return b.Put([]byte("version"), []byte(strconv.Itoa(len(migrations))))
	})
	if err != nil {
		log.Fatal(err)
	}

	go m.cleanChars()

	//http.HandleFunc("/favicon.ico", faviconHandler)
	http.Handle("/favicon.ico", http.FileServer(http.Dir("public")))
	// Mapping client endpoints
	http.HandleFunc("/client/", m.client)

	http.HandleFunc("/login", m.login)
	http.HandleFunc("/logout", m.logout)
	http.HandleFunc("/", m.index)
	http.HandleFunc("/generateToken", m.generateToken)
	http.HandleFunc("/password", m.changePassword)
	http.HandleFunc("/settings", m.settings)

	// Admin endpoints
	http.HandleFunc("/admin/", m.admin)
	http.HandleFunc("/admin/user", m.adminUser)
	http.HandleFunc("/admin/public", m.adminPublic)
	http.HandleFunc("/admin/deleteUser", m.deleteUser)
	http.HandleFunc("/admin/wipe", m.wipe)
	http.HandleFunc("/admin/setPrefix", m.setPrefix)
	http.HandleFunc("/admin/setDefaultHide", m.setDefaultHide)
	http.HandleFunc("/admin/setTitle", m.setTitle)
	http.HandleFunc("/admin/rebuildZooms", m.rebuildZooms)
	http.HandleFunc("/admin/uploadPingSound", m.uploadPingSound)
	http.HandleFunc("/admin/setPingSound", m.setPingSound)
	http.HandleFunc("/admin/export", m.export)
	http.HandleFunc("/admin/merge", m.merge)
	http.HandleFunc("/admin/map", m.adminMap)
	http.HandleFunc("/admin/mapic", m.adminICMap)

	// Map frontend endpoints
	http.HandleFunc("/map/api/v1/characters", m.getChars)
	http.HandleFunc("/map/api/v1/markers", m.getMarkers)
	http.HandleFunc("/map/api/v1/roads", m.getRoads)
	http.HandleFunc("/map/api/admin/addRoad", m.addRoad)
	http.HandleFunc("/map/api/admin/deleteRoad", m.deleteRoad)
	http.HandleFunc("/map/api/v1/ping", m.sendPing)
	http.HandleFunc("/map/api/v1/pingSound", m.getPingSound)
	http.HandleFunc("/map/api/v1/customMarkers", m.getCustomMarkers)
	http.HandleFunc("/map/api/admin/addCustomMarker", m.addCustomMarker)
	http.HandleFunc("/map/api/admin/deleteCustomMarker", m.deleteCustomMarker)
	http.HandleFunc("/map/api/config", m.config)
	http.HandleFunc("/map/api/admin/wipeTile", m.wipeTile)
	http.HandleFunc("/map/api/admin/setCoords", m.setCoords)
	http.HandleFunc("/map/api/admin/hideMarker", m.hideMarker)
	http.HandleFunc("/map/updates", m.watchGridUpdates)
	http.HandleFunc("/map/grids/", m.gridTile)
	http.HandleFunc("/map/api/maps", m.getMaps)
	//http.Handle("/map/grids/", http.StripPrefix("/map/grids", http.FileServer(http.Dir(m.gridStorage))))

	http.Handle("/map/", http.StripPrefix("/map", http.FileServer(http.Dir("frontend"))))

	http.Handle("/js/", http.FileServer(http.Dir("public")))

	log.Printf("Listening on port %d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

type Character struct {
	Name     string   `json:"name"`
	ID       int      `json:"id"`
	Map      int      `json:"map"`
	Position Position `json:"position"`
	Type     string   `json:"type"`
	group    Group
	updated  time.Time
}

type Marker struct {
	Name     string   `json:"name"`
	ID       int      `json:"id"`
	GridID   string   `json:"gridID"`
	Position Position `json:"position"`
	Image    string   `json:"image"`
	Hidden   bool     `json:"hidden"`
}

type FrontendMarker struct {
	Name     string   `json:"name"`
	ID       int      `json:"id"`
	Map      int      `json:"map"`
	Position Position `json:"position"`
	Image    string   `json:"image"`
	Hidden   bool     `json:"hidden"`
}

type MapInfo struct {
	ID       int
	Name     string
	Hidden   bool
	Priority bool
	// RequiredAuth, when non-empty, is an Auths entry (e.g. "g1", "writer")
	// a user must have for this map to appear in their map list. Empty
	// means visible to anyone with the base "map" auth, same as before
	// this field existed.
	RequiredAuth string
}

type GridData struct {
	ID         string
	Coord      Coord
	NextUpdate time.Time
	Map        int
}

type Coord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func (c Coord) Name() string {
	return fmt.Sprintf("%d_%d", c.X, c.Y)
}

func (c Coord) Parent() Coord {
	if c.X < 0 {
		c.X--
	}
	if c.Y < 0 {
		c.Y--
	}
	return Coord{
		X: c.X / 2,
		Y: c.Y / 2,
	}
}

type Auths []string

type Group []int

func (a Auths) Has(auth string) bool {
	for _, v := range a {
		if v == auth {
			return true
		}
	}
	return false
}

const (
	AUTH_ADMIN   = "admin"
	AUTH_WRITER  = "writer"
	AUTH_POINTER = "point"
	AUTH_MAP     = "map"
	AUTH_MARKERS = "markers"
	AUTH_UPLOAD  = "upload"
	AUTH_GROUP1  = "g1"
	AUTH_GROUP2  = "g2"
	AUTH_GROUP3  = "g3"
	AUTH_GROUP4  = "g4"
	AUTH_GROUP5  = "g5"
)

type User struct {
	Pass   []byte
	Auths  Auths
	Tokens []string
	// ShowPlayerNames/HideCharacterNames are personal display preferences,
	// set on the /settings page (see settings.go) -- pointers so an unset
	// preference (nil) can be told apart from an explicit false, letting
	// the frontend fall back to its own default for every account that
	// hasn't visited that page yet, rather than silently flipping to
	// Go's zero value.
	ShowPlayerNames    *bool
	HideCharacterNames *bool
	// ThingwallScale is a personal multiplier (1 = unmodified live default)
	// on top of Thingwall icons' zoom-based scaling -- see buildIcon in
	// Marker.js. Same nil-means-unset convention as the two above.
	ThingwallScale *float64
}

func (m *Map) getSession(req *http.Request) *Session {
	c, err := req.Cookie("session")
	if err != nil {
		return nil
	}
	var s *Session
	m.db.View(func(tx *bbolt.Tx) error {
		sessions := tx.Bucket([]byte("sessions"))
		if sessions == nil {
			return nil
		}
		session := sessions.Get([]byte(c.Value))
		if session == nil {
			return nil
		}
		err := json.Unmarshal(session, &s)
		if err != nil {
			return err
		}
		if s.TempAdmin {
			s.Auths = Auths{AUTH_ADMIN}
			return nil
		}
		users := tx.Bucket([]byte("users"))
		if users == nil {
			return nil
		}
		raw := users.Get([]byte(s.Username))
		if raw == nil {
			s = nil
			return nil
		}
		u := User{}
		err = json.Unmarshal(raw, &u)
		if err != nil {
			s = nil
			return err
		}
		s.Auths = u.Auths
		return nil
	})
	return s
}

// PublicConfig is the admin-editable "Public" settings section (see
// adminPublic in admin.go) -- a single, always-present config record, not
// a row in the users bucket. Enabled is a standalone kill switch: even
// with Auths populated, anonymous access stays off until this is true.
type PublicConfig struct {
	Enabled bool
	Auths   Auths
}

func (m *Map) getPublicConfig() PublicConfig {
	cfg := PublicConfig{}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		if b == nil {
			return nil
		}
		if raw := b.Get([]byte("public")); raw != nil {
			json.Unmarshal(raw, &cfg)
		}
		return nil
	})
	return cfg
}

// getSessionOrPublic behaves like getSession, but falls back to the
// PublicConfig auths (see getPublicConfig) when there's no real session,
// instead of returning nil -- only if Public access is Enabled. The
// returned Session has an empty Username, which is how callers (see
// Config.Public in map.go) tell a real session from an anonymous one. Use
// this only in read-only, map-viewing endpoints -- anonymous access should
// never reach write/admin handlers, which must keep using getSession
// directly.
func (m *Map) getSessionOrPublic(req *http.Request) *Session {
	if s := m.getSession(req); s != nil {
		return s
	}
	cfg := m.getPublicConfig()
	if !cfg.Enabled {
		return nil
	}
	return &Session{Auths: cfg.Auths}
}

func (m *Map) deleteSession(s *Session) {
	m.db.Update(func(tx *bbolt.Tx) error {
		sessions, err := tx.CreateBucketIfNotExists([]byte("sessions"))
		if err != nil {
			return err
		}
		return sessions.Delete([]byte(s.ID))
	})
}

func (m *Map) saveSession(s *Session) {
	m.db.Update(func(tx *bbolt.Tx) error {
		sessions, err := tx.CreateBucketIfNotExists([]byte("sessions"))
		if err != nil {
			return err
		}
		buf, err := json.Marshal(s)
		if err != nil {
			return err
		}
		return sessions.Put([]byte(s.ID), buf)
	})
}

type Page struct {
	Title string `json:"title"`
}

func (m *Map) getPage(req *http.Request) Page {
	p := Page{}
	m.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte("config"))
		if c == nil {
			return nil
		}
		p.Title = string(c.Get([]byte("title")))
		return nil
	})
	return p
}

func (m *Map) getUser(user, pass string) (u *User) {
	m.db.View(func(tx *bbolt.Tx) error {
		users := tx.Bucket([]byte("users"))
		if users == nil {
			if user == "admin" && pass == "admin" {
				u = &User{
					Auths: Auths{"admin", "tempadmin"},
				}
			}
			return nil
		}
		raw := users.Get([]byte(user))
		if raw != nil {
			json.Unmarshal(raw, &u)
			if bcrypt.CompareHashAndPassword(u.Pass, []byte(pass)) != nil {
				u = nil
				return nil
			}
		}
		return nil
	})
	return u
}

func (m *Map) cleanChars() {
	for range time.Tick(time.Second * 10) {
		m.chmu.Lock()
		for n, c := range m.characters {
			if c.updated.Before(time.Now().Add(-10 * time.Second)) {
				delete(m.characters, n)
			}
		}
		m.chmu.Unlock()
	}
}
