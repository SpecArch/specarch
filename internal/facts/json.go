package facts

import (
	"bytes"
	"fmt"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// obj is a JSON object that keeps its keys in the order they were first
// set, as a dump writes them.
type obj struct {
	keys []string
	vals []any
}

func newObj(kv ...any) *obj {
	o := &obj{}
	for i := 0; i+1 < len(kv); i += 2 {
		o.set(kv[i].(string), kv[i+1])
	}
	return o
}

func (o *obj) set(k string, v any) {
	for i, kk := range o.keys {
		if kk == k {
			o.vals[i] = v
			return
		}
	}
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
}

func (o *obj) get(k string) any {
	for i, kk := range o.keys {
		if kk == k {
			return o.vals[i]
		}
	}
	return nil
}

func (o *obj) has(k string) bool {
	for _, kk := range o.keys {
		if kk == k {
			return true
		}
	}
	return false
}

func (o *obj) del(k string) {
	for i, kk := range o.keys {
		if kk == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			o.vals = append(o.vals[:i], o.vals[i+1:]...)
			return
		}
	}
}

// merge sets every key of p on o, in p's order.
func (o *obj) merge(p *obj) {
	if p == nil {
		return
	}
	for i, k := range p.keys {
		o.set(k, p.vals[i])
	}
}

// encode writes v compactly, as a JavaScript program's JSON.stringify
// writes it.
func encode(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.Itoa(x))
	case string:
		jsString(b, x)
	case *obj:
		if x == nil {
			b.WriteString("null")
			return
		}
		b.WriteByte('{')
		for i, k := range x.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			jsString(b, k)
			b.WriteByte(':')
			encode(b, x.vals[i])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			encode(b, e)
		}
		b.WriteByte(']')
	default:
		panic(fmt.Sprintf("facts: cannot write a %T", v))
	}
}

func encoded(v any) string {
	var b bytes.Buffer
	encode(&b, v)
	return b.String()
}

// jsString writes a string as JSON.stringify does: a quote, a backslash
// and a control character escaped, a lone surrogate (kept in s as its
// three-byte form) as \u, and every other character as it is.
func jsString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch c {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\b':
				b.WriteString(`\b`)
			case '\f':
				b.WriteString(`\f`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if c < 0x20 {
					fmt.Fprintf(b, `\u%04x`, c)
				} else {
					b.WriteByte(c)
				}
			}
			i++
			continue
		}
		if c == 0xED && i+2 < len(s) && s[i+1] >= 0xA0 {
			u := rune(c&0x0F)<<12 | rune(s[i+1]&0x3F)<<6 | rune(s[i+2]&0x3F)
			fmt.Fprintf(b, `\u%04x`, u)
			i += 3
			continue
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		b.WriteString(s[i : i+n])
		i += n
	}
	b.WriteByte('"')
}

// units is a string as JavaScript holds it, in UTF-16 code units.
func units(s string) []uint16 {
	var out []uint16
	for i := 0; i < len(s); {
		c := s[i]
		if c == 0xED && i+2 < len(s) && s[i+1] >= 0xA0 {
			out = append(out, uint16(rune(c&0x0F)<<12|rune(s[i+1]&0x3F)<<6|rune(s[i+2]&0x3F)))
			i += 3
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		if r >= 0x10000 {
			a, z := utf16.EncodeRune(r)
			out = append(out, uint16(a), uint16(z))
		} else {
			out = append(out, uint16(r))
		}
		i += n
	}
	return out
}

// fromUnits is the string of UTF-16 code units, a lone surrogate kept in
// its three-byte form.
func fromUnits(u []uint16) string {
	var b []byte
	for i := 0; i < len(u); i++ {
		c := rune(u[i])
		switch {
		case c >= 0xD800 && c < 0xDC00 && i+1 < len(u) && u[i+1] >= 0xDC00 && u[i+1] < 0xE000:
			b = utf8.AppendRune(b, utf16.DecodeRune(c, rune(u[i+1])))
			i++
		case c >= 0xD800 && c < 0xE000:
			b = append(b, 0xED, byte(0x80|(c>>6)&0x3F), byte(0x80|c&0x3F))
		default:
			b = utf8.AppendRune(b, c)
		}
	}
	return string(b)
}

// jsLess compares two strings as JavaScript's < does, by UTF-16 code
// units.
func jsLess(a, b string) bool {
	ua, ub := units(a), units(b)
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}
