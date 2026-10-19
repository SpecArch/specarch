package gendxlib

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/genopenapi"
)

// The transport of an operation (ADR-145, ADR-146): the pre-key envelope a
// handshake gives it, and the stream it carries a file as. dxlib has one
// envelope, its datablock over X25519 shared keys, an Ed25519 signature and
// AES-256-CBC, opened and sealed by two hooks the service sets; the
// exchange that establishes it is dxlib_module's pre-login, a0, a1 and a2
// answered with i, b0, b1 and b2. The generator writes the exchange's
// handler, the hooks over a store of handshakes, the helpers that stream a
// body in and out, and a client that does the same from the other side.

// ClientFileName is the second file the plug-in writes: a Go client of the
// service's operations, which performs the handshake and the streams.
const ClientFileName = "specarch_dxlib_client.go"

// dxlibEnvelope are the primitives dxlib's envelope carries.
var dxlibEnvelope = map[string]string{"keyAgreement": "x25519", "signature": "ed25519", "cipher": "aes-256-cbc"}

// exchangeRequest and exchangeResponse are the fields dxlib_module's
// pre-login takes and answers, which the client and the hooks assume.
var (
	exchangeRequest  = []string{"a0", "a1", "a2"}
	exchangeResponse = []string{"i", "b0", "b1", "b2"}
)

// handshake is the one handshake the file serves, when the design declares
// one dxlib's envelope carries.
type handshake struct {
	name          string
	node          map[string]any
	establishedBy string
	lifetime      int64 // seconds, 0 for none
	uses          int64 // 0 for any number
}

