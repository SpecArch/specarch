package capability

// The facts below are printed by capabilities --json, guide and guide
// --agent alike, and AGENTS.md repeats the agent guide, so each is said
// once, here.

// Workflow is the order of work, one step each.
var Workflow = []string{
	"Pin the program: record the module version and checksum `go version -m` prints for the installed specarch in the repository's agent instructions, and check them at the start of each session; `specarch capabilities --json` names the version, the formats and the plug-ins this install has.",
	"Specification first: change the specification (specarch.yaml, its stage folders, the implementation files) before the code made from it, never a generated file; a generated file's first lines say which command made it.",
	"After every edit run `specarch validate <folder>`: exit 0 is valid, 1 lists the errors one per line, 2 is a usage or read error.",
	"Run `specarch gaps <folder>` for the open questions and what each holds up; take each must or should question to the owner and record the answer with `specarch decide`.",
	"Write what people read with `specarch document <target> --out <folder> <folder>` (techspec, requirements, problems, questions, manual and the others); once the owner has read them, they record the approval with `specarch approve`.",
	"Write code with `specarch generate <target> <folder>`; it refuses an unapproved specification unless `--unapproved` makes the output a draft. `--dry-run` lists what it would write; `--check` fails when the files on disk differ, so run it before each commit and in CI.",
	"For a system that exists already: `specarch extract <source>` once per surface (documents, database, router, code), `specarch merge` the trees, then validate and gaps; `specarch check` compares the specification with the code's own tables later.",
	"When a release moves the formats: `specarch upgrade --dry-run <folder>` lists the changes and the steps left to the author, `specarch upgrade <folder>` makes them.",
}

// ReadingProblems is how to read a problem line and the problems file.
var ReadingProblems = []string{
	"A problem line is file:line:column: severity: path: rule: message [id], the same in validate's output, in problems.txt and in --json as fields.",
	"The severity is error (the specification is invalid: generate and approve refuse it, document writes with the error marked and exits 1), warning (valid, but something is missing or doubtful), question (an open question; must and should questions block what they name) or note.",
	"The path is the YAML path of the entry in the merged specification; the rule names the check, the same word in every release; the id is stable across runs, so a problem can be tracked.",
	"problems.txt, which `specarch document problems` writes beside problems.sarif, starts with the counts, then lists every problem, one line each, with the file paths relative to it; each one is also marked above its entry in the YAML files, on a `# specarch-problem:` comment line, and in the documents written.",
	"Fix an error at the file and line it names, then validate again; do not silence a warning by deleting the entry it is at.",
}

// AskTheOwner is when an agent stops and asks.
var AskTheOwner = []string{
	"Every open must or should question, and any choice between two sources that disagree (a document and the code): the agent records the question, never picks an answer; the owner answers and `specarch decide` records it.",
	"Approving a specification: `specarch approve --by <stakeholder>` records a person's approval, so only that person's word runs it.",
	"Generating a draft with --unapproved for anything that will be used, and overwriting a file built by hand.",
	"Any error the agent cannot settle from the specification and its sources, a refusal it does not understand, and anything a remedy does not cover.",
}

// Limits are what the program does not promise.
var Limits = []string{
	"No promise of complete generation: a plug-in writes what its target covers from the specification, marks each problem at its entry, and names what it does not write; the rest is written by hand.",
	"Hand-written islands: keep code written by hand in files a target does not write, since generate writes every file its plug-in answers (sqlc and scripts never write over a file they did not write); `generate --check` reads a file without the generated header as built by hand and says whether it matches the output but for the header; an element an implementation file marks ownedBy a stakeholder is generated nowhere.",
	"A specification is never complete: an incomplete one is still read, and every gap is a problem or a question rather than a refusal to read.",
	"The program never runs or deploys the code it generates; a plug-in is found on PATH, answers on standard output and never touches the disk, and specarch writes its files.",
	"Prompts: none. Every command runs without asking anything, so no --yes exists; a step that needs a person is a command the person runs (decide, approve).",
}

// Conventions hold for every command.
var Conventions = []string{
	"--json, on any command, prints one JSON object on standard output (key specarchOutput; model and capabilities print their own, specarchModel and specarchCapabilities, on exit 0) and, on exit 2, one error object on standard error (key specarchError, a stable code), and nothing else on either.",
	"Without --json, results go to standard output and counts and reasons to standard error, as plain text with no colour or terminal codes, whether or not the output is a terminal; NO_COLOR is honoured by having no colour.",
	"--dry-run, on every command that writes, lists each file it would write or remove on standard output and writes nothing.",
	"Exit status: 0 done, 1 the command ran and found what it reports (errors, differences, open questions), 2 a usage or read error or a failed plug-in; each command's own exit codes and remedies are in its tool entry.",
	"Arguments after -- are paths, even when they start with -.",
}

// ErrorCodes are the codes of the error object --json prints on standard
// error.
var ErrorCodes = []ErrorCodeMeaning{
	{"usage", "The command line is wrong: an unknown command or option, or a missing argument; the message says which."},
	{"read", "A path given or needed could not be read."},
	{"write", "A file could not be written."},
	{"plugin", "A generator plug-in was not found on PATH, failed, or answered something that is not specarch's JSON."},
	{"failed", "Any other failure of exit status 2; the message and detail say what happened."},
}

// Outputs describe the two objects --json prints.
var Outputs = map[string]any{
	"standardOutput": map[string]any{
		"specarchOutput": "the version of this object, 1.0.0",
		"command":        "the command run, such as validate or idioms diff",
		"exitStatus":     "the exit status",
		"meaning":        "what the exit status means for this command",
		"remedy":         "what to do about it, when the command names a remedy",
		"diagnostics":    "the problem lines, each as file, line, column, severity, path, rule, message and id",
		"lines":          "every other line the command printed on standard output",
		"messages":       "the lines the command printed on standard error: counts, progress and reasons",
		"exceptions":     "model and capabilities, which print one JSON object of their own, print it as it is on exit 0: specarchModel and specarchCapabilities",
	},
	"standardError": map[string]any{
		"specarchError": "the version of this object, 1.0.0",
		"command":       "the command run",
		"exitStatus":    "2",
		"code":          "a stable code, one of errorCodes",
		"message":       "the first line the command printed on standard error",
		"detail":        "the lines after it",
		"remedy":        "what to do about it",
	},
}
