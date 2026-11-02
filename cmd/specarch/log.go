package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/disk"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
)

// logResult is what specarch log did, for --json: the record written, or
// what is missing for it to be written.
type logResult struct {
	// Record is the path of the record written, "" when nothing was.
	Record string `json:"record"`
	// ID is the record's ID, "" when nothing was written.
	ID string `json:"id"`
	// Missing are the options the record needs that were not given, each
	// with the question to put to the analyst and the choices the
	// specification offers, in the order to ask them.
	Missing []logMissing `json:"missing"`
	// Next is the decide line that records an answer as the decision,
	// "" for a record that is no answer.
	Next string `json:"next"`
}

// logMissing is one option specarch log needs and was not given.
type logMissing struct {
	Option  string   `json:"option"`
	Ask     string   `json:"ask"`
	Choices []string `json:"choices"`
}

// logOptions are the options of specarch log that take one value, in the
// order the record is written in, and logLists the ones that take several.
var (
	logOptions = []string{"direction", "from", "to", "person", "at", "channel", "type", "words", "form", "reply", "recorded-by", "session", "source"}
	logLists   = []string{"evidence", "question", "decision", "element", "change", "defect", "idea"}
)

var (
	logDirections = []string{"in", "out"}
	logChannels   = []string{"meeting", "chat", "email", "call", "document"}
	logTypes      = []string{"answer", "question", "request", "information", "objection"}
	logForms      = []string{"verbatim", "summary"}
)

// runLog records one communication with a stakeholder (ADR-220).
func runLog(args []string, stdout, stderr io.Writer) int {
	return logInto(args, stdout, stderr, nil)
}

