package gendxlib

import (
	"fmt"
	"go/format"
	"sort"
	"strings"
)

// The client of the service (ADR-145, ADR-146): one Go file beside the
// service's, with a method per operation that calls it as the dxlib dialect
// serves it, a POST at /<operationId>, and does what the design says a
// client must: establishes the handshake and travels in its envelope,
// streams an upload in and a download out.

// client writes the client file; false when the specification has no
// operation to call.
func (g *gen) client(pkg string, info map[string]any) (string, bool) {
	ops := g.operations()
	if len(ops) == 0 {
		return "", false
	}
	imports := map[string]bool{"bytes": true, "context": true, "encoding/json": true, "fmt": true, "io": true, "net/http": true, "strings": true}
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	hasUpload, hasDownload := false, false
	for _, o := range ops {
		switch stream(o) {
		case "upload":
			hasUpload = true
		case "download":
			hasDownload = true
		}
	}
	h := g.handshake
	w("// Client calls %s over the dxlib dialect's API, every operation a POST at", text(info["title"]))
	w("// /<operationId> on BaseURL, through HTTP (http.DefaultClient when nil). A")
	w("// method per operation calls it as the design says a client must.")
	w("type Client struct {")
	w("\tBaseURL string")
	w("\tHTTP    *http.Client")
	if h != nil {
		w("\tpreKey  *clientPreKey // the handshake held, nil before Handshake")
	}
	w("}")
	w("")
	w("// NewClient is a client of the service at baseURL.")
	w("func NewClient(baseURL string) *Client {")
	w("\treturn &Client{BaseURL: strings.TrimRight(baseURL, \"/\"), HTTP: http.DefaultClient}")
	w("}")
	w("")
	w("func (c *Client) do(req *http.Request) (*http.Response, error) {")
	w("\thttpClient := c.HTTP")
	w("\tif httpClient == nil {")
	w("\t\thttpClient = http.DefaultClient")
	w("\t}")
	w("\treturn httpClient.Do(req)")
	w("}")
	w("")
	w("// call posts a JSON body and reads a JSON answer: the status and the answer's")
	w("// members, and an error only when the call itself failed or the answer is")
	w("// not JSON.")
	w("func (c *Client) call(ctx context.Context, operationId string, body map[string]any) (int, map[string]any, error) {")
	w("\tif body == nil {")
	w("\t\tbody = map[string]any{}")
	w("\t}")
	w("\tpayload, err := json.Marshal(body)")
	w("\tif err != nil {")
	w("\t\treturn 0, nil, err")
	w("\t}")
	w("\treq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+\"/\"+operationId, bytes.NewReader(payload))")
	w("\tif err != nil {")
	w("\t\treturn 0, nil, err")
	w("\t}")
	w("\treq.Header.Set(\"Content-Type\", \"application/json\")")
	w("\tresp, err := c.do(req)")
	w("\tif err != nil {")
	w("\t\treturn 0, nil, err")
	w("\t}")
	w("\tdefer resp.Body.Close()")
	w("\treturn readAnswer(operationId, resp)")
	w("}")
	w("")
	w("// readAnswer reads a JSON answer, empty for an empty body.")
	w("func readAnswer(operationId string, resp *http.Response) (int, map[string]any, error) {")
	w("\tdata, err := io.ReadAll(resp.Body)")
	w("\tif err != nil {")
	w("\t\treturn resp.StatusCode, nil, err")
	w("\t}")
	w("\tanswer := map[string]any{}")
	w("\tif len(bytes.TrimSpace(data)) > 0 {")
	w("\t\tif err := json.Unmarshal(data, &answer); err != nil {")
	w("\t\t\treturn resp.StatusCode, nil, fmt.Errorf(\"%%s answered %%d with a body that is not JSON: %%w\", operationId, resp.StatusCode, err)")
	w("\t\t}")
	w("\t}")
	w("\treturn resp.StatusCode, answer, nil")
	w("}")
	w("")
	if h != nil {
		for _, i := range []string{"crypto/ed25519", "encoding/base64", "encoding/binary", "encoding/hex", "errors", "github.com/donnyhardyanto/dxlib/utils/crypto/datablock", "github.com/donnyhardyanto/dxlib/utils/crypto/x25519", "github.com/donnyhardyanto/dxlib/utils/lv"} {
			imports[i] = true
		}
		g.clientHandshake(w, h)
	}
	if hasUpload {
		w("// upload posts a file as the request body, streamed from r, with size as its")
		w("// Content-Length (below 0 for unknown, sent chunked) and mediaType as its")
		w("// Content-Type, and the parameters, when there are any, as one JSON object in")
		w("// the X-Var header, where dxlib reads an upload's parameters; it reads the JSON")
		w("// answer.")
		w("func (c *Client) upload(ctx context.Context, operationId, mediaType string, parameters map[string]any, r io.Reader, size int64) (int, map[string]any, error) {")
		w("\treq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+\"/\"+operationId, r)")
		w("\tif err != nil {")
		w("\t\treturn 0, nil, err")
		w("\t}")
		w("\treq.ContentLength = size")
		w("\treq.Header.Set(\"Content-Type\", mediaType)")
		w("\tif len(parameters) > 0 {")
		w("\t\tvalues, err := json.Marshal(parameters)")
		w("\t\tif err != nil {")
		w("\t\t\treturn 0, nil, err")
		w("\t\t}")
		w("\t\treq.Header.Set(\"X-Var\", string(values))")
		w("\t}")
		w("\tresp, err := c.do(req)")
		w("\tif err != nil {")
		w("\t\treturn 0, nil, err")
		w("\t}")
		w("\tdefer resp.Body.Close()")
		w("\treturn readAnswer(operationId, resp)")
		w("}")
		w("")
	}
	if hasDownload {
		w("// download posts a JSON body, in the envelope when encrypted, and streams the")
		w("// answer's bytes into w as they arrive: the status, the bytes written, and the")
		w("// JSON a refusal answers instead of the file, nil when the file came.")
		w("func (c *Client) download(ctx context.Context, operationId string, body map[string]any, encrypted bool, w io.Writer) (int, int64, map[string]any, error) {")
		w("\tif body == nil {")
		w("\t\tbody = map[string]any{}")
		w("\t}")
		if h != nil {
			w("\tif encrypted {")
			w("\t\tenvelope, err := c.seal(body)")
			w("\t\tif err != nil {")
			w("\t\t\treturn 0, 0, nil, err")
			w("\t\t}")
			w("\t\tbody = envelope")
			w("\t}")
		} else {
			w("\tif encrypted {")
			w("\t\treturn 0, 0, nil, fmt.Errorf(\"%%s: the specification declares no handshake this client performs\", operationId)")
			w("\t}")
		}
		w("\tpayload, err := json.Marshal(body)")
		w("\tif err != nil {")
		w("\t\treturn 0, 0, nil, err")
		w("\t}")
		w("\treq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+\"/\"+operationId, bytes.NewReader(payload))")
		w("\tif err != nil {")
		w("\t\treturn 0, 0, nil, err")
		w("\t}")
		w("\treq.Header.Set(\"Content-Type\", \"application/json\")")
		w("\tresp, err := c.do(req)")
		w("\tif err != nil {")
		w("\t\treturn 0, 0, nil, err")
		w("\t}")
		w("\tdefer resp.Body.Close()")
		w("\tif strings.HasPrefix(resp.Header.Get(\"Content-Type\"), \"application/json\") {")
		w("\t\tstatus, answer, err := readAnswer(operationId, resp)")
		w("\t\treturn status, 0, answer, err")
		w("\t}")
		w("\tn, err := io.Copy(w, resp.Body)")
		w("\treturn resp.StatusCode, n, nil, err")
		w("}")
		w("")
	}
	for _, o := range ops {
		if g.isExchange(o) {
			continue
		}
		name := pascal(o.id)
		encrypted := text(o.node["handshake"]) != "" && h != nil && text(o.node["handshake"]) == h.name
		switch {
		case stream(o) == "upload" && len(g.params(o)) > 0:
			w("// %s calls %s: it streams the file from r as the request body, of the media", name, o.id)
			w("// type the design gives it, with size as its length (below 0 for unknown) and")
			w("// the parameters beside it, and answers the status and the JSON answer.")
			w("func (c *Client) %s(ctx context.Context, parameters map[string]any, r io.Reader, size int64) (int, map[string]any, error) {", name)
			w("\treturn c.upload(ctx, %q, %q, parameters, r, size)", o.id, uploadMediaType(o))
		case stream(o) == "upload":
			w("// %s calls %s: it streams the file from r as the request body, of the media", name, o.id)
			w("// type the design gives it, with size as its length (below 0 for unknown), and")
			w("// answers the status and the JSON answer.")
			w("func (c *Client) %s(ctx context.Context, r io.Reader, size int64) (int, map[string]any, error) {", name)
			w("\treturn c.upload(ctx, %q, %q, nil, r, size)", o.id, uploadMediaType(o))
		case stream(o) == "download" && encrypted:
			w("// %s calls %s: the request travels in the envelope of handshake %s, after", name, o.id, h.name)
			w("// Handshake, and the file comes back plain, streamed into w.")
			w("func (c *Client) %s(ctx context.Context, body map[string]any, w io.Writer) (int, int64, map[string]any, error) {", name)
			w("\treturn c.download(ctx, %q, body, true, w)", o.id)
		case stream(o) == "download":
			w("// %s calls %s and streams the file it answers into w.", name, o.id)
			w("func (c *Client) %s(ctx context.Context, body map[string]any, w io.Writer) (int, int64, map[string]any, error) {", name)
			w("\treturn c.download(ctx, %q, body, false, w)", o.id)
		case encrypted:
			w("// %s calls %s in the envelope of handshake %s, after Handshake: the body is", name, o.id, h.name)
			w("// sealed, the answer opened, and both the status and the answer are the ones")
			w("// inside the envelope.")
			w("func (c *Client) %s(ctx context.Context, body map[string]any) (int, map[string]any, error) {", name)
			w("\treturn c.callEncrypted(ctx, %q, body)", o.id)
		default:
			w("// %s calls %s with a JSON body and answers the status and the JSON answer.", name, o.id)
			w("func (c *Client) %s(ctx context.Context, body map[string]any) (int, map[string]any, error) {", name)
			w("\treturn c.call(ctx, %q, body)", o.id)
		}
		w("}")
		w("")
	}
	var head strings.Builder
	fmt.Fprintf(&head, "// Code generated by specarch-gen-go-dxlib from %s, version %s; meta-model %s. DO NOT EDIT.\n", g.relRoot, text(info["version"]), g.specarch)
	if g.draft != "" {
		fmt.Fprintf(&head, "// %s\n", g.draft)
	}
	head.WriteString("\n")
	fmt.Fprintf(&head, "// This file is the client of %s: a method per operation, calling it as the\n// dxlib dialect serves it and as the design says a client must.\n", text(info["title"]))
	fmt.Fprintf(&head, "package %s\n\nimport (\n", pkg)
	var std, ext []string
	for i := range imports {
		if strings.Contains(strings.Split(i, "/")[0], ".") {
			ext = append(ext, i)
		} else {
			std = append(std, i)
		}
	}
	sort.Strings(std)
	sort.Strings(ext)
	for _, i := range std {
		fmt.Fprintf(&head, "\t%q\n", i)
	}
	if len(ext) > 0 {
		head.WriteString("\n")
	}
	for _, i := range ext {
		fmt.Fprintf(&head, "\t%q\n", i)
	}
	head.WriteString(")\n\n")
	src, err := format.Source([]byte(head.String() + b.String()))
	if err != nil {
		g.problem("error", "/", "the generated Go client does not parse (%v); this is a bug in specarch-gen-go-dxlib", err)
		return "", false
	}
	return string(src), true
}

