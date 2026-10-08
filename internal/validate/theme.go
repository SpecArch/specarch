package validate

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/SpecArch/specarch/internal/source"
)

// The theme of docs/ui-design.md: design tokens in the format of the W3C
// Design Tokens Community Group (Format Module 2025.10), the modes that
// give a token another value, and the pairs of colours used as text on a
// background, whose contrast WCAG 2.2 sets a floor for.

// token is one design token: its dotted path, its type, its value and the
// key it is written under.
type token struct {
	path, typ  string
	value, key *yaml.Node
}

// tokenTypes are the token types SpecArch takes from the format.
var tokenTypes = map[string]bool{"color": true, "dimension": true, "fontFamily": true, "fontWeight": true, "duration": true, "number": true}

var (
	aliasPattern = regexp.MustCompile(`^\{([^{}]+)\}$`)
	hexPattern   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	fontWeights  = map[string]bool{"thin": true, "hairline": true, "extra-light": true, "ultra-light": true, "light": true, "normal": true, "regular": true, "book": true,
		"medium": true, "semi-bold": true, "demi-bold": true, "bold": true, "extra-bold": true, "ultra-bold": true, "black": true, "heavy": true, "extra-black": true, "ultra-black": true}
)

// srgbLinear is WCAG 2.2's linear value of each 8-bit sRGB channel: c/12.92
// up to 0.04045, ((c+0.055)/1.055)^2.4 above, with c the channel over 255.
// It is a table so that both builds compute contrast from the same numbers.
var srgbLinear = [256]float64{
	0.0, 0.0003035269835488375, 0.000607053967097675, 0.0009105809506465125,
	0.00121410793419535, 0.0015176349177441874, 0.001821161901293025, 0.0021246888848418626,
	0.0024282158683907, 0.0027317428519395373, 0.003035269835488375, 0.0033465357638991586,
	0.003676507324047435, 0.004024717018496304, 0.0043914420374102925, 0.0047769534806937275,
	0.005181516702338385, 0.005605391624202721, 0.006048833022857052, 0.006512090792594472,
	0.006995410187265385, 0.00749903204322617, 0.008023192985384993, 0.008568125618069302,
	0.009134058702220785, 0.009721217320237844, 0.010329823029626936, 0.01096009400648824,
	0.011612245179743881, 0.012286488356915867, 0.012983032342173007, 0.013702083047289683,
	0.014443843596092541, 0.015208514422912706, 0.015996293365509628, 0.016807375752887377,
	0.017641954488384078, 0.01850022012837969, 0.019382360956935723, 0.02028856305665239,
	0.021219010376003555, 0.022173884793387375, 0.023153366178110403, 0.02415763244850475,
	0.025186859627361623, 0.02624122189484989, 0.02732089163907489, 0.028426039504420787,
	0.029556834437808797, 0.03071344373299362, 0.03189603307301152, 0.03310476657088505,
	0.03433980680868216, 0.03560131487502032, 0.03688945040110002, 0.03820437159534648,
	0.03954623527673283, 0.04091519690685317, 0.042311410620809654, 0.04373502925697345,
	0.04518620438567554, 0.04666508633688008, 0.048171824226889405, 0.04970656598412722,
	0.051269458374043224, 0.05286064702318025, 0.054480276442442355, 0.05612849004960008,
	0.05780543019106721, 0.059511238162981185, 0.061246054231617594, 0.06301001765316766,
	0.06480326669290576, 0.06662593864377288, 0.06847816984440015, 0.07036009569659588,
	0.07227185068231748, 0.07421356838014961, 0.07618538148130781, 0.07818742180518633,
	0.08021982031446831, 0.0822827071298148, 0.08437621154414877, 0.08650046203654974,
	0.08865558628577294, 0.09084171118340768, 0.09305896284668742, 0.09530746663096466,
	0.09758734714186242, 0.09989872824711389, 0.10224173308810128, 0.10461648409110416,
	0.10702310297826759, 0.10946171077829933, 0.11193242783690557, 0.11443537382697372,
	0.11697066775851081, 0.1195384279883456, 0.12213877222960184, 0.12477181756095046,
	0.12743768043564743, 0.13013647669036427, 0.13286832155381792, 0.13563332965520564,
	0.13843161503245183, 0.14126329114027164, 0.14412847085805772, 0.14702726649759498,
	0.14995978981060853, 0.15292615199615014, 0.15592646370782734, 0.15896083506088035,
	0.162029375639111, 0.1651321945016676, 0.1682694001896907, 0.17144110073282254,
	0.17464740365558498, 0.17788841598362912, 0.18116424424986013, 0.1844749945004409,
	0.18782077230067779, 0.19120168274079136, 0.1946178304415757, 0.1980693195599488,
	0.20155625379439707, 0.2050787363903169, 0.20863687014525567, 0.2122307574140551,
	0.21586050011389915, 0.21952619972926918, 0.22322795731680842, 0.22696587351009834,
	0.23074004852434896, 0.23455058216100508, 0.23839757381227095, 0.24228112246555472,
	0.2462013267078354, 0.2501582847299533, 0.2541520943308267, 0.25818285292159576,
	0.262250657529696, 0.2663556048028623, 0.27049779101306576, 0.27467731206038454,
	0.27889426347681034, 0.28314874042999194, 0.2874408377269174, 0.29177064981753587,
	0.29613827079832095, 0.3005437944157764, 0.3049873140698861, 0.30946892281750843,
	0.3139887133757175, 0.31854677812509175, 0.3231432091129507, 0.32777809805654207,
	0.3324515363461792, 0.33716361504833026, 0.34191442490866075, 0.3467040563550295,
	0.3515325995004392, 0.3564001441459434, 0.36130677978350945, 0.3662525955988394,
	0.37123768047414896, 0.3762621229909062, 0.38132601143253, 0.3864294337870489,
	0.3915724777497231, 0.3967552307256268, 0.4019777798321956, 0.40724021190173665,
	0.4125426134839036, 0.41788507084813725, 0.4232676699860715, 0.42869049661390657,
	0.4341536361747488, 0.43965717384091874, 0.44520119451622775, 0.45078578283822335,
	0.4564110231804045, 0.46207699965440685, 0.4677837961121588, 0.4735314961480093,
	0.47932018310082664, 0.4851499400560704, 0.49102084984783545, 0.49693299506087035,
	0.5028864580325684, 0.5088813208549335, 0.5149176653765213, 0.5209955732043541,
	0.527115125705813, 0.533276404010505, 0.539479489012107, 0.5457244613701866,
	0.5520114015119999, 0.5583403896342677, 0.5647115057049289, 0.5711248294648729,
	0.5775804404296505, 0.584078417891164, 0.5906188409193368, 0.5972017883637631,
	0.6038273388553375, 0.6104955708078647, 0.6172065624196509, 0.6239603916750759,
	0.6307571363461467, 0.6375968739940324, 0.644479681970582, 0.6514056374198239,
	0.6583748172794482, 0.6653872982822719, 0.6724431569576873, 0.6795424696330937,
	0.6866853124353132, 0.6938717612919898, 0.7011018919329731, 0.7083757798916867,
	0.7156935005064805, 0.7230551289219689, 0.7304607400903533, 0.7379104087727307,
	0.7454042095403872, 0.7529422167760778, 0.7605245046752922, 0.7681511472475069,
	0.7758222183174234, 0.7835377915261932, 0.79129794033263, 0.7991027380144087,
	0.8069522576692514, 0.8148465722161011, 0.8227857543962833, 0.8307698767746545,
	0.8387990117407399, 0.8468732315098577, 0.8549926081242336, 0.863157213454102,
	0.8713671191987971, 0.8796223968878317, 0.8879231178819664, 0.8962693533742666,
	0.9046611743911491, 0.9130986517934189, 0.9215818562772945, 0.9301108583754234,
	0.9386857284578878, 0.9473065367331996, 0.955973353249286, 0.9646862478944651,
	0.9734452903984123, 0.982250550333117, 0.9911020971138297, 1.0,
}

