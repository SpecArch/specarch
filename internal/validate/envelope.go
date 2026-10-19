package validate

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The keywords of an operation's transport: the handshake whose envelope
// it travels in (ADR-145) and the stream it carries a file as (ADR-146).

// streamMethods are the methods whose request carries a body in dxlib and
// in HTTP's own terms, so an upload stream sits on one of them.
var streamMethods = map[string]bool{"post": true, "put": true}

// checkHandshakes checks each declared handshake: the operation that
// establishes it exists and travels in no envelope itself.
func (c *checker) checkHandshakes(d *design) {
	for _, h := range source.Pairs(source.Child(d.root, "handshakes")) {
		by := source.Child(h.Value, "establishedBy")
		if by == nil {
			continue
		}
		ptr := source.Pointer("handshakes", h.Key.Value, "establishedBy")
		o, ok := d.operations[by.Value]
		if !ok {
			c.add(by, ptr, RuleHandshake, "%s is not an operationId of the specification; name the operation a client calls to establish the handshake%s", by.Value, suggestOperations(by.Value, d))
			continue
		}
		if in := source.Child(o.node, "handshake"); in != nil {
			c.add(by, ptr, RuleHandshake, "%s travels in the envelope of handshake %s, and the operation that establishes a handshake is called before any envelope exists; give it no handshake", by.Value, in.Value)
		}
	}
}

// checkHandshake checks that an operation's handshake is declared and that
// the operation carries no upload, whose body is the file and not a JSON
// body the envelope wraps.
func (c *checker) checkHandshake(d *design, o operation) {
	n := source.Child(o.node, "handshake")
	if n == nil {
		return
	}
	ptr := o.pointer("handshake")
	if d.handshakes[n.Value] == nil {
		c.add(n, ptr, RuleHandshake, "%s is not a handshake of the specification%s", n.Value, suggest(n.Value, d.handshakes))
	}
	if source.Str(source.Child(source.Child(o.node, "stream"), "direction")) == "upload" {
		c.add(n, ptr, RuleHandshake, "%s streams an upload, whose request body is the file itself, and the envelope of %s carries a JSON body; an upload cannot travel in it", o.id, n.Value)
	}
}

// checkStream checks an operation's stream against the body it names: an
// upload on a post or a put whose request body is one binary media type, a
// download whose success responses each carry one binary media type, and
// maxResponseBytes only beside a download.
func (c *checker) checkStream(o operation) {
	s := source.Child(o.node, "stream")
	dir := source.Str(source.Child(s, "direction"))
	if n := source.Child(source.Child(o.node, "limits"), "maxResponseBytes"); n != nil && dir != "download" {
		c.add(n, o.pointer("limits", "maxResponseBytes"), RuleStream, "maxResponseBytes bounds the file a download streams, and %s streams none; remove it, or give the operation stream: {direction: download}", o.id)
	}
	if s == nil {
		return
	}
	ptr := o.pointer("stream")
	switch dir {
	case "upload":
		if !streamMethods[o.method] {
			c.add(source.Child(s, "direction"), ptr+"/direction", RuleStream, "%s streams an upload on %s, and a stream is sent in the body of a post or a put", o.id, o.method)
		}
		body := source.Child(o.node, "requestBody")
		if body == nil {
			c.add(s, ptr, RuleStream, "%s streams an upload and declares no request body; declare one with the file's media type and a schema of type string and format binary", o.id)
			return
		}
		if what := notOneBinary(source.Child(body, "content")); what != "" {
			c.add(s, ptr, RuleStream, "an upload stream's request body is one media type whose schema is a string of format binary, and %s declares %s", o.id, what)
		}
	case "download":
		found := false
		for _, r := range source.Pairs(source.Child(o.node, "responses")) {
			if !strings.HasPrefix(r.Key.Value, "2") {
				continue
			}
			found = true
			if what := notOneBinary(source.Child(r.Value, "content")); what != "" {
				c.add(s, ptr, RuleStream, "a download stream's success response carries one media type whose schema is a string of format binary, and the %s response of %s declares %s", r.Key.Value, o.id, what)
			}
		}
		if !found {
			c.add(s, ptr, RuleStream, "%s streams a download and declares no success response to carry it; declare a 2xx response with the file's media type and a schema of type string and format binary", o.id)
		}
	}
}

// notOneBinary says what a content map declares when it is not one media
// type whose schema is a string of format binary, or "" when it is.
func notOneBinary(content *yaml.Node) string {
	pairs := source.Pairs(content)
	switch {
	case len(pairs) == 0:
		return "no content"
	case len(pairs) > 1:
		var types []string
		for _, p := range pairs {
			types = append(types, p.Key.Value)
		}
		return fmt.Sprintf("%d media types (%s)", len(pairs), strings.Join(types, ", "))
	}
	schema := source.Child(pairs[0].Value, "schema")
	t, f := source.Str(source.Child(schema, "type")), source.Str(source.Child(schema, "format"))
	if t == "string" && f == "binary" {
		return ""
	}
	if schema == nil {
		return pairs[0].Key.Value + " with no schema"
	}
	if ref := source.Str(source.Child(schema, "$ref")); ref != "" {
		return pairs[0].Key.Value + " with a reference to " + ref
	}
	if t == "" {
		t = "no type"
	}
	if f == "" {
		return pairs[0].Key.Value + " with " + t + " and no format"
	}
	return pairs[0].Key.Value + " with " + t + " of format " + f
}

// suggestOperations is the nearest operationId, as suggest names a
// nearest key.
func suggestOperations(name string, d *design) string {
	valid := map[string]*yaml.Node{}
	for id, o := range d.operations {
		valid[id] = o.node
	}
	return suggest(name, valid)
}
