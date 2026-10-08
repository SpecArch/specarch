import Foundation

// The theme of docs/ui-design.md: design tokens in the format of the W3C
// Design Tokens Community Group (Format Module 2025.10), the modes that
// give a token another value, and the pairs of colours used as text on a
// background, whose contrast WCAG 2.2 sets a floor for.

/// One design token: its dotted path, its type, its value and its key.
struct DesignToken {
    let path: String, typ: String
    let value: YNode, key: YNode
}

let tokenTypes: Set<String> = ["color", "dimension", "fontFamily", "fontWeight", "duration", "number"]
let fontWeights: Set<String> = ["thin", "hairline", "extra-light", "ultra-light", "light", "normal", "regular", "book",
                                "medium", "semi-bold", "demi-bold", "bold", "extra-bold", "ultra-bold", "black", "heavy", "extra-black", "ultra-black"]
let aliasPattern = try! NSRegularExpression(pattern: "^\\{([^{}]+)\\}$")
let hexPattern = try! NSRegularExpression(pattern: "^#[0-9a-fA-F]{6}$")

/// WCAG 2.2's linear value of each 8-bit sRGB channel: c/12.92 up to
/// 0.04045, ((c+0.055)/1.055)^2.4 above, with c the channel over 255. It is
/// a table so that both builds compute contrast from the same numbers.
let srgbLinear: [Double] = [
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
    0.9734452903984123, 0.982250550333117, 0.9911020971138297, 1.0
]

/// The alias a value names, or nil.
func aliasOf(_ v: YNode) -> String? {
    guard v.kind == .scalar else { return nil }
    let s = v.value
    guard let m = aliasPattern.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)),
          let r = Range(m.range(at: 1), in: s) else { return nil }
    return String(s[r])
}

func hexMatches(_ s: String) -> Bool {
    hexPattern.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)) != nil
}

/// Walks a group of the token tree, the type a group gives passing down.
func collectTokens(_ group: YNode?, _ prefix: String, _ typ: String, _ out: inout [DesignToken]) {
    var typ = typ
    let t = str(group?.child("$type"))
    if !t.isEmpty { typ = t }
    for p in pairs(group) where !p.key.value.hasPrefix("$") {
        let path = prefix.isEmpty ? p.key.value : prefix + "." + p.key.value
        if let v = p.value.child("$value") {
            let own = str(p.value.child("$type"))
            out.append(DesignToken(path: path, typ: own.isEmpty ? typ : own, value: v, key: p.key))
            continue
        }
        collectTokens(p.value, path, typ, &out)
    }
}

/// A YAML number, or nil for anything else.
func numberOf(_ n: YNode?) -> Double? {
    guard let n, n.kind == .scalar, n.tag == "!!int" || n.tag == "!!float" else { return nil }
    return Double(n.value)
}

func channel8(_ c: Double) -> Int { Int((c * 255).rounded()) }

