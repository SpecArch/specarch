package muxfallback

import "net/http"

// Handler serves the health route itself, hands the API's routes to its
// own mux, and forwards the rest.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", health)
	mux.HandleFunc("GET /files/{path...}", serve)
	api := http.NewServeMux()
	api.HandleFunc("GET /loans", loans)
	mux.Handle("/", api)
	fallback := http.NewServeMux()
	fallback.HandleFunc("GET /status", health)
	fallback.Handle("/", http.HandlerFunc(proxy))
	return mux
}

func health(w http.ResponseWriter, r *http.Request) {}
func serve(w http.ResponseWriter, r *http.Request)  {}
func loans(w http.ResponseWriter, r *http.Request)  {}
func proxy(w http.ResponseWriter, r *http.Request)  {}
