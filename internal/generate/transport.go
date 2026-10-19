package generate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The transport of an operation in the technical specification (ADR-145,
// ADR-146): the handshakes a client performs, what each operation travels
// in, and the files it streams in place of a body read whole.

// handshakesSection writes the handshakes of the design: a table, and for
// each what a client does, in order, so a client on any stack reads it.
func handshakesSection(d *doc, root *yaml.Node) {
	hs := pairs(root, "handshakes")
	if len(hs) == 0 {
		return
	}
	d.heading(3, "Handshakes")
	d.para("Before it calls an operation that names one, a client performs the handshake; its requests on that operation and the answers then travel in an envelope encrypted with the cipher under a key both sides derived and signed by the sender, so that nothing between them reads or alters a body. The bytes of the envelope are the implementation's.")
	d.line("| Handshake | Key agreement | Signature | Cipher | Established by | Lifetime | Envelopes per handshake |")
	d.line("|---|---|---|---|---|---|---|")
	for _, h := range hs {
		lifetime := str(h.Value, "lifetime")
		if lifetime == "" {
			lifetime = "does not expire by time"
		}
		uses := str(h.Value, "uses")
		if uses == "" {
			uses = "any number"
		}
		d.line("| %s | %s | %s | %s | %s | %s | %s |", h.Key.Value, str(h.Value, "keyAgreement"), str(h.Value, "signature"), str(h.Value, "cipher"), cell(establishedByText(root, str(h.Value, "establishedBy"))), lifetime, uses)
	}
	d.blank()
	d.explainRows(rowsOf(hs))
	for _, h := range hs {
		d.para(str(h.Value, "description"))
		d.para(fmt.Sprintf("What a client does for %s:", h.Key.Value))
		by := establishedByText(root, str(h.Value, "establishedBy"))
		d.line("1. It makes a signing key pair (%s) and two key-agreement pairs (%s).", str(h.Value, "signature"), str(h.Value, "keyAgreement"))
		d.line("2. It calls %s with its three public keys and receives the service's three public keys and the handshake's index.", by)
		d.line("3. It derives two shared secrets by %s, one from each agreement pair: the first seals its requests, the second opens the answers.", str(h.Value, "keyAgreement"))
		d.line("4. For each operation under the handshake it encrypts the request body with %s under the first secret, signs the result with its %s key and sends it with the index; it opens each answer under the second secret and verifies the service's signature.", str(h.Value, "cipher"), str(h.Value, "signature"))
		renew := []string{}
		if l := str(h.Value, "lifetime"); l != "" {
			renew = append(renew, "after "+l)
		}
		if u := str(h.Value, "uses"); u != "" {
			envelopes := "envelopes"
			if u == "1" {
				envelopes = "envelope"
			}
			renew = append(renew, fmt.Sprintf("after it has carried %s %s", u, envelopes))
		}
		if len(renew) > 0 {
			d.line("5. %s, the handshake is no longer accepted, and the client performs it again before its next call.", capitalise(strings.Join(renew, ", or ")))
		} else {
			d.line("5. The handshake is accepted until the service refuses it, when the client performs it again before its next call.")
		}
		d.blank()
	}
}

// establishedByText names the operation that establishes a handshake with
// its method and path, or the id alone when it is not declared.
func establishedByText(root *yaml.Node, id string) string {
	for _, o := range operations(root) {
		if o.id == id {
			return fmt.Sprintf("%s (%s %s)", id, strings.ToUpper(o.method), o.path)
		}
	}
	return id
}

// streamDirection is an operation's stream direction, "" for none.
func streamDirection(op *yaml.Node) string {
	return str(get(op, "stream"), "direction")
}

// streamMediaType is the media type of the body an operation streams: the
// request body's for an upload, the first success response's for a download.
func streamMediaType(op *yaml.Node) string {
	switch streamDirection(op) {
	case "upload":
		for _, c := range pairs(get(op, "requestBody"), "content") {
			return c.Key.Value
		}
	case "download":
		for _, r := range pairs(op, "responses") {
			if !strings.HasPrefix(r.Key.Value, "2") {
				continue
			}
			for _, c := range pairs(r.Value, "content") {
				return c.Key.Value
			}
		}
	}
	return ""
}

// operationTransport writes what an operation travels in and what it
// streams, before its parameters.
func operationTransport(d *doc, root *yaml.Node, op *yaml.Node) {
	direction := streamDirection(op)
	if h := str(op, "handshake"); h != "" {
		text := fmt.Sprintf("Travels in the envelope of handshake %s, established by %s: the client seals and signs the request below and opens the answer", h, establishedByText(root, str(get(get(root, "handshakes"), h), "establishedBy")))
		if direction == "download" {
			text += "; the file it answers comes back outside the envelope, as a stream"
		}
		d.para(text + " (see Handshakes above).")
	}
	switch direction {
	case "upload":
		text := "Streams the request body in as a file"
		if mt := streamMediaType(op); mt != "" {
			text += ", " + mt
		}
		if n := str(get(op, "limits"), "maxRequestBytes"); n != "" {
			text += ", of at most " + n + " bytes"
		}
		d.para(text + ", read as it arrives and never held whole; the body is the file and carries no fields.")
	case "download":
		text := "Streams the response body out as a file"
		if mt := streamMediaType(op); mt != "" {
			text += ", " + mt
		}
		if n := str(get(op, "limits"), "maxResponseBytes"); n != "" {
			text += ", of at most " + n + " bytes"
		}
		d.para(text + ", written as it is read and never held whole; a refusal is answered as a problem document instead of the file.")
	}
}

// streamBodyText is the body cell of a response that streams the file, or
// "" for a response that does not.
func streamBodyText(op *yaml.Node, code string) string {
	if streamDirection(op) != "download" || !strings.HasPrefix(code, "2") {
		return ""
	}
	text := "the file, streamed"
	if mt := streamMediaType(op); mt != "" {
		text += ", " + mt
	}
	if n := str(get(op, "limits"), "maxResponseBytes"); n != "" {
		text += ", at most " + n + " bytes"
	}
	return text
}
