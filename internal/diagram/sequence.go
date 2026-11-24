package diagram

import (
	"math"
	"sort"
)

const (
	partMax    = 110.0
	messageMax = 200.0
	noteMax    = 220.0
	seqSize    = 7.5
)

// sequence lays a sequence diagram out: a box per participant at the top
// and the bottom with its lifeline between, the messages in order down
// the page, each arrow with its text above it, and the notes in boxes
// over the lifelines they name.
func (d *Diagram) sequence(m Measure) *Drawing {
	g := &Drawing{}
	n := len(d.Participants)
	at := map[string]int{}
	lines := make([][]string, n)
	ws, hs := make([]float64, n), make([]float64, n)
	boxH := 0.0
	for i, p := range d.Participants {
		at[p.ID] = i
		g.Names = append(g.Names, p.Label)
		lines[i] = wrap(m, p.Label, nodeSize, true, partMax)
		ws[i] = widest(m, lines[i], nodeSize, true) + 2*padX
		hs[i] = float64(len(lines[i]))*nodeSize*lead + 2*padY
		boxH = math.Max(boxH, hs[i])
	}
	gaps := make([]float64, max(n-1, 0))
	for k := range gaps {
		gaps[k] = (ws[k]+ws[k+1])/2 + 18
	}
	type need struct {
		lo, hi int
		w      float64
	}
	var needs []need
	texts := make([][]string, len(d.Steps))
	for i, s := range d.Steps {
		lo, hi := at[s.From], at[s.To]
		if lo > hi {
			lo, hi = hi, lo
		}
		switch {
		case s.Note:
			texts[i] = wrap(m, s.Text, seqSize, false, noteMax)
			w := widest(m, texts[i], seqSize, false) + 12
			switch s.Place {
			case "left of":
				if lo > 0 {
					needs = append(needs, need{lo - 1, lo, w + ws[lo-1]/2 + 10})
				}
			case "right of":
				if lo < n-1 {
					needs = append(needs, need{lo, lo + 1, w + ws[lo+1]/2 + 10})
				}
			default:
				if hi > lo {
					needs = append(needs, need{lo, hi, w - 20})
				}
			}
		case lo == hi:
			texts[i] = wrap(m, s.Text, seqSize, false, messageMax)
			w := widest(m, texts[i], seqSize, false) + 34
			if lo < n-1 {
				needs = append(needs, need{lo, lo + 1, w})
			}
		default:
			texts[i] = wrap(m, s.Text, seqSize, false, messageMax)
			needs = append(needs, need{lo, hi, widest(m, texts[i], seqSize, false) + 24})
		}
	}
	sort.SliceStable(needs, func(i, j int) bool { return needs[i].hi-needs[i].lo < needs[j].hi-needs[j].lo })
	for _, nd := range needs {
		have := 0.0
		for k := nd.lo; k < nd.hi; k++ {
			have += gaps[k]
		}
		if have < nd.w {
			add := (nd.w - have) / float64(nd.hi-nd.lo)
			for k := nd.lo; k < nd.hi; k++ {
				gaps[k] += add
			}
		}
	}
	xs := make([]float64, n)
	if n > 0 {
		xs[0] = ws[0] / 2
	}
	for k := 1; k < n; k++ {
		xs[k] = xs[k-1] + gaps[k-1]
	}

	// The steps, down the page.
	y := boxH + 14
	ys := make([]float64, len(d.Steps))
	for i, s := range d.Steps {
		th := float64(len(texts[i])) * seqSize * lead
		ys[i] = y
		switch {
		case s.Note:
			y += th + 8 + 10
		case s.From == s.To:
			y += th + 3 + 12 + 12
		default:
			y += th + 3 + 12
		}
	}
	bottom := y + 4

	// Lifelines and the boxes at the top and the bottom.
	for i := range d.Participants {
		g.path(Path{Segs: polyline([]Point{{xs[i], boxH}, {xs[i], bottom}}), Stroke: col(groupEdg), Width: 0.6, Dash: true})
	}
	for i := range d.Participants {
		for _, top := range []float64{0, bottom} {
			bx := xs[i] - ws[i]/2
			g.path(Path{Segs: rounded(bx, top+(boxH-hs[i])/2, ws[i], hs[i], 3), Fill: col(headFill), Stroke: col(nodeEdge), Width: 0.8})
			g.centred(m, lines[i], xs[i], top+boxH/2, nodeSize, true, ink)
		}
	}
	for i, s := range d.Steps {
		a, b := at[s.From], at[s.To]
		th := float64(len(texts[i])) * seqSize * lead
		y := ys[i]
		switch {
		case s.Note:
			w := widest(m, texts[i], seqSize, false) + 12
			lo, hi := xs[min(a, b)], xs[max(a, b)]
			var x0, x1 float64
			switch s.Place {
			case "left of":
				x1 = lo - 6
				x0 = x1 - w
			case "right of":
				x0 = lo + 6
				x1 = x0 + w
			default:
				c := (lo + hi) / 2
				span := math.Max(w, hi-lo+30)
				x0, x1 = c-span/2, c+span/2
			}
			g.path(Path{Segs: rect(x0, y, x1-x0, th+8), Fill: col(noteFill), Stroke: col(noteEdge), Width: 0.6})
			g.centred(m, texts[i], (x0+x1)/2, y+(th+8)/2, seqSize, false, ink)
		case a == b:
			x := xs[a]
			ay := y + th + 3
			g.Texts = append(g.Texts, textLines(m, texts[i], x+6, y, seqSize)...)
			g.path(Path{Segs: polyline([]Point{{x, ay}, {x + 26, ay}, {x + 26, ay + 12}, {x + 1, ay + 12}}), Stroke: col(line), Width: 0.8, Dash: s.Dashed})
			if s.Head {
				g.arrowHead(Point{x + 26, ay + 12}, Point{x + 1, ay + 12}, line)
			}
		default:
			x0, x1 := xs[a], xs[b]
			ay := y + th + 3
			g.centred(m, texts[i], (x0+x1)/2, y+th/2, seqSize, false, ink)
			end := x1 - math.Copysign(1, x1-x0)
			g.path(Path{Segs: polyline([]Point{{x0, ay}, {end, ay}}), Stroke: col(line), Width: 0.8, Dash: s.Dashed})
			if s.Head {
				g.arrowHead(Point{x0, ay}, Point{end, ay}, line)
			}
		}
	}
	g.normalise(m)
	return g
}

// textLines are lines of text from (x, top), left-aligned.
func textLines(m Measure, lines []string, x, top, size float64) []Text {
	var out []Text
	y := top + size*0.95
	for _, l := range lines {
		if l != "" {
			out = append(out, Text{X: x, Y: y, Size: size, Color: ink, S: l})
		}
		y += size * lead
	}
	return out
}
