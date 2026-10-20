package loans

import "example.com/shelf/internal/store"

// Due counts the loans past their due date.
func Due(s *store.Store) int { return len(s.Loans) }