// tokens walks a group of the token tree, the type a group gives passing
// down to what it holds.
func tokens(group *yaml.Node, prefix, typ string, out *[]token) {
	if t := source.Str(source.Child(group, "$type")); t != "" {
		typ = t
	}
	for _, p := range source.Pairs(group) {
		if strings.HasPrefix(p.Key.Value, "$") {
			continue
		}
		path := p.Key.Value
		if prefix != "" {
			path = prefix + "." + path
		}
		if source.Child(p.Value, "$value") != nil {
			t := typ
			if own := source.Str(source.Child(p.Value, "$type")); own != "" {
				t = own
			}
			*out = append(*out, token{path: path, typ: t, value: source.Child(p.Value, "$value"), key: p.Key})
			continue
		}
		tokens(p.Value, path, typ, out)
	}
}

// number reads a YAML number, or reports false for anything else.
func number(n *yaml.Node) (float64, bool) {
	if n == nil || n.Kind != yaml.ScalarNode || (n.Tag != "!!int" && n.Tag != "!!float") {
		return 0, false
	}
	f, err := strconv.ParseFloat(n.Value, 64)
	return f, err == nil
}

// channel8 is a colour component as its 8-bit value.
func channel8(c float64) int { return int(math.Round(c * 255)) }

// tokenValue says why a value is not one of its type, or "".
func tokenValue(typ string, v *yaml.Node) string {
	switch typ {
	case "color":
		if source.Str(source.Child(v, "colorSpace")) != "srgb" {
			return "a colour here is in the srgb colour space, the one WCAG 2.2 computes contrast in; give colorSpace: srgb and its three components"
		}
		comps := source.Items(source.Child(v, "components"))
		if len(comps) != 3 {
			return "an srgb colour has three components, red, green and blue, each from 0 to 1"
		}
		var rgb [3]int
		for i, c := range comps {
			f, ok := number(c)
			if !ok || f < 0 || f > 1 {
				return "an srgb colour has three components, red, green and blue, each from 0 to 1"
			}
			rgb[i] = channel8(f)
		}
		if a := source.Child(v, "alpha"); a != nil {
			if f, ok := number(a); !ok || f < 0 || f > 1 {
				return "alpha is a number from 0 to 1"
			}
		}
		if h := source.Child(v, "hex"); h != nil {
			if !hexPattern.MatchString(h.Value) {
				return "hex is the six digits of CSS hex notation, such as #1b1f24"
			}
			want := fmt.Sprintf("#%02x%02x%02x", rgb[0], rgb[1], rgb[2])
			if strings.ToLower(h.Value) != want {
				return "hex " + h.Value + " is not the colour the components give, " + want + "; correct one of them"
			}
		}
	case "dimension", "duration":
		units := map[string]bool{"px": true, "rem": true}
		if typ == "duration" {
			units = map[string]bool{"ms": true, "s": true}
		}
		if _, ok := number(source.Child(v, "value")); !ok || !units[source.Str(source.Child(v, "unit"))] {
			if typ == "duration" {
				return "a duration is a value and a unit, ms or s"
			}
			return "a dimension is a value and a unit, px or rem"
		}
	case "fontFamily":
		if v.Kind == yaml.ScalarNode && v.Tag == "!!str" && v.Value != "" {
			return ""
		}
		for _, n := range source.Items(v) {
			if n.Kind != yaml.ScalarNode || n.Tag != "!!str" || n.Value == "" {
				return "a font family is a name, or a list of names in order of preference"
			}
		}
		if v.Kind != yaml.SequenceNode || len(v.Content) == 0 {
			return "a font family is a name, or a list of names in order of preference"
		}
	case "fontWeight":
		if f, ok := number(v); ok && f >= 1 && f <= 1000 {
			return ""
		}
		if v.Kind == yaml.ScalarNode && v.Tag == "!!str" && fontWeights[v.Value] {
			return ""
		}
		return "a font weight is a number from 1 to 1000, or one of the format's names such as normal or bold"
	case "number":
		if _, ok := number(v); !ok {
			return "a number token's value is a number"
		}
	}
	return ""
}

