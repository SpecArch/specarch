package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/vocabulary"
)

// A test script read: what it says before the test runs, the steps it
// takes and the result it expects, with where each test is in its file
// (ADR-235). Every part is kept as the script words it; a part the script
// leaves out is a question, never a guess.
type scriptTest struct {
	title    string
	heading  string // the scenario's or the row's title, for its clause
	file     *scriptFile
	clause   string // where the test is: line 12, or A5:F5 in a sheet
	given    []string
	when     []string
	then     []string
	subject  string // what the script names as the thing exercised, or ""
	scenario string // golden or red, when the script says which; "" otherwise
	says     string // the test as the script writes it, on one line
	// unfilled is, for given, when and then, each Outline placeholder its
	// Examples row gives no value for.
	unfilled map[string][]gap
}

// gap is a placeholder an Examples row gives no value for, and why.
type gap struct{ name, why string }

type scriptFile struct {
	Name  string // the file's name
	Rel   string // from the repository's root
	Key   string // the source key
	Kind  string // feature or sheet
	Title string
}

const testFormats = "Gherkin feature files (.feature) and test sheets in CSV (.csv), TSV (.tsv, .tab) or Excel (.xlsx)"

// Tests reads existing test scripts and test sheets, complete or not, into
// a tree of tests: one test per scenario or row, each inferred from the
// script, and one must question for each part the script leaves out, its
// given, its step or its expected result, and for the subject it
// exercises, which only the specification it is merged with can name.
func Tests(paths []string, out, key string) (*Result, error) {
	for _, p := range paths {
		if !strings.EqualFold(filepath.Ext(p), ".feature") {
			if _, _, ok := tabFormat(p); !ok {
				return nil, refuse("%s is not a test script this build reads: it reads %s, by the file's extension; save the script in one of these formats and read that file", p, testFormats)
			}
		}
	}
	r, err := Open(paths)
	if err != nil {
		return nil, err
	}
	res := &Result{Tree: newTree()}
	commitLine(res, r)
	keys := map[string]string{}
	var files []*scriptFile
	var tests []*scriptTest
	for _, p := range paths {
		abs, err := absolute(p)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(r.Repository.Root, abs)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		f := &scriptFile{Name: filepath.Base(p), Rel: filepath.ToSlash(rel), Title: filepath.Base(p)}
		f.Key = kebab(strings.TrimSuffix(f.Name, filepath.Ext(f.Name)))
		if key != "" && len(paths) == 1 {
			f.Key = key
		}
		if !sourceKeyForm.MatchString(f.Key) {
			return nil, refuse("the file name %s gives no source key; name one with --source-key", f.Name)
		}
		if other, ok := keys[f.Key]; ok {
			return nil, refuse("%s and %s both give the source key %s; one source is written per file, so rename one of them", other, p, f.Key)
		}
		keys[f.Key] = p
		var read []*scriptTest
		if strings.EqualFold(filepath.Ext(p), ".feature") {
			f.Kind = "feature"
			read, err = readFeature(f, string(data))
		} else {
			f.Kind = "sheet"
			read, err = readTestSheet(f, p, data, res)
		}
		if err != nil {
			return nil, err
		}
		res.say("read %s: %s", f.Name, plural(len(read), "test"))
		files = append(files, f)
		tests = append(tests, read...)
	}
	if len(tests) == 0 {
		return nil, refuse("no test was read from %s: a feature file holds Scenario or Example blocks, and a test sheet a header row naming at least one of its columns as %s", strings.Join(paths, ", "), sheetColumnWords())
	}
	writeTests(res, out, r, files, tests)
	return res, nil
}

