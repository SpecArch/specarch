package extract

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/ooxml"
	"github.com/SpecArch/specarch/internal/vocabulary"
)

func TestWordNumbering(t *testing.T) {
	wn := &wordNumbering{
		abstract: map[string]map[int]wordLevel{
			"0": {0: {start: 1, format: "decimal", text: "%1."}, 1: {start: 1, format: "decimal", text: "%1.%2"}, 2: {start: 1, format: "lowerRoman", text: "(%3)"}},
			"1": {0: {start: 1, format: "upperLetter", text: "Annex %1"}},
			"2": {0: {start: 1, format: "upperRoman", text: "%1"}, 1: {start: 1, format: "lowerLetter", text: "%1.%2", legal: true}},
			"3": {0: {start: 1, format: "ordinalText", text: "%1"}},
		},
		nums:     map[string]string{"1": "0", "2": "0", "3": "1", "4": "0", "5": "2", "6": "3"},
		starts:   map[string]map[int]int{"4": {0: 7}},
		counters: map[string][]int{},
	}
	steps := []struct{ num, lvl, want string }{
		{"1", "0", "1."}, {"1", "1", "1.1"}, {"1", "1", "1.2"}, {"1", "2", "(i)"}, {"1", "2", "(ii)"},
		{"2", "0", "2."}, // another num of the same abstract numbering counts on
		{"1", "1", "2.1"}, {"1", "2", "(i)"},
		{"3", "0", "Annex A"}, {"3", "0", "Annex B"},
		{"4", "0", "7."}, {"4", "1", "7.1"}, // a start override restarts its own list
		{"5", "1", "1.1"}, // a skipped level counts from its start; legal numbering is decimal
		{"0", "0", ""}, {"", "0", ""}, {"9", "0", ""},
	}
	for _, s := range steps {
		if got, _ := wn.next(s.num, s.lvl); got != s.want {
			t.Errorf("next(%s, %s) = %q, want %q", s.num, s.lvl, got, s.want)
		}
	}
	if got, f := wn.next("6", "0"); got != "" || f != "ordinalText" {
		t.Errorf("ordinalText gave %q, %q", got, f)
	}
}

func TestRoman(t *testing.T) {
	for n, want := range map[int]string{1: "I", 4: "IV", 9: "IX", 14: "XIV", 1999: "MCMXCIX", 0: "", 4000: ""} {
		if got := roman(n); got != want {
			t.Errorf("roman(%d) = %s, want %s", n, got, want)
		}
	}
}

// A PDF encrypted with a user password is refused with the reason, not as
// a file that does not parse.
func TestEncryptedPDF(t *testing.T) {
	data, err := os.ReadFile("../../spec/tests/extract-documents-pdf-encrypted/docs/notice.pdf")
	if err != nil {
		t.Fatal(err)
	}
	_, err = readPDF("notice.pdf", data, 0)
	if err == nil || !strings.Contains(err.Error(), "the PDF is encrypted with a password") {
		t.Fatalf("readPDF on an encrypted PDF gave %v, want the password named", err)
	}
}

