package extract

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/ooxml"
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