// writeTests puts the tests, the questions and the sources into the tree.
func writeTests(res *Result, out string, r *Read, files []*scriptFile, tests []*scriptTest) {
	questions := &yaml.Node{Kind: yaml.MappingNode}
	next := 0
	ask := func(text string, blocks string, why string, cite *yaml.Node) {
		next++
		set(questions, fmt.Sprintf("Q-%d", next), mapping(
			"question", text,
			"kind", "decision",
			"priority", "must",
			"blocks", []string{blocks},
			"decidedBy", owner,
			"why", why,
			"cites", []*yaml.Node{cite},
		))
	}
	taken := map[string]bool{}
	cited := map[string][]*yaml.Node{}
	seenClause := map[string]bool{}
	missing := map[string]int{}
	for _, t := range tests {
		// A name taken by an earlier test, even one renamed to it, gets
		// the first number from 2 that is free, as merge names tests.
		base := testName(t.title, t.file.Key)
		name := base
		for n := 2; taken[name]; n++ {
			name = fmt.Sprintf("%s-%d", base, n)
		}
		taken[name] = true
		cite := citation(t.file.Key, t.clause, t.says)
		if !seenClause[t.file.Key+"\x00"+t.clause] {
			seenClause[t.file.Key+"\x00"+t.clause] = true
			cited[t.file.Key] = append(cited[t.file.Key], flow(mapping("clause", t.clause, "title", oneLineText(t.heading))))
		}
		where := fmt.Sprintf("%s, %s", t.file.Name, t.clause)
		ptr := "#/tests/" + name
		test := &yaml.Node{Kind: yaml.MappingNode}
		set(test, "level", "system")
		scenario, scenarioWhy := t.scenario, "the script says which"
		switch {
		case scenario != "":
		case len(t.then) == 0:
			scenario, scenarioWhy = "golden", "the script gives no expected result to tell, and the answer to the question about it may change it"
		case refusal.MatchString(strings.Join(t.then, " ")):
			scenario, scenarioWhy = "red", "the expected result names a refusal or an error"
		default:
			scenario, scenarioWhy = "golden", "the expected result names no refusal or error"
		}
		if t.scenario == "" && len(t.unfilled["then"]) > 0 {
			scenarioWhy += ", and the answer to the question about its placeholder may change it"
		}
		set(test, "scenario", scenario)
		if len(t.given) > 0 {
			set(test, "given", oneLineText(strings.Join(t.given, "; ")))
		}
		if len(t.when) > 0 {
			set(test, "when", oneLineText(strings.Join(t.when, "; ")))
		}
		if len(t.then) > 0 {
			set(test, "then", oneLineText(strings.Join(t.then, "; ")))
		}
		set(test, "origin", "inferred")
		set(test, "why", fmt.Sprintf("Read from %s (%s). The level is system, as a script exercises the system from outside; the scenario is %s, since %s.", where, oneLineText(t.title), scenario, scenarioWhy))
		set(test, "cites", []*yaml.Node{cite})
		res.Tree.put("tests/"+name+"/test.yaml", test)

		subject := "The script names no operation, command or page it exercises."
		if t.subject != "" {
			subject = fmt.Sprintf("The script names %s.", oneLineText(t.subject))
		}
		ask(fmt.Sprintf("Which operation, command, page or other subject of the specification does test %s exercise? %s", name, subject), ptr,
			"A test is about one subject of the specification, and a script names its own words for it, which only the specification the test is merged with can resolve.", cite)
		missing["subject"]++
		if len(t.given) == 0 {
			ask(fmt.Sprintf("What is given before test %s runs: the records that exist, who calls, and the test data it uses?", name), ptr+"/given",
				fmt.Sprintf("%s gives no precondition and no test data for this test.", where), cite)
			missing["given"]++
		}
		if len(t.when) == 0 {
			ask(fmt.Sprintf("What step does test %s take?", name), ptr+"/when",
				fmt.Sprintf("%s gives no step for this test.", where), cite)
			missing["when"]++
		}
		if len(t.then) == 0 {
			ask(fmt.Sprintf("What is the expected result of test %s?", name), ptr+"/then",
				fmt.Sprintf("%s gives no expected result for this test.", where), cite)
			missing["then"]++
		}
		for _, part := range []string{"given", "when", "then"} {
			if len(t.unfilled[part]) == 0 {
				continue
			}
			var names, whys []string
			for _, g := range t.unfilled[part] {
				names, whys = append(names, g.name), append(whys, g.why)
			}
			ask(fmt.Sprintf("What value does test %s take for %s in its %s?", name, strings.Join(names, " and "), part), ptr+"/"+part,
				fmt.Sprintf("%s: %s, so the step keeps the placeholder as written.", where, strings.Join(whys, "; ")), cite)
			missing["placeholder"]++
		}
	}
	res.say("wrote %s and %s: one per subject a test exercises, which a script cannot name in the specification's words, %d for a test with no given, %d with no step, %d with no expected result and %d for a part whose Examples row gives a placeholder no value",
		plural(len(tests), "test"), plural(next, "question"), missing["given"], missing["when"], missing["then"], missing["placeholder"])
	res.Tree.put("tests/questions.yaml", mapping("questions", questions))
	res.Tree.put("requirements/requirements.yaml", mapping("stakeholders", ownerStakeholder()))
	sources := &yaml.Node{Kind: yaml.MappingNode}
	var names []string
	for _, f := range files {
		url, err := relativeURL(out, filepath.Join(r.Repository.Root, filepath.FromSlash(f.Rel)))
		if err != nil {
			url = f.Rel
		}
		src := mapping("kind", "document", "title", f.Title, "edition", r.Commit, "url", url)
		set(src, "clauses", cited[f.Key])
		set(sources, f.Key, src)
		names = append(names, f.Name)
	}
	description := fmt.Sprintf("The tests in %s, read at commit %s: each scenario of a feature file and each row of a test sheet is a test, inferred from the script, its precondition and test data as given, its steps as when and its expected result as then; each part the script leaves out, and the subject each test exercises, is a question.\n", strings.Join(names, ", "), r.Commit)
	res.Tree.put("specarch.yaml", rootFile("Tests in "+strings.Join(names, ", "), description, []string{"requirements", "tests"}, sources))
}

