package extract

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// gateHost is what a reader gives the gates: its questions, its
// citations, its lines and the implementation file that names the checks.
// The gates are read the same way by every reader that reads routes; only
// where a reader finds a check's call is its own (ADR-154, ADR-159).
type gateHost struct {
	question func(priority, text string, blocks []string, why string, cites ...*yaml.Node)
	at       func(clause, says string) *yaml.Node
	say      func(format string, args ...any)
	file     string
}

// loadGates reads the checks an implementation file names under
// bindings.http.clientChecks, signatureChecks, applicationChecks,
// callerChecks, audienceChecks and eventWriters: gates on a route that
// read no permission (ADR-154, ADR-159). It returns checks with the gates
// after them, so that a function named twice is asked about against every
// check named before it.
func loadGates(h gateHost, binding *yaml.Node, checks []permissionCheck) []permissionCheck {
	for _, kind := range []struct{ key, gate, word, argument string }{
		{"clientChecks", "client", "client", "clientArgument"},
		{"signatureChecks", "signature", "signature", ""},
		{"applicationChecks", "application", "application", "applicationArgument"},
		{"callerChecks", "caller", "caller", "callerArgument"},
		{"audienceChecks", "audience", "audience", "audienceArgument"},
		{"eventWriters", "event", "event writer", ""},
	} {
		list := child(binding, kind.key)
		if list == nil {
			continue
		}
		var names []string
		for i, item := range list.Content {
			pkg, fn := scalar(child(item, "package")), scalar(child(item, "function"))
			if pkg == "" || fn == "" {
				h.question("must", fmt.Sprintf("The implementation file %s names a %s check (item %d of bindings.http.%s) without its package and its function. Which check is it?", h.file, kind.word, i+1, kind.key),
					[]string{"paths"}, "A check named in part cannot be found in the source, and the operations it guards would be read as open to people.")
				continue
			}
			if named := namedCheck(checks, pkg, fn); named != "" {
				h.question("must", fmt.Sprintf("The implementation file %s names %s.%s as a %s (item %d of bindings.http.%s), and also as a %s. Which is it?", h.file, pkg, fn, gateWord(kind.gate), i+1, kind.key, named),
					[]string{"paths"}, "A call is read as one check; the reader reads it as the one named first, and the other would be left unread.")
				continue
			}
			n := 0
			if kind.argument != "" {
				if v := scalar(child(item, kind.argument)); v != "" {
					n, _ = strconv.Atoi(v)
				}
			}
			checks = append(checks, permissionCheck{pkg: pkg, function: fn, gate: kind.gate, argument: n})
			names = append(names, pkg+"."+fn)
		}
		if len(names) > 0 {
			word := "check"
			if len(names) > 1 {
				word = "checks"
			}
			if kind.gate == "event" {
				h.say("checks: %s names the %s %s", h.file, pluralWord(len(names), "event writer"), joinAnd(names))
			} else {
				h.say("checks: %s names the %s %s %s", h.file, kind.word, word, joinAnd(names))
			}
		}
	}
	return checks
}

// gateWord names a kind of gate: a client check, an event writer.
func gateWord(gate string) string {
	if gate == "event" {
		return "event writer"
	}
	return gate + " check"
}

// namedCheck is what an earlier item names a function as, "" when none
// does.
func namedCheck(checks []permissionCheck, pkg, fn string) string {
	for _, c := range checks {
		if c.pkg == pkg && c.function == fn {
			if c.gate == "" {
				return "permission check"
			}
			return gateWord(c.gate)
		}
	}
	return ""
}

// goQuoted quotes each name as Go writes a string.
func goQuoted(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = strconv.Quote(n)
	}
	return out
}

// gateKey is a gate and whether its argument gave names as literals: the
// operations where it does and the ones where it reads them otherwise are
// asked about apart.
type gateKey struct {
	check   *permissionCheck
	literal bool
}

// trailingNames is one gate call that passes string literals after the
// argument that names, and the blocks of the operations it guards.
type trailingNames struct {
	use checkUse
	at  []string
}