// clientHandshake writes the client's side of the handshake: Handshake, and
// the sealing and opening of the envelope as dxlib's datablock does them.
func (g *gen) clientHandshake(w func(string, ...any), h *handshake) {
	w("// clientPreKey is the handshake this client holds: its signing key, the")
	w("// service's, and the two shared keys the exchange derived.")
	w("type clientPreKey struct {")
	w("\tindex             string")
	w("\tsigningKey        ed25519.PrivateKey")
	w("\tserviceSigningKey []byte")
	w("\tsharedKey1        []byte")
	w("\tsharedKey2        []byte")
	w("}")
	w("")
	w("// Handshake establishes handshake %s through %s: it sends the client's", h.name, h.establishedBy)
	w("// Ed25519 key and two X25519 keys as a0, a1 and a2, reads the service's as b0,")
	w("// b1 and b2 with the index i, and derives the two shared keys. Every encrypted")
	w("// call after travels under it until the service refuses it as expired or used")
	w("// up, when Handshake is called again.")
	w("func (c *Client) Handshake(ctx context.Context) error {")
	w("\ta0Public, a0Private, err := ed25519.GenerateKey(nil)")
	w("\tif err != nil {")
	w("\t\treturn err")
	w("\t}")
	w("\ta1Public, a1Private, err := x25519.GenerateKeyPair()")
	w("\tif err != nil {")
	w("\t\treturn err")
	w("\t}")
	w("\ta2Public, a2Private, err := x25519.GenerateKeyPair()")
	w("\tif err != nil {")
	w("\t\treturn err")
	w("\t}")
	w("\tstatus, answer, err := c.call(ctx, %q, map[string]any{\"a0\": hex.EncodeToString(a0Public), \"a1\": hex.EncodeToString(a1Public), \"a2\": hex.EncodeToString(a2Public)})", h.establishedBy)
	w("\tif err != nil {")
	w("\t\treturn err")
	w("\t}")
	w("\tif status != http.StatusOK {")
	w("\t\treturn fmt.Errorf(\"%s answered %%d: %%v\", status, answer)", h.establishedBy)
	w("\t}")
	w("\tindex, _ := answer[\"i\"].(string)")
	w("\tif index == \"\" {")
	w("\t\treturn errors.New(\"%s answered no index i\")", h.establishedBy)
	w("\t}")
	w("\tb0, err := hexAnswer(answer, \"b0\", ed25519.PublicKeySize)")
	w("\tif err != nil {")
	w("\t\treturn err")
	w("\t}")
	w("\tb1, err := hexAnswer(answer, \"b1\", 32)")
	w("\tif err != nil {")
	w("\t\treturn err")
	w("\t}")
	w("\tb2, err := hexAnswer(answer, \"b2\", 32)")
	w("\tif err != nil {")
	w("\t\treturn err")
	w("\t}")
	w("\tshared1, err := x25519.ComputeSharedSecret(a1Private, b1)")
	w("\tif err != nil {")
	w("\t\treturn err")
	w("\t}")
	w("\tshared2, err := x25519.ComputeSharedSecret(a2Private, b2)")
	w("\tif err != nil {")
	w("\t\treturn err")
	w("\t}")
	w("\tc.preKey = &clientPreKey{index: index, signingKey: a0Private, serviceSigningKey: b0, sharedKey1: shared1, sharedKey2: shared2}")
	w("\treturn nil")
	w("}")
	w("")
	w("// hexAnswer reads a key the service answers as hex, of the length the curve asks.")
	w("func hexAnswer(answer map[string]any, name string, size int) ([]byte, error) {")
	w("\tvalue, _ := answer[name].(string)")
	w("\tkey, err := hex.DecodeString(value)")
	w("\tif err != nil || len(key) != size {")
	w("\t\treturn nil, fmt.Errorf(\"%%s is not a %%d-byte key in hex\", name, size)")
	w("\t}")
	w("\treturn key, nil")
	w("}")
	w("")
	w("// seal packs a request body into the envelope: a header and the body, each as")
	w("// base64 JSON, in dxlib's datablock under the first shared key, signed with the")
	w("// client's key, named by the handshake's index.")
	w("func (c *Client) seal(body map[string]any) (map[string]any, error) {")
	w("\tif c.preKey == nil {")
	w("\t\treturn nil, errors.New(\"no handshake is held; call Handshake first\")")
	w("\t}")
	w("\theader, err := json.Marshal(map[string]string{})")
	w("\tif err != nil {")
	w("\t\treturn nil, err")
	w("\t}")
	w("\tpayload, err := json.Marshal(body)")
	w("\tif err != nil {")
	w("\t\treturn nil, err")
	w("\t}")
	w("\tlvHeader, err := lv.NewLV([]byte(base64.StdEncoding.EncodeToString(header)))")
	w("\tif err != nil {")
	w("\t\treturn nil, err")
	w("\t}")
	w("\tlvBody, err := lv.NewLV([]byte(base64.StdEncoding.EncodeToString(payload)))")
	w("\tif err != nil {")
	w("\t\treturn nil, err")
	w("\t}")
	w("\tdata, err := datablock.PackLVPayload(c.preKey.index, c.preKey.signingKey, c.preKey.sharedKey1, lvHeader, lvBody)")
	w("\tif err != nil {")
	w("\t\treturn nil, err")
	w("\t}")
	w("\treturn map[string]any{\"i\": c.preKey.index, \"d\": data}, nil")
	w("}")
	w("")
	w("// open reads an answer out of the envelope: the status, the headers and the")
	w("// body dxlib sealed under the second shared key and its signature. An answer")
	w("// with no envelope, which dxlib sends plain when it does not accept the")
	w("// handshake, is returned as it came, with the status the service gave.")
	w("func (c *Client) open(status int, answer map[string]any) (int, map[string]any, error) {")
	w("\tdata, ok := answer[\"d\"].(string)")
	w("\tif !ok {")
	w("\t\treturn status, answer, nil")
	w("\t}")
	w("\telements, err := datablock.UnpackLVPayload(c.preKey.index, c.preKey.serviceSigningKey, c.preKey.sharedKey2, data)")
	w("\tif err != nil {")
	w("\t\treturn status, nil, fmt.Errorf(\"the answer's envelope does not open: %%w\", err)")
	w("\t}")
	w("\tif len(elements) < 3 {")
	w("\t\treturn status, nil, fmt.Errorf(\"the answer's envelope holds %%d elements, not a status, a header and a body\", len(elements))")
	w("\t}")
	w("\tstatusBytes, err := base64.StdEncoding.DecodeString(string(elements[0].Value))")
	w("\tif err != nil || len(statusBytes) != 8 {")
	w("\t\treturn status, nil, errors.New(\"the answer's envelope carries no status\")")
	w("\t}")
	w("\tbodyBytes, err := base64.StdEncoding.DecodeString(string(elements[2].Value))")
	w("\tif err != nil {")
	w("\t\treturn status, nil, fmt.Errorf(\"the answer's body is not base64: %%w\", err)")
	w("\t}")
	w("\tbody := map[string]any{}")
	w("\tif len(bytes.TrimSpace(bodyBytes)) > 0 {")
	w("\t\tif err := json.Unmarshal(bodyBytes, &body); err != nil {")
	w("\t\t\treturn status, nil, fmt.Errorf(\"the answer's body is not JSON: %%w\", err)")
	w("\t\t}")
	w("\t}")
	w("\treturn int(binary.BigEndian.Uint64(statusBytes)), body, nil")
	w("}")
	w("")
	w("// callEncrypted posts a body in the envelope and opens the answer.")
	w("func (c *Client) callEncrypted(ctx context.Context, operationId string, body map[string]any) (int, map[string]any, error) {")
	w("\tif body == nil {")
	w("\t\tbody = map[string]any{}")
	w("\t}")
	w("\tenvelope, err := c.seal(body)")
	w("\tif err != nil {")
	w("\t\treturn 0, nil, err")
	w("\t}")
	w("\tstatus, answer, err := c.call(ctx, operationId, envelope)")
	w("\tif err != nil {")
	w("\t\treturn status, nil, err")
	w("\t}")
	w("\treturn c.open(status, answer)")
	w("}")
	w("")
}