// refusal is what an expected result says when the test is a red path.
var refusal = regexp.MustCompile(`(?i)\b(refuse[sd]?|reject(s|ed)?|den(y|ies|ied)|forbidden|not allowed|unauthori[sz]ed|invalid|error|fails?|failed|[45]\d\d)\b`)

// operationText is a request the script names, such as POST /notes.
var operationText = regexp.MustCompile(`\b(GET|POST|PUT|PATCH|DELETE)\s+(/[^\s"'<>,;]*)`)

// placeholder is an Outline's placeholder in a step, such as <id>.
var placeholder = regexp.MustCompile(`<([^<>\n]+)>`)

// testName is a test's folder name from its title: kebab-case with a
// letter first, the source key in front when the title gives none.
func testName(title, key string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	name := b.String()
	if len(name) > 60 {
		name = strings.TrimRight(name[:60], "-")
	}
	if name == "" {
		return key + "-test"
	}
	if name[0] < 'a' || name[0] > 'z' {
		name = key + "-" + name
	}
	return name
}

func oneLineText(s string) string { return strings.Join(strings.Fields(s), " ") }

// readFeature reads a Gherkin feature file: each Scenario or Example a
// test, the Background's Given steps before each (a Rule's own Background
// after the Feature's, for that Rule's scenarios), a Scenario Outline one
// test per row of its Examples blocks, each block under its own header
// row, And and But continuing the step before, and a doc string or a data
// table joined to the step it follows.
func readFeature(f *scriptFile, text string) ([]*scriptTest, error) {
	type exampleRow struct {
		header, row []string
		line        int
	}
	var out []*scriptTest
	var featureBackground, background []string
	var cur *scriptTest
	var outline *scriptTest
	var examples []exampleRow
	var header []string
	var tags []string
	section, kind, inRule := "", "", false
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	finish := func() {
		if cur == nil {
			return
		}
		if outline != nil && len(examples) > 0 {
			for i, ex := range examples {
				t := *cur
				t.title = fmt.Sprintf("%s %d", cur.title, i+1)
				// A placeholder the row has no column for, or leaves blank,
				// keeps its text in the step and is asked about: a blank
				// cell may be an empty value on purpose or one left out,
				// and only a person can tell which.
				values, columns := map[string]string{}, map[string]bool{}
				for j, h := range ex.header {
					columns[h] = true
					if j < len(ex.row) {
						values[h] = ex.row[j]
					}
				}
				sub := func(s string) string {
					return placeholder.ReplaceAllStringFunc(s, func(m string) string {
						if v := values[m[1:len(m)-1]]; strings.TrimSpace(v) != "" {
							return v
						}
						return m
					})
				}
				t.unfilled = map[string][]gap{}
				for part, lines := range map[string][]string{"given": cur.given, "when": cur.when, "then": cur.then} {
					seen := map[string]bool{}
					for _, m := range placeholder.FindAllStringSubmatch(strings.Join(lines, "\n"), -1) {
						h := m[1]
						if seen[h] || strings.TrimSpace(values[h]) != "" {
							continue
						}
						seen[h] = true
						why := fmt.Sprintf("the Examples row on line %d has no column for <%s>", ex.line, h)
						if columns[h] {
							why = fmt.Sprintf("the Examples row on line %d leaves <%s> blank, which may be an empty value on purpose or one left out", ex.line, h)
						}
						t.unfilled[part] = append(t.unfilled[part], gap{"<" + h + ">", why})
					}
				}
				t.given, t.when, t.then = mapStrings(cur.given, sub), mapStrings(cur.when, sub), mapStrings(cur.then, sub)
				t.subject, t.says = sub(cur.subject), sub(cur.says)
				out = append(out, &t)
			}
		} else {
			out = append(out, cur)
		}
		cur, outline, examples, header = nil, nil, nil, nil
	}
	inDoc, inTable := false, false
	var doc []string
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, `"""`) || strings.HasPrefix(line, "```") {
			if inDoc {
				joinStep(cur, &background, section, kind, strings.Join(doc, " "), "")
				doc = nil
			}
			inDoc = !inDoc
			continue
		}
		if inDoc {
			if line != "" {
				doc = append(doc, line)
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "@") {
			tags = append(tags, strings.Fields(line)...)
			continue
		}
		head, rest, keyword := strings.Cut(line, ":")
		if keyword {
			switch strings.TrimSpace(head) {
			case "Feature":
				finish()
				f.Title = strings.TrimSpace(rest)
				section, tags, inRule, featureBackground, background = "", nil, false, nil, nil
				continue
			case "Rule":
				finish()
				if !inRule {
					featureBackground = append([]string(nil), background...)
				}
				background = append([]string(nil), featureBackground...)
				section, tags, inRule = "", nil, true
				continue
			case "Background":
				finish()
				section, kind = "background", ""
				continue
			case "Scenario", "Example", "Scenario Outline", "Scenario Template":
				finish()
				cur = &scriptTest{title: strings.TrimSpace(rest), heading: strings.TrimSpace(rest), file: f, clause: fmt.Sprintf("line %d", i+1), scenario: tagScenario(tags)}
				cur.given = append(cur.given, background...)
				cur.says = line
				if strings.Contains(head, "Outline") || strings.Contains(head, "Template") {
					outline = cur
				}
				section, kind, tags = "scenario", "", nil
				continue
			case "Examples", "Scenarios":
				section, header = "examples", nil
				continue
			}
		}
		if strings.HasPrefix(line, "|") {
			cells := tableCells(line)
			switch {
			case section == "examples" && header == nil:
				header = cells
			case section == "examples":
				examples = append(examples, exampleRow{header, cells, i + 1})
			default:
				sep := ""
				if inTable {
					sep = "; "
				}
				joinStep(cur, &background, section, kind, strings.Join(cells, ", "), sep)
				inTable = true
			}
			continue
		}
		inTable = false
		word, step, _ := strings.Cut(line, " ")
		switch word {
		case "Given", "When", "Then":
			kind = strings.ToLower(word)
		case "And", "But", "*":
			if kind == "" {
				kind = "given"
			}
		default:
			// Free text under a keyword is its description, not a step.
			continue
		}
		step = strings.TrimSpace(step)
		if cur != nil {
			cur.says += " " + line
			if m := operationText.FindString(step); m != "" && cur.subject == "" {
				cur.subject = m
			}
		}
		appendStep(cur, &background, section, kind, step)
	}
	finish()
	return out, nil
}

