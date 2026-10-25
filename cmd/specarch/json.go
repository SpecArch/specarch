package main

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// jsonFormat is the version of the objects --json prints.
const jsonFormat = "1.0.0"

// jsonOutput is the one object --json prints on standard output.
type jsonOutput struct {
	Format      string           `json:"specarchOutput"`
	Command     string           `json:"command"`
	ExitStatus  int              `json:"exitStatus"`
	Meaning     string           `json:"meaning,omitempty"`
	Remedy      string           `json:"remedy,omitempty"`
	Diagnostics []jsonDiagnostic `json:"diagnostics"`
	Lines       []string         `json:"lines"`
	Messages    []string         `json:"messages"`
}

// jsonDiagnostic is a problem line, by its fields.
type jsonDiagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	ID       string `json:"id"`
}

// jsonError is the one object --json prints on standard error, on exit 2.
type jsonError struct {
	Format     string   `json:"specarchError"`
	Command    string   `json:"command"`
	ExitStatus int      `json:"exitStatus"`
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	Detail     []string `json:"detail"`
	Remedy     string   `json:"remedy,omitempty"`
}

// problemLine is a problem line as validate.Diagnostic.String writes it.
var problemLine = regexp.MustCompile(`^(.+?):(\d+):(\d+): (error|warning|question|note): (.*?): ([a-z][a-z0-9_]*): (.*) \[([^\]]+)\]$`)

// takeJSON takes --json out of the arguments before any --, and says
// whether it was there. capabilities keeps it too, wherever it was given:
// its --json is its listing.
func takeJSON(args []string) ([]string, bool) {
	found := false
	var rest []string
	for i, a := range args {
		if a == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		if a == "--json" {
			found = true
			continue
		}
		rest = append(rest, a)
	}
	if !found {
		return args, false
	}
	if len(rest) > 0 && rest[0] == "capabilities" {
		rest = append([]string{"capabilities", "--json"}, rest[1:]...)
	}
	return rest, true
}

// commandName is the name of the command the arguments run, as the
// specification names it: idioms diff for idioms diff, help for -h.
func commandName(args []string) string {
	if len(args) == 0 {
		return ""
	}
	switch {
	case args[0] == "-h" || args[0] == "--help":
		return "help"
	case args[0] == "idioms" && len(args) > 1 && args[1] == "diff":
		return "idioms diff"
	}
	return args[0]
}

// runJSON runs a command with its output caught and prints it as one
// object on standard output, and on exit 2 an error object on standard
// error. model's document and capabilities' listing, printed on exit 0,
// are already one JSON object each and are printed as they are.
func runJSON(args []string, stdout, stderr io.Writer) int {
	var out, errText bytes.Buffer
	status := dispatch(args, &out, &errText)
	name := commandName(args)
	if (name == "model" || name == "capabilities") && status == 0 && json.Valid(out.Bytes()) {
		stdout.Write(out.Bytes())
		return status
	}
	meaning, remedy := exitMeaning(name, status)
	o := jsonOutput{Format: jsonFormat, Command: name, ExitStatus: status, Meaning: meaning, Remedy: remedy, Diagnostics: []jsonDiagnostic{}, Lines: []string{}, Messages: []string{}}
	for _, line := range lines(out.String()) {
		if m := problemLine.FindStringSubmatch(line); m != nil {
			l, _ := strconv.Atoi(m[2])
			c, _ := strconv.Atoi(m[3])
			o.Diagnostics = append(o.Diagnostics, jsonDiagnostic{File: m[1], Line: l, Column: c, Severity: m[4], Path: m[5], Rule: m[6], Message: m[7], ID: m[8]})
			continue
		}
		o.Lines = append(o.Lines, line)
	}
	messages := withoutUsage(lines(errText.String()))
	if status != 2 {
		o.Messages = append(o.Messages, messages...)
	}
	writeJSON(stdout, o)
	if status == 2 {
		e := jsonError{Format: jsonFormat, Command: name, ExitStatus: status, Code: errorCode(errText.String()), Detail: []string{}, Remedy: remedy}
		if len(messages) > 0 {
			e.Message = messages[0]
			e.Detail = append(e.Detail, messages[1:]...)
		}
		if e.Remedy == "" {
			e.Remedy = "Run specarch help for the commands and their arguments, correct the command line, and run it again."
		}
		writeJSON(stderr, e)
	}
	return status
}

// exitMeaning is what an exit status means for a command, and the remedy,
// from its definition.
func exitMeaning(name string, status int) (string, string) {
	commands, err := ownCommands()
	if err != nil {
		return "", ""
	}
	for _, c := range commands {
		if c.Name != name {
			continue
		}
		for _, e := range c.ExitCodes {
			if e.Status == status {
				return e.Meaning, e.Remedy
			}
		}
	}
	return "", ""
}

// errorCode is the stable code of a failure, from what the command
// printed: a usage error ends with the usage text.
func errorCode(text string) string {
	switch {
	case strings.Contains(text, "\nusage:\n") || strings.HasPrefix(text, "usage:\n") || strings.Contains(text, "has no topic") || strings.Contains(text, "takes no argument"):
		return "usage"
	case strings.Contains(text, "plug-in"):
		return "plugin"
	case strings.Contains(text, "cannot write"):
		return "write"
	case strings.Contains(text, "cannot read"):
		return "read"
	}
	return "failed"
}

// withoutUsage drops the usage text a usage error prints after its reason.
func withoutUsage(ls []string) []string {
	for i, l := range ls {
		if l == "usage:" {
			ls = ls[:i]
			break
		}
	}
	for len(ls) > 0 && ls[len(ls)-1] == "" {
		ls = ls[:len(ls)-1]
	}
	return ls
}

func lines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func writeJSON(w io.Writer, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		data = []byte(`{"specarchError":"` + jsonFormat + `","code":"failed","message":"cannot encode the output as JSON"}`)
	}
	w.Write(append(data, '\n'))
}
