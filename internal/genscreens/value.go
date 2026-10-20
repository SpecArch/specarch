package genscreens

// A page's schema is a tree of values a generator prints in its own
// stack: a literal (a quoted string, a number, true, false or null), an
// object of members in order, keyed by the idiom's names, and an array.
// An object may be a dictionary, whose keys are data (a route parameter's
// name, a field filled), not the names of a type's members; an array may
// be a rule, the operator and its operands the screens evaluate.

// Value is a value of a schema.
type Value interface {
	isValue()
}

// Member is one key and its value in an object.
type Member struct {
	Key string
	Val Value
}

// Literal is a scalar: a string, which each stack quotes its own way, or
// a number, true, false or null, printed as given.
type Literal struct {
	Text string
	// String is true for a string, Text being its text unquoted.
	String bool
}

// ObjectValue is an object of members in order; a dictionary when its keys
// are data.
type ObjectValue struct {
	Members    []Member
	Dictionary bool
}

// ArrayValue is an array of values; a rule when it is an operator and its
// operands.
type ArrayValue struct {
	Items []Value
	Rule  bool
}

func (Literal) isValue()     {}
func (ObjectValue) isValue() {}
func (ArrayValue) isValue()  {}

// Str is a string.
func Str(s string) Value { return Literal{Text: s, String: true} }

// Raw is a literal printed as given: a number, true, false or null.
func Raw(s string) Value { return Literal{Text: s} }

// Object is an object of members.
func Object(ms []Member) Value { return ObjectValue{Members: ms} }

// Dictionary is an object whose keys are data.
func Dictionary(ms []Member) Value { return ObjectValue{Members: ms, Dictionary: true} }

// Array is an array of values.
func Array(vs []Value) Value { return ArrayValue{Items: vs} }

// RuleOf is a rule: an operator and its operands.
func RuleOf(vs []Value) Value { return ArrayValue{Items: vs, Rule: true} }