func appendStep(cur *scriptTest, background *[]string, section, kind, step string) {
	if step == "" {
		return
	}
	switch {
	case section == "background":
		if kind == "" || kind == "given" {
			*background = append(*background, step)
		}
	case cur == nil:
	case kind == "when":
		cur.when = append(cur.when, step)
	case kind == "then":
		cur.then = append(cur.then, step)
	default:
		cur.given = append(cur.given, step)
	}
}

// joinStep adds a doc string or a data table row to the step before it,
// after sep, or after a colon when sep is "" (a space when the step ends
// in one); or as a step of its own when there is none.
func joinStep(cur *scriptTest, background *[]string, section, kind, text, sep string) {
	if text == "" {
		return
	}
	var steps *[]string
	switch {
	case section == "background":
		steps = background
	case cur == nil:
		return
	case kind == "when":
		steps = &cur.when
	case kind == "then":
		steps = &cur.then
	default:
		steps = &cur.given
	}
	if n := len(*steps); n > 0 {
		if sep == "" {
			sep = ": "
			if strings.HasSuffix((*steps)[n-1], ":") {
				sep = " "
			}
		}
		(*steps)[n-1] += sep + text
		return
	}
	appendStep(cur, background, section, kind, text)
}

func tagScenario(tags []string) string {
	for _, t := range tags {
		switch strings.ToLower(strings.TrimPrefix(t, "@")) {
		case "negative", "red", "error", "unhappy":
			return "red"
		case "positive", "golden", "happy":
			return "golden"
		}
	}
	return ""
}