// gateWriter gathers the gates of every operation a reader writes and
// asks about each once, after the last operation.
type gateWriter struct {
	h             gateHost
	gated         map[gateKey]*bypassedCheck
	trailing      map[string]*trailingNames // by the call's clause and check
	trailingOrder []string
}

func newGateWriter(h gateHost) *gateWriter {
	return &gateWriter{h: h, gated: map[gateKey]*bypassedCheck{}, trailing: map[string]*trailingNames{}}
}

// route records the gates among an operation's checks, at at, the
// operation's pointer, and gives its callers and via: what the literal
// client checks and the literal caller checks on it admit together.
func (w *gateWriter) route(label, at string, uses []checkUse) (callers, via []string) {
	var clientUses, callerUses []checkUse
	for _, c := range uses {
		if c.bound.gate == "" {
			continue
		}
		k := gateKey{c.bound, c.names != nil}
		b := w.gated[k]
		if b == nil {
			b = &bypassedCheck{check: c.bound, clause: c.clause}
			w.gated[k] = b
		}
		block := at + "/callers"
		switch c.bound.gate {
		case "caller":
			block = at + "/via"
		case "application", "audience", "event":
			block = at
		}
		if !contains(b.at, block) {
			b.at = append(b.at, block)
		}
		b.labels = append(b.labels, label)
		if !slices.ContainsFunc(b.uses, func(u checkUse) bool { return u.clause == c.clause }) {
			b.uses = append(b.uses, c)
		}
		for _, n := range c.names {
			if !contains(b.names, n) {
				b.names = append(b.names, n)
			}
		}
		if c.bound.argument > 0 && c.names == nil && b.setting == "" {
			b.setting = c.argument
		}
		switch {
		case c.bound.gate == "client" && c.names != nil:
			clientUses = append(clientUses, c)
		case c.bound.gate == "caller" && c.names != nil:
			callerUses = append(callerUses, c)
		}
		if len(c.trailing) > 0 {
			key := c.clause + "|" + c.check
			t := w.trailing[key]
			if t == nil {
				t = &trailingNames{use: c}
				w.trailing[key] = t
				w.trailingOrder = append(w.trailingOrder, key)
			}
			if !contains(t.at, block) {
				t.at = append(t.at, block)
			}
		}
	}
	return w.sequencedNames(label, at+"/callers", "a", "caller", clientUses), w.sequencedNames(label, at+"/via", "an", "immediate caller", callerUses)
}

// permission adds the permission an operation checks to each gate on it,
// for the application check's question.
func (w *gateWriter) permission(uses []checkUse, permission string) {
	if permission == "" {
		return
	}
	for _, c := range uses {
		if b := w.gated[gateKey{c.bound, c.names != nil}]; b != nil && !contains(b.perms, permission) {
			b.perms = append(b.perms, permission)
		}
	}
}

// sequencedNames is what the literal gates of one kind on a route admit
// together. Each runs in turn and refuses a name it does not list, so
// only the names every one lists pass; where the lists differ, that set
// is written and asked at the block, and none is written when it is
// empty, since a list that silently replaced another would drop a
// definition.
func (w *gateWriter) sequencedNames(label, block, article, who string, uses []checkUse) []string {
	if len(uses) == 0 {
		return nil
	}
	out := append([]string(nil), uses[0].names...)
	differ := false
	for _, u := range uses[1:] {
		var kept []string
		for _, n := range out {
			if contains(u.names, n) {
				kept = append(kept, n)
			}
		}
		if len(kept) != len(out) || len(u.names) != len(uses[0].names) {
			differ = true
		}
		out = kept
	}
	if !differ {
		return out
	}
	var calls []string
	var cites []*yaml.Node
	for _, u := range uses {
		calls = append(calls, fmt.Sprintf("%s with %s", u.check, joinAnd(goQuoted(u.names))))
		cites = append(cites, w.h.at(u.clause, fmt.Sprintf("Calls %s with %s.", u.check, joinAnd(goQuoted(u.names)))))
	}
	passes := "only " + joinAnd(out) + map[bool]string{true: " passes", false: " pass"}[len(out) == 1] + " every one, written as its " + block[strings.LastIndex(block, "/")+1:]
	if len(out) == 0 {
		passes = "no " + who + " passes every one, and none is written"
	}
	w.h.question("must", fmt.Sprintf("%s calls %s in turn, and each refuses %s %s it does not list, so %s. Which %ss is it meant to admit?", label, joinAnd(calls), article, who, passes, who),
		[]string{block}, "Checks in sequence admit only the names every one lists; lists that disagree are as often a mistake as a narrowing, so the set the code admits is written and asked, never one list chosen.", cites...)
	if len(out) == 0 {
		return nil
	}
	return out
}

