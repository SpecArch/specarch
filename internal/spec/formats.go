package spec

// DocumentTargets are the document targets of the design, in the order
// they are listed (DocumentTarget).
var DocumentTargets = []string{"intent", "techspec", "requirements", "testplan", "traceability", "deployment", "commissioning", "handover", "questions", "problems", "comparison", "changes", "releases", "communications", "timeline", "testruns", "progress", "html", "manual", "operations", "brd", "books"}

// DocumentFormats are the formats of a document (DocumentFormat, ADR-249).
var DocumentFormats = []string{"md", "docx", "pdf"}

// FormatsOf are the formats a document target writes, the one it writes
// when none is named first: a Markdown document writes md, docx and pdf;
// the books pdf and docx, being made from the Markdown documents. It is
// nil for html and problems, which write their own files only, and for a
// target that is not a document's.
func FormatsOf(target string) []string {
	switch target {
	case "books":
		return []string{"pdf", "docx"}
	case "html", "problems":
		return nil
	}
	for _, t := range DocumentTargets {
		if t == target {
			return DocumentFormats
		}
	}
	return nil
}

// OwnFiles names the files a document target that takes no format
// writes, for the messages that say so.
func OwnFiles(target string) string {
	switch target {
	case "html":
		return "index.html"
	case "problems":
		return "problems.txt and problems.sarif"
	}
	return target + ".md"
}