/// Why a value is not one of its type, or "".
func tokenValue(_ typ: String, _ v: YNode) -> String {
    switch typ {
    case "color":
        if str(v.child("colorSpace")) != "srgb" {
            return "a colour here is in the srgb colour space, the one WCAG 2.2 computes contrast in; give colorSpace: srgb and its three components"
        }
        let comps = items(v.child("components"))
        let three = "an srgb colour has three components, red, green and blue, each from 0 to 1"
        if comps.count != 3 { return three }
        var rgb: [Int] = []
        for c in comps {
            guard let f = numberOf(c), f >= 0, f <= 1 else { return three }
            rgb.append(channel8(f))
        }
        if let a = v.child("alpha") {
            guard let f = numberOf(a), f >= 0, f <= 1 else { return "alpha is a number from 0 to 1" }
        }
        if let h = v.child("hex") {
            if !hexMatches(h.value) { return "hex is the six digits of CSS hex notation, such as #1b1f24" }
            let want = "#" + rgb.map { String(format: "%02x", $0) }.joined()
            if h.value.lowercased() != want {
                return "hex " + h.value + " is not the colour the components give, " + want + "; correct one of them"
            }
        }
    case "dimension", "duration":
        let units: Set<String> = typ == "duration" ? ["ms", "s"] : ["px", "rem"]
        if numberOf(v.child("value")) == nil || !units.contains(str(v.child("unit"))) {
            return typ == "duration" ? "a duration is a value and a unit, ms or s" : "a dimension is a value and a unit, px or rem"
        }
    case "fontFamily":
        if v.kind == .scalar && v.tag == "!!str" && !v.value.isEmpty { return "" }
        let family = "a font family is a name, or a list of names in order of preference"
        for n in v.items where n.kind != .scalar || n.tag != "!!str" || n.value.isEmpty { return family }
        if v.kind != .sequence || v.items.isEmpty { return family }
    case "fontWeight":
        if let f = numberOf(v), f >= 1, f <= 1000 { return "" }
        if v.kind == .scalar && v.tag == "!!str" && fontWeights.contains(v.value) { return "" }
        return "a font weight is a number from 1 to 1000, or one of the format's names such as normal or bold"
    case "number":
        if numberOf(v) == nil { return "a number token's value is a number" }
    default:
        break
    }
    return ""
}

/// WCAG 2.2's relative luminance of an srgb colour, its components taken
/// as 8-bit values.
func relativeLuminance(_ v: YNode) -> Double {
    var lin: [Double] = [0, 0, 0]
    for (i, c) in items(v.child("components")).enumerated() where i < 3 {
        lin[i] = srgbLinear[channel8(numberOf(c) ?? 0)]
    }
    let r = 0.2126 * lin[0], g = 0.7152 * lin[1], b = 0.0722 * lin[2]
    return r + g + b
}

func contrastRatio(_ a: Double, _ b: Double) -> Double {
    let (hi, lo) = a < b ? (b, a) : (a, b)
    return (hi + 0.05) / (lo + 0.05)
}

/// A contrast ratio to two decimals, cut rather than rounded.
func ratioText(_ r: Double) -> String {
    let h = Int((r * 100).rounded(.down))
    return "\(h / 100)." + String(format: "%02d", h % 100) + ":1"
}

/// The contrast WCAG 2.2 asks of a pair's use at a level, as a number and
/// as written, and the criterion that asks it.
func pairFloor(_ use: String, _ level: String) -> (Double, String, String) {
    if use == "control" { return (3, "3", "1.4.11") }
    if use == "largeText" { return level == "AAA" ? (4.5, "4.5", "1.4.6") : (3, "3", "1.4.3") }
    return level == "AAA" ? (7, "7", "1.4.6") : (4.5, "4.5", "1.4.3")
}

