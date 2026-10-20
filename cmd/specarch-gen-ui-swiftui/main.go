// Command specarch-gen-ui-swiftui is the plug-in behind specarch
// generate ui for a ui target of platform iphone in an implementation file
// in Swift: it writes the screens of a specification in SwiftUI, into a
// Swift package. It reads specarch's request on standard input and answers
// with the files on standard output; it never touches the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch/internal/genswiftui"
)

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-swiftui: cannot read the request: %v\n", err)
		return 2
	}
	req, err := genswiftui.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-swiftui: the request is not specarch's JSON (%v); run this plug-in through specarch generate ui\n", err)
		return 2
	}
	out, err := json.Marshal(genswiftui.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-swiftui: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-ui-swiftui: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
