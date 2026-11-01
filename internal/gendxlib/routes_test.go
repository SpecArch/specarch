package gendxlib

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRouteHelpersServe compiles the handlers go-dxlib writes for the
// fixed routes (ADR-216), with a Routes type as the generator writes it,
// and drives them: the document with a fresh nonce in its policy and never
// stored, 405 with Allow on another method, 403 when the permission is
// refused and 500 when nothing can check it, the files with their caching
// and no listing, and the health check 200 or 503 with the reason.
func TestRouteHelpersServe(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module routes\n\ngo 1.24\n")
	write("routes.go", `package routes

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
)

type Routes struct {
	App     func(nonce string) ([]byte, error)
	Assets  http.FileSystem
	Health  func(ctx context.Context) error
	Allowed func(r *http.Request, permission string) error
}

`+routeHelpers)
	write("routes_test.go", `package routes

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func serve(h http.Handler, method, path string) *http.Response {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec.Result()
}

func body(r *http.Response) string {
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

func TestApplication(t *testing.T) {
	r := Routes{App: func(nonce string) ([]byte, error) { return []byte("<script nonce=\"" + nonce + "\"></script>"), nil }}
	h := routeApplication(r, "app", "public", "no-store", []string{"default-src 'self'", "script-src 'self' 'nonce-{nonce}'"}, r.App)
	res := serve(h, http.MethodGet, "/")
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	policy := res.Header.Get("Content-Security-Policy")
	i := strings.Index(policy, "'nonce-")
	if i < 0 || strings.Contains(policy, "{nonce}") {
		t.Fatalf("policy %q", policy)
	}
	nonce := strings.TrimSuffix(strings.TrimPrefix(policy[i:], "'nonce-"), "'")
	if !strings.Contains(body(res), "nonce=\""+nonce+"\"") {
		t.Fatalf("the document does not carry the nonce %q", nonce)
	}
	if res.Header.Get("Cache-Control") != "no-store" || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("headers %v", res.Header)
	}
	again := serve(h, http.MethodGet, "/")
	if again.Header.Get("Content-Security-Policy") == policy {
		t.Fatal("two responses carry one nonce")
	}
	post := serve(h, http.MethodPost, "/")
	if post.StatusCode != 405 || post.Header.Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST: %d %v", post.StatusCode, post.Header)
	}
	head := serve(h, http.MethodHead, "/")
	if head.StatusCode != 200 || body(head) != "" {
		t.Fatalf("HEAD: %d %q", head.StatusCode, body(head))
	}
}

func TestFiles(t *testing.T) {
	files := http.FS(fstest.MapFS{"a.js": {Data: []byte("x")}, "sub/b.css": {Data: []byte("y")}})
	public := Routes{Assets: files}
	h := routeFiles(public, "assets", "public", "public, max-age=31536000, immutable", "/assets/", public.Assets)
	res := serve(h, http.MethodGet, "/assets/a.js")
	if res.StatusCode != 200 || res.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" || body(res) != "x" {
		t.Fatalf("file: %d %v %q", res.StatusCode, res.Header, body(res))
	}
	if res := serve(h, http.MethodGet, "/assets/sub/"); res.StatusCode != 404 {
		t.Fatalf("folder: %d %q", res.StatusCode, body(res))
	}
	if res := serve(h, http.MethodGet, "/assets/missing.js"); res.StatusCode != 404 {
		t.Fatalf("missing: %d", res.StatusCode)
	}
	refused := Routes{Assets: files, Allowed: func(*http.Request, string) error { return errors.New("no role grants it") }}
	h = routeFiles(refused, "reports", "reports.read", "private, max-age=600", "/reports/", refused.Assets)
	if res := serve(h, http.MethodGet, "/reports/a.js"); res.StatusCode != 403 || !strings.Contains(body(res), "no role grants it") {
		t.Fatalf("refused: %d %q", res.StatusCode, body(res))
	}
	unchecked := Routes{Assets: files}
	h = routeFiles(unchecked, "reports", "reports.read", "private, max-age=600", "/reports/", unchecked.Assets)
	if res := serve(h, http.MethodGet, "/reports/a.js"); res.StatusCode != 500 || !strings.Contains(body(res), "Routes.Allowed is nil") {
		t.Fatalf("unchecked: %d %q", res.StatusCode, body(res))
	}
	empty := Routes{}
	h = routeFiles(empty, "assets", "public", "no-store", "/assets/", empty.Assets)
	if res := serve(h, http.MethodGet, "/assets/a.js"); res.StatusCode != 500 || !strings.Contains(body(res), "has no folder") {
		t.Fatalf("no folder: %d %q", res.StatusCode, body(res))
	}
}

func TestHealth(t *testing.T) {
	ok := Routes{Health: func(context.Context) error { return nil }}
	h := routeHealth(ok, "health", "public", "no-store", ok.Health)
	if res := serve(h, http.MethodGet, "/health"); res.StatusCode != 200 || body(res) != "healthy\n" || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("healthy: %d %q", res.StatusCode, body(res))
	}
	down := Routes{Health: func(context.Context) error { return errors.New("the database does not answer") }}
	h = routeHealth(down, "health", "public", "no-store", down.Health)
	if res := serve(h, http.MethodGet, "/health"); res.StatusCode != 503 || !strings.Contains(body(res), "the database does not answer") {
		t.Fatalf("unhealthy: %d %q", res.StatusCode, body(res))
	}
}
`)
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the route handlers do not pass their tests: %v\n%s", err, out)
	}
}
