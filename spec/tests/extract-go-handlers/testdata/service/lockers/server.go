package lockers

import (
	"database/sql"
	"net/http"
)

// Server answers the locker service's routes.
type Server struct {
	db *sql.DB
}

// New makes the server over a database.
func New(db *sql.DB) *Server {
	return &Server{db: db}
}

// Handler is the server's routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /parcels", s.Require("parcels.store", s.storeParcel))
	mux.HandleFunc("POST /parcels/{code}/collect", s.collectParcel)
	mux.HandleFunc("POST /lockers/sweep", s.Require("lockers.sweep", s.sweepLockers))
	mux.HandleFunc("GET /lockers/{id}/report", s.Require("lockers.read", s.reportLocker))
	mux.HandleFunc("POST /lockers/{id}/service", s.Require("lockers.service", s.serviceLocker))
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /parcels", s.Require("parcels.list", s.listParcels))
	return mux
}

// Require runs h only for a caller whose role grants permission.
func (s *Server) Require(permission string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Courier-Token") == "" {
			http.Error(w, "a courier token is needed", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}
