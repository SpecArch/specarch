package gendxlib

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/SpecArch/specarch/internal/genopenapi"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// TestEnvelopeStreamRoundTrip generates the vault, a service with a pre-key
// handshake, an encrypted operation, a streamed upload and two streamed
// downloads, binds the generated service to a real dxlib API through the
// document the dxlib dialect writes, and drives the generated client against
// it: the handshake, an encrypted call whose answer is opened, a second
// envelope on the one-use handshake refused as dxlib refuses it, an upload
// larger than any buffer streamed in and hashed, the same bytes streamed
// back and hashed, a refusal on a download, and an encrypted request
// answered with a plain stream. It needs a dxlib checkout, named by
// SPECARCH_DXLIB, and is skipped without one (ADR-145, ADR-146).
func TestEnvelopeStreamRoundTrip(t *testing.T) {
	dxlib := os.Getenv("SPECARCH_DXLIB")
	if dxlib == "" {
		t.Skip("SPECARCH_DXLIB names no dxlib checkout, so the generated service cannot run here")
	}
	service := generateFrom(t, envelopeFixture, func(map[string]any) {}, nil)
	if len(service.Files) != 2 || len(service.Diagnostics) != 0 {
		t.Fatalf("want the service file and the client with no diagnostic, got %d files and %v", len(service.Files), service.Diagnostics)
	}
	document := openapiDocument(t, envelopeFixture)
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":            "module vault\n\ngo 1.27.1\n\nrequire github.com/donnyhardyanto/dxlib v0.0.0\n\nreplace github.com/donnyhardyanto/dxlib => " + dxlib + "\n",
		"openapi.yaml":      document,
		"bodies.go":         roundTripBodies,
		"roundtrip_test.go": roundTripTest,
	}
	for _, f := range service.Files {
		files[f.Path] = f.Content
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Skipf("the dxlib module cannot be resolved here (%v): %s", err, out)
	}
	run := exec.Command("go", "test", "-count=1", "./...")
	run.Dir = dir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("the generated service and client do not pass the round trip: %v\n%s", err, out)
	}
}

// openapiDocument is the dxlib-dialect document of a fixture, as specarch
// generate openapi writes it.
func openapiDocument(t *testing.T, fixture string) string {
	t.Helper()
	s := spec.Load(fixture)
	if len(s.Problems) > 0 {
		t.Fatalf("the fixture does not load: %v", s.Problems)
	}
	var impls []map[string]any
	for _, impl := range s.Implementations {
		root := source.Parse(impl.Data).Root
		var idioms []map[string]any
		for _, u := range validate.IdiomUses(impl.Path, root, s) {
			i := map[string]any{"name": u.Name, "version": u.Version, "as": u.As}
			if u.As == "shipped" {
				i["content"] = source.ValueOf(validate.ShippedIdioms()[u.Name].Root)
			}
			idioms = append(idioms, i)
		}
		impls = append(impls, map[string]any{"file": impl.Path, "content": source.ValueOf(root), "idioms": idioms})
	}
	data, err := json.Marshal(map[string]any{"specarch": "0.3", "target": "openapi", "root": filepath.ToSlash(s.RootFile),
		"specification": s.Value, "implementations": impls, "output": filepath.ToSlash(filepath.Join(fixture, "..", "out"))})
	if err != nil {
		t.Fatal(err)
	}
	r, err := genopenapi.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	resp := genopenapi.Generate(r)
	if len(resp.Files) != 1 {
		t.Fatalf("want one document, got %d and %v", len(resp.Files), resp.Diagnostics)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == "error" {
			t.Fatalf("the document has an error: %v", d)
		}
	}
	return resp.Files[0].Content
}

