package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"go.etcd.io/bbolt"
)

type CustomMarker struct {
	ID    int    `json:"id"`
	Map   int    `json:"map"`
	Name  string `json:"name"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Color string `json:"color"`
}

func (m *Map) getCustomMarkers(rw http.ResponseWriter, req *http.Request) {
	s := m.getSessionOrPublic(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !s.Auths.Has(AUTH_MARKERS) {
		json.NewEncoder(rw).Encode([]interface{}{})
		return
	}
	markers := []CustomMarker{}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("customMarkers"))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			cm := CustomMarker{}
			json.Unmarshal(v, &cm)
			if cm.Color == "" {
				cm.Color = "white"
			}
			markers = append(markers, cm)
			return nil
		})
	})
	json.NewEncoder(rw).Encode(markers)
}

func (m *Map) addCustomMarker(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !(s.Auths.Has(AUTH_ADMIN) || s.Auths.Has(AUTH_WRITER)) {
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
	name := req.FormValue("name")
	if name == "" {
		http.Error(rw, "name required", http.StatusBadRequest)
		return
	}
	color := req.FormValue("color")
	if color == "" {
		color = "white"
	}

	marker := CustomMarker{
		Map:   mapid,
		Name:  name,
		X:     x,
		Y:     y,
		Color: color,
	}

	err = m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("customMarkers"))
		if err != nil {
			return err
		}
		id, err := b.NextSequence()
		if err != nil {
			return err
		}
		marker.ID = int(id)
		raw, err := json.Marshal(marker)
		if err != nil {
			return err
		}
		return b.Put([]byte(fmt.Sprintf("%d", marker.ID)), raw)
	})
	if err != nil {
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(rw).Encode(marker)
}

func (m *Map) deleteCustomMarker(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !(s.Auths.Has(AUTH_ADMIN) || s.Auths.Has(AUTH_WRITER)) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	m.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("customMarkers"))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(req.FormValue("id")))
	})
	rw.WriteHeader(200)
}