// relativeLuminance is WCAG 2.2's relative luminance of an srgb colour, its
// components taken as 8-bit values. The conversions keep each product
// apart, so no fused multiply-add changes the result between builds.
func relativeLuminance(v *yaml.Node) float64 {
	var lin [3]float64
	for i, c := range source.Items(source.Child(v, "components")) {
		f, _ := number(c)
		lin[i] = srgbLinear[channel8(f)]
	}
	return float64(0.2126*lin[0]) + float64(0.7152*lin[1]) + float64(0.0722*lin[2])
}

// contrast is the ratio of two luminances, the lighter over the darker.
func contrast(a, b float64) float64 {
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}

// ratioText writes a contrast ratio to two decimals, cut rather than
// rounded, so a ratio just below a floor never prints as the floor.
func ratioText(r float64) string {
	h := int(math.Floor(r * 100))
	return fmt.Sprintf("%d.%02d:1", h/100, h%100)
}

// pairFloor is the contrast WCAG 2.2 asks of a pair's use at a level, as a
// number and as written, and the criterion that asks it.
func pairFloor(use, level string) (float64, string, string) {
	switch {
	case use == "control":
		return 3, "3", "1.4.11"
	case use == "largeText" && level == "AAA":
		return 4.5, "4.5", "1.4.6"
	case use == "largeText":
		return 3, "3", "1.4.3"
	case level == "AAA":
		return 7, "7", "1.4.6"
	}
	return 4.5, "4.5", "1.4.3"
}

