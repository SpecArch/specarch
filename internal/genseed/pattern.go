package genseed

import (
	"regexp"
	"regexp/syntax"
	"strings"
)

// A value for a field with a pattern is drawn from the pattern's syntax
// tree, as Go's own regexp package parses it (RE2 syntax, which the design
// shares with JSON Schema's subset): a literal is written as it is, a
// character class takes one of its runes, a repetition takes a count
// between its bounds (an open bound stops at three more), an alternation
// takes one branch. What the tree cannot give a value for (a word
// boundary, an empty-width assertion other than ^ and $) is refused, and
// every value drawn is matched against the pattern before it is used.

// unbounded is how many repetitions beyond the minimum an open bound
// allows, so x* and x+ stay short.
const unbounded = 3

// patternValue draws one value matching pattern, or says why it cannot.
func patternValue(pattern string, r *random) (string, string) {
	tree, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return "", "the pattern does not parse: " + err.Error()
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", "the pattern does not compile: " + err.Error()
	}
	var b strings.Builder
	if why := walk(tree.Simplify(), r, &b); why != "" {
		return "", why
	}
	s := b.String()
	if !re.MatchString(s) {
		return "", "a value drawn from the pattern does not match it"
	}
	return s, ""
}

func walk(n *syntax.Regexp, r *random, b *strings.Builder) string {
	switch n.Op {
	case syntax.OpLiteral:
		for _, c := range n.Rune {
			if n.Flags&syntax.FoldCase != 0 && r.chance(2) {
				b.WriteString(strings.ToUpper(string(c)))
				continue
			}
			b.WriteRune(c)
		}
	case syntax.OpCharClass:
		// n.Rune holds pairs, lo and hi of each range. A class that holds
		// any printable ASCII (a negated class spans nearly all of
		// Unicode) draws from that part alone, so the value is readable
		// and one UTF-16 unit a rune, as SQL Server counts a column's
		// length.
		ranges := printable(n.Rune)
		if len(ranges) == 0 {
			ranges = n.Rune
		}
		var total int64
		for i := 0; i < len(ranges); i += 2 {
			total += int64(ranges[i+1]-ranges[i]) + 1
		}
		if total == 0 {
			return "a character class matches nothing"
		}
		pick := r.intn(total)
		for i := 0; i < len(ranges); i += 2 {
			size := int64(ranges[i+1]-ranges[i]) + 1
			if pick < size {
				c := ranges[i] + rune(pick)
				if c == '\n' || c == '\r' || c == '\'' || c == '\\' {
					// A class that spans a line break or a quote would make
					// a value a SQL literal cannot hold plainly; take the
					// printable neighbour the class also holds when it does.
					c = plainIn(ranges, c)
				}
				b.WriteRune(c)
				return ""
			}
			pick -= size
		}
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		b.WriteRune(rune('a' + r.intn(26)))
	case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText, syntax.OpEmptyMatch:
	case syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return "the pattern uses a word boundary, which no value is drawn for"
	case syntax.OpCapture:
		return walk(n.Sub[0], r, b)
	case syntax.OpStar:
		return repeat(n.Sub[0], int(r.intn(unbounded+1)), r, b)
	case syntax.OpPlus:
		return repeat(n.Sub[0], 1+int(r.intn(unbounded)), r, b)
	case syntax.OpQuest:
		return repeat(n.Sub[0], int(r.intn(2)), r, b)
	case syntax.OpRepeat:
		hi := n.Max
		if hi < 0 {
			hi = n.Min + unbounded
		}
		return repeat(n.Sub[0], int(r.in(int64(n.Min), int64(hi))), r, b)
	case syntax.OpConcat:
		for _, s := range n.Sub {
			if why := walk(s, r, b); why != "" {
				return why
			}
		}
	case syntax.OpAlternate:
		return walk(n.Sub[r.intn(int64(len(n.Sub)))], r, b)
	default:
		return "the pattern uses " + n.Op.String() + ", which no value is drawn for"
	}
	return ""
}

func repeat(n *syntax.Regexp, count int, r *random, b *strings.Builder) string {
	for i := 0; i < count; i++ {
		if why := walk(n, r, b); why != "" {
			return why
		}
	}
	return ""
}

// printable is the part of the class ranges inside printable ASCII (space
// to tilde), as pairs; empty when the class holds none.
func printable(ranges []rune) []rune {
	var out []rune
	for i := 0; i < len(ranges); i += 2 {
		lo, hi := max(ranges[i], ' '), min(ranges[i+1], '~')
		if lo <= hi {
			out = append(out, lo, hi)
		}
	}
	return out
}

// plainIn is a printable rune of the class ranges other than c when the
// class has one, and c otherwise.
func plainIn(ranges []rune, c rune) rune {
	for i := 0; i < len(ranges); i += 2 {
		for x := ranges[i]; x <= ranges[i+1] && x < ranges[i]+128; x++ {
			if x >= ' ' && x != '\'' && x != '\\' && x != 0x7f {
				return x
			}
		}
	}
	return c
}