// logInto is runLog keeping its result for --json.
func logInto(args []string, stdout, stderr io.Writer, kept **logResult) int {
	values := map[string]string{}
	lists := map[string][]string{}
	w := disk.Real
	var rest []string
	single := map[string]bool{}
	for _, o := range logOptions {
		single[o] = true
	}
	list := map[string]bool{}
	for _, o := range logLists {
		list[o] = true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if a == "--dry-run" {
			w = disk.DryRun(stdout)
			continue
		}
		if !strings.HasPrefix(a, "-") || len(a) == 1 {
			rest = append(rest, a)
			continue
		}
		name, value, inline := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		if !single[name] && !list[name] {
			fmt.Fprintf(stderr, "specarch log has no option %s; its options are --%s, --%s and --dry-run, each but --dry-run with a value\n\n%s", a, strings.Join(logOptions, ", --"), strings.Join(logLists, ", --"), usage)
			return 2
		}
		if !inline {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "specarch log: %s needs a value\n\n%s", a, usage)
				return 2
			}
			value = args[i+1]
			i++
		}
		if single[name] {
			values[name] = value
		} else {
			lists[name] = append(lists[name], value)
		}
	}
	if len(rest) != 1 {
		fmt.Fprintf(stderr, "specarch log needs one folder, the specification's\n\n%s", usage)
		return 2
	}
	folder := rest[0]
	specs, invalid, status := readSpecs([]string{folder}, "log", stdout, stderr)
	if status != 0 {
		return status
	}
	if len(specs) != 1 {
		fmt.Fprintf(stderr, "specarch log: %s holds %d specifications; name the folder of the one the communication is about\n", folder, len(specs))
		return 2
	}
	l := specs[0]
	root := l.spec.Root
	if root == nil {
		fmt.Fprintf(stderr, "specarch log: nothing of %s could be read, so no stakeholder or question can be checked; fix the errors above first\n", l.spec.RootFile)
		return 1
	}
	result := &logResult{Missing: []logMissing{}}
	if kept != nil {
		*kept = result
	}
	stakeholders := sortedPairKeys(source.Child(root, "stakeholders"))
	zone := source.Str(source.Child(root, "timeZone"))
	if zone == "" {
		zone = "UTC, since the specification declares no timeZone"
	}

	// What is missing: each line is a question for the analyst.
	miss := func(option, ask string, choices []string) {
		if choices == nil {
			choices = []string{}
		}
		result.Missing = append(result.Missing, logMissing{Option: option, Ask: ask, Choices: choices})
	}
	if values["direction"] == "" {
		miss("direction", "which way it went: did the stakeholder say it to the project (in), or the project to the stakeholder (out)?", logDirections)
	}
	if values["from"] == "" {
		miss("from", "who said it, by role", stakeholders)
	}
	if values["to"] == "" {
		miss("to", "who it was said to, by role", stakeholders)
	}
	if values["at"] == "" {
		miss("at", fmt.Sprintf("when it was said: the date, the time and the offset of the clock it was read on (the specification's time zone is %s); written as 2026-10-31T10:15:00+07:00", zone), nil)
	}
	if values["channel"] == "" {
		miss("channel", "how it was said", logChannels)
	}
	if values["type"] == "" {
		miss("type", "what it is: an answer to a question of the specification, a question the stakeholder asked, a request for the system to be different, information, or an objection", logTypes)
	}
	if values["words"] == "" {
		miss("words", "what was said, as it was said or in summary", nil)
	}
	if values["form"] == "" {
		miss("form", "whether the words are as they were said or written (verbatim), or a summary", logForms)
	}
	if values["recorded-by"] == "" {
		miss("recorded-by", "who is writing it down, by role: the analyst's own role", stakeholders)
	}
	if values["type"] == "answer" && len(lists["question"]) == 0 {
		miss("question", "which question of the specification it answers, by ID", openQuestionIDs(root))
	}
	if len(result.Missing) > 0 {
		for _, m := range result.Missing {
			line := fmt.Sprintf("missing --%s: ask %s", m.Option, m.Ask)
			if len(m.Choices) > 0 {
				line += "; one of " + strings.Join(m.Choices, ", ")
			}
			fmt.Fprintln(stdout, line)
		}
		fmt.Fprintf(stderr, "specarch log: %s not given; ask the analyst what each line above asks, and run it again with every option; nothing written\n", plural(len(result.Missing), "option"))
		return 1
	}

	// What is given, checked against the specification and the records.
	refuse := func(format string, args ...any) int {
		fmt.Fprintf(stderr, "specarch log: "+format+"; nothing written\n", args...)
		return 1
	}
	if !oneOfValues(logDirections, values["direction"]) {
		return refuse("--direction %s is neither in nor out", values["direction"])
	}
	for _, o := range []string{"from", "to", "recorded-by"} {
		if source.Child(source.Child(root, "stakeholders"), values[o]) == nil {
			return refuse("--%s %s is not a stakeholder of %s; use one of %s", o, values[o], l.spec.Dir, strings.Join(stakeholders, ", "))
		}
	}
	at, err := time.Parse(time.RFC3339, values["at"])
	if err != nil {
		return refuse("--at %s is not an instant with its offset; write it as RFC 3339 does, such as 2026-10-31T10:15:00+07:00, with the date and the offset the analyst confirms", values["at"])
	}
	_ = at
	if !oneOfValues(logChannels, values["channel"]) {
		return refuse("--channel %s is none of %s", values["channel"], strings.Join(logChannels, ", "))
	}
	if !oneOfValues(logTypes, values["type"]) {
		return refuse("--type %s is none of %s", values["type"], strings.Join(logTypes, ", "))
	}
	if !oneOfValues(logForms, values["form"]) {
		return refuse("--form %s is neither verbatim nor summary", values["form"])
	}
	if values["reply"] != "" && values["type"] != "question" {
		return refuse("--reply goes with --type question, the stakeholder's question the reply answers; this record is %s", typeArticle(values["type"]))
	}
	if src := values["source"]; src != "" && source.Child(source.Child(root, "sources"), src) == nil {
		return refuse("--source %s is not a source declared under sources in %s", src, spec.RootFile)
	}
	records := parsedRecords(l.spec)
	if s := values["session"]; s != "" && records.has("session", s) == "" && records.has("test-session", s) == "" {
		return refuse("--session %s is not a session or a test session: there is no records/sessions/%s.yaml or records/test-sessions/%s.yaml", s, s, s)
	}
	decisions := source.Child(root, "decisions")
	for _, q := range lists["question"] {
		if source.Child(source.Child(root, "questions"), q) == nil && !answeredBy(decisions, q) {
			return refuse("--question %s is not a question of the specification: it is not open under questions and no decision answers it", q)
		}
	}
	for _, id := range lists["decision"] {
		if source.Child(decisions, id) == nil {
			return refuse("--decision %s is not a decision of the specification", id)
		}
	}
	for _, ref := range lists["element"] {
		if !resolvesIn(root, ref) {
			return refuse("--element %s does not resolve in the specification; give a requirement ID or a #/ pointer to an element that exists", ref)
		}
	}
	for _, o := range []struct{ option, kind, folder string }{{"change", "change", "changes"}, {"defect", "defect", "defects"}, {"idea", "idea", "ideas"}} {
		for _, id := range lists[o.option] {
			if records.has(o.kind, id) == "" && !inTracker(root, o.kind, id) {
				return refuse("--%s %s is not %s: there is no records/%s/%s.yaml", o.option, id, typeArticle(o.kind), o.folder, id)
			}
		}
	}
	type evidence struct{ path, mediaType, sha string }
	var evidences []evidence
	for _, e := range lists["evidence"] {
		p, mediaType, _ := strings.Cut(e, "=")
		full, err := filepath.Abs(p)
		if err != nil {
			return refuse("--evidence %s cannot be read: %s", p, spec.PlainIOError(err))
		}
		recordsDir, err := filepath.Abs(l.spec.RecordsDir)
		if err != nil {
			return refuse("--evidence %s cannot be placed under %s: %s", p, l.spec.RecordsDir, spec.PlainIOError(err))
		}
		rel, err := filepath.Rel(recordsDir, full)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return refuse("--evidence %s is not under %s; put the file under records/ beside the specification's folder, such as records/evidence/, and name it from there", p, l.spec.RecordsDir)
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return refuse("--evidence %s cannot be read: %s", p, spec.PlainIOError(err))
		}
		if mediaType == "" {
			mediaType = mediaTypeOf(p)
		}
		if mediaType == "" {
			return refuse("--evidence %s has an extension no media type is known for; give it as --evidence %s=<media type>", p, p)
		}
		sum := sha256.Sum256(data)
		evidences = append(evidences, evidence{filepath.ToSlash(rel), mediaType, "sha256:" + hex.EncodeToString(sum[:])})
	}

	// The record.
	id := nextRecordID(records.ids("communication"), "COM")
	recordPath := filepath.Join(l.spec.RecordsDir, "communications", id+".yaml")
	if _, err := os.Stat(recordPath); err == nil {
		fmt.Fprintf(stderr, "specarch log: %s exists although no communication record carries %s; read it, and name it after the record it holds or remove it\n", recordPath, id)
		return 2
	}
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	add := func(key string, n *yaml.Node) { m.Content = append(m.Content, scalarNode(key), n) }
	quoted := func(s string) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s, Style: yaml.DoubleQuotedStyle}
	}
	add("specarchRecord", quoted("1.0.0"))
	add("kind", scalarNode("communication"))
	add("id", scalarNode(id))
	add("direction", scalarNode(values["direction"]))
	add("from", scalarNode(values["from"]))
	add("to", scalarNode(values["to"]))
	if values["person"] != "" {
		add("person", scalarNode(values["person"]))
	}
	add("at", quoted(values["at"]))
	add("channel", scalarNode(values["channel"]))
	add("type", scalarNode(values["type"]))
	add("words", wordsNode(values["form"], values["words"]))
	if values["reply"] != "" {
		add("reply", wordsNode(values["form"], values["reply"]))
	}
	add("recordedBy", scalarNode(values["recorded-by"]))
	if values["session"] != "" {
		add("session", scalarNode(values["session"]))
	}
	if values["source"] != "" {
		add("source", scalarNode(values["source"]))
	}
	if len(evidences) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, e := range evidences {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle, Content: []*yaml.Node{
				scalarNode("path"), scalarNode(e.path), scalarNode("mediaType"), scalarNode(e.mediaType), scalarNode("sha256"), scalarNode(e.sha),
			}})
		}
		add("evidence", seq)
	}
	links := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, o := range []struct{ option, key string }{{"question", "questions"}, {"decision", "decisions"}, {"element", "elements"}, {"change", "changes"}, {"defect", "defects"}, {"idea", "ideas"}} {
		if len(lists[o.option]) == 0 {
			continue
		}
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		seen := map[string]bool{}
		for _, v := range lists[o.option] {
			if seen[v] {
				continue
			}
			seen[v] = true
			n := scalarNode(v)
			if strings.HasPrefix(v, "#/") {
				n.Style = yaml.DoubleQuotedStyle
			}
			seq.Content = append(seq.Content, n)
		}
		links.Content = append(links.Content, scalarNode(o.key), seq)
	}
	if len(links.Content) > 0 {
		add("links", links)
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Written by specarch log from what %s reported. Edit it as any record.\n", values["recorded-by"])
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		fmt.Fprintf(stderr, "specarch log: cannot write the record: %v\n", err)
		return 2
	}
	if err := enc.Close(); err != nil {
		fmt.Fprintf(stderr, "specarch log: cannot write the record: %v\n", err)
		return 2
	}
	if !w.Dry() {
		if err := os.MkdirAll(filepath.Dir(recordPath), 0o755); err != nil {
			fmt.Fprintf(stderr, "specarch log: cannot make %s: %v\n", filepath.Dir(recordPath), err)
			return 2
		}
	}
	if err := w.WriteFile(recordPath, b.Bytes(), 0o644); err != nil {
		fmt.Fprintf(stderr, "specarch log: cannot write %s: %v\n", recordPath, err)
		return 2
	}
	result.Record, result.ID = filepath.ToSlash(recordPath), id
	if values["type"] == "answer" {
		result.Next = fmt.Sprintf("specarch decide %s --by %s --answer %s --record %s %s", lists["question"][0], values["from"], strconv.Quote(values["words"]), id, folder)
	}
	if w.Dry() {
		fmt.Fprint(stdout, b.String())
		fmt.Fprintf(stderr, "specarch log: %s would be written to %s; nothing written (--dry-run)\n", id, result.Record)
	} else {
		fmt.Fprintf(stdout, "%s\n", result.Record)
		fmt.Fprintf(stderr, "specarch log: %s written to %s\n", id, result.Record)
	}
	if result.Next != "" {
		fmt.Fprintf(stdout, "next: %s\n", result.Next)
	}
	if invalid {
		fmt.Fprintln(stderr, "specarch log: the input has errors; they are printed above")
		return 1
	}
	return 0
}

func scalarNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

// wordsNode is a words object: its form and its text, the text a literal
// block when it holds more than one line.
func wordsNode(form, text string) *yaml.Node {
	t := scalarNode(text)
	if strings.Contains(strings.TrimSpace(text), "\n") {
		t.Style = yaml.LiteralStyle
	}
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalarNode("form"), scalarNode(form), scalarNode("text"), t}}
}

func oneOfValues(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func sortedPairKeys(n *yaml.Node) []string {
	var out []string
	for _, p := range source.Pairs(n) {
		out = append(out, p.Key.Value)
	}
	sort.Strings(out)
	return out
}

func openQuestionIDs(root *yaml.Node) []string {
	return sortedPairKeys(source.Child(root, "questions"))
}

// answeredBy reports whether a decision names the question under answers.
func answeredBy(decisions *yaml.Node, id string) bool {
	for _, p := range source.Pairs(decisions) {
		for _, item := range source.Items(source.Child(p.Value, "answers")) {
			if item.Value == id {
				return true
			}
		}
	}
	return false
}

// resolvesIn reports whether a requirement ID or a #/ pointer names
// something of the specification, or a requirement of a declared set.
func resolvesIn(root *yaml.Node, ref string) bool {
	if strings.HasPrefix(ref, "#/") {
		var tokens []string
		for _, t := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			tokens = append(tokens, source.UnescapeToken(t))
		}
		_, ok := source.Resolve(root, tokens)
		return ok
	}
	if source.Child(source.Child(root, "requirements"), ref) != nil {
		return true
	}
	return inTracker(root, "requirement", ref)
}

