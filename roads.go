package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"go.etcd.io/bbolt"
)

type Road struct {
	ID     int      `json:"id"`
	Map    int      `json:"map"`
	Name   string   `json:"name"`
	PointA Position `json:"pointA"`
	PointB Position `json:"pointB"`
}

func (m *Map) getRoads(rw http.ResponseWriter, req *http.Request) {
	s := m.getSessionOrPublic(req)
	if s == nil || !s.Auths.Has(AUTH_MAP) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !s.Auths.Has(AUTH_MARKERS) {
		json.NewEncoder(rw).Encode([]interface{}{})
		return
	}
	roads := []Road{}
	m.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("roads"))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			r := Road{}
			json.Unmarshal(v, &r)
			roads = append(roads, r)
			return nil
		})
	})
	json.NewEncoder(rw).Encode(roads)
}

func (m *Map) addRoad(rw http.ResponseWriter, req *http.Request) {
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
	ax, err := strconv.Atoi(req.FormValue("ax"))
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
		return
	}
	ay, err := strconv.Atoi(req.FormValue("ay"))
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
		return
	}
	bx, err := strconv.Atoi(req.FormValue("bx"))
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
		return
	}
	by, err := strconv.Atoi(req.FormValue("by"))
	if err != nil {
		http.Error(rw, "coord parse failed", http.StatusBadRequest)
		return
	}
	name := req.FormValue("name")
	if name == "" {
		http.Error(rw, "name required", http.StatusBadRequest)
		return
	}

	road := Road{
		Map:    mapid,
		Name:   name,
		PointA: Position{X: ax, Y: ay},
		PointB: Position{X: bx, Y: by},
	}

	err = m.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("roads"))
		if err != nil {
			return err
		}
		id, err := b.NextSequence()
		if err != nil {
			return err
		}
		road.ID = int(id)
		raw, err := json.Marshal(road)
		if err != nil {
			return err
		}
		return b.Put([]byte(fmt.Sprintf("%d", road.ID)), raw)
	})
	if err != nil {
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(rw).Encode(road)
}

func (m *Map) deleteRoad(rw http.ResponseWriter, req *http.Request) {
	s := m.getSession(req)
	if s == nil || !(s.Auths.Has(AUTH_ADMIN) || s.Auths.Has(AUTH_WRITER)) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	m.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("roads"))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(req.FormValue("id")))
	})
	rw.WriteHeader(200)
}