// writeTrailing asks, once per call, about the string literals a gate's
// call passes after the argument that names.
func (w *gateWriter) writeTrailing() {
	for _, key := range w.trailingOrder {
		t := w.trailing[key]
		c := t.use
		clause := c.clause
		names := c.argument
		if c.names != nil {
			names = joinAnd(goQuoted(c.names))
		}
		ask := "Are they names the check admits too, or something else, such as a reason?"
		if len(c.trailing) == 1 {
			ask = "Is it a name the check admits too, or something else, such as a reason?"
		}
		w.h.question("must", fmt.Sprintf("%s: %s passes %s after %s, the argument the implementation file names as the check's. %s", clause, c.check, joinAnd(goQuoted(c.trailing)), names, ask),
			t.at, "Syntax does not say whether a check takes its names as one argument or as every argument from it on; the reader writes the named argument alone and asks about the rest, so neither a name is dropped nor a reason admitted as a caller.",
			w.h.at(clause, fmt.Sprintf("Calls %s with %s, then %s.", c.check, c.argument, joinAnd(goQuoted(c.trailing)))))
	}
}

// writeGated asks about every gate, once per check, blocking the callers
// of each operation it guards (ADR-154) or, for the checks of ADR-159,
// the via, the roles or the tokens. Syntax shows that a check admits
// clients or verifies a signature, and not which clients, which algorithm
// or which parts of the request: those are in a key or a token the code
// reads at run time. callers names its clients one by one (ADR-110), so
// the reader writes those the code names as literals and asks for the
// rest. checkedBy is the operations that check each permission.
func (w *gateWriter) writeGated(checkedBy map[string][]string) {
	h := w.h
	var list []*bypassedCheck
	for _, b := range w.gated {
		list = append(list, b)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].check.gate != list[j].check.gate {
			return list[i].check.gate < list[j].check.gate
		}
		if list[i].check.function != list[j].check.function {
			return list[i].check.function < list[j].check.function
		}
		if list[i].check.pkg != list[j].check.pkg {
			return list[i].check.pkg < list[j].check.pkg
		}
		return len(list[i].names) > len(list[j].names)
	})
	for _, b := range list {
		c := b.check
		guards := plural(len(b.at), "operation")
		switch {
		case c.gate == "client" && len(b.names) > 0:
			h.say("gate: %s names %s.%s as a client check, and it guards %s, whose callers are the clients it names", h.file, c.pkg, c.function, guards)
			h.question("must", fmt.Sprintf("The check %s admits only the %s %s, written as the callers of %s. Declare each under clients with its kind, the roles that grant the permissions it calls with and its proof, here or in the identity module. Do people call any of them?", c.name(), pluralWord(len(b.names), "client"), joinAnd(b.names), guards),
				b.at, "The code names the clients and not what each is: its kind, its roles and its proof are the registration's, never in syntax. callers names who may call one by one (ADR-110).",
				w.namedCites(b, "a client check")...)
		case c.gate == "client":
			h.say("gate: %s names %s.%s as a client check, and it guards %s", h.file, c.pkg, c.function, guards)
			h.question("must", fmt.Sprintf("The check %s admits only a client, a caller on its own account rather than a person's, and it guards %s. Which clients call each of them, and do people call any?", c.name(), guards),
				b.at, "Syntax shows that a check admits clients and not which ones, which a key or a token read at run time says. callers names who may call one by one, people and each client, with no word for every client, since a client added later would then be admitted without anyone writing it (ADR-110).",
				h.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file names as a client check.", c.function)))
		case c.gate == "signature":
			h.say("gate: %s names %s.%s as a signature check, and it guards %s", h.file, c.pkg, c.function, guards)
			h.question("must", fmt.Sprintf("The check %s verifies a request's signature, and it guards %s. Which client signs the requests, with which algorithm, and over which parts of the request: the client's proof?", c.name(), guards),
				b.at, "A signature's signer, algorithm and covered parts are in a key and a header the code reads at run time, never in syntax. A client's proof names them (ADR-096), and the operation names the client in callers.",
				h.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file names as a signature check.", c.function)))
		case c.gate == "caller" && len(b.names) > 0:
			h.say("gate: %s names %s.%s as a caller check, and it guards %s, whose via are the services it names", h.file, c.pkg, c.function, guards)
			h.question("must", fmt.Sprintf("The check %s admits only the %s %s as the immediate caller on a person's behalf, written as the via of %s. Declare each as a client of kind service with its workload-identity proof, mutualTls, spiffe or wimse, here or in the identity module, and the transaction token the operations accept with it.", c.name(), pluralWord(len(b.names), "service"), joinAnd(b.names), guards),
				b.at, "The code names the services and not how each proves itself, which is the deployment's; via names the immediate callers one by one, each with a workload-identity proof, and needs the transaction token it is checked with (ADR-111).",
				w.namedCites(b, "a caller check")...)
		case c.gate == "caller":
			h.say("gate: %s names %s.%s as a caller check, and it guards %s", h.file, c.pkg, c.function, guards)
			h.question("must", fmt.Sprintf("The check %s compares the immediate caller's workload identity with the services %s gives, and it guards %s. Which services call each of them on a person's behalf: its via?", c.name(), gateSource(b.setting), guards),
				b.at, "Names read at run time are not known by syntax. via names the immediate callers one by one, each a service with a workload-identity proof, with no wildcard, since a hop added later would then be admitted without anyone writing it (ADR-111).",
				h.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file names as a caller check.", c.function)))
		case c.gate == "application":
			h.say("gate: %s names %s.%s as an application check, and it guards %s", h.file, c.pkg, c.function, guards)
			apps := "applications " + gateSource(b.setting)
			if len(b.names) > 0 {
				apps = pluralWord(len(b.names), "application") + " " + joinAnd(b.names)
			}
			var elsewhere []string
			for _, p := range b.perms {
				for _, l := range checkedBy[p] {
					if !contains(b.labels, l) && !contains(elsewhere, l) {
						elsewhere = append(elsewhere, l)
					}
				}
			}
			text := fmt.Sprintf("The check %s admits only the %s, the one the person signed in through, at %s, which check %s. On every role that grants %s, write through with the applications the check admits, here or in the identity module.", c.name(), apps, guards, joinAnd(b.perms), map[bool]string{true: "it", false: "them"}[len(b.perms) == 1])
			if len(b.perms) == 0 {
				text = fmt.Sprintf("The check %s admits only the %s, the one the person signed in through, at %s. Which permission does it narrow, and on which roles: their through?", c.name(), apps, guards)
			} else if len(elsewhere) > 0 {
				text += fmt.Sprintf(" %s check the same without it, and through narrows the grant there too: is it meant there?", joinAnd(elsewhere))
			}
			h.question("must", text, []string{"roles"}, "The application is a condition on a grant, written on the role keyed by the permission (ADR-106); the reader writes no role, and the code shows the check, not which grant it narrows.",
				h.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file names as an application check.", c.function)))
		case c.gate == "audience":
			h.say("gate: %s names %s.%s as an audience check, and it guards %s", h.file, c.pkg, c.function, guards)
			text := fmt.Sprintf("The check %s compares a token's aud with %s at %s. Which audience names this service: the const of aud on the access token it accepts? A value per environment goes in the implementation file's deployments.", c.name(), gateSource(b.setting), guards)
			if len(b.names) > 0 {
				text = fmt.Sprintf("The check %s compares a token's aud with %s at %s. Write it as the const of aud on the access token they accept, or say which value per environment the implementation file's deployments give.", c.name(), joinAnd(goQuoted(b.names)), guards)
			}
			h.question("must", text, []string{"tokens"}, "A token accepted without a fixed audience is refused, since a token minted for one service could be replayed at the next (ADR-109); the reader writes no token, and an audience read from a setting is the deployment's value.",
				h.at(b.clause, fmt.Sprintf("Calls %s, which the implementation file names as an audience check.", c.function)))
		case c.gate == "event":
			h.say("gate: %s names %s.%s as an event writer, and it guards %s; every security event is derived from the refusal that writes it, so the reader writes nothing from it", h.file, c.pkg, c.function, guards)
		}
	}
}

// namedCites cites each call of a gate that names as literals at its own
// clause, with the names that call gives, so no line is cited for a name
// another line passes.
func (w *gateWriter) namedCites(b *bypassedCheck, word string) []*yaml.Node {
	var cites []*yaml.Node
	for _, u := range b.uses {
		cites = append(cites, w.h.at(u.clause, fmt.Sprintf("Calls %s with %s, which the implementation file names as %s.", b.check.function, joinAnd(goQuoted(u.names)), word)))
	}
	return cites
}

// gateSource says where a gate's names come from when they are not
// literals.
func gateSource(setting string) string {
	if setting == "" || setting == "nothing" {
		return "no argument gives"
	}
	return setting
}

// gateNamesOf reads the names a gate's argument n, from 1, gives, for a
// reader whose values are of type V: a string, or a list of strings. A
// name that is not a literal, such as a setting, gives no names and the
// argument as the code writes it. The string literals after argument n
// are returned apart, as for Go's gateNames: they are asked, never
// written.
func gateNamesOf[V any](args []V, n int, str func(V) (string, bool), list func(V) ([]V, bool), describe func(V) string) ([]string, string, []string) {
	if n > len(args) {
		return nil, "nothing", nil
	}
	arg := args[n-1]
	var trailing []string
	for _, e := range args[n:] {
		if s, ok := str(e); ok {
			trailing = append(trailing, s)
		}
	}
	if items, ok := list(arg); ok {
		var names []string
		for _, e := range items {
			s, ok := str(e)
			if !ok {
				return nil, describe(arg), trailing
			}
			names = append(names, s)
		}
		return names, "[" + strings.Join(goQuoted(names), ", ") + "]", trailing
	}
	if s, ok := str(arg); ok {
		return []string{s}, strconv.Quote(s), trailing
	}
	return nil, describe(arg), trailing
}

// askTransactionToken asks, at an operation whose handler reads the
// Txn-Token header, for the transaction token it accepts and its via: the
// header is where the Transaction Tokens draft carries one (section 12),
// so the operation accepts a transaction token (ADR-111). Every reader
// asks it in these words (ADR-173); header is the name as the code writes
// it.
func askTransactionToken(h gateHost, clause, label, at, header string) {
	h.question("must", fmt.Sprintf("%s: the handler of %s reads the Txn-Token header, so the operation accepts a transaction token. Declare it under tokens with tokenType txn_token and its claims, name it in the operation's token, and list the services that call it on a person's behalf in via.", clause, label),
		[]string{at + "/token", at + "/via"}, "The Txn-Token header carries a transaction token and nothing else (the Transaction Tokens draft, section 12); its claims and the immediate callers are the design's, and the code shows only that the header is read.",
		h.at(clause, "Reads the header "+header+"."))
}

// isTxnTokenHeader says whether a header's name is Txn-Token; a header's
// name is matched in any case (RFC 9110, section 5.1).
func isTxnTokenHeader(name string) bool {
	return strings.EqualFold(name, "Txn-Token")
}