extension Checker {
    /// Checks the theme: every token has a type SpecArch takes and a value
    /// of that type, an alias names a token of the same type and does not
    /// lead back to itself, a mode gives tokens that exist a value of their
    /// type, and every pair of colours has the contrast its use asks for at
    /// the target's level, AA when none is named, in every mode.
    func checkTheme(_ d: Design) {
        guard let theme = d.root.child("theme") else { return }
        var list: [DesignToken] = []
        collectTokens(theme.child("tokens"), "", "", &list)
        var byPath: [String: DesignToken] = [:]
        var known: [String: YNode] = [:]
        for t in list { byPath[t.path] = t; known[t.path] = t.key }
        func ptr(_ path: String, _ more: [String] = []) -> String {
            pointer(["theme", "tokens"] + path.split(separator: ".", omittingEmptySubsequences: false).map(String.init) + more)
        }
        func resolve(_ typ: String, _ v: YNode, _ overrides: [String: YNode]) -> (YNode?, String) {
            var v = v
            var seen = Set<String>()
            while let alias = aliasOf(v) {
                guard let target = byPath[alias] else { return (nil, alias + " is not a token of the theme" + suggest(alias, known)) }
                if target.typ != typ { return (nil, alias + " is a " + target.typ + " token, and this one is a " + typ) }
                if seen.contains(alias) { return (nil, "the aliases lead back to " + alias) }
                seen.insert(alias)
                v = overrides[alias] ?? target.value
            }
            return (v, "")
        }
        for t in list {
            if t.typ.isEmpty {
                add(t.key, ptr(t.path), .theme, "\(t.path) has no $type, and no group above it gives one; give it the type of its value")
                continue
            }
            if !tokenTypes.contains(t.typ) {
                add(t.key, ptr(t.path), .theme, "\(t.path) is a \(t.typ) token, which SpecArch does not take yet; it takes color, dimension, fontFamily, fontWeight, duration and number")
                continue
            }
            var (v, why) = resolve(t.typ, t.value, [:])
            if why.isEmpty, let v { why = tokenValue(t.typ, v) }
            _ = v
            if !why.isEmpty { add(t.value, ptr(t.path, ["$value"]), .theme, "\(t.path): \(why)") }
        }
        var modes: [String: [String: YNode]] = [:]
        var modeNames: [String] = []
        for m in pairs(theme.child("modes")) {
            modeNames.append(m.key.value)
            modes[m.key.value] = [:]
            for kv in pairs(m.value) {
                let at = pointer("theme", "modes", m.key.value, kv.key.value)
                guard let t = byPath[kv.key.value] else {
                    add(kv.key, at, .theme, "\(kv.key.value) is not a token of the theme\(suggest(kv.key.value, known))")
                    continue
                }
                modes[m.key.value]![kv.key.value] = kv.value
                if !tokenTypes.contains(t.typ) { continue }
                var (v, why) = resolve(t.typ, kv.value, modes[m.key.value]!)
                if why.isEmpty, let v { why = tokenValue(t.typ, v) }
                _ = v
                if !why.isEmpty { add(kv.value, at, .theme, "\(kv.key.value) in the \(m.key.value) mode: \(why)") }
            }
        }
        var level = str(d.root.child("accessibility")?.child("level"))
        if level.isEmpty { level = "AA" }
        for (i, p) in items(theme.child("pairs")).enumerated() {
            let at = ["theme", "pairs", "\(i)"]
            var bad = false
            for k in ["text", "background"] {
                let n = p.child(k)
                if let t = byPath[str(n)] {
                    if t.typ != "color" {
                        add(n, pointer(at + [k]), .theme, "\(str(n)) is a \(t.typ) token, and a pair is of colours")
                        bad = true
                    }
                } else {
                    add(n, pointer(at + [k]), .theme, "\(str(n)) is not a token of the theme\(suggest(str(n), known))")
                    bad = true
                }
            }
            if bad { continue }
            let text = str(p.child("text")), background = str(p.child("background"))
            let use = str(p.child("use"))
            let (floor, floorText, criterion) = pairFloor(use, level)
            for mode in [""] + modeNames {
                let overrides = modes[mode] ?? [:]
                func value(_ path: String) -> YNode? {
                    resolve("color", overrides[path] ?? byPath[path]!.value, overrides).0
                }
                guard let tv = value(text), let bv = value(background),
                      tokenValue("color", tv).isEmpty, tokenValue("color", bv).isEmpty else { continue } // reported with the token
                let inMode = mode.isEmpty ? "" : " in the " + mode + " mode"
                let opaque = [tv, bv].allSatisfy { (numberOf($0.child("alpha")) ?? 1) >= 1 }
                if !opaque {
                    add(p, pointer(at), .theme, "\(text) on \(background)\(inMode) is translucent, so its contrast depends on what lies beneath; pair opaque colours")
                    break // once is enough for a pair
                }
                let r = contrastRatio(relativeLuminance(tv), relativeLuminance(bv))
                if r < floor {
                    let what = ["text": "text", "largeText": "large text", "control": "the parts of a control"][use] ?? ""
                    add(p, pointer(at), .theme, "\(text) on \(background) has a contrast of \(ratioText(r))\(inMode), below the \(floorText):1 WCAG 2.2 asks of \(what) at level \(level) (\(criterion))")
                }
            }
        }
    }
}
