package store

// Store holds the loans.
type Store struct{ Loans []string }

// Open opens the store.
func Open() *Store { return &Store{} }
