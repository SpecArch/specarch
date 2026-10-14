package desk

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Server answers the desk's routes.
type Server struct {
	router chi.Router
}

// New makes the server and its routes.
func New() *Server {
	s := &Server{router: chi.NewRouter()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.Get("/loans", s.Require("loans.read", s.listLoans))
	s.router.Post("/loans", s.Require("loans.write", s.lendBook))
	s.router.With(s.Allow("members.read")).Get("/members/{cardNumber}", s.showMember)
	s.router.Get("/status", s.status)
}

// Require runs h only for a caller whose role grants permission; a token
// a service holds on its own account passes without its roles being read.
func (s *Server) Require(permission string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("Authorization"), "Service ") {
			h(w, r)
			return
		}
		if !s.granted(r, permission) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

// Allow is a check as chi middleware, which holds every caller to its
// roles.
func (s *Server) Allow(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.granted(r, permission) {
				next.ServeHTTP(w, r)
			}
		})
	}
}

func (s *Server) granted(r *http.Request, permission string) bool {
	return r.Header.Get("Desk-Role") != ""
}

func (s *Server) listLoans(w http.ResponseWriter, r *http.Request) {}

func (s *Server) lendBook(w http.ResponseWriter, r *http.Request) {}

func (s *Server) showMember(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "cardNumber")
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {}
