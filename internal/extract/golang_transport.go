package extract

import (
	"fmt"
	"go/ast"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The transport a dxlib endpoint's type gives it (ADR-145, ADR-146): the
// envelope of a handshake, a stream in or out, or a plain JSON body.

// preKeyHandshake is the name the reader gives dxlib's pre-key exchange: an
// endpoint of type EndPointTypeHTTPEndToEndEncryptionV2 travels in its
// envelope, and EndPointTypeHTTPDownloadStreamV2 sends its request in it.
const preKeyHandshake = "preKey"

// preKeyParameters are the parameters dxlib_module's pre-login declares, the
// client's signing key and its two agreement keys; a registration that
// declares exactly these establishes the handshake.
var preKeyParameters = []string{"a0", "a1", "a2"}

// dxTransport is what each endpoint type the reader knows gives an
// operation: its stream direction and whether it travels in the pre-key
// envelope. A type not in the map is asked about.
var dxTransport = map[string]struct {
	stream    string
	handshake bool
}{
	"EndPointTypeHTTPJSON":                 {},
	"EndPointTypeHTTPUploadStream":         {stream: "upload"},
	"EndPointTypeHTTPDownloadStream":       {stream: "download"},
	"EndPointTypeHTTPDownloadStreamV2":     {stream: "download", handshake: true},
	"EndPointTypeHTTPEndToEndEncryptionV2": {handshake: true},
}

// endpointType records the type argument of a registration, by the
// constant's name when it is one.
func (g *goReader) endpointType(ep *goEndpoint, e ast.Expr) {
	ep.typeText = exprText(e)
	if sel, ok := e.(*ast.SelectorExpr); ok {
		ep.typeName = sel.Sel.Name
	}
}

// writeTransport writes an operation's handshake and stream from its
// endpoint type, or asks what the type does not say.
func (g *goReader) writeTransport(ep *goEndpoint, op *yaml.Node, regAt, label string) {
	t, known := dxTransport[ep.typeName]
	switch {
	case known:
	case ep.typeName == "EndPointTypeHTTPEndToEndEncryptionV1":
		g.notHeld = append(g.notHeld, notHeld{text: fmt.Sprintf("%s: NewEndPoint registers %s with EndPointTypeHTTPEndToEndEncryptionV1, which dxlib itself refuses with 422 as not implemented; written as a plain operation", regAt, label), clause: regAt, blocks: []string{ep.at}})
		return
	case ep.typeName == "EndPointTypeHTTPEndToEndEncryptionV3", ep.typeName == "EndPointTypeHTTPEndToEndEncryptionV4":
		version := strings.TrimPrefix(ep.typeName, "EndPointTypeHTTPEndToEndEncryption")
		g.question("must", fmt.Sprintf("%s registers %s with %s, a persistent-session envelope whose key exchange, cipher and session store the service supplies through dxlib's OnE2EE%sUnpack and OnE2EE%sPack hooks. Which handshake does it travel in, with its key agreement, signature, cipher and the operation that establishes it?", regAt, label, ep.typeName, version, version),
			[]string{ep.at + "/handshake"}, "dxlib routes a "+version+" body to the host's hooks and fixes nothing of the envelope, so the reader cannot write the handshake from the type.", g.at(regAt, "Registers "+label+" with "+ep.typeName+"."))
		return
	default:
		g.question("must", fmt.Sprintf("%s registers %s with the endpoint type %s, which is not one of dxlib's constants the reader knows. Does it carry a JSON body, travel in an envelope, or stream a file, and which way?", regAt, label, ep.typeText),
			[]string{ep.at + "/handshake", ep.at + "/stream"}, "The endpoint type decides whether a request is plain JSON, an envelope or a stream, and only a constant of dxlib's names one.", g.at(regAt, "Registers "+label+" with the endpoint type "+ep.typeText+"."))
		return
	}
	if t.handshake {
		ep.handshake = preKeyHandshake
		set(op, "handshake", preKeyHandshake)
		if _, seen := g.handshakeAt[preKeyHandshake]; !seen {
			if g.handshakeAt == nil {
				g.handshakeAt = map[string]string{}
			}
			g.handshakeAt[preKeyHandshake] = regAt
		}
	}
	switch t.stream {
	case "upload":
		if d := ep.decl; d != nil && d.contentOK && d.contentType != "application/octet-stream" {
			ct := "no content type"
			if d.contentType != "" {
				ct = "the content type " + d.contentType
			}
			g.question("must", fmt.Sprintf("%s registers %s as an upload stream with %s, and dxlib hands a handler the body unread, as a stream, only for application/octet-stream. Which media type does the upload take, and is it streamed?", regAt, label, ct),
				[]string{ep.at + "/stream", ep.at + "/requestBody"}, "An upload stream is the request body passed through as it arrives, and dxlib does that for one content type; with another the type and the content type disagree.", g.at(regAt, "Registers "+label+" as an upload stream with "+ct+"."))
			return
		}
		ep.stream = "upload"
		set(op, "stream", flow(mapping("direction", "upload")))
	case "download":
		ep.stream = "download"
		set(op, "stream", flow(mapping("direction", "download")))
	}
}

// writeLimits writes the largest request a registration accepts, its
// requestMaxContentLength: dxlib refuses a request whose declared
// Content-Length is above it, so it is the operation's maxRequestBytes,
// which the dxlib dialect writes back as x-dxlib-max-content-length. Zero
// is no limit; a value that is not an integer literal is asked about.
func (g *goReader) writeLimits(ep *goEndpoint, op *yaml.Node, regAt, label string) {
	if ep.maxContent == nil {
		return
	}
	n, ok := numberLit(ep.maxContent)
	if v, err := strconv.ParseInt(n, 0, 64); ok && err == nil {
		if v > 0 {
			set(op, "limits", flow(mapping("maxRequestBytes", numberNode(strconv.FormatInt(v, 10)))))
		}
		return
	}
	text := exprText(ep.maxContent)
	g.question("should", fmt.Sprintf("%s registers %s with the request's largest content length %s, which is not an integer literal. How large may a request be, in bytes?", regAt, label, text),
		[]string{ep.at + "/limits/maxRequestBytes"}, "dxlib refuses a request whose declared Content-Length is above that value, and only a literal says it by syntax.", g.at(regAt, "Registers "+label+" with the largest content length "+text+"."))
}

// streamsOut says whether a response of the endpoint is the file a
// download stream answers: a success code of a download stream. Such a
// response carries the file's bytes, so a JSON body the registration or the
// handler declares for it is never written, and nothing is asked about it
// but whether it is answered at all.
func (ep *goEndpoint) streamsOut(code string) bool {
	if ep.stream != "download" || !strings.HasPrefix(code, "2") {
		return false
	}
	if ep.templated == nil {
		ep.templated = map[string]bool{}
	}
	return true
}

// streamResponses gives a download stream's success response the file it
// answers: application/octet-stream, as dxlib's own sender sets it, a
// binary string; a data template the registration or the handler declares
// for it was left unwritten, and is a question.
func (g *goReader) streamResponses(ep *goEndpoint, declared *yaml.Node, codes *[]string, label, regAt string) {
	if ep.stream != "download" {
		return
	}
	var success []string
	for _, c := range *codes {
		if strings.HasPrefix(c, "2") {
			success = append(success, c)
		}
	}
	if len(success) == 0 {
		success = []string{"200"}
		*codes = append(*codes, "200")
	}
	for _, c := range success {
		resp := child(declared, c)
		if resp == nil {
			resp = mapping()
			set(declared, c, resp)
		}
		if ep.templated[c] {
			g.question("should", fmt.Sprintf("%s registers %s as a download stream, whose %s response is the file's bytes, and its registration or handler gives that response a JSON body. Which does it answer?", regAt, label, c),
				[]string{ep.at + "/responses/" + c + "/content"}, "A download stream writes the file to the response as dxlib's sender does, with no JSON body, so a body declared for it is what the running system does not send.", g.at(regAt, "Registers "+label+" as a download stream with a body on its "+c+" response."))
		}
		set(resp, "content", mapping("application/octet-stream", mapping("schema", flow(mapping("type", "string", "format", "binary")))))
	}
}

// transportSays is what a registration says of its transport, for the
// operation's citation; "" for a plain JSON endpoint.
func transportSays(ep *goEndpoint) string {
	var parts []string
	if ep.handshake != "" {
		if ep.stream == "download" {
			parts = append(parts, "its request travels in the pre-key envelope")
		} else {
			parts = append(parts, "it travels in the pre-key envelope")
		}
	}
	switch ep.stream {
	case "upload":
		parts = append(parts, "it streams its request body as a file")
	case "download":
		parts = append(parts, "it streams its response body as a file")
	}
	if len(parts) == 0 {
		return ""
	}
	return "; " + strings.Join(parts, ", and ")
}

// writeHandshakes writes the handshakes the endpoints travel in: dxlib's
// pre-key exchange, whose primitives its type fixes, established by the
// registration that declares the exchange's parameters, or asked for.
func (g *goReader) writeHandshakes() {
	regAt, used := g.handshakeAt[preKeyHandshake]
	if !used {
		return
	}
	at := "#/handshakes/" + preKeyHandshake
	h := mapping("description", "dxlib's pre-key exchange: the client sends its Ed25519 signing key and two X25519 agreement keys, receives the service's three and the pre-key's index, and both derive one shared key for requests and one for answers; every envelope after names the index.")
	set(h, "keyAgreement", "x25519")
	set(h, "signature", "ed25519")
	set(h, "cipher", "aes-256-cbc")
	var establishing []*goEndpoint
	for _, ep := range g.endpoints {
		if ep.id == "" || ep.handshake != "" || ep.decl == nil || ep.decl.paramsWhy != "" || len(ep.decl.params) != len(preKeyParameters) {
			continue
		}
		var names []string
		for _, p := range ep.decl.params {
			names = append(names, p.name)
		}
		sort.Strings(names)
		if strings.Join(names, ",") == strings.Join(preKeyParameters, ",") {
			establishing = append(establishing, ep)
		}
	}
	cites := []*yaml.Node{g.at(regAt, "Registers an endpoint with dxlib's pre-key envelope, EndPointTypeHTTPEndToEndEncryptionV2 or EndPointTypeHTTPDownloadStreamV2, whose type fixes X25519, Ed25519 and AES-256-CBC.")}
	switch len(establishing) {
	case 1:
		ep := establishing[0]
		set(h, "establishedBy", ep.id)
		cites = append(cites, g.at(fmt.Sprintf("%s:%d", ep.file.path, ep.line), fmt.Sprintf("Registers %s %s with the parameters %s, the client's keys of the pre-key exchange, so it establishes the handshake.", ep.method, ep.uri, joinAnd(preKeyParameters))))
	case 0:
		g.question("must", fmt.Sprintf("Which operation establishes the handshake %s? Its envelope is opened with the keys a pre-key exchange stored, and no registration of the module declares the exchange's parameters %s.", preKeyHandshake, joinAnd(preKeyParameters)),
			[]string{at + "/establishedBy"}, "A handshake names the operation a client calls to establish it, and the reader finds that operation only by the parameters dxlib_module's pre-login declares.", g.at(regAt, "Registers an endpoint with dxlib's pre-key envelope."))
	default:
		var labels []string
		for _, ep := range establishing {
			labels = append(labels, ep.method+" "+ep.uri)
		}
		g.question("must", fmt.Sprintf("Which operation establishes the handshake %s? %s each declare the exchange's parameters %s, and a handshake names one.", preKeyHandshake, joinAnd(labels), joinAnd(preKeyParameters)),
			[]string{at + "/establishedBy"}, "A handshake is established by one operation, and two registrations declare the exchange's parameters.", g.at(regAt, "Registers an endpoint with dxlib's pre-key envelope."))
	}
	g.question("should", fmt.Sprintf("How long is a pre-key of handshake %s accepted, and how many envelopes does one carry? The lifetime is the service's configuration (sessions.prekey_ttl_in_seconds in dxlib_module), and whether a pre-key is deleted after its first envelope is the service's own code.", preKeyHandshake),
		[]string{at + "/lifetime", at + "/uses"}, "dxlib's type fixes the exchange and the cipher, not how long a pre-key lives or how often it is used, and a client must know both.", g.at(regAt, "Registers an endpoint with dxlib's pre-key envelope."))
	set(h, "origin", "inferred")
	set(h, "why", "The endpoint type EndPointTypeHTTPEndToEndEncryptionV2 is dxlib's pre-key envelope, packed by its datablock with X25519 shared keys, an Ed25519 signature and AES-256-CBC; the type fixes the primitives, and the registration that declares the exchange's parameters establishes it.")
	set(h, "cites", cites)
	g.handshakes = mapping(preKeyHandshake, h)
}
