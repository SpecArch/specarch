package genseed

// A SplitMix64 generator (Steele, Lea and Flood, 2014), written here rather
// than taken from the standard library, so that the rows a seed gives are
// the same bytes on every platform and every Go release: the algorithm is
// a dozen lines and fixed, while a library's generator may change the
// stream it draws from a seed between releases.
type random struct{ state uint64 }

func newRandom(seed uint64) *random { return &random{state: seed} }

// next is the next 64 bits.
func (r *random) next() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// intn is a value in [0, n); n must be positive.
func (r *random) intn(n int64) int64 {
	if n <= 0 {
		return 0
	}
	return int64(r.next() % uint64(n))
}

// in is a value in [lo, hi].
func (r *random) in(lo, hi int64) int64 {
	if hi <= lo {
		return lo
	}
	return lo + r.intn(hi-lo+1)
}

// float is a value in [0, 1).
func (r *random) float() float64 {
	return float64(r.next()>>11) / (1 << 53)
}

// bytes are n bytes.
func (r *random) bytes(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(r.next())
	}
	return out
}

// chance is true once in n.
func (r *random) chance(n int64) bool { return r.intn(n) == 0 }