func tableCells(line string) []string {
	parts := strings.Split(strings.Trim(line, "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func mapStrings(in []string, f func(string) string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = f(s)
	}
	return out
}

// The columns of a test sheet, by the words its header row may use for
// them, compared in lower case with the space collapsed.
var sheetColumns = []struct {
	part  string
	words []string
}{
	{"id", []string{"id", "test id", "test case id", "case id", "tc id", "no", "no.", "number", "#"}},
	{"title", []string{"title", "test case", "test case name", "test name", "name", "scenario", "summary", "description"}},
	{"subject", []string{"operation", "api", "endpoint", "request", "subject", "service", "feature", "module"}},
	{"given", []string{"precondition", "preconditions", "pre-condition", "pre-conditions", "given", "setup"}},
	{"input", []string{"test data", "input", "inputs", "data", "input data"}},
	{"when", []string{"step", "steps", "test steps", "action", "actions", "when", "procedure"}},
	{"then", []string{"expected", "expected result", "expected results", "expected outcome", "then"}},
	{"type", []string{"type", "test type", "scenario type", "kind", "polarity"}},
}

func sheetColumnWords() string {
	var words []string
	for _, c := range sheetColumns {
		words = append(words, c.words[0])
	}
	return strings.Join(words, ", ")
}

func sheetColumn(header string) string {
	h := strings.ToLower(oneLineText(header))
	for _, c := range sheetColumns {
		for _, w := range c.words {
			if h == w {
				return c.part
			}
		}
	}
	return ""
}

// readTestSheet reads a test sheet: every table whose header row names a
// step, an expected result, a precondition, test data or a title, one test
// per row that holds any text.
func readTestSheet(f *scriptFile, path string, data []byte, res *Result) ([]*scriptTest, error) {
	tf, err := readTabFile(path, f.Rel, data, vocabulary.English())
	if err != nil {
		return nil, err
	}
	f.Title = tf.Title
	var out []*scriptTest
	for _, tb := range tf.Tables {
		cols := map[string]int{}
		for i, h := range tb.Header {
			if part := sheetColumn(h.Value); part != "" {
				if _, seen := cols[part]; !seen {
					cols[part] = i
				}
			}
		}
		if !hasCol(cols, "title", "when", "then", "given", "input") {
			res.say("not read: %s, whose header row names no column of a test sheet (%s)", tb.Range(), sheetColumnWords())
			continue
		}
		for _, row := range tb.Rows {
			cell := func(part string) string {
				i, ok := cols[part]
				if !ok || i >= len(row) {
					return ""
				}
				return strings.TrimSpace(row[i].Value)
			}
			blank := true
			for _, c := range row {
				blank = blank && c.Blank()
			}
			if blank {
				continue
			}
			title := cell("title")
			if id := cell("id"); id != "" {
				if title == "" {
					title = id
				} else {
					title = id + " " + title
				}
			}
			t := &scriptTest{title: title, file: f, clause: tb.Ref(row[0].Row, row[0].Col) + ":" + tb.Ref(row[len(row)-1].Row, row[len(row)-1].Col)}
			if t.title == "" {
				t.title = "row " + fmt.Sprint(row[0].Row)
			}
			t.heading = t.title
			var said []string
			for i, c := range row {
				if !c.Blank() && i < len(tb.Header) {
					said = append(said, oneLineText(tb.Header[i].Value)+": "+oneLineText(c.Value))
				}
			}
			t.says = strings.Join(said, "; ")
			if g := cell("given"); g != "" {
				t.given = append(t.given, g)
			}
			if in := cell("input"); in != "" {
				t.given = append(t.given, "test data: "+in)
			}
			if w := cell("when"); w != "" {
				t.when = append(t.when, w)
			}
			if th := cell("then"); th != "" {
				t.then = append(t.then, th)
			}
			t.subject = cell("subject")
			if t.subject == "" {
				t.subject = operationText.FindString(cell("when"))
			}
			switch strings.ToLower(cell("type")) {
			case "positive", "golden", "happy", "happy path":
				t.scenario = "golden"
			case "negative", "red", "error", "unhappy", "unhappy path":
				t.scenario = "red"
			}
			out = append(out, t)
		}
	}
	return out, nil
}

func hasCol(cols map[string]int, parts ...string) bool {
	for _, p := range parts {
		if _, ok := cols[p]; ok {
			return true
		}
	}
	return false
}
