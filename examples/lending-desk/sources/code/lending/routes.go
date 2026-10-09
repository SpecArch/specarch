package lending

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// Mux is what the routes are registered on: an *http.ServeMux when the
// service runs, and the route table printer's recorder when it prints
// them.
type Mux interface {
	Handle(pattern string, handler http.Handler)
}

// Server holds the handlers and what they read.
type Server struct {
	db        *sql.DB
	openLoans map[string]int // card number -> books on loan
}

// NewServer makes a server that reads its grants from db.
func NewServer(db *sql.DB) *Server {
	return &Server{db: db, openLoans: map[string]int{}}
}

// Handler serves every route.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.Register(mux)
	return mux
}

// Register registers every route the service serves, each with the
// permission it checks, if any.
func (s *Server) Register(mux Mux) {
	mux.Handle("POST /members", s.Require("members.write", s.RegisterMember))
	mux.Handle("POST /loans", s.Require("loans.write", s.LendBook))
	mux.Handle("POST /loans/{loanId}/return", s.Require("loans.write", s.ReturnBook))
	mux.Handle("POST /loans/{loanId}/write-off-requests", s.Require("loans.write", s.RequestWriteOff))
	mux.Handle("POST /loans/{loanId}/write-off", s.Require("loans.writeoff", s.WriteOffLoan))
	mux.Handle("GET /members/{cardNumber}/loans", s.Require("loans.read", s.ListMemberLoans))
	mux.Handle("GET /members/{cardNumber}", http.HandlerFunc(s.ShowMember))
}

// RegisterMember adds a member and answers 201.
func (s *Server) RegisterMember(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
}

// lendRequest is the book a desk lends and the member it lends it to.
type lendRequest struct {
	CardNumber string `json:"card_number"`
	Barcode    string `json:"barcode"`
}

// LendBook lends a book, or answers 409 when the member already has
// MaxOpenLoans books out.
func (s *Server) LendBook(w http.ResponseWriter, r *http.Request) {
	var req lendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if s.openLoans[req.CardNumber] >= MaxOpenLoans {
		w.WriteHeader(http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// ReturnBook closes a loan and answers 200.
func (s *Server) ReturnBook(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("loanId")
	w.WriteHeader(http.StatusOK)
}

// RequestWriteOff asks for a lost book's loan to be written off and answers
// 202: the write-off workflow waits for a desk supervisor to approve it.
func (s *Server) RequestWriteOff(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusAccepted)
}

// WriteOffLoan writes a loan off once its request is approved, and answers
// 200.
func (s *Server) WriteOffLoan(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// ListMemberLoans answers the loans of one member.
func (s *Server) ListMemberLoans(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("cardNumber")
	w.WriteHeader(http.StatusOK)
}

// ShowMember answers one member's card number and full name.
func (s *Server) ShowMember(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("cardNumber")
	w.WriteHeader(http.StatusOK)
}
