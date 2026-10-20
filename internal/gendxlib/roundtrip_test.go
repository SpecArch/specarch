package gendxlib

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/SpecArch/specarch/internal/genopenapi"
	"github.com/SpecArch/specarch/internal/genuits"
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
	roundTrip(t, envelopeFixture)
}

// extractedFixture is the vault read back: the tree extract go writes from
// the vault written by hand on dxlib, with every question it asked answered
// by a decision and the entries written from the answers.
const extractedFixture = "../../spec/tests/generate-go-dxlib-extracted-vault/project"

// extractedFrom is the tree extract go writes from that vault, as its
// conformance case records it.
const extractedFrom = "../../spec/tests/extract-go-dxlib-vault/expected/spec"

// TestExtractedRoundTrip is the extract leg of the round trip: the vault
// written by hand on dxlib, read with extract go, its questions answered by
// the decisions committed beside the tree, regenerates to a service and a
// client that pass the same round trip as the vault written as a
// specification, so code read into a specification regenerates to code that
// behaves the same. It first checks that the decisions answer every
// question the reader asked, and that none is left open.
func TestExtractedRoundTrip(t *testing.T) {
	asked := spec.Load(extractedFrom)
	answered := spec.Load(extractedFixture)
	if len(asked.Problems) > 0 || len(answered.Problems) > 0 {
		t.Fatalf("the trees do not load: %v %v", asked.Problems, answered.Problems)
	}
	questions, _ := asked.Value.(map[string]any)["questions"].(map[string]any)
	if len(questions) == 0 {
		t.Fatal("the extracted tree asks no question; the case no longer shows the decisions at work")
	}
	if open, _ := answered.Value.(map[string]any)["questions"].(map[string]any); len(open) > 0 {
		t.Fatalf("the answered tree leaves %d questions open", len(open))
	}
	answers := map[string]string{}
	decisions, _ := answered.Value.(map[string]any)["decisions"].(map[string]any)
	for id, d := range decisions {
		list, _ := d.(map[string]any)["answers"].([]any)
		for _, q := range list {
			answers[q.(string)] = id
		}
	}
	for q := range questions {
		if answers[q] == "" {
			t.Errorf("%s, which extract go asks of the vault, is answered by no decision", q)
		}
	}
	for q := range answers {
		if questions[q] == nil {
			t.Errorf("a decision answers %s, which extract go no longer asks; take it out or extract again", q)
		}
	}
	roundTrip(t, extractedFixture)
}