// readHandshake finds the handshake the file serves: the design's one
// handshake of dxlib's primitives. A second is refused, since dxlib sets one
// pair of hooks; one of other primitives is refused by the dxlib dialect of
// the document, and left out here.
func (g *gen) readHandshake() {
	hs := obj(g.spec["handshakes"])
	var names []string
	for _, name := range sortedKeys(hs) {
		h := obj(hs[name])
		fits := true
		for k, v := range dxlibEnvelope {
			fits = fits && text(h[k]) == v
		}
		if fits {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return
	}
	if len(names) > 1 {
		g.problem("error", "/handshakes/"+names[1], "the specification declares %d handshakes of dxlib's primitives (%s), and dxlib opens and seals its pre-key envelope through one pair of hooks; declare one", len(names), strings.Join(names, ", "))
		return
	}
	h := obj(hs[names[0]])
	g.handshake = &handshake{name: names[0], node: h, establishedBy: text(h["establishedBy"])}
	if d := text(h["lifetime"]); d != "" {
		if s, ok := seconds(d); ok {
			g.handshake.lifetime = s
		} else {
			g.problem("warning", "/handshakes/"+names[0]+"/lifetime", "the lifetime %s is not a duration go-dxlib writes as seconds; the handshake is written as never expiring by time", d)
		}
	}
	if u := text(h["uses"]); u != "" {
		fmt.Sscan(u, &g.handshake.uses)
	}
}

// isExchange says whether an operation establishes the handshake.
func (g *gen) isExchange(o operation) bool {
	return g.handshake != nil && o.id == g.handshake.establishedBy
}

// stream is an operation's stream direction, "" for none.
func stream(o operation) string {
	return text(obj(o.node["stream"])["direction"])
}

// streamMediaType is the media type of a download stream's success
// response, octet-stream when it names none.
func streamMediaType(o operation) string {
	for _, code := range sortedKeys(obj(o.node["responses"])) {
		if !strings.HasPrefix(code, "2") {
			continue
		}
		for _, mt := range sortedKeys(obj(obj(obj(o.node["responses"])[code])["content"])) {
			return mt
		}
	}
	return "application/octet-stream"
}

// limitOf is one of an operation's limits as an integer, 0 for none.
func limitOf(o operation, key string) int64 {
	var n int64
	fmt.Sscan(text(obj(o.node["limits"])[key]), &n)
	return n
}

// exchangeFits checks that the exchange operation declares what dxlib's
// pre-key exchange takes and answers, and reports what differs.
func (g *gen) exchangeFits(o operation, params []param) bool {
	var got []string
	for _, p := range params {
		got = append(got, p.wire)
	}
	sort.Strings(got)
	ok := true
	if strings.Join(got, ",") != strings.Join(exchangeRequest, ",") {
		g.problem("error", o.pointer(), "%s establishes handshake %s, and dxlib's pre-key exchange takes the client's keys as %s; it declares %s", o.id, g.handshake.name, strings.Join(exchangeRequest, ", "), joinOrNone(got))
		ok = false
	}
	var answered []string
	for _, code := range sortedKeys(obj(o.node["responses"])) {
		if !strings.HasPrefix(code, "2") {
			continue
		}
		for _, c := range obj(obj(obj(o.node["responses"])[code])["content"]) {
			for k := range obj(obj(obj(c)["schema"])["properties"]) {
				answered = append(answered, genopenapi.Wire(k))
			}
		}
	}
	sort.Strings(answered)
	wantAnswered := append([]string(nil), exchangeResponse...)
	sort.Strings(wantAnswered)
	if strings.Join(answered, ",") != strings.Join(wantAnswered, ",") {
		g.problem("error", o.pointer()+"/responses", "%s establishes handshake %s, and dxlib's pre-key exchange answers the service's keys and the index as %s; its success declares %s", o.id, g.handshake.name, strings.Join(exchangeResponse, ", "), joinOrNone(answered))
		ok = false
	}
	return ok
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

// exchangeBody writes the exchange: the client's keys read as hex, the
// service's three key pairs made, the two shared secrets derived, the
// handshake stored under a fresh index, and the service's public keys
// answered with the index.
func (g *gen) exchangeBody(o operation) {
	for _, i := range []string{"crypto/ed25519", "encoding/hex", "net/http", "time", "github.com/donnyhardyanto/dxlib/utils", "github.com/donnyhardyanto/dxlib/utils/crypto/x25519"} {
		g.imports[i] = true
	}
	g.helper("preKeys")
	g.line("\ta0, err := hexKey(aepr, \"a0\", r.A0, ed25519.PublicKeySize)")
	g.line("\tif err != nil {")
	g.line("\t\treturn err")
	g.line("\t}")
	g.line("\ta1, err := hexKey(aepr, \"a1\", r.A1, 32)")
	g.line("\tif err != nil {")
	g.line("\t\treturn err")
	g.line("\t}")
	g.line("\ta2, err := hexKey(aepr, \"a2\", r.A2, 32)")
	g.line("\tif err != nil {")
	g.line("\t\treturn err")
	g.line("\t}")
	g.line("\tb0Public, b0Private, err := ed25519.GenerateKey(nil)")
	g.line("\tif err != nil {")
	g.line("\t\treturn err")
	g.line("\t}")
	g.line("\tb1Public, b1Private, err := x25519.GenerateKeyPair()")
	g.line("\tif err != nil {")
	g.line("\t\treturn err")
	g.line("\t}")
	g.line("\tb2Public, b2Private, err := x25519.GenerateKeyPair()")
	g.line("\tif err != nil {")
	g.line("\t\treturn err")
	g.line("\t}")
	g.line("\tshared1, err := x25519.ComputeSharedSecret(b1Private, a1)")
	g.line("\tif err != nil {")
	g.line("\t\treturn aepr.WriteResponseAndNewErrorf(http.StatusUnprocessableEntity, \"\", \"INVALID_PARAMETER:a1:%%s\", err.Error())")
	g.line("\t}")
	g.line("\tshared2, err := x25519.ComputeSharedSecret(b2Private, a2)")
	g.line("\tif err != nil {")
	g.line("\t\treturn aepr.WriteResponseAndNewErrorf(http.StatusUnprocessableEntity, \"\", \"INVALID_PARAMETER:a2:%%s\", err.Error())")
	g.line("\t}")
	g.line("\tindex, err := newPreKeyIndex()")
	g.line("\tif err != nil {")
	g.line("\t\treturn err")
	g.line("\t}")
	g.line("\tkey := &PreKey{Index: index, ClientSigningKey: a0, SigningKey: b0Private, SharedKey1: shared1, SharedKey2: shared2}")
	g.line("\tif preKeyLifetime > 0 {")
	g.line("\t\tkey.ExpiresAt = time.Now().Add(preKeyLifetime)")
	g.line("\t}")
	g.line("\tif err := PreKeys.Put(aepr.Context, key); err != nil {")
	g.line("\t\treturn err")
	g.line("\t}")
	g.line("\taepr.WriteResponseAsJSON(http.StatusOK, nil, utils.JSON{\"i\": index, \"b0\": hex.EncodeToString(b0Public), \"b1\": hex.EncodeToString(b1Public), \"b2\": hex.EncodeToString(b2Public)})")
	g.line("\treturn nil")
}

// transport writes what the handshake and the streams need once: the
// handshake's record, store and hooks, and the stream helpers.
func (g *gen) transport() {
	if g.handshake != nil {
		g.hooks()
	}
	if g.helpers["uploadStream"] {
		g.imports["io"] = true
		g.imports["net/http"] = true
		g.line("// UploadStream is the request body of an upload stream as it arrives, held to")
		g.line("// maxBytes when the operation declares one (0 for none): dxlib caps a streamed")
		g.line("// upload only by its declared Content-Length, and a chunked upload declares none.")
		g.line("func UploadStream(aepr *api.DXAPIEndPointRequest, maxBytes int64) io.Reader {")
		g.line("\tif maxBytes > 0 {")
		g.line("\t\treturn http.MaxBytesReader(*aepr.ResponseWriter, aepr.Request.Body, maxBytes)")
		g.line("\t}")
		g.line("\treturn aepr.Request.Body")
		g.line("}")
		g.line("")
	}
	if g.helpers["sendStream"] {
		for _, i := range []string{"fmt", "io", "net/http", "strconv"} {
			g.imports[i] = true
		}
		g.line("// SendStream writes a download stream as dxlib's own sender does: the media")
		g.line("// type, the length when known (size below 0 for unknown) and the file's name")
		g.line("// in the headers, the status, then the bytes. A file above maxBytes (0 for")
		g.line("// none) is refused before a byte is sent; a copy that fails abandons the")
		g.line("// response, so a client never keeps a short file as a whole one.")
		g.line("func SendStream(aepr *api.DXAPIEndPointRequest, mediaType, filename string, size, maxBytes int64, r io.Reader) error {")
		g.line("\tif maxBytes > 0 && size > maxBytes {")
		g.line("\t\treturn aepr.WriteResponseAndNewErrorf(http.StatusInternalServerError, \"\", \"FILE_ABOVE_LIMIT:%%d>%%d\", size, maxBytes)")
		g.line("\t}")
		g.line("\tw := *aepr.GetResponseWriter()")
		g.line("\tw.Header().Set(\"Content-Type\", mediaType)")
		g.line("\tif size >= 0 {")
		g.line("\t\tw.Header().Set(\"Content-Length\", strconv.FormatInt(size, 10))")
		g.line("\t}")
		g.line("\tif filename != \"\" {")
		g.line("\t\tw.Header().Set(\"Content-Disposition\", fmt.Sprintf(\"attachment; filename=%%q\", filename))")
		g.line("\t}")
		g.line("\taepr.ResponseHeaderSent = true")
		g.line("\tw.WriteHeader(http.StatusOK)")
		g.line("\taepr.ResponseStatusCode = http.StatusOK")
		g.line("\tif _, err := io.Copy(w, r); err != nil {")
		g.line("\t\taepr.Log.Errorf(err, \"SEND_STREAM_ERROR:%%s\", err.Error())")
		g.line("\t\tpanic(http.ErrAbortHandler)")
		g.line("\t}")
		g.line("\taepr.ResponseBodySent = true")
		g.line("\treturn nil")
		g.line("}")
		g.line("")
	}
}

// hooks writes the handshake's record, its store, the exchange's helpers
// and the two hooks dxlib calls to open and seal the envelope.
func (g *gen) hooks() {
	h := g.handshake
	for _, i := range []string{"context", "crypto/ed25519", "crypto/rand", "encoding/hex", "errors", "net/http", "sync", "time", "github.com/donnyhardyanto/dxlib/utils", "github.com/donnyhardyanto/dxlib/utils/crypto/datablock", "github.com/donnyhardyanto/dxlib/utils/lv"} {
		g.imports[i] = true
	}
	g.line("// PreKey is one handshake of %s: the keys both sides derived in the exchange", h.name)
	g.line("// and the index that names it in every envelope after.")
	g.line("type PreKey struct {")
	g.line("\tIndex            string")
	g.line("\tClientSigningKey []byte             // the client's Ed25519 public key (a0), which verifies its envelopes")
	g.line("\tSigningKey       ed25519.PrivateKey // the service's Ed25519 private key (b0), which signs its answers")
	g.line("\tSharedKey1       []byte             // X25519 of b1 and a1, which opens the client's envelopes")
	g.line("\tSharedKey2       []byte             // X25519 of b2 and a2, which seals the service's answers")
	g.line("\tExpiresAt        time.Time          // zero when the handshake does not expire by time")
	g.line("\tUses             int                // the envelopes opened so far")
	g.line("}")
	g.line("")
	g.line("// PreKeyStore keeps the handshakes between the exchange and the envelopes they")
	g.line("// open. The service replaces the in-memory store with one its instances share.")
	g.line("type PreKeyStore interface {")
	g.line("\tPut(ctx context.Context, key *PreKey) error")
	g.line("\tGet(ctx context.Context, index string) (*PreKey, error) // nil when none")
	g.line("\tDelete(ctx context.Context, index string) error")
	g.line("}")
	g.line("")
	g.line("// PreKeys is the store the exchange writes and the hooks read.")
	g.line("var PreKeys PreKeyStore = NewMemoryPreKeys()")
	g.line("")
	g.line("// preKeyLifetime and preKeyUses are what the design declares of %s: how long a", h.name)
	g.line("// handshake is accepted (0 for no limit by time) and how many envelopes one")
	g.line("// carries (0 for any number).")
	g.line("const (")
	g.line("\tpreKeyLifetime = %d * time.Second", h.lifetime)
	g.line("\tpreKeyUses     = %d", h.uses)
	g.line(")")
	g.line("")
	g.line("type memoryPreKeys struct {")
	g.line("\tmu   sync.Mutex")
	g.line("\tkeys map[string]*PreKey")
	g.line("}")
	g.line("")
	g.line("// NewMemoryPreKeys is a store in this process's memory, for one instance.")
	g.line("func NewMemoryPreKeys() PreKeyStore { return &memoryPreKeys{keys: map[string]*PreKey{}} }")
	g.line("")
	g.line("func (m *memoryPreKeys) Put(ctx context.Context, key *PreKey) error {")
	g.line("\tm.mu.Lock()")
	g.line("\tdefer m.mu.Unlock()")
	g.line("\tcopied := *key")
	g.line("\tm.keys[key.Index] = &copied")
	g.line("\treturn nil")
	g.line("}")
	g.line("")
	g.line("func (m *memoryPreKeys) Get(ctx context.Context, index string) (*PreKey, error) {")
	g.line("\tm.mu.Lock()")
	g.line("\tdefer m.mu.Unlock()")
	g.line("\tkey, ok := m.keys[index]")
	g.line("\tif !ok {")
	g.line("\t\treturn nil, nil")
	g.line("\t}")
	g.line("\tcopied := *key")
	g.line("\treturn &copied, nil")
	g.line("}")
	g.line("")
	g.line("func (m *memoryPreKeys) Delete(ctx context.Context, index string) error {")
	g.line("\tm.mu.Lock()")
	g.line("\tdefer m.mu.Unlock()")
	g.line("\tdelete(m.keys, index)")
	g.line("\treturn nil")
	g.line("}")
	g.line("")
	g.line("// newPreKeyIndex is a fresh index for a handshake, 16 random bytes in hex.")
	g.line("func newPreKeyIndex() (string, error) {")
	g.line("\tvar b [16]byte")
	g.line("\tif _, err := rand.Read(b[:]); err != nil {")
	g.line("\t\treturn \"\", err")
	g.line("\t}")
	g.line("\treturn \"PREKEY_\" + hex.EncodeToString(b[:]), nil")
	g.line("}")
	g.line("")
	g.line("// hexKey reads a key the client sends as hex, of the length the curve asks.")
	g.line("func hexKey(aepr *api.DXAPIEndPointRequest, name, value string, size int) ([]byte, error) {")
	g.line("\tkey, err := hex.DecodeString(value)")
	g.line("\tif err != nil || len(key) != size {")
	g.line("\t\treturn nil, aepr.WriteResponseAndNewErrorf(http.StatusUnprocessableEntity, \"\", \"INVALID_PARAMETER:%%s:not a %%d-byte key in hex\", name, size)")
	g.line("\t}")
	g.line("\treturn key, nil")
	g.line("}")
	g.line("")
	g.line("// openPreKeyEnvelope is dxlib's OnE2EEPrekeyUnPack: it finds the handshake the")
	g.line("// envelope names, refuses one that has expired or carried its envelopes, counts")
	g.line("// this one, and opens the envelope with the client's signing key and the first")
	g.line("// shared key. dxlib answers a refusal here as a plain 422 that tells the client")
	g.line("// to establish a new handshake.")
	g.line("func openPreKeyEnvelope(aepr *api.DXAPIEndPointRequest, index string, data string) ([]*lv.LV, []byte, []byte, utils.JSON, error) {")
	g.line("\tkey, err := PreKeys.Get(aepr.Context, index)")
	g.line("\tif err != nil {")
	g.line("\t\treturn nil, nil, nil, nil, err")
	g.line("\t}")
	g.line("\tif key == nil {")
	g.line("\t\treturn nil, nil, nil, nil, errors.New(\"PREKEY_NOT_FOUND\")")
	g.line("\t}")
	g.line("\tif !key.ExpiresAt.IsZero() && time.Now().After(key.ExpiresAt) {")
	g.line("\t\t_ = PreKeys.Delete(aepr.Context, index)")
	g.line("\t\treturn nil, nil, nil, nil, errors.New(\"PREKEY_EXPIRED\")")
	g.line("\t}")
	g.line("\tif preKeyUses > 0 && key.Uses >= preKeyUses {")
	g.line("\t\t_ = PreKeys.Delete(aepr.Context, index)")
	g.line("\t\treturn nil, nil, nil, nil, errors.New(\"PREKEY_USED_UP\")")
	g.line("\t}")
	g.line("\tkey.Uses++")
	g.line("\tif preKeyUses > 0 && key.Uses >= preKeyUses {")
	g.line("\t\terr = PreKeys.Delete(aepr.Context, index)")
	g.line("\t} else {")
	g.line("\t\terr = PreKeys.Put(aepr.Context, key)")
	g.line("\t}")
	g.line("\tif err != nil {")
	g.line("\t\treturn nil, nil, nil, nil, err")
	g.line("\t}")
	g.line("\telements, err := datablock.UnpackLVPayload(index, key.ClientSigningKey, key.SharedKey1, data)")
	g.line("\tif err != nil {")
	g.line("\t\treturn nil, nil, nil, nil, err")
	g.line("\t}")
	g.line("\treturn elements, key.SharedKey2, []byte(key.SigningKey), utils.JSON{\"index\": index}, nil")
	g.line("}")
	g.line("")
	g.line("// sealPreKeyEnvelope is dxlib's OnE2EEPrekeyPack: it seals the answer with the")
	g.line("// service's signing key and the second shared key, as dxlib's datablock does.")
	g.line("func sealPreKeyEnvelope(aepr *api.DXAPIEndPointRequest, index string, signingKey []byte, sharedKey2 []byte, payloads ...*lv.LV) (string, error) {")
	g.line("\treturn datablock.PackLVPayload(index, signingKey, sharedKey2, payloads...)")
	g.line("}")
	g.line("")
}