// A shape in mc:AlternateContent is read once, from mc:Choice: its
// mc:Fallback repeats it for older readers.
func TestSlideAlternateContent(t *testing.T) {
	slide := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><p:cSld><p:spTree>` +
		`<mc:AlternateContent><mc:Choice Requires="a14"><p:sp><p:txBody><a:p><a:r><a:t>A member may borrow ten books.</a:t></a:r></a:p></p:txBody></p:sp></mc:Choice>` +
		`<mc:Fallback><p:sp><p:txBody><a:p><a:r><a:t>A member may borrow ten books.</a:t></a:r></a:p></p:txBody></p:sp></mc:Fallback></mc:AlternateContent>` +
		`<p:sp><p:txBody><a:p><a:r><a:t>A loan lasts three weeks.</a:t></a:r></a:p></p:txBody></p:sp>` +
		`</p:spTree></p:cSld></p:sld>`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(slide)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := readSlide(ooxml.Files(z), "ppt/slides/slide1.xml", false)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, it := range got.items {
		texts = append(texts, it.text)
	}
	want := []string{"A member may borrow ten books.", "A loan lasts three weeks."}
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Fatalf("readSlide read %q, want %q", texts, want)
	}
}

// Two forwarded messages of one subject stay two clauses: the second is
// numbered.
func TestEmailRepeatedSubject(t *testing.T) {
	inner := "From: office@example.org\r\nSubject: Re: Loan limits\r\n\r\nA loan lasts three weeks.\r\n"
	msg := "From: board@example.org\r\nSubject: Fwd: Loan limits\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=b\r\n\r\n" +
		"--b\r\nContent-Type: text/plain\r\n\r\nBoth replies.\r\n" +
		"--b\r\nContent-Type: message/rfc822\r\n\r\n" + inner +
		"--b\r\nContent-Type: message/rfc822\r\n\r\n" + inner +
		"--b--\r\n"
	in, err := readEML("thread.eml", []byte(msg), 0)
	if err != nil {
		t.Fatal(err)
	}
	var sections []string
	for _, b := range in.blocks {
		if b.kind == blockSection && b.level == 1 {
			sections = append(sections, b.id)
		}
	}
	want := []string{"body", `message "Re: Loan limits"`, `message "Re: Loan limits" 2`}
	if strings.Join(sections, "|") != strings.Join(want, "|") {
		t.Fatalf("readEML named the clauses %q, want %q", sections, want)
	}
}

// readTwin reads one fixture of extract documents into its requirements,
// its questions and its clauses, as the reader writes them.
func readTwin(t *testing.T, path string) (requirements, questions map[string]any, clauses []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	format, _ := documentFormat(path)
	in, err := format.read(filepath.Base(path), data, 0)
	if err != nil {
		t.Fatal(err)
	}
	d := &docReader{key: "lending-rules", res: &Result{Tree: newTree()}, questions: &yaml.Node{Kind: yaml.MappingNode}, words: newDocWords(vocabulary.English()), counted: in.counted}
	d.read(in.blocks)
	d.write()
	decode := func(n *yaml.Node) map[string]any {
		out := map[string]any{}
		if n != nil {
			if err := n.Decode(&out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	for _, c := range d.clauses {
		if c.cited && !c.used || c.children > 0 && !c.own {
			continue
		}
		clauses = append(clauses, c.id+" "+c.title)
	}
	return decode(d.requirements), decode(d.questions), clauses
}

// A PDF and its Markdown twin give the same requirements and questions,
// citing the same clauses with the same words; the PDF alone cites the
// pages a Markdown document has none of: page 1, before its first
// heading, and page 2, which holds no text.
func TestPDFMarkdownTwin(t *testing.T) {
	pdfReq, pdfQ, pdfClauses := readTwin(t, "../../spec/tests/extract-documents-pdf/docs/lending-rules.pdf")
	mdReq, mdQ, mdClauses := readTwin(t, "../../spec/tests/extract-documents-pdf-markdown-twin/docs/lending-rules.md")
	if len(pdfReq) != 3 || !reflect.DeepEqual(pdfReq, mdReq) {
		t.Errorf("the requirements differ:\nPDF      %v\nMarkdown %v", pdfReq, mdReq)
	}
	pageOnly := map[string]any{}
	for id, q := range pdfQ {
		if _, ok := mdQ[id]; ok {
			continue
		}
		pageOnly[id] = q
	}
	for id, q := range mdQ {
		if !reflect.DeepEqual(pdfQ[id], q) {
			t.Errorf("question %s differs:\nPDF      %v\nMarkdown %v", id, pdfQ[id], q)
		}
	}
	if len(pageOnly) != 1 || !strings.Contains(fmt.Sprint(pageOnly["Q-3"]), "clause:page 2") {
		t.Errorf("the PDF's own questions are %v, want Q-3 citing page 2", pageOnly)
	}
	var pages, others []string
	for _, c := range pdfClauses {
		if strings.HasPrefix(c, "page ") {
			pages = append(pages, c)
		} else {
			others = append(others, c)
		}
	}
	if !reflect.DeepEqual(others, mdClauses) {
		t.Errorf("the clauses differ:\nPDF      %q\nMarkdown %q", others, mdClauses)
	}
	if want := []string{"page 1 page 1"}; !reflect.DeepEqual(pages, want) {
		t.Errorf("the PDF's page clauses are %q, want %q", pages, want)
	}
}

// A PDF none of whose pages holds text is refused as a scanned document,
// not read as a document that commits nothing.
func TestPDFWithoutTextLayer(t *testing.T) {
	data, err := os.ReadFile("../../spec/tests/extract-documents-pdf-no-text/docs/lending-rules.pdf")
	if err != nil {
		t.Fatal(err)
	}
	in, err := readPDF("lending-rules.pdf", data, 0)
	if err == nil || !strings.Contains(err.Error(), "the PDF has no text layer: none of its 2 pages holds text") {
		t.Fatalf("readPDF on a scanned PDF gave %v, %v; want the missing text layer named", in, err)
	}
}

// A PDF with no text layer attached to an email is a line and a could
// question, as any attachment that is refused, and the message is read.
func TestEmailAttachedPDFWithoutTextLayer(t *testing.T) {
	data, err := os.ReadFile("../../spec/tests/extract-documents-pdf-no-text/docs/lending-rules.pdf")
	if err != nil {
		t.Fatal(err)
	}
	msg := "From: board@example.org\r\nSubject: Lending rules\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=b\r\n\r\n" +
		"--b\r\nContent-Type: text/plain\r\n\r\nA loan lasts three weeks.\r\n" +
		"--b\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=\"lending-rules.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\n" +
		base64.StdEncoding.EncodeToString(data) + "\r\n--b--\r\n"
	in, err := readEML("rules.eml", []byte(msg), 0)
	if err != nil {
		t.Fatal(err)
	}
	var notRead []string
	for _, b := range in.blocks {
		if b.kind == blockNotHeld {
			notRead = append(notRead, b.place+": "+b.text)
		}
	}
	if len(notRead) != 1 || !strings.Contains(notRead[0], "lending-rules.pdf: the attachment lending-rules.pdf cannot be read as a PDF document: the PDF has no text layer") {
		t.Fatalf("readEML set aside %q, want the attached PDF's missing text layer as one line", notRead)
	}
}

// buildPDF writes a PDF of the given objects, numbered from 1, with the
// cross-reference table a reader needs; object 1 is the catalog.
func buildPDF(objects ...string) []byte {
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return []byte(b.String())
}

func pdfPage(content string) []string {
	stream := fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
	return []string{
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		stream,
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	}
}

// A page tree that gives no page to read does not parse: it is refused as
// such, not as a scanned document to be read by OCR.
func TestPDFPageTreeWithoutPages(t *testing.T) {
	for _, c := range []struct {
		name, pages, want string
	}{
		{"no page counted", "<< /Type /Pages /Kids [] /Count 0 >>", "the PDF does not parse: its page tree counts 0 pages"},
		{"no page object", "<< /Type /Pages /Kids [] /Count 2 >>", "the PDF does not parse: its page tree counts 2 pages, but no page object is there for any of them"},
	} {
		in, err := readPDF("lending-rules.pdf", buildPDF("<< /Type /Catalog /Pages 2 0 R >>", c.pages), 0)
		if err == nil || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), "text layer") {
			t.Errorf("%s: readPDF gave %v, %v; want %q", c.name, in, err, c.want)
		}
	}
}

// A page the page tree counts but holds no object for is a line at that
// page; the pages that read are read. When none of those holds text, the
// refusal names the missing text layer and the pages that do not parse.
func TestPDFPageWithoutObject(t *testing.T) {
	catalog, pages := "<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 2 >>"
	text := append([]string{catalog, pages}, pdfPage("BT /F1 12 Tf 72 720 Td (A loan lasts three weeks.) Tj ET")...)
	in, err := readPDF("lending-rules.pdf", buildPDF(text...), 0)
	if err != nil {
		t.Fatal(err)
	}
	var notRead []string
	read := false
	for _, b := range in.blocks {
		if b.kind == blockNotHeld {
			notRead = append(notRead, b.place+": "+b.text)
		}
		if b.kind == blockParagraph && strings.Contains(b.text, "three weeks") {
			read = true
		}
	}
	if !read || len(notRead) != 1 || !strings.HasPrefix(notRead[0], "page 2: page 2 does not parse") {
		t.Fatalf("readPDF read the text: %v; set aside %q; want page 1 read and page 2 named as not parsing", read, notRead)
	}
	blank := append([]string{catalog, pages}, pdfPage("")...)
	_, err = readPDF("lending-rules.pdf", buildPDF(blank...), 0)
	if err == nil || !strings.Contains(err.Error(), "the PDF has no text layer: none of its 1 pages holds text (pages 2 of 2 do not parse") {
		t.Fatalf("readPDF on a blank page and a missing one gave %v; want the text layer and page 2 named", err)
	}
}