// checkTheme checks the theme: every token has a type SpecArch takes and a
// value of that type, an alias names a token of the same type and does not
// lead back to itself, a mode gives tokens that exist a value of their
// type, and every pair of colours has the contrast its use asks for at the
// target's level, AA when none is named, in the default mode and each
// other.
func (c *checker) checkTheme(d *design) {
	theme := source.Child(d.root, "theme")
	if theme == nil {
		return
	}
	var list []token
	tokens(source.Child(theme, "tokens"), "", "", &list)
	byPath := map[string]token{}
	for _, t := range list {
		byPath[t.path] = t
	}
	known := map[string]*yaml.Node{}
	for _, t := range list {
		known[t.path] = t.key
	}
	ptr := func(path string, more ...string) string {
		return source.Pointer(append(append([]string{"theme", "tokens"}, strings.Split(path, ".")...), more...)...)
	}
	// resolve follows aliases from a value to a literal, or says why it
	// cannot.
	resolve := func(typ string, v *yaml.Node, overrides map[string]*yaml.Node) (*yaml.Node, string) {
		seen := map[string]bool{}
		for {
			m := aliasPattern.FindStringSubmatch(source.Str(v))
			if v.Kind != yaml.ScalarNode || m == nil {
				return v, ""
			}
			target, ok := byPath[m[1]]
			switch {
			case !ok:
				return nil, m[1] + " is not a token of the theme" + suggest(m[1], known)
			case target.typ != typ:
				return nil, m[1] + " is a " + target.typ + " token, and this one is a " + typ
			case seen[m[1]]:
				return nil, "the aliases lead back to " + m[1]
			}
			seen[m[1]] = true
			v = target.value
			if o := overrides[m[1]]; o != nil {
				v = o
			}
		}
	}
	for _, t := range list {
		switch {
		case t.typ == "":
			c.add(t.key, ptr(t.path), RuleTheme, "%s has no $type, and no group above it gives one; give it the type of its value", t.path)
			continue
		case !tokenTypes[t.typ]:
			c.add(t.key, ptr(t.path), RuleTheme, "%s is a %s token, which SpecArch does not take yet; it takes color, dimension, fontFamily, fontWeight, duration and number", t.path, t.typ)
			continue
		}
		v, why := resolve(t.typ, t.value, nil)
		if why == "" {
			why = tokenValue(t.typ, v)
		}
		if why != "" {
			c.add(t.value, ptr(t.path, "$value"), RuleTheme, "%s: %s", t.path, why)
		}
	}
	modes := map[string]map[string]*yaml.Node{}
	var modeNames []string
	for _, m := range source.Pairs(source.Child(theme, "modes")) {
		modeNames = append(modeNames, m.Key.Value)
		modes[m.Key.Value] = map[string]*yaml.Node{}
		for _, kv := range source.Pairs(m.Value) {
			at := source.Pointer("theme", "modes", m.Key.Value, kv.Key.Value)
			t, ok := byPath[kv.Key.Value]
			if !ok {
				c.add(kv.Key, at, RuleTheme, "%s is not a token of the theme%s", kv.Key.Value, suggest(kv.Key.Value, known))
				continue
			}
			modes[m.Key.Value][kv.Key.Value] = kv.Value
			if !tokenTypes[t.typ] {
				continue
			}
			v, why := resolve(t.typ, kv.Value, modes[m.Key.Value])
			if why == "" {
				why = tokenValue(t.typ, v)
			}
			if why != "" {
				c.add(kv.Value, at, RuleTheme, "%s in the %s mode: %s", kv.Key.Value, m.Key.Value, why)
			}
		}
	}
	level := source.Str(source.Child(source.Child(d.root, "accessibility"), "level"))
	if level == "" {
		level = "AA"
	}
	for i, p := range source.Items(source.Child(theme, "pairs")) {
		at := []string{"theme", "pairs", fmt.Sprint(i)}
		bad := false
		for _, k := range []string{"text", "background"} {
			n := source.Child(p, k)
			switch t, ok := byPath[source.Str(n)]; {
			case !ok:
				c.add(n, source.Pointer(append(at, k)...), RuleTheme, "%s is not a token of the theme%s", source.Str(n), suggest(source.Str(n), known))
				bad = true
			case t.typ != "color":
				c.add(n, source.Pointer(append(at, k)...), RuleTheme, "%s is a %s token, and a pair is of colours", source.Str(n), t.typ)
				bad = true
			}
		}
		if bad {
			continue
		}
		text, background := source.Str(source.Child(p, "text")), source.Str(source.Child(p, "background"))
		use := source.Str(source.Child(p, "use"))
		floor, floorText, criterion := pairFloor(use, level)
		for _, mode := range append([]string{""}, modeNames...) {
			value := func(path string) *yaml.Node {
				v := byPath[path].value
				if o := modes[mode][path]; o != nil {
					v = o
				}
				r, _ := resolve("color", v, modes[mode])
				return r
			}
			tv, bv := value(text), value(background)
			if tv == nil || bv == nil || tokenValue("color", tv) != "" || tokenValue("color", bv) != "" {
				continue // reported with the token
			}
			in := ""
			if mode != "" {
				in = " in the " + mode + " mode"
			}
			opaque := true
			for _, v := range []*yaml.Node{tv, bv} {
				if a, ok := number(source.Child(v, "alpha")); ok && a < 1 {
					opaque = false
				}
			}
			if !opaque {
				c.add(p, source.Pointer(at...), RuleTheme, "%s on %s%s is translucent, so its contrast depends on what lies beneath; pair opaque colours", text, background, in)
				break // once is enough for a pair
			}
			if r := contrast(relativeLuminance(tv), relativeLuminance(bv)); r < floor {
				c.add(p, source.Pointer(at...), RuleTheme, "%s on %s has a contrast of %s%s, below the %s:1 WCAG 2.2 asks of %s at level %s (%s)", text, background, ratioText(r), in, floorText, map[string]string{"text": "text", "largeText": "large text", "control": "the parts of a control"}[use], level, criterion)
			}
		}
	}
}

