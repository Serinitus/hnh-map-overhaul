package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

func clientIP(req *http.Request) string {
	if fwd := req.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return req.RemoteAddr
}

// requestOrigin resolves the scheme+host a client actually used to
// reach this server, so a token URL built from it is guaranteed valid
// for copy/pasting straight back in -- whatever page it's rendered on
// was necessarily reached the same way. Checks X-Forwarded-Proto/Host
// first (set by a reverse proxy such as nginx terminating TLS in front
// of this server, same reasoning as clientIP's X-Forwarded-For check
// above), falling back to req.TLS and req.Host for a direct connection.
func requestOrigin(req *http.Request) string {
	host := req.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = req.Host
	}
	scheme := req.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if req.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + host
}

func (m *Map) index(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil {
		pub := m.getPublicConfig()
		if !pub.Enabled {
			http.Redirect(rw, req, "/login", 302)
			return
		}
		if !req.URL.Query().Has("tokens") {
			// Bare "/" (typed directly, a bookmark, etc) still goes
			// straight to the map for a public visitor -- only the
			// explicit "Tokens" nav link (which points at /?tokens)
			// shows the locked token view below.
			http.Redirect(rw, req, "/map/", 302)
			return
		}
		// Public access, no real session -- show the locked single
		// shared token (if the admin set one) instead of a personal
		// token list. Reached only via the navbar/app-bar "Tokens"
		// link, not the bare root path (see the redirect just above).
		tokens := []string{}
		if pub.Token != "" {
			tokens = []string{pub.Token}
		}
		m.ExecuteTemplate(rw, "index.tmpl", struct {
			Page         Page
			Session      *Session
			UploadTokens []string
			Prefix       string
			Locked       bool
		}{
			Page:         m.getPage(req),
			Session:      &Session{Auths: pub.Auths},
			UploadTokens: tokens,
			Prefix:       requestOrigin(req),
			Locked:       true,
		})
		return
	}

	tokens := []string{}
	// Auto-resolved from the request that's loading this very page --
	// whatever origin got used to reach here is necessarily a valid one
	// to paste back into a client tool. The stored "prefix" config
	// (set via /admin/setPrefix) still overrides this when explicitly
	// set, for the rare case a server sits behind something that makes
	// the request's own origin unusable (e.g. an internal hostname that
	// doesn't match the public one clients need).
	prefix := requestOrigin(req)
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
		if b == nil {
			return nil
		}
		uRaw := b.Get([]byte(s.Username))
		if uRaw == nil {
			return nil
		}
		u := User{}
		json.Unmarshal(uRaw, &u)
		tokens = u.Tokens

		config := tx.Bucket([]byte("config"))
		if config != nil {
			if stored := string(config.Get([]byte("prefix"))); stored != "" {
				prefix = stored
			}
		}
		return nil
	})

	m.ExecuteTemplate(rw, "index.tmpl", struct {
		Page         Page
		Session      *Session
		UploadTokens []string
		Prefix       string
		Locked       bool
	}{
		Page:         m.getPage(req),
		Session:      s,
		UploadTokens: tokens,
		Prefix:       prefix,
		Locked:       false,
	})
}

func (m *Map) login(rw http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		u := m.getUser(req.FormValue("user"), req.FormValue("pass"))
		if u != nil {
			log.Println("login: ", req.FormValue("user"), " from ", clientIP(req))
			session := make([]byte, 32)
			rand.Read(session)
			http.SetCookie(rw, &http.Cookie{
				Name:    "session",
				Expires: time.Now().Add(time.Hour * 24 * 7),
				Value:   hex.EncodeToString(session),
			})
			s := &Session{
				ID:        hex.EncodeToString(session),
				Username:  req.FormValue("user"),
				TempAdmin: u.Auths.Has("tempadmin"),
			}
			m.saveSession(s)
			http.Redirect(rw, req, "/map/", 302)
			return
		}
	}
	m.ExecuteTemplate(rw, "login.tmpl", struct {
		Page Page
	}{
		Page: m.getPage(req),
	})
}

func (m *Map) logout(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s != nil {
		m.deleteSession(s)
	}
	if m.getPublicConfig().Enabled {
		http.Redirect(rw, req, "/map/", 302)
		return
	}
	http.Redirect(rw, req, "/login", 302)
	return
}

func (m *Map) generateToken(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !s.Auths.Has(AUTH_UPLOAD) {
		http.Redirect(rw, req, "/", 302)
		return
	}
	tokenRaw := make([]byte, 16)
	_, err := rand.Read(tokenRaw)
	if err != nil {
		rw.WriteHeader(500)
		return
	}
	token := hex.EncodeToString(tokenRaw)
	m.db.Update(func(tx *bbolt.Tx) error {
		ub, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return err
		}
		uRaw := ub.Get([]byte(s.Username))
		if uRaw == nil {
			return nil
		}
		u := User{}
		err = json.Unmarshal(uRaw, &u)
		if err != nil {
			return err
		}
		u.Tokens = append(u.Tokens, token)
		buf, err := json.Marshal(u)
		if err != nil {
			return err
		}
		err = ub.Put([]byte(s.Username), buf)
		if err != nil {
			return err
		}
		b, err := tx.CreateBucketIfNotExists([]byte("tokens"))
		if err != nil {
			return err
		}
		return b.Put([]byte(token), []byte(s.Username))
	})
	http.Redirect(rw, req, "/", 302)
}

func (m *Map) changePassword(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil {
		http.Redirect(rw, req, "/", 302)
		return
	}

	if req.Method == "POST" {
		req.ParseForm()
		password := req.FormValue("pass")
		m.db.Update(func(tx *bbolt.Tx) error {
			users, err := tx.CreateBucketIfNotExists([]byte("users"))
			if err != nil {
				return err
			}
			u := User{}
			raw := users.Get([]byte(s.Username))
			if raw != nil {
				json.Unmarshal(raw, &u)
			}
			if password != "" {
				u.Pass, _ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			}
			raw, _ = json.Marshal(u)
			users.Put([]byte(s.Username), raw)
			return nil
		})
		http.Redirect(rw, req, "/", 302)
	}

	m.ExecuteTemplate(rw, "password.tmpl", struct {
		Page    Page
		Session *Session
	}{
		Page:    m.getPage(req),
		Session: s,
	})
}
