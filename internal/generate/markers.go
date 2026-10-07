package generate

import (
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// MarkerError is a problem with the markers of a hand-written document.
type MarkerError struct {
	Line    int
	Message string
}

var (
	beginMarker = regexp.MustCompile(`^\s*<!--\s*specarch:generate\s+(.+?)\s*-->\s*$`)
	endMarker   = regexp.MustCompile(`^\s*<!--\s*specarch:end\s*-->\s*$`)
)

// RewriteMarkers replaces the text between each pair of markers with what
// the begin marker names, drawn from the design file, and leaves every
// other line as it is. It follows the markersWellFormed algorithm: regions
// never nest, every begin has an end, and a marker for an object that does
// not exist is an error, never an empty block.
func RewriteMarkers(text string, design *yaml.Node) (string, []MarkerError) {
	lines := strings.SplitAfter(text, "\n")
	var out strings.Builder
	var errs []MarkerError
	openLine, openWhat := 0, ""
	for i, line := range lines {
		n := i + 1
		trimmed := strings.TrimRight(line, "\n")
		if m := beginMarker.FindStringSubmatch(trimmed); m != nil {
			if openLine != 0 {
				errs = append(errs, MarkerError{n, fmt.Sprintf("a specarch:generate marker inside the region opened on line %d; close that region with <!-- specarch:end --> first", openLine)})
				continue
			}
			openLine, openWhat = n, m[1]
			out.WriteString(line)
			continue
		}
		if endMarker.MatchString(trimmed) {
			if openLine == 0 {
				errs = append(errs, MarkerError{n, "a specarch:end marker with no specarch:generate before it; remove it or add the begin marker"})
				out.WriteString(line)
				continue
			}
			body, err := region(design, openWhat)
			if err != nil {
				errs = append(errs, MarkerError{openLine, err.Error() + "; correct the marker or the design file"})
			}
			out.WriteString(body)
			out.WriteString(line)
			openLine = 0
			continue
		}
		if openLine == 0 {
			out.WriteString(line)
		}
	}
	if openLine != 0 {
		errs = append(errs, MarkerError{openLine, "this specarch:generate marker has no <!-- specarch:end --> after it; add one"})
	}
	return out.String(), errs
}