// roundTripBodies are the bodies the service writes beside the generated
// file: a note echoed, an attachment hashed as it streams in, the same
// bytes streamed back, and an export streamed on request.
const roundTripBodies = `package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sync"

	"github.com/donnyhardyanto/dxlib/api"
	"github.com/donnyhardyanto/dxlib/utils"
)

// pattern yields n bytes, byte i being i modulo 251, without holding them.
type pattern struct{ n, at int64 }

func (p *pattern) Read(b []byte) (int, error) {
	if p.at >= p.n {
		return 0, io.EOF
	}
	i := 0
	for ; i < len(b) && p.at < p.n; i++ {
		b[i] = byte(p.at % 251)
		p.at++
	}
	return i, nil
}

var (
	storedMu sync.Mutex
	stored   = map[string]int64{} // attachment id -> size; the bytes are the pattern
)

func bodyAddNote(aepr *api.DXAPIEndPointRequest, r AddNoteRequest) error {
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"note_id": "n-" + r.Title})
	return nil
}

func bodyUploadAttachment(aepr *api.DXAPIEndPointRequest, r UploadAttachmentRequest, body io.Reader) error {
	h := sha256.New()
	n, err := io.Copy(h, body)
	if err != nil {
		return aepr.WriteResponseAndNewErrorf(http.StatusRequestEntityTooLarge, "", "UPLOAD_FAILED:%s", err.Error())
	}
	id := hex.EncodeToString(h.Sum(nil))[:16]
	storedMu.Lock()
	stored[id] = n
	storedMu.Unlock()
	aepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{"attachment_id": id, "size": n})
	return nil
}

func bodyDownloadAttachment(aepr *api.DXAPIEndPointRequest, r DownloadAttachmentRequest) error {
	storedMu.Lock()
	size, ok := stored[r.AttachmentId]
	storedMu.Unlock()
	if !ok {
		return aepr.WriteResponseAndNewErrorf(http.StatusNotFound, "ATTACHMENT_NOT_FOUND", "no attachment %s", r.AttachmentId)
	}
	return SendStream(aepr, DownloadAttachmentStatus, DownloadAttachmentMediaType, r.AttachmentId+".bin", size, DownloadAttachmentMaxBytes, &pattern{n: size})
}

func bodyExportVault(aepr *api.DXAPIEndPointRequest, r ExportVaultRequest) error {
	return SendStream(aepr, ExportVaultStatus, ExportVaultMediaType, "vault."+r.Format, exportSize, ExportVaultMaxBytes, &pattern{n: exportSize})
}

const exportSize = 24 << 20
`