// roundTrip generates a fixture's service and client, binds the service to
// a real dxlib API through the document the dxlib dialect writes, and runs
// roundTripTest and screensTest against it.
func roundTrip(t *testing.T, fixture string) {
	t.Helper()
	dxlib := os.Getenv("SPECARCH_DXLIB")
	if dxlib == "" {
		t.Skip("SPECARCH_DXLIB names no dxlib checkout, so the generated service cannot run here")
	}
	service := generateFrom(t, fixture, func(map[string]any) {}, nil)
	if len(service.Files) != 2 || len(service.Diagnostics) != 0 {
		t.Fatalf("want the service file and the client with no diagnostic, got %d files and %v", len(service.Files), service.Diagnostics)
	}
	document := openapiDocument(t, fixture)
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":            "module vault\n\ngo 1.27.1\n\nrequire github.com/donnyhardyanto/dxlib v0.0.0\n\nreplace github.com/donnyhardyanto/dxlib => " + dxlib + "\n",
		"openapi.yaml":      document,
		"bodies.go":         roundTripBodies,
		"roundtrip_test.go": roundTripTest,
		"transport.ts":      genuits.ScreenFile("transport.ts"),
		"screens.test.ts":   screensTest,
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
	if node, err := exec.LookPath("node"); err == nil {
		run.Env = append(os.Environ(), "SPECARCH_NODE="+node)
	} else {
		t.Log("node is not on PATH, so the screens' transport is not driven against the service")
	}
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
	"os"
	"os/exec"
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

// TestScreensRoundTrip drives the TypeScript screens' transport, as
// specarch-gen-ui-typescript writes it, against the same service on Node:
// the handshake established on the first encrypted call through the Web
// Crypto API, the envelope sealed and opened, and a used-up handshake
// renewed when the service answers REFRESH_PREKEY.
func TestScreensRoundTrip(t *testing.T) {
	node := os.Getenv("SPECARCH_NODE")
	if node == "" {
		t.Skip("SPECARCH_NODE names no node, so the screens' transport cannot run here")
	}
	a := startVault(t)
	run := exec.Command(node, "--test", "screens.test.ts")
	run.Env = append(os.Environ(), "VAULT_URL=http://"+a.Address)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("the screens' transport does not pass against the service: %v\n%s", err, out)
	}
	t.Logf("%s", out)
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

// screensTest drives the screens' transport against the vault on Node.
const screensTest = `import assert from "node:assert/strict";
import { test } from "node:test";
import { createHash } from "node:crypto";
import { createTransport } from "./transport.ts";

const service = process.env.VAULT_URL ?? "";
const handshakes = { preKey: { operation: "issuePreKey" } };
const addNote = { operation: "addNote", path: "/notes", handshake: "preKey" };

test("the first encrypted call establishes the handshake and opens the answer", async () => {
  const transport = createTransport({ service, dialect: "dxlib", handshakes });
  const response = await transport.call(addNote, { body: { title: "hello" } });
  assert.equal(response.status, 200);
  assert.equal((await response.json()).note_id, "n-hello");
});

test("a used-up handshake is renewed when the service answers REFRESH_PREKEY", async () => {
  const transport = createTransport({ service, dialect: "dxlib", handshakes });
  for (const title of ["one", "two", "three"]) {
    const response = await transport.call(addNote, { body: { title } });
    assert.equal(response.status, 200, title);
    assert.equal((await response.json()).note_id, "n-" + title);
  }
});

test("a plain call in the dxlib dialect is a POST at the operation", async () => {
  const transport = createTransport({ service, dialect: "dxlib", handshakes });
  const response = await transport.call({ operation: "downloadAttachment", path: "/attachments/download" }, { body: { attachment_id: "none" } });
  assert.equal(response.status, 404);
  assert.equal((await response.json()).reason, "ATTACHMENT_NOT_FOUND");
});

/** n bytes of the vault's pattern, byte i being i modulo 251. */
function pattern(n: number): Uint8Array {
  const out = new Uint8Array(n);
  for (let i = 0; i < n; i++) {
    out[i] = i % 251;
  }
  return out;
}

async function digest(body: ReadableStream<Uint8Array> | null): Promise<{ hash: string; size: number }> {
  const hash = createHash("sha256");
  let size = 0;
  for await (const chunk of body ?? []) {
    hash.update(chunk);
    size += chunk.length;
  }
  return { hash: hash.digest("hex"), size };
}

const uploadAttachment = { operation: "uploadAttachment", path: "/attachments", stream: "upload", mediaType: "application/octet-stream" } as const;
const downloadAttachment = { operation: "downloadAttachment", path: "/attachments/download", stream: "download" } as const;

test("a file uploaded as a Blob and as a stream is streamed back whole", async () => {
  const transport = createTransport({ service, dialect: "dxlib", handshakes });
  const bytes = pattern(3 << 20);
  const sent = createHash("sha256").update(bytes).digest("hex");
  const asBlob = await transport.upload(uploadAttachment, new Blob([bytes]));
  assert.equal(asBlob.status, 200);
  const stored = await asBlob.json();
  assert.equal(stored.attachment_id, sent.slice(0, 16));
  assert.equal(stored.size, bytes.length);
  const asStream = await transport.upload(uploadAttachment, new Blob([bytes]).stream());
  assert.equal(asStream.status, 200);
  assert.equal((await asStream.json()).attachment_id, sent.slice(0, 16));
  const back = await transport.call(downloadAttachment, { body: { attachment_id: stored.attachment_id } });
  assert.equal(back.status, 200);
  assert.deepEqual(await digest(back.body), { hash: sent, size: bytes.length });
});

test("a download asked for in the envelope answers its bytes plain", async () => {
  const transport = createTransport({ service, dialect: "dxlib", handshakes });
  const response = await transport.call({ operation: "exportVault", path: "/vault/export", handshake: "preKey", stream: "download" }, { body: { format: "zip" } });
  assert.equal(response.status, 200);
  const size = 24 << 20;
  assert.deepEqual(await digest(response.body), { hash: createHash("sha256").update(pattern(size)).digest("hex"), size });
});

test("an envelope another service signed is refused", async () => {
  const transport = createTransport({ service, dialect: "dxlib", handshakes });
  const tampered = createTransport({
    service,
    dialect: "dxlib",
    handshakes,
    fetch: async (input, init) => {
      const response = await fetch(input, init);
      const text = await response.text();
      const value = JSON.parse(text);
      if (typeof value.d === "string") {
        const last = value.d.slice(-2) === "00" ? "01" : "00";
        value.d = value.d.slice(0, -2) + last;
      }
      return new Response(JSON.stringify(value), { status: response.status, headers: response.headers });
    },
  });
  await assert.rejects(tampered.call(addNote, { body: { title: "x" } }), /signature/);
  const response = await transport.call(addNote, { body: { title: "after" } });
  assert.equal(response.status, 200);
});
`
