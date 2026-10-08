package extract

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Result is what a reader wrote and what it prints: the tree, and the lines
// of its standard output in order.
type Result struct {
	Tree  *Tree
	Lines []string
}

func (res *Result) say(format string, args ...any) {
	res.Lines = append(res.Lines, fmt.Sprintf(format, args...))
}

// commitLine names the commit read and the paths it was the last change to.
func commitLine(res *Result, r *Read) {
	res.say("commit %s: the last change to %s", r.Commit, strings.Join(r.Paths, ", "))
}

// Outline reads the tracked files under the paths into a tree that lists
// them as clauses of one code source and holds no element, so that
// specarch gaps shows each as producing nothing until a reader reads it.
func Outline(paths []string, out, key string) (*Result, error) {
	r, err := Open(paths)
	if err != nil {
		return nil, err
	}
	res := &Result{Tree: newTree()}
	commitLine(res, r)
	res.say("counted %s: the tracked files under %s, each once", plural(len(r.Files), "file"), strings.Join(r.Paths, ", "))
	var clauses []*yaml.Node
	for _, f := range r.Files {
		clauses = append(clauses, flow(mapping("clause", f, "title", "Not read yet")))
		if line, text, ok := generatedMark(filepath.Join(r.Repository.Root, filepath.FromSlash(f))); ok {
			res.say("generated: %s:%d says it is generated from another source: %s", f, line, text)
		}
	}
	src, err := codeSource(r, out, clauses)
	if err != nil {
		return nil, err
	}
	description := fmt.Sprintf("The outline of %s at commit %s: every tracked file is a clause, and no element is written, since no reader reads these files yet. specarch gaps shows each file as producing nothing until a reader or a reviewer cites it.\n", strings.Join(r.Paths, ", "), r.Commit)
	res.Tree.put("specarch.yaml", rootFile("Outline of "+strings.Join(r.Paths, ", "), description, nil, mapping(key, src)))
	return res, nil
}

// A line that says its file is generated and must not be edited, as Go's
// "Code generated ... DO NOT EDIT." and the like do.
var (
	generatedWord = regexp.MustCompile(`(?i)\b(auto-?)?generated\b`)
	doNotEdit     = regexp.MustCompile(`(?i)\b(do not|don't|must not be) edit`)
)

// generatedMark finds the first line of a text file that says the file is
// generated and must not be edited.
func generatedMark(path string) (int, string, bool) {
	data, err := os.ReadFile(path)
	if err != nil || bytes.IndexByte(data, 0) >= 0 {
		return 0, "", false
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if generatedWord.MatchString(line) && doNotEdit.MatchString(line) {
			return n, strings.TrimSpace(line), true
		}
	}
	return 0, "", false
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	if strings.HasSuffix(word, "y") && !strings.HasSuffix(word, "ey") {
		return fmt.Sprintf("%d %sies", n, strings.TrimSuffix(word, "y"))
	}
	if strings.HasSuffix(word, "x") || strings.HasSuffix(word, "s") {
		return fmt.Sprintf("%d %ses", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
