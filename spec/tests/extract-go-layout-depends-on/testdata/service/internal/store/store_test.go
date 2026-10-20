package store_test

import (
	"testing"

	"example.com/shelf/internal/loans"
	"example.com/shelf/internal/store"
)

func TestDue(t *testing.T) {
	if loans.Due(store.Open()) != 0 {
		t.Fatal("an empty store has loans due")
	}
}