// inTracker reports whether an ID falls in a declared requirement-set,
// change-set or defect-set by its prefix.
func inTracker(root *yaml.Node, kind, id string) bool {
	prefix, _, _ := strings.Cut(id, "-")
	for _, p := range source.Pairs(source.Child(root, "sources")) {
		if source.Str(source.Child(p.Value, "kind")) == kind+"-set" && source.Str(source.Child(p.Value, "prefix")) == prefix {
			return true
		}
	}
	return false
}

// typeArticle is a type of communication, or a kind of record, with its
// article.
func typeArticle(t string) string {
	switch t {
	case "answer", "information", "objection", "idea":
		return "an " + t
	case "change":
		return "a change request"
	}
	return "a " + t
}

// mediaTypeOf is the media type a file's extension says, "" when none is
// known for it.
func mediaTypeOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".log":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".eml":
		return "message/rfc822"
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".html", ".htm":
		return "text/html"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".xml":
		return "application/xml"
	case ".zip":
		return "application/zip"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".mp4":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".ogg", ".opus":
		return "audio/ogg"
	case ".mov":
		return "video/quicktime"
	}
	return ""
}

// recordIndex is the records beside a specification, parsed, by kind.
type recordIndex map[string][]*yaml.Node

// parsedRecords reads every record beside the specification that parses.
func parsedRecords(s *spec.Spec) recordIndex {
	out := recordIndex{}
	for _, r := range s.Records {
		doc := source.Parse(r.Data)
		if doc.Root == nil || doc.Root.Kind != yaml.MappingNode {
			continue
		}
		kind := source.Str(source.Child(doc.Root, "kind"))
		out[kind] = append(out[kind], doc.Root)
	}
	return out
}

// has is the ID of a record of the kind with that ID, "" when none.
func (ri recordIndex) has(kind, id string) string {
	for _, r := range ri[kind] {
		if source.Str(source.Child(r, "id")) == id {
			return id
		}
	}
	return ""
}

func (ri recordIndex) ids(kind string) []string {
	var out []string
	for _, r := range ri[kind] {
		out = append(out, source.Str(source.Child(r, "id")))
	}
	return out
}

// nextRecordID is the prefix, a dash and the largest number the IDs carry
// after that prefix plus one; 1 for the first.
func nextRecordID(ids []string, prefix string) string {
	largest := 0
	for _, id := range ids {
		rest, ok := strings.CutPrefix(id, prefix+"-")
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(rest); err == nil && n > largest {
			largest = n
		}
	}
	return fmt.Sprintf("%s-%d", prefix, largest+1)
}
