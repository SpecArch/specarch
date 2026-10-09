package chirouter

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

// Router serves the members' routes on chi.
func Router() http.Handler {
	r := chi.NewRouter()
	r.Route("/members", func(r chi.Router) {
		r.Get("/", listMembers)
		r.Post("/", addMember)
		r.Route("/{cardNumber}", func(r chi.Router) {
			r.Get("/", showMember)
			r.Method(http.MethodDelete, "/", removeMember)
		})
	})
	r.Group(func(r chi.Router) {
		r.With(audit).Put("/members/{cardNumber:[0-9]+}/address", showMember)
	})
	r.Mount("/branches", branches())
	r.Handle("/metrics", http.HandlerFunc(showMember))
	if os.Getenv("DESK_DEBUG") != "" {
		r.Get("/debug", showMember)
	}
	r.Get("/assets/*", showMember)
	prefix := os.Getenv("DESK_PREFIX")
	r.Route(prefix, func(r chi.Router) {
		r.Get("/ping", showMember)
	})
	return r
}

func branches() chi.Router {
	r := chi.NewRouter()
	r.Get("/{branchId}", showBranch)
	return r
}

func audit(next http.Handler) http.Handler { return next }

func listMembers(w http.ResponseWriter, r *http.Request)  {}
func addMember(w http.ResponseWriter, r *http.Request)    {}
func showMember(w http.ResponseWriter, r *http.Request)   { _ = chi.URLParam(r, "cardNumber") }
func removeMember(w http.ResponseWriter, r *http.Request) {}
func showBranch(w http.ResponseWriter, r *http.Request)   { _ = chi.URLParam(r, "id") }
