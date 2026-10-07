package validate

import (
	"math/big"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// maxSafe is 2^53 - 1, the largest integer a JavaScript number holds
// exactly.
var maxSafe = big.NewRat(9007199254740991, 1)

// checkIntegers refuses an int64 or uint64 carried as a JSON number unless
// its bounds keep it inside 2^53, where every JSON reader holds it exactly.
func (c *checker) checkIntegers() {
	walk(c.root, nil, func(n *yaml.Node, path []string) {
		format := source.Str(source.Child(n, "format"))
		if format != "int64" && format != "uint64" || !carriesAs(n, "integer") {
			return
		}
		low, high := bound(n, "minimum"), bound(n, "maximum")
		negMax := new(big.Rat).Neg(maxSafe)
		safe := high != nil && high.Cmp(maxSafe) <= 0
		if format == "int64" {
			safe = safe && low != nil && low.Cmp(negMax) >= 0
		}
		if !safe {
			c.add(source.Child(n, "format"), source.Pointer(append(path, "format")...), RuleUnsafeInteger,
				"an %s sent as a JSON number loses digits above 2^53 in JavaScript and other readers; carry it as type: string, format: %s, or bound it with minimum and maximum inside 9007199254740991", format, format)
		}
	})
}

func carriesAs(n *yaml.Node, t string) bool {
	typ := source.Child(n, "type")
	if source.Str(typ) == t {
		return true
	}
	for _, item := range source.Items(typ) {
		if item.Value == t {
			return true
		}
	}
	return false
}

func bound(n *yaml.Node, key string) *big.Rat {
	v := source.Child(n, key)
	if v == nil {
		return nil
	}
	r, ok := new(big.Rat).SetString(v.Value)
	if !ok {
		return nil
	}
	return r
}
