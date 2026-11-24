package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	reader "github.com/ledongthuc/pdf"
)

// TestDiagramsInEveryFormat reads what the design tests
// document-diagrams-drawn-in-pdf and document-html-draws-diagrams wrote
// for one specification with two related entities and a workflow: the
// Markdown carries the entity diagram and the workflow as Mermaid blocks,
// the pdf draws both with their labels as text and prints no Mermaid
// source, and the html draws the entity diagram as SVG and the workflow
// as its BPMN picture, with no Mermaid source either.
func TestDiagramsInEveryFormat(t *testing.T) {
	cases := filepath.Join("..", "..", "spec", "tests")
	md, err := os.ReadFile(filepath.Join(cases, "document-diagrams-drawn-in-pdf", "expected", "docs", "techspec.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"```mermaid\nerDiagram\n", "Member ||--o{ Waiver", "```mermaid\nflowchart", "start([\"askWaiver answers 202\"])"} {
		if !bytes.Contains(md, []byte(want)) {
			t.Errorf("techspec.md does not carry %q as Mermaid", want)
		}
	}

	data, err := os.ReadFile(filepath.Join(cases, "document-diagrams-drawn-in-pdf", "expected", "docs", "techspec.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := reader.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("techspec.pdf does not read as a PDF: %v", err)
	}
	var all strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		text, err := r.Page(i).GetPlainText(nil)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		all.WriteString(text + "\n")
	}
	text := all.String()
	for _, source := range []string{"erDiagram", "flowchart", "sequenceDiagram", "-->", "->>", "||--o{", "participant", "printed as its source"} {
		if strings.Contains(text, source) {
			t.Errorf("techspec.pdf prints the Mermaid source %q; it must draw the diagram", source)
		}
	}
	for _, label := range []string{"askWaiver answers 202", "lead: lead with waivers.approve", "grantWaiver", "memberId", "waivers"} {
		if !strings.Contains(strings.Join(strings.Fields(text), " "), label) {
			t.Errorf("techspec.pdf does not draw the label %q as text", label)
		}
	}
	figures := 0
	var walk func(v reader.Value)
	walk = func(v reader.Value) {
		if v.Kind() == reader.Dict && v.Key("S").Name() == "Figure" {
			figures++
			if v.Key("Alt").Text() == "" {
				t.Errorf("a Figure has no description")
			}
		}
		k := v.Key("K")
		for i := 0; i < k.Len(); i++ {
			walk(k.Index(i))
		}
	}
	walk(r.Trailer().Key("Root").Key("StructTreeRoot"))
	if blocks := bytes.Count(md, []byte("```mermaid")); figures != blocks {
		t.Errorf("techspec.pdf has %d figures for the %d Mermaid blocks of techspec.md", figures, blocks)
	}

	html, err := os.ReadFile(filepath.Join(cases, "document-html-draws-diagrams", "expected", "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<svg class="er"`, `<text class="er-name"`, "Member", "Waiver", `alt="The workflow waiver as BPMN 2.0"`} {
		if !bytes.Contains(html, []byte(want)) {
			t.Errorf("index.html does not draw %q", want)
		}
	}
	for _, source := range []string{"```mermaid", "erDiagram", "flowchart LR", "flowchart TD"} {
		if bytes.Contains(html, []byte(source)) {
			t.Errorf("index.html prints the Mermaid source %q", source)
		}
	}
}
