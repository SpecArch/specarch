package source

import (
	"bytes"
	"strings"
)

// MarkPrefix starts every line SpecArch writes into a YAML file to mark a
// problem at the entry below it (ADR-065, ADR-066). A line that starts with
// it after its indentation is SpecArch's own; no other line is.
const MarkPrefix = "# specarch-problem:"

// IsMark tells whether a line of a file is a problem mark.
func IsMark(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(strings.TrimSuffix(line, "\r"), " \t"), MarkPrefix)
}

// StripMarks is a file's text without its problem marks: what the author
// wrote, which the marks of a run do not change.
func StripMarks(data []byte) []byte {
	if !bytes.Contains(data, []byte(MarkPrefix)) {
		return data
	}
	lines := strings.SplitAfter(string(data), "\n")
	var b strings.Builder
	for _, l := range lines {
		if !IsMark(l) {
			b.WriteString(l)
		}
	}
	return []byte(b.String())
}
