package service

import (
	"strings"
	"sync"

	"github.com/donnyhardyanto/dxlib/api"
)

// notice is one notice on the board.
type notice struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Pinned bool   `json:"pinned"`
}

// board keeps the notices in memory, newest last.
type board struct {
	mu      sync.Mutex
	next    int64
	notices []notice
}

var (
	store       = &board{}
	maintenance bool
)

func (b *board) list(filter string) []notice {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []notice
	for i := len(b.notices) - 1; i >= 0; i-- {
		if strings.Contains(b.notices[i].Title, filter) {
			out = append(out, b.notices[i])
		}
	}
	return out
}

func (b *board) get(id int64) (notice, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, n := range b.notices {
		if n.ID == id {
			return n, true
		}
	}
	return notice{}, false
}

func (b *board) titleTaken(title string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, n := range b.notices {
		if n.Title == title {
			return true
		}
	}
	return false
}

func (b *board) add(title, body string, pinned bool) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	b.notices = append(b.notices, notice{ID: b.next, Title: title, Body: body, Pinned: pinned})
	return b.next
}

func (b *board) remove(id int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, n := range b.notices {
		if n.ID == id {
			b.notices = append(b.notices[:i], b.notices[i+1:]...)
			return true
		}
	}
	return false
}

func setMaintenance(on bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	maintenance = on
}

// streamNotices is the stream's periodic hook: it sends nothing of its
// own, so the library's ping keeps the connection open.
func streamNotices(aepr *api.DXAPIEndPointRequest) (err error) {
	return nil
}
