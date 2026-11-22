// The formats a document target writes, and the check of the formats an
// implementation file's targets name (ADR-249).

/// The document targets of the design, in the order they are listed.
let documentTargets = ["intent", "techspec", "requirements", "testplan", "traceability", "deployment", "commissioning", "handover", "questions", "problems", "comparison", "changes", "releases", "communications", "timeline", "testruns", "progress", "html", "manual", "operations", "brd", "books"]

/// The formats of a document.
let documentFormats = ["md", "docx", "pdf"]

/// The formats a document target writes, the one it writes when none is
/// named first; nil for html and problems, which write their own files
/// only, and for a target that is not a document's.
func formatsOf(_ target: String) -> [String]? {
    switch target {
    case "books": return ["pdf", "docx"]
    case "html", "problems": return nil
    default: return documentTargets.contains(target) ? documentFormats : nil
    }
}

/// The files a document target that takes no format writes.
func ownFiles(_ target: String) -> String {
    switch target {
    case "html": return "index.html"
    case "problems": return "problems.txt and problems.sarif"
    default: return target + ".md"
    }
}

extension Checker {
    /// Checks the formats each target names: only a document target that
    /// writes more than one form takes them, and each must be one it
    /// writes. A value outside md, docx and pdf is the schema's to report.
    func checkTargetFormats() {
        for t in pairs(root.child("targets")) {
            guard let key = t.value.key("formats") else { continue }
            let target = t.key.value
            let at = pointer(["targets", target, "formats"])
            guard let has = formatsOf(target) else {
                if documentTargets.contains(target) {
                    add(key, at, .documentFormat, "\(target) writes \(ownFiles(target)) only and takes no formats; remove formats")
                } else {
                    add(key, at, .documentFormat, "\(target) is not a document target, so formats says nothing for it; formats is read only for a document target and for books; remove it")
                }
                continue
            }
            for (i, item) in items(t.value.child("formats")).enumerated() {
                let f = item.value
                if !documentFormats.contains(f) || has.contains(f) { continue }
                let p = pointer(["targets", target, "formats", "\(i)"])
                if target == "books" && f == "md" {
                    add(item, p, .documentFormat, "books has no format md: a book is made from the Markdown documents, which each document target writes as <target>.md; its formats are pdf and docx")
                    continue
                }
                add(item, p, .documentFormat, "\(target) has no format \(f); its formats are \(has.joined(separator: ", "))")
            }
        }
    }
}
