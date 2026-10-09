package desk

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// Server answers the desk's routes.
type Server struct {
	router   chi.Router
	validate *validator.Validate
}

// New makes the server and its routes.
func New() *Server {
	s := &Server{router: chi.NewRouter(), validate: validator.New()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.Post("/loans", s.Require("loans.write", s.lendBook))
	s.router.Get("/loans/{loanId}", s.Require("loans.read", s.showLoan))
	s.router.With(s.Allow("members.read")).Get("/members/{cardNumber}", s.showMember)
	s.router.Route("/fines", func(r chi.Router) {
		r.Use(s.Allow("fines.read"))
		r.Get("/{fineId}", s.showFine)
		r.Post("/{fineId}/waivers", s.Require("fines.waive", s.waiveFine))
	})
	scope := os.Getenv("DESK_SCOPE")
	s.router.Get("/reports", s.Require(scope, s.reports))
	s.router.Get("/status", s.status)
	s.router.Put("/members/{cardNumber}", s.Require("members.write", s.updateMember))
}

// lendRequest is what a desk sends to lend a book.
type lendRequest struct {
	CardNumber string            `json:"card_number" validate:"required,len=10,numeric"`
	Barcode    string            `json:"barcode" validate:"required,max=20,alphanum"`
	Copies     int               `json:"copies" validate:"min=1,max=3"`
	DueOn      *time.Time        `json:"due_on"`
	Notes      []note            `json:"notes" validate:"max=5,dive"`
	Channel    string            `json:"channel" validate:"oneof=desk kiosk web"`
	Extra      map[string]string `json:"extra"`
	internal   string
	Skipped    string `json:"-"`
	Email      string `json:"contact_email" validate:"omitempty,email"`
}

type note struct {
	Text  string `json:"text" validate:"required,excludesall=<>"`
	Staff int64  `json:"staff_id"`
}

func (s *Server) lendBook(w http.ResponseWriter, r *http.Request) {
	var req lendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	resp, err := http.Get("https://catalogue.example.com/v1/books")
	if err == nil {
		resp.Body.Close()
	}
}

func (s *Server) showLoan(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "loanId")
	_ = chi.URLParam(r, "loan")
}

func (s *Server) showMember(w http.ResponseWriter, r *http.Request) {
	name := "cardNumber"
	_ = chi.URLParam(r, name)
}

func (s *Server) showFine(w http.ResponseWriter, r *http.Request) {}

func (s *Server) waiveFine(w http.ResponseWriter, r *http.Request) {
	if !s.Check(r, "fines.override") {
		http.Error(w, "forbidden", http.StatusForbidden)
	}
}

func (s *Server) reports(w http.ResponseWriter, r *http.Request) {}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	req, _ := http.NewRequest(http.MethodGet, os.Getenv("DESK_UPSTREAM")+"/health", nil)
	_ = req
}

func (s *Server) updateMember(w http.ResponseWriter, r *http.Request) {
	var m Member
	_ = json.NewDecoder(r.Body).Decode(&m)
}

// Member is a member as the desk updates it.
type Member struct {
	FullName string
	Address
}

// Address is where a member lives.
type Address struct {
	Street string `json:"street"`
}

// Handler is the server's router.
func (s *Server) Handler() http.Handler { return s.router }
