package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go.etcd.io/bbolt"
)

// settings is a self-service page for any logged-in account (not
// admin-only, unlike everything under /admin/*) to manage its own personal
// display preferences -- Players Names, Hide Character Names, and the
// Thingwall icon scale slider -- moved/added here since none of these
// affect what data an account can access, only how it's shown to that one
// account.
func (m *Map) settings(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil {
		http.Redirect(rw, req, "/login", 302)
		return
	}

	if req.Method == "POST" {
		req.ParseForm()
		showPlayerNames := req.FormValue("showPlayerNames") != ""
		hideCharacterNames := req.FormValue("hideCharacterNames") != ""
		thingwallScale, err := strconv.ParseFloat(req.FormValue("thingwallScale"), 64)
		if err != nil || thingwallScale < 0.25 || thingwallScale > 4 {
			thingwallScale = 1
		}
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
			u.ShowPlayerNames = &showPlayerNames
			u.HideCharacterNames = &hideCharacterNames
			u.ThingwallScale = &thingwallScale
			buf, err := json.Marshal(u)
			if err != nil {
				return err
			}
			return users.Put([]byte(s.Username), buf)
		})
		http.Redirect(rw, req, "/settings", 302)
		return
	}

	u := User{}
	m.db.View(func(tx *bbolt.Tx) error {
		users := tx.Bucket([]byte("users"))
		if users == nil {
			return nil
		}
		raw := users.Get([]byte(s.Username))
		if raw == nil {
			return nil
		}
		return json.Unmarshal(raw, &u)
	})

	// Same fallback the frontend uses: an account that's never visited
	// this page has no stored preference yet, so default both to on,
	// matching the behavior before these were persisted per-account.
	showPlayerNames := true
	if u.ShowPlayerNames != nil {
		showPlayerNames = *u.ShowPlayerNames
	}
	hideCharacterNames := true
	if u.HideCharacterNames != nil {
		hideCharacterNames = *u.HideCharacterNames
	}
	thingwallScale := 1.0
	if u.ThingwallScale != nil {
		thingwallScale = *u.ThingwallScale
	}

	m.ExecuteTemplate(rw, "settings.tmpl", struct {
		Page               Page
		Session            *Session
		ShowPlayerNames    bool
		HideCharacterNames bool
		ThingwallScale     float64
	}{
		Page:               m.getPage(req),
		Session:            s,
		ShowPlayerNames:    showPlayerNames,
		HideCharacterNames: hideCharacterNames,
		ThingwallScale:     thingwallScale,
	})
}
