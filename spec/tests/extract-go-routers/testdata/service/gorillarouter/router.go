package gorillarouter

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Router serves the holds on gorilla/mux.
func Router() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/holds", listHolds).Methods("GET")
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/holds/{holdId:[0-9]+}", showHold).Methods(http.MethodGet)
	api.HandleFunc("/holds/{holdId}", cancelHold).Methods("DELETE", "POST")
	api.Path("/holds").Methods("POST").HandlerFunc(placeHold)
	r.HandleFunc("/holds/export", listHolds)
	r.HandleFunc("/holds/search", listHolds).Methods("GET").Queries("q", "{q}")
	return r
}

func listHolds(w http.ResponseWriter, r *http.Request)  {}
func showHold(w http.ResponseWriter, r *http.Request)   {}
func cancelHold(w http.ResponseWriter, r *http.Request) { _ = mux.Vars(r)["holdId"] }
func placeHold(w http.ResponseWriter, r *http.Request)  {}
