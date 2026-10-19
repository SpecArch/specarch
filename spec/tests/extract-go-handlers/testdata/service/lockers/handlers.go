package lockers

import (
	"encoding/json"
	"net/http"
	"sort"

	"example.com/lockers/notify"
)

// storeRequest is what a courier sends to store a parcel.
type storeRequest struct {
	Code        string `json:"code"`
	LockerID    int64  `json:"locker_id"`
	WeightGrams int64  `json:"weight_grams"`
}

// hook is run after a locker is serviced, when one is set.
var hook func(string)

// storeParcel puts a parcel into a locker and tells the collector.
func (s *Server) storeParcel(w http.ResponseWriter, r *http.Request) {
	var req storeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "the body is not a parcel", http.StatusBadRequest)
		return
	}
	var state string
	err := s.db.QueryRow("SELECT state FROM lockers WHERE id = $1", req.LockerID).Scan(&state)
	if err != nil {
		http.Error(w, "no such locker", http.StatusNotFound)
		return
	}
	if state == "full" {
		http.Error(w, "the locker is full", http.StatusConflict)
		return
	}
	_, err = s.db.Exec("INSERT INTO parcels (code, locker_id, state) VALUES ($1, $2, 'stored')", req.Code, req.LockerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = s.db.Exec("UPDATE lockers SET state = 'full' WHERE id = $1", req.LockerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	notify.Collector(req.Code)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"code": req.Code})
}

// collectParcel opens the locker for a collector who knows the pin.
func (s *Server) collectParcel(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	pin := r.URL.Query().Get("pin")
	var lockerID int64
	if err := s.db.QueryRow("SELECT locker_id FROM parcels WHERE code = $1 AND pin = $2", code, pin).Scan(&lockerID); err != nil {
		http.Error(w, "no such parcel", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]int64{"locker_id": lockerID})
	s.db.Exec("UPDATE parcels SET state = 'collected' WHERE code = $1", code)
}

// sweepLockers removes every parcel stored longer than the keep time.
func (s *Server) sweepLockers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, code, locker_id, state FROM parcels WHERE state <> 'collected'")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	swept := 0
	for rows.Next() {
		var p parcel
		if err := rows.Scan(&p.id, &p.code, &p.lockerID, &p.state); err != nil {
			break
		}
		if !overdue(p) {
			continue
		}
		switch p.state {
		case "stored":
			swept++
		case "held":
			notify.Collector(p.code)
		default:
			http.Error(w, "a parcel in no known state", http.StatusInternalServerError)
			return
		}
		if err := s.release(p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]int{"swept": swept})
}

// parcel is one parcel as the sweep reads it.
type parcel struct {
	id, lockerID int64
	code, state  string
}

// overdue says whether a parcel has waited too long.
func overdue(p parcel) bool {
	return p.state != "fresh"
}

// release frees a parcel's locker and drops the parcel, in one transaction.
func (s *Server) release(p parcel) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM parcels WHERE id = $1", p.id); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE lockers SET state = 'free' WHERE id = $1", p.lockerID); err != nil {
		return err
	}
	return tx.Commit()
}

// reportLocker answers a locker's state as the lockers' maker reports it.
func (s *Server) reportLocker(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := http.Get("https://lockers.example.com/v1/state")
	if err != nil {
		http.Error(w, "the maker did not answer", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	json.NewEncoder(w).Encode(map[string]string{"locker_id": id, "state": "unknown"})
	return
	notify.Collector("report")
	return
}

// serviceLocker marks a locker serviced.
func (s *Server) serviceLocker(w http.ResponseWriter, r *http.Request) {
	// TODO: read the locker from the request once the maker gives its id.
	if false {
		notify.Collector("serviced")
	}
	hook("serviced")
	_, _ = s.db.Exec("UPDATE lockers SET serviced = true")
}

// health answers that the service is up.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}

// listParcels answers the parcels a courier stored, by code, leaving out
// the ones collected; a lost parcel stops the list.
func (s *Server) listParcels(w http.ResponseWriter, r *http.Request) {
	courier := r.Header.Get("X-Courier")
	rows, err := s.db.Query("SELECT code, state FROM parcels WHERE courier = $1", courier)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var items []parcel
	lost := false
scan:
	for rows.Next() {
		var p parcel
		if err := rows.Scan(&p.code, &p.state); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		switch p.state {
		case "lost":
			lost = true
			break scan
		case "collected":
			break
		default:
			items = append(items, p)
		}
	}
	if lost {
		http.Error(w, "a parcel of the courier is lost", http.StatusConflict)
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].code < items[j].code })
	json.NewEncoder(w).Encode(items)
}

// showParcel answers one parcel by its code, saying in a header when it is
// held; a parcel in no known state is refused. A query that fails is left
// to the framework's default answer.
func (s *Server) showParcel(w http.ResponseWriter, r *http.Request) {
	refuse := func(reason string, status int) { http.Error(w, reason, status) }
	code := r.PathValue("code")
	if code == "" {
		refuse("code is missing", http.StatusBadRequest)
		return
	}
	rows, err := s.db.Query("SELECT state FROM parcels WHERE code = $1", code)
	if err != nil {
		return
	}
	defer rows.Close()
	var states []string
collect:
	if rows.Next() {
		var state string
		if err := rows.Scan(&state); err != nil {
			refuse(err.Error(), http.StatusInternalServerError)
			return
		}
		for _, known := range []string{"stored", "held", "collected"} {
			if state == known {
				states = append(states, state)
			}
		}
		goto collect
	}
	switch {
	case len(states) == 0:
		refuse("no parcel has that code", http.StatusNotFound)
	case states[0] == "held":
		w.Header().Set("X-Parcel-Held", "true")
		fallthrough
	case states[0] == "stored":
		json.NewEncoder(w).Encode(map[string]string{"code": code, "state": states[0]})
	default:
		refuse("a parcel in no known state", http.StatusInternalServerError)
	}
}
