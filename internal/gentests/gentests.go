// Package gentests writes the tests of a specification's design tests and
// worked examples, in Go, Swift or Dart: the logic of the
// specarch-gen-tests-go, -swift and -dart plug-ins. Every stack reads the
// specification into the same Suite and differs only in how it writes it.
// The tests call a Harness the project writes, so what is stack-specific
// (how a caller signs in, how a record is stored, how a request is sent)
// stays in the project and the generated file stays the same on every
// project of that language.
package gentests

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/SpecArch/specarch/internal/mark"
)

// Request is what specarch writes on a plug-in's standard input.
type Request struct {
	Specarch        string           `json:"specarch"`
	Target          string           `json:"target"`
	Root            string           `json:"root"`
	Specification   map[string]any   `json:"specification"`
	Implementations []Implementation `json:"implementations"`
	Output          string           `json:"output"`
	// Draft is the notice a draft carries, or "" for the approved output.
	Draft string `json:"draft"`
	// Problems are the problems to mark, in the order of the problems file.
	Problems []mark.Problem `json:"problems"`
}

// Implementation is one implementation file in the request.
type Implementation struct {
	File     string         `json:"file"`
	Content  map[string]any `json:"content"`
	Settings map[string]any `json:"settings,omitempty"`
}

// File is one file the plug-in answers with.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Diagnostic is one problem the plug-in reports, in the validator's fields.
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

// Response is what the plug-in writes on its standard output.
type Response struct {
	Files       []File       `json:"files"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Decode reads a request, keeping numbers as their text.
func Decode(data []byte) (*Request, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	var r Request
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// relRoot is the root file's path as seen from the output folder, so the
// header reads the same wherever specarch was run from.
// The plug-in runs in specarch's working folder, so both resolve from it.
func relRoot(root, output string) string {
	absRoot, err1 := filepath.Abs(filepath.FromSlash(root))
	absOut, err2 := filepath.Abs(filepath.FromSlash(output))
	if err1 != nil || err2 != nil {
		return root
	}
	rel, err := filepath.Rel(absOut, absRoot)
	if err != nil {
		return root
	}
	return filepath.ToSlash(rel)
}

func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		return strconv.FormatBool(x)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func escape(token string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(token)
}

// pascal turns a test name such as lend-a-copy into LendACopy.
func pascal(s string) string {
	var b strings.Builder
	up := true
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			up = true
			continue
		}
		if up {
			r = unicode.ToUpper(r)
			up = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// writer collects the lines of a generated file.
type writer struct{ b strings.Builder }

func (w *writer) line(format string, args ...any) { fmt.Fprintf(&w.b, format+"\n", args...) }

// marks writes the problems placed at an element as comment lines.
func (w *writer) marks(s Suite, ptr, indent string) {
	for _, m := range s.Marks[ptr] {
		w.line("%s// %s", indent, m)
	}
}

// head writes the draft notice and the marks of the file as a whole under
// a header.
func (w *writer) head(s Suite, marks bool) {
	if s.Draft != "" {
		w.line("// %s", s.Draft)
	}
	if marks {
		w.marks(s, mark.File, "")
	}
}

// failed is the answer of a plug-in that could not write its file.
func failed(r *Request, message string) Response {
	return Response{Files: []File{}, Diagnostics: []Diagnostic{{File: r.Root, Line: 1, Severity: "error", Path: "/", Rule: "generator", Message: message}}}
}