// roundTripTest drives the generated client against the generated service
// bound to a dxlib API.
const roundTripTest = `package vault

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"net"
	"testing"
	"time"

	"github.com/donnyhardyanto/dxlib/api"
	"golang.org/x/sync/errgroup"
)

// hashing counts and hashes what is written to it, holding nothing.
type hashing struct {
	h hash.Hash
	n int64
}

func (w *hashing) Write(b []byte) (int, error) {
	w.n += int64(len(b))
	return w.h.Write(b)
}

func startVault(t *testing.T) *api.DXAPI {
	t.Helper()
	a, err := api.Manager.NewAPI("vault-roundtrip")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	a.Address = ln.Addr().String()
	_ = ln.Close()
	a.WriteTimeoutSec, a.ReadTimeoutSec = 120, 120
	Register(a)
	doc, err := api.ReadOpenAPIFile("openapi.yaml")
	if err != nil {
		t.Fatalf("the generated document does not read: %v", err)
	}
	if err := a.BindOpenAPI(doc); err != nil {
		t.Fatalf("the generated document does not bind: %v", err)
	}
	group, _ := errgroup.WithContext(context.Background())
	if err := a.StartAndWait(group); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if a.HTTPServer != nil {
			_ = a.HTTPServer.Close()
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", a.Address, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return a
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the vault did not start listening at %s", a.Address)
	return nil
}

func TestRoundTrip(t *testing.T) {
	a := startVault(t)
	ctx := context.Background()
	c := NewClient("http://" + a.Address)

	// An encrypted call before any handshake is refused by the client itself.
	if _, _, err := c.AddNote(ctx, map[string]any{"title": "early"}); err == nil {
		t.Fatal("an encrypted call before Handshake was not refused")
	}
	if err := c.Handshake(ctx); err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	status, answer, err := c.AddNote(ctx, map[string]any{"title": "hello"})
	if err != nil || status != 200 || answer["note_id"] != "n-hello" {
		t.Fatalf("AddNote in the envelope: status %d, answer %v, err %v", status, answer, err)
	}
	// The handshake carries one envelope: a second is refused plain, telling
	// the client to establish a new one.
	status, answer, err = c.AddNote(ctx, map[string]any{"title": "again"})
	if err != nil || status != 422 || answer["reason"] != "REFRESH_PREKEY" {
		t.Fatalf("a second envelope on a one-use handshake: status %d, answer %v, err %v", status, answer, err)
	}

	// An upload larger than any buffer, streamed in and hashed on both sides.
	const size = 40 << 20
	sent := &hashing{h: sha256.New()}
	var expected [32]byte
	{
		p := &pattern{n: size}
		buf := make([]byte, 64<<10)
		for {
			n, err := p.Read(buf)
			if n > 0 {
				_, _ = sent.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		copy(expected[:], sent.h.Sum(nil))
	}
	status, answer, err = c.UploadAttachment(ctx, &pattern{n: size}, size)
	if err != nil || status != 200 {
		t.Fatalf("UploadAttachment: status %d, answer %v, err %v", status, answer, err)
	}
	id, _ := answer["attachment_id"].(string)
	if id != hex.EncodeToString(expected[:])[:16] {
		t.Fatalf("the service hashed the upload as %s, the client as %s", id, hex.EncodeToString(expected[:])[:16])
	}
	if got, _ := answer["size"].(float64); int64(got) != size {
		t.Fatalf("the service counted %v bytes, %d were sent", answer["size"], size)
	}

	// The same bytes streamed back.
	received := &hashing{h: sha256.New()}
	status, n, refusal, err := c.DownloadAttachment(ctx, map[string]any{"attachment_id": id}, received)
	if err != nil || status != 200 || n != size || refusal != nil {
		t.Fatalf("DownloadAttachment: status %d, %d bytes, refusal %v, err %v", status, n, refusal, err)
	}
	if !bytes.Equal(received.h.Sum(nil), expected[:]) {
		t.Fatal("the download's bytes differ from the upload's")
	}
	// A download of nothing is the declared refusal, as JSON.
	status, n, refusal, err = c.DownloadAttachment(ctx, map[string]any{"attachment_id": "none"}, received)
	if err != nil || status != 404 || n != 0 || refusal["reason"] != "ATTACHMENT_NOT_FOUND" {
		t.Fatalf("DownloadAttachment of nothing: status %d, %d bytes, refusal %v, err %v", status, n, refusal, err)
	}

	// An encrypted request answered with a plain stream, under a fresh handshake.
	if err := c.Handshake(ctx); err != nil {
		t.Fatalf("Handshake again: %v", err)
	}
	exported := &hashing{h: sha256.New()}
	status, n, refusal, err = c.ExportVault(ctx, map[string]any{"format": "zip"}, exported)
	if err != nil || status != 200 || n != exportSize || refusal != nil {
		t.Fatalf("ExportVault: status %d, %d bytes, refusal %v, err %v", status, n, refusal, err)
	}
	want := &hashing{h: sha256.New()}
	p := &pattern{n: exportSize}
	buf := make([]byte, 64<<10)
	for {
		k, err := p.Read(buf)
		if k > 0 {
			_, _ = want.Write(buf[:k])
		}
		if err != nil {
			break
		}
	}
	if !bytes.Equal(exported.h.Sum(nil), want.h.Sum(nil)) {
		t.Fatal("the export's bytes are not the pattern")
	}
}
`