// PairContrast is a pair of the theme with its contrast in each mode, the
// default first, written to two decimals; a mode in which a colour cannot
// be read has "".
type PairContrast struct {
	Text, Background, Use string
	Ratios                []string
}

// ThemeContrasts are the contrasts of the theme's pairs, for the documents,
// with the names of the modes after the default.
func ThemeContrasts(root *yaml.Node) ([]PairContrast, []string) {
	theme := source.Child(root, "theme")
	var list []token
	tokens(source.Child(theme, "tokens"), "", "", &list)
	byPath := map[string]token{}
	for _, t := range list {
		byPath[t.path] = t
	}
	modes := map[string]map[string]*yaml.Node{}
	var modeNames []string
	for _, m := range source.Pairs(source.Child(theme, "modes")) {
		modeNames = append(modeNames, m.Key.Value)
		modes[m.Key.Value] = map[string]*yaml.Node{}
		for _, kv := range source.Pairs(m.Value) {
			modes[m.Key.Value][kv.Key.Value] = kv.Value
		}
	}
	colour := func(path, mode string) *yaml.Node {
		seen := map[string]bool{}
		for !seen[path] {
			seen[path] = true
			t, ok := byPath[path]
			if !ok || t.typ != "color" {
				return nil
			}
			v := t.value
			if o := modes[mode][path]; o != nil {
				v = o
			}
			m := aliasPattern.FindStringSubmatch(source.Str(v))
			if v.Kind != yaml.ScalarNode || m == nil {
				if tokenValue("color", v) != "" {
					return nil
				}
				return v
			}
			path = m[1]
		}
		return nil
	}
	var out []PairContrast
	for _, p := range source.Items(source.Child(theme, "pairs")) {
		pc := PairContrast{Text: source.Str(source.Child(p, "text")), Background: source.Str(source.Child(p, "background")), Use: source.Str(source.Child(p, "use"))}
		for _, mode := range append([]string{""}, modeNames...) {
			tv, bv := colour(pc.Text, mode), colour(pc.Background, mode)
			if tv == nil || bv == nil {
				pc.Ratios = append(pc.Ratios, "")
				continue
			}
			pc.Ratios = append(pc.Ratios, ratioText(contrast(relativeLuminance(tv), relativeLuminance(bv))))
		}
		out = append(out, pc)
	}
	return out, modeNames
}
