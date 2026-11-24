package diagram

import (
	"math"
	"sort"
)

// The graph layout, for flowcharts, state diagrams and entity diagrams:
// layered, as Sugiyama's method lays a directed graph out. A cycle is
// broken at the arrows that lead back, each node takes the rank of the
// longest path to it, every arrow is cut into one link per rank with a
// point between ranks, a label being such a point the size of its text,
// the nodes of a rank are ordered to cross fewer links, and each is
// placed as near the mean of its neighbours as the order allows. Every
// step reads the nodes and arrows in the order the source gives them, so
// one source always gives one drawing.

// vertex is a node of the layout: a node of the diagram, or a point of an
// arrow between two ranks.
type vertex struct {
	n     *Node
	lines []string // a node's or a label's text
	w, h  float64
	rank  int
	lane  int
	pos   int // its index in its rank
	cross float64
	main  float64
	label *Edge // the arrow whose label this point carries
	attrW [3]float64
	index int
}

// link joins two vertices of adjacent ranks.
type link struct{ a, b int }

// chain is an arrow cut into its vertices, from its first rank to its
// last, and whether it runs against its arrow.
type chain struct {
	e        *Edge
	vs       []int
	reversed bool
}

const (
	padX     = 8.0
	padY     = 5.0
	nodeMax  = 120.0
	labelMax = 110.0
	gapCross = 14.0
	gapDummy = 6.0
	laneGap  = 26.0
	attrSize = 7.0
	attrRow  = 10.5
	headRow  = 15.0
)

// measureNode gives a vertex of a node its text and size.
func measureNode(m Measure, n *Node, v *vertex) {
	if n.Shape == Table {
		v.lines = []string{n.ID}
		nameW := m(n.ID, nodeSize, true) + 2*padX
		for _, a := range n.Attrs {
			v.attrW[0] = math.Max(v.attrW[0], m(a.Type, attrSize, false))
			v.attrW[1] = math.Max(v.attrW[1], m(a.Name, attrSize, false))
			v.attrW[2] = math.Max(v.attrW[2], m(a.Keys, attrSize, false))
		}
		rows := v.attrW[0] + v.attrW[1] + v.attrW[2] + 2*padX + 16
		v.w = math.Max(math.Max(nameW, rows), 60)
		v.h = headRow + float64(len(n.Attrs))*attrRow + 3
		return
	}
	switch n.Shape {
	case Start:
		v.w, v.h = 11, 11
		return
	case End:
		v.w, v.h = 13, 13
		return
	}
	max := nodeMax
	if n.Shape == Diamond {
		max = 80
	}
	v.lines = wrap(m, n.Label, nodeSize, false, max)
	tw := widest(m, v.lines, nodeSize, false)
	th := float64(len(v.lines)) * nodeSize * lead
	w, h := tw+2*padX, th+2*padY
	switch n.Shape {
	case Stadium:
		w += h / 2
	case Circle:
		d := math.Max(tw, th) + 12
		w, h = d, d
	case Cylinder:
		h += 8
	case Subroutine:
		w += 10
	case Diamond:
		w, h = tw+th*1.2+24, th+tw*0.35+16
	case Hexagon:
		w += h * 0.6
	case Parallelogram, Trapezoid:
		w += h * 0.7
	case Asymmetric:
		w += h * 0.45
	}
	v.w, v.h = w, h
}

// graph lays a flowchart, a state diagram or an entity diagram out across
// (LR) or down (TD).
func (d *Diagram) graph(m Measure, dir string, fit Fit) *Drawing {
	across := dir == "LR"
	var vs []*vertex
	of := map[string]int{}
	for i, n := range d.Nodes {
		v := &vertex{n: n, index: i, lane: n.Group + 1}
		measureNode(m, n, v)
		of[n.ID] = len(vs)
		vs = append(vs, v)
	}
	// mainSize and crossSize are a vertex's size along and across the
	// ranks.
	mainSize := func(v *vertex) float64 {
		if across {
			return v.w
		}
		return v.h
	}
	crossSize := func(v *vertex) float64 {
		if across {
			return v.h
		}
		return v.w
	}

	// The arrows between two nodes, and the loops of a node on itself.
	var edges, loops []*Edge
	linked := make([]bool, len(vs))
	for _, e := range d.Edges {
		if e.From == e.To {
			loops = append(loops, e)
			continue
		}
		edges = append(edges, e)
		linked[of[e.From]], linked[of[e.To]] = true, true
	}

	// Break the cycles: an arrow to a node on the path of the depth-first
	// walk leads back, and is ranked the other way.
	out := make([][]int, len(vs))
	for i, e := range edges {
		out[of[e.From]] = append(out[of[e.From]], i)
	}
	back := make([]bool, len(edges))
	state := make([]int, len(vs)) // 0 not seen, 1 on the path, 2 done
	var walk func(u int)
	walk = func(u int) {
		state[u] = 1
		for _, i := range out[u] {
			w := of[edges[i].To]
			switch state[w] {
			case 0:
				walk(w)
			case 1:
				back[i] = true
			}
		}
		state[u] = 2
	}
	// Start from the nodes no arrow reaches, then any left.
	reached := make([]bool, len(vs))
	for _, e := range edges {
		reached[of[e.To]] = true
	}
	for u := range vs {
		if linked[u] && !reached[u] && state[u] == 0 {
			walk(u)
		}
	}
	for u := range vs {
		if linked[u] && state[u] == 0 {
			walk(u)
		}
	}
	dirOf := func(i int) (int, int) {
		a, b := of[edges[i].From], of[edges[i].To]
		if back[i] {
			return b, a
		}
		return a, b
	}

	// Rank: the longest path to each node, then a node no arrow reaches
	// pulled down next to the first of the nodes it leads to.
	indeg := make([]int, len(vs))
	succ := make([][]int, len(vs))
	pred := make([][]int, len(vs))
	for i := range edges {
		a, b := dirOf(i)
		indeg[b]++
		succ[a] = append(succ[a], b)
		pred[b] = append(pred[b], a)
	}
	rank := make([]int, len(vs))
	var topo []int
	queue := []int{}
	for u := range vs {
		if linked[u] && indeg[u] == 0 {
			queue = append(queue, u)
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		topo = append(topo, u)
		for _, w := range succ[u] {
			rank[w] = max(rank[w], rank[u]+1)
			indeg[w]--
			if indeg[w] == 0 {
				queue = append(queue, w)
			}
		}
	}
	for i := len(topo) - 1; i >= 0; i-- {
		u := topo[i]
		if len(pred[u]) == 0 && len(succ[u]) > 0 {
			lo := math.MaxInt
			for _, w := range succ[u] {
				lo = min(lo, rank[w])
			}
			rank[u] = lo - 1
		}
	}
	for u, v := range vs {
		v.rank = rank[u] * 2
	}

	// Cut each arrow into one link per rank; the point after its first
	// rank carries its label.
	var chains []chain
	var links []link
	for i, e := range edges {
		a, b := dirOf(i)
		c := chain{e: e, vs: []int{a}, reversed: back[i]}
		prev := a
		for r := vs[a].rank + 1; r < vs[b].rank; r++ {
			p := &vertex{rank: r, lane: vs[a].lane, index: len(vs)}
			if r == vs[a].rank+1 && e.Label != "" {
				p.label = e
				p.lines = wrap(m, e.Label, labelSize, false, labelMax)
				p.w = widest(m, p.lines, labelSize, false) + 6
				p.h = float64(len(p.lines))*labelSize*lead + 3
			}
			vs = append(vs, p)
			links = append(links, link{prev, len(vs) - 1})
			c.vs = append(c.vs, len(vs)-1)
			prev = len(vs) - 1
		}
		links = append(links, link{prev, b})
		c.vs = append(c.vs, b)
		chains = append(chains, c)
	}

	// The ranks, each in the order of the source, lanes kept together.
	top := 0
	for i, v := range vs {
		if i < len(d.Nodes) && !linked[i] {
			continue
		}
		top = max(top, v.rank)
	}
	ranks := make([][]int, top+1)
	for i, v := range vs {
		if i < len(d.Nodes) && !linked[i] {
			continue
		}
		ranks[v.rank] = append(ranks[v.rank], i)
	}
	up := make([][]int, len(vs))   // neighbours one rank before
	down := make([][]int, len(vs)) // neighbours one rank after
	for _, l := range links {
		down[l.a] = append(down[l.a], l.b)
		up[l.b] = append(up[l.b], l.a)
	}
	setPos := func() {
		for _, r := range ranks {
			for i, u := range r {
				vs[u].pos = i
			}
		}
	}
	setPos()
	order := func(r []int, nb [][]int) {
		key := make(map[int]float64, len(r))
		for _, u := range r {
			if len(nb[u]) == 0 {
				key[u] = float64(vs[u].pos)
				continue
			}
			s := 0.0
			for _, w := range nb[u] {
				s += float64(vs[w].pos)
			}
			key[u] = s / float64(len(nb[u]))
		}
		sort.SliceStable(r, func(i, j int) bool {
			a, b := vs[r[i]], vs[r[j]]
			if a.lane != b.lane {
				return a.lane < b.lane
			}
			return key[r[i]] < key[r[j]]
		})
		for i, u := range r {
			vs[u].pos = i
		}
	}
	for _, r := range ranks {
		sort.SliceStable(r, func(i, j int) bool { return vs[r[i]].lane < vs[r[j]].lane })
	}
	setPos()
	best := crossings(ranks, down, vs)
	saved := snapshot(ranks)
	for round := 0; round < 6 && best > 0; round++ {
		if round%2 == 0 {
			for r := 1; r < len(ranks); r++ {
				order(ranks[r], up)
			}
		} else {
			for r := len(ranks) - 2; r >= 0; r-- {
				order(ranks[r], down)
			}
		}
		if c := crossings(ranks, down, vs); c < best {
			best, saved = c, snapshot(ranks)
		}
	}
	ranks = saved
	setPos()

	// Place each rank's vertices across, each as near the mean of its
	// neighbours as the order and the gaps allow.
	gap := func(a, b *vertex) float64 {
		g := gapCross
		if a.n == nil && a.label == nil || b.n == nil && b.label == nil {
			g = gapDummy
		}
		if a.lane != b.lane {
			g += laneGap
		}
		return g
	}
	for _, r := range ranks {
		c := 0.0
		for i, u := range r {
			if i > 0 {
				c += crossSize(vs[r[i-1]])/2 + gap(vs[r[i-1]], vs[u]) + crossSize(vs[u])/2
			}
			vs[u].cross = c
		}
	}
	place := func(r []int, nb [][]int) {
		want := make([]float64, len(r))
		for i, u := range r {
			want[i] = vs[u].cross
			if len(nb[u]) > 0 {
				s := 0.0
				for _, w := range nb[u] {
					s += vs[w].cross
				}
				want[i] = s / float64(len(nb[u]))
			}
		}
		sep := make([]float64, len(r))
		for i := 1; i < len(r); i++ {
			sep[i] = sep[i-1] + crossSize(vs[r[i-1]])/2 + gap(vs[r[i-1]], vs[r[i]]) + crossSize(vs[r[i]])/2
		}
		got := isotonic(want, sep)
		for i, u := range r {
			vs[u].cross = got[i]
		}
	}
	for round := 0; round < 8; round++ {
		if round%2 == 0 {
			for r := 1; r < len(ranks); r++ {
				place(ranks[r], up)
			}
		} else {
			for r := len(ranks) - 2; r >= 0; r-- {
				place(ranks[r], down)
			}
		}
	}
	// Place the ranks along.
	rankGap := 16.0
	if !across && len(d.Groups) > 0 {
		rankGap = 24
	}
	at := 0.0
	for r, members := range ranks {
		size := 0.0
		for _, u := range members {
			size = math.Max(size, mainSize(vs[u]))
		}
		if r > 0 {
			at += rankGap
		}
		for _, u := range members {
			vs[u].main = at + size/2
		}
		at += size
	}
	xy := func(v *vertex) (float64, float64) {
		if across {
			return v.main, v.cross
		}
		return v.cross, v.main
	}

	// A subgraph's frame holds only its own nodes: a lane whose frame
	// would cover a node or a frame laid out before it moves across, past
	// what it covers.
	type span struct{ m0, m1, c0, c1 float64 }
	boxOf := func(v *vertex) span {
		return span{v.main - mainSize(v)/2, v.main + mainSize(v)/2, v.cross - crossSize(v)/2, v.cross + crossSize(v)/2}
	}
	var laneIDs []int
	members := map[int][]int{}
	for _, r := range ranks {
		for _, u := range r {
			if vs[u].n == nil {
				continue
			}
			l := vs[u].lane
			if _, ok := members[l]; !ok {
				laneIDs = append(laneIDs, l)
			}
			members[l] = append(members[l], u)
		}
	}
	sort.Ints(laneIDs)
	title := labelSize*lead + 4
	var placed []span
	for _, l := range laneIDs {
		var f span
		for k, u := range members[l] {
			b := boxOf(vs[u])
			if k == 0 {
				f = b
				continue
			}
			f = span{math.Min(f.m0, b.m0), math.Max(f.m1, b.m1), math.Min(f.c0, b.c0), math.Max(f.c1, b.c1)}
		}
		if l > 0 {
			f.m0, f.m1, f.c0, f.c1 = f.m0-8, f.m1+8, f.c0-8, f.c1+8
			if across {
				f.c0 -= title
			} else {
				f.m0 -= title
			}
		}
		shift := 0.0
		for moved := true; moved; {
			moved = false
			for _, o := range placed {
				if f.m0 < o.m1+6 && o.m0 < f.m1+6 && f.c0+shift < o.c1+10 && o.c0 < f.c1+shift+10 {
					shift = o.c1 + 10 - f.c0
					moved = true
				}
			}
		}
		if shift != 0 {
			for _, r := range ranks {
				for _, u := range r {
					if vs[u].lane == l {
						vs[u].cross += shift
					}
				}
			}
		}
		f.c0, f.c1 = f.c0+shift, f.c1+shift
		if l > 0 {
			placed = append(placed, f)
		} else {
			for _, u := range members[l] {
				placed = append(placed, boxOf(vs[u]))
			}
		}
	}

	g := &Drawing{}
	for _, n := range d.Nodes {
		if n.Shape != Start && n.Shape != End {
			g.Names = append(g.Names, n.Label)
		}
	}
	box := func(v *vertex) (float64, float64, float64, float64) {
		x, y := xy(v)
		return x - v.w/2, y - v.h/2, v.w, v.h
	}

	// The nodes no arrow touches, in a grid per subgraph below the rest.
	var bottom, right float64
	hasLinked := false
	for i := range d.Nodes {
		if linked[i] {
			x, y, w, h := box(vs[i])
			bottom, right = math.Max(bottom, y+h), math.Max(right, x+w)
			hasLinked = true
		}
	}
	type frame struct {
		x, y, w, h float64
		title      string
	}
	var frames []frame
	{
		blockY := 0.0
		if hasLinked {
			blockY = bottom + 24
		}
		limit := math.Max(right, fit.Width)
		if limit == 0 {
			limit = 480
		}
		bx, rowH := 0.0, 0.0
		for gi := -1; gi < len(d.Groups); gi++ {
			var members []int
			for i, n := range d.Nodes {
				if !linked[i] && n.Group == gi {
					members = append(members, i)
				}
			}
			if len(members) == 0 {
				continue
			}
			cw, ch := 0.0, 0.0
			for _, i := range members {
				cw, ch = math.Max(cw, vs[i].w), math.Max(ch, vs[i].h)
			}
			cw += gapCross
			ch += 12
			cols := len(members)
			for cols > 1 && float64(cols)*cw > limit-16 {
				cols--
			}
			rows := (len(members) + cols - 1) / cols
			pad, title := 0.0, 0.0
			if gi >= 0 {
				pad, title = 8, labelSize*lead+4
			}
			fw, fh := float64(cols)*cw+2*pad, float64(rows)*ch+2*pad+title
			if bx > 0 && bx+fw > limit {
				bx = 0
				blockY += rowH + 16
				rowH = 0
			}
			for k, i := range members {
				cx := bx + pad + float64(k%cols)*cw + cw/2
				cy := blockY + pad + title + float64(k/cols)*ch + ch/2
				if across {
					vs[i].main, vs[i].cross = cx, cy
				} else {
					vs[i].cross, vs[i].main = cx, cy
				}
			}
			if gi >= 0 {
				frames = append(frames, frame{bx, blockY, fw, fh, d.Groups[gi].Title})
			}
			bx += fw + 16
			rowH = math.Max(rowH, fh)
		}
	}
	// The frames of the subgraphs whose nodes are linked.
	for gi, gr := range d.Groups {
		x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for i, n := range d.Nodes {
			if n.Group != gi || !linked[i] {
				continue
			}
			x, y, w, h := box(vs[i])
			x0, y0, x1, y1 = math.Min(x0, x), math.Min(y0, y), math.Max(x1, x+w), math.Max(y1, y+h)
		}
		if math.IsInf(x0, 1) {
			continue
		}
		title := labelSize*lead + 4
		frames = append(frames, frame{x0 - 8, y0 - 8 - title, x1 - x0 + 16, y1 - y0 + 16 + title, gr.Title})
	}
	for _, f := range frames {
		g.path(Path{Segs: rounded(f.x, f.y, f.w, f.h, 4), Fill: col(group), Stroke: col(groupEdg), Width: 0.6})
		g.Texts = append(g.Texts, Text{X: f.x + 6, Y: f.y + labelSize + 3, Size: labelSize, Bold: true, Color: soft, S: f.title})
	}

	// The arrows, under the nodes.
	type labelAt struct {
		v *vertex
		x float64
		y float64
	}
	var labels []labelAt
	paths := make([][]Point, len(chains))
	for ci, c := range chains {
		var pts []Point
		for _, u := range c.vs {
			x, y := xy(vs[u])
			pts = append(pts, Point{x, y})
			if vs[u].label != nil {
				labels = append(labels, labelAt{vs[u], x, y})
			}
		}
		if c.reversed {
			for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
				pts[i], pts[j] = pts[j], pts[i]
			}
		}
		paths[ci] = pts
	}
	ports(paths, chains, vs, of, across, xy)
	for ci, c := range chains {
		pts := paths[ci]
		segs, tail := smooth(pts)
		p := Path{Segs: segs, Stroke: col(line), Width: 0.8, Dash: c.e.Dashed}
		g.path(p)
		if c.e.Head {
			g.arrowHead(tail, pts[len(pts)-1], line)
		}
		if d.Kind == Entity {
			g.cardinality(pts[0], pts[1], c.e.Card[0])
			g.cardinality(pts[len(pts)-1], pts[len(pts)-2], c.e.Card[1])
		}
	}
	for _, l := range labels {
		g.path(Path{Segs: rounded(l.x-l.v.w/2, l.y-l.v.h/2, l.v.w, l.v.h, 2), Fill: col(white)})
		g.centred(m, l.v.lines, l.x, l.y, labelSize, false, soft)
	}
	for _, e := range loops {
		v := vs[of[e.From]]
		x, y, w, _ := box(v)
		rx := x + w
		cy := y + v.h/2
		g.path(Path{Segs: []Seg{{'M', []Point{{rx, cy - 4}}}, {'C', []Point{{rx + 22, cy - 16}, {rx + 22, cy + 16}, {rx + 1, cy + 4}}}}, Stroke: col(line), Width: 0.8, Dash: e.Dashed})
		if e.Head {
			g.arrowHead(Point{rx + 8, cy + 8}, Point{rx + 1, cy + 4}, line)
		}
		if e.Label != "" {
			ls := wrap(m, e.Label, labelSize, false, labelMax)
			lw := widest(m, ls, labelSize, false)
			g.centred(m, ls, rx+20+lw/2, cy, labelSize, false, soft)
		}
	}

	// The nodes.
	for i, n := range d.Nodes {
		g.node(m, n, vs[i], xy)
	}
	g.normalise(m)
	return g
}

// ports gives each arrow the point where it meets its nodes: where
// several arrows meet one side of a box, they are spread along that side
// in the order of where they come from, so their ends and an entity's
// cardinality marks stand apart; else the point where the line to the
// node's centre crosses its outline.
func ports(paths [][]Point, chains []chain, vs []*vertex, of map[string]int, across bool, xy func(*vertex) (float64, float64)) {
	type end struct {
		path, at int // the path and the index of its end point
		toward   Point
	}
	sides := map[[2]int][]end{} // by node and side, -1 before it, 1 after it
	var keys [][2]int
	for pi, pts := range paths {
		for k, at := range [][2]int{{of[chains[pi].e.From], 0}, {of[chains[pi].e.To], len(pts) - 1}} {
			v := vs[at[0]]
			near := pts[1]
			if k == 1 {
				near = pts[len(pts)-2]
			}
			pts[at[1]] = clip(v, near, xy)
			if v.n == nil || !boxLike(v.n.Shape, across) {
				continue
			}
			cx, cy := xy(v)
			side := 1
			if across && near.X < cx || !across && near.Y < cy {
				side = -1
			}
			key := [2]int{at[0], side}
			if _, ok := sides[key]; !ok {
				keys = append(keys, key)
			}
			sides[key] = append(sides[key], end{pi, at[1], near})
		}
	}
	for _, key := range keys {
		ends := sides[key]
		if len(ends) < 2 {
			continue
		}
		v := vs[key[0]]
		cx, cy := xy(v)
		sort.SliceStable(ends, func(i, j int) bool {
			if across {
				return ends[i].toward.Y < ends[j].toward.Y
			}
			return ends[i].toward.X < ends[j].toward.X
		})
		span := v.h * 0.7
		if !across {
			span = v.w * 0.7
		}
		for j, e := range ends {
			off := (float64(j)+1)/float64(len(ends)+1)*span - span/2
			if across {
				paths[e.path][e.at] = Point{cx + float64(key[1])*v.w/2, cy + off}
			} else {
				paths[e.path][e.at] = Point{cx + off, cy + float64(key[1])*v.h/2}
			}
		}
	}
}

// boxLike says whether the sides of a shape that arrows meet, its left
// and right across or its top and bottom down, are straight, so arrows
// can meet it anywhere along one.
func boxLike(s Shape, across bool) bool {
	switch s {
	case Rect, Rounded, Subroutine, Table:
		return true
	case Cylinder:
		return across
	case Stadium, Hexagon, Parallelogram, Trapezoid, Asymmetric:
		return !across
	}
	return false
}

// snapshot copies the order of the ranks.
func snapshot(ranks [][]int) [][]int {
	out := make([][]int, len(ranks))
	for i, r := range ranks {
		out[i] = append([]int(nil), r...)
	}
	return out
}

// crossings counts the links that cross between each pair of adjacent
// ranks.
func crossings(ranks [][]int, down [][]int, vs []*vertex) int {
	n := 0
	for _, r := range ranks {
		var ls [][2]int
		for _, u := range r {
			for _, w := range down[u] {
				ls = append(ls, [2]int{vs[u].pos, vs[w].pos})
			}
		}
		for i := range ls {
			for j := i + 1; j < len(ls); j++ {
				if (ls[i][0]-ls[j][0])*(ls[i][1]-ls[j][1]) < 0 {
					n++
				}
			}
		}
	}
	return n
}

// isotonic places values as near want as they can be while each is at
// least sep[i]-sep[i-1] after the one before it: the least-squares fit
// by pooling adjacent violators.
func isotonic(want, sep []float64) []float64 {
	type block struct {
		sum   float64
		count int
	}
	var bs []block
	for i := range want {
		bs = append(bs, block{want[i] - sep[i], 1})
		for len(bs) > 1 {
			a, b := bs[len(bs)-2], bs[len(bs)-1]
			if a.sum/float64(a.count) <= b.sum/float64(b.count) {
				break
			}
			bs = bs[:len(bs)-2]
			bs = append(bs, block{a.sum + b.sum, a.count + b.count})
		}
	}
	out := make([]float64, 0, len(want))
	for _, b := range bs {
		v := b.sum / float64(b.count)
		for k := 0; k < b.count; k++ {
			out = append(out, v+sep[len(out)])
		}
	}
	return out
}

// clip is where a line from the centre of v toward p leaves v's outline.
func clip(v *vertex, p Point, xy func(*vertex) (float64, float64)) Point {
	cx, cy := xy(v)
	dx, dy := p.X-cx, p.Y-cy
	if dx == 0 && dy == 0 {
		return Point{cx, cy}
	}
	hw, hh := v.w/2, v.h/2
	var t float64
	shape := Rect
	if v.n != nil {
		shape = v.n.Shape
	}
	switch shape {
	case Circle, Start, End:
		t = 1 / math.Sqrt(dx*dx/(hw*hw)+dy*dy/(hh*hh))
	case Diamond:
		t = 1 / (math.Abs(dx)/hw + math.Abs(dy)/hh)
	default:
		t = math.Min(hw/math.Max(math.Abs(dx), 1e-9), hh/math.Max(math.Abs(dy), 1e-9))
	}
	if t > 1 {
		t = 1
	}
	return Point{cx + dx*t, cy + dy*t}
}

// smooth draws a line through points: straight for two, else a curve
// through each of them. It returns the path and the point the last piece
// comes from, for the arrowhead.
func smooth(pts []Point) ([]Seg, Point) {
	if len(pts) == 2 {
		return polyline(pts), pts[0]
	}
	segs := []Seg{{'M', []Point{pts[0]}}}
	var c2 Point
	for i := 0; i+1 < len(pts); i++ {
		p0 := pts[max(i-1, 0)]
		p1, p2 := pts[i], pts[i+1]
		p3 := pts[min(i+2, len(pts)-1)]
		c1 := Point{p1.X + (p2.X-p0.X)/6, p1.Y + (p2.Y-p0.Y)/6}
		c2 = Point{p2.X - (p3.X-p1.X)/6, p2.Y - (p3.Y-p1.Y)/6}
		segs = append(segs, Seg{'C', []Point{c1, c2, p2}})
	}
	return segs, c2
}

// cardinality draws an entity relation's end at p, the line coming from
// q: a bar for one, a ring for zero, a crow's foot for many.
func (g *Drawing) cardinality(p, q Point, card string) {
	dx, dy := q.X-p.X, q.Y-p.Y
	l := math.Hypot(dx, dy)
	if l == 0 || card == "" {
		return
	}
	ux, uy := dx/l, dy/l // from the entity outward
	px, py := -uy, ux
	at := func(d, s float64) Point { return Point{p.X + ux*d + px*s, p.Y + uy*d + py*s} }
	stroke := func(a, b Point) {
		g.path(Path{Segs: polyline([]Point{a, b}), Stroke: col(line), Width: 0.8})
	}
	many := card == "}o" || card == "o{" || card == "}|" || card == "|{"
	zero := card == "|o" || card == "o|" || card == "}o" || card == "o{"
	if many {
		stroke(at(8, 0), at(0, -4))
		stroke(at(8, 0), at(0, 4))
	} else {
		stroke(at(4, -4), at(4, 4))
	}
	if zero {
		c := at(13, 0)
		g.path(Path{Segs: ellipse(c.X, c.Y, 2.8, 2.8), Fill: col(white), Stroke: col(line), Width: 0.8})
	} else {
		stroke(at(11, -4), at(11, 4))
	}
}

// node draws a node in its shape, with its text.
func (g *Drawing) node(m Measure, n *Node, v *vertex, xy func(*vertex) (float64, float64)) {
	cx, cy := xy(v)
	x, y, w, h := cx-v.w/2, cy-v.h/2, v.w, v.h
	fill, edge := col(nodeFill), col(nodeEdge)
	shape := func(s []Seg) { g.path(Path{Segs: s, Fill: fill, Stroke: edge, Width: 0.8}) }
	switch n.Shape {
	case Start:
		g.path(Path{Segs: ellipse(cx, cy, w/2, h/2), Fill: col(ink)})
		return
	case End:
		g.path(Path{Segs: ellipse(cx, cy, w/2, h/2), Fill: col(white), Stroke: col(ink), Width: 0.9})
		g.path(Path{Segs: ellipse(cx, cy, w/2-2.6, h/2-2.6), Fill: col(ink)})
		return
	case Table:
		shape(rect(x, y, w, h))
		g.path(Path{Segs: rect(x, y, w, headRow), Fill: col(headFill), Stroke: edge, Width: 0.8})
		g.centred(m, []string{n.ID}, cx, y+headRow/2, nodeSize, true, ink)
		ry := y + headRow
		for _, a := range n.Attrs {
			base := ry + attrSize + 1.5
			g.Texts = append(g.Texts, Text{X: x + padX, Y: base, Size: attrSize, Color: soft, S: a.Type})
			g.Texts = append(g.Texts, Text{X: x + padX + v.attrW[0] + 8, Y: base, Size: attrSize, Color: ink, S: a.Name})
			if a.Keys != "" {
				g.Texts = append(g.Texts, Text{X: x + padX + v.attrW[0] + v.attrW[1] + 16, Y: base, Size: attrSize, Bold: true, Color: soft, S: a.Keys})
			}
			ry += attrRow
		}
		return
	case Rect:
		shape(rect(x, y, w, h))
	case Rounded:
		shape(rounded(x, y, w, h, 5))
	case Stadium:
		shape(rounded(x, y, w, h, h/2))
	case Circle:
		shape(ellipse(cx, cy, w/2, h/2))
	case Cylinder:
		ry := 4.0
		k := ry * kappa
		hw := w / 2
		shape([]Seg{
			{'M', []Point{{x, y + ry}}},
			{'C', []Point{{x, y + ry - k}, {cx - hw*kappa, y}, {cx, y}}},
			{'C', []Point{{cx + hw*kappa, y}, {x + w, y + ry - k}, {x + w, y + ry}}},
			{'L', []Point{{x + w, y + h - ry}}},
			{'C', []Point{{x + w, y + h - ry + k}, {cx + hw*kappa, y + h}, {cx, y + h}}},
			{'C', []Point{{cx - hw*kappa, y + h}, {x, y + h - ry + k}, {x, y + h - ry}}},
			{'Z', nil},
		})
		g.path(Path{Segs: []Seg{
			{'M', []Point{{x, y + ry}}},
			{'C', []Point{{x, y + ry + k}, {cx - hw*kappa, y + 2*ry}, {cx, y + 2*ry}}},
			{'C', []Point{{cx + hw*kappa, y + 2*ry}, {x + w, y + ry + k}, {x + w, y + ry}}},
		}, Stroke: edge, Width: 0.8})
		cy += ry / 2
	case Subroutine:
		shape(rect(x, y, w, h))
		g.path(Path{Segs: polyline([]Point{{x + 5, y}, {x + 5, y + h}}), Stroke: edge, Width: 0.8})
		g.path(Path{Segs: polyline([]Point{{x + w - 5, y}, {x + w - 5, y + h}}), Stroke: edge, Width: 0.8})
	case Diamond:
		shape(polygon(Point{cx, y}, Point{x + w, cy}, Point{cx, y + h}, Point{x, cy}))
	case Hexagon:
		s := h * 0.3
		shape(polygon(Point{x + s, y}, Point{x + w - s, y}, Point{x + w, cy}, Point{x + w - s, y + h}, Point{x + s, y + h}, Point{x, cy}))
	case Parallelogram:
		s := h * 0.35
		shape(polygon(Point{x + s, y}, Point{x + w, y}, Point{x + w - s, y + h}, Point{x, y + h}))
	case Trapezoid:
		s := h * 0.35
		shape(polygon(Point{x + s, y}, Point{x + w - s, y}, Point{x + w, y + h}, Point{x, y + h}))
	case Asymmetric:
		s := h * 0.45
		shape(polygon(Point{x, y}, Point{x + w, y}, Point{x + w, y + h}, Point{x, y + h}, Point{x + s, cy}))
		cx += s / 2
	}
	g.centred(m, v.lines, cx, cy, nodeSize, false, ink)
}

// normalise moves the drawing so that everything drawn starts a margin
// from its top left corner, and sizes it to hold all of it.
func (g *Drawing) normalise(m Measure) {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	see := func(x, y float64) {
		x0, y0, x1, y1 = math.Min(x0, x), math.Min(y0, y), math.Max(x1, x), math.Max(y1, y)
	}
	for _, p := range g.Paths {
		for _, s := range p.Segs {
			for _, q := range s.Pts {
				see(q.X, q.Y)
			}
		}
	}
	for _, t := range g.Texts {
		see(t.X, t.Y-t.Size)
		see(t.X+m(t.S, t.Size, t.Bold), t.Y+t.Size*0.3)
	}
	if math.IsInf(x0, 1) {
		return
	}
	const margin = 2.0
	dx, dy := margin-x0, margin-y0
	for i := range g.Paths {
		for j := range g.Paths[i].Segs {
			for k := range g.Paths[i].Segs[j].Pts {
				g.Paths[i].Segs[j].Pts[k].X += dx
				g.Paths[i].Segs[j].Pts[k].Y += dy
			}
		}
	}
	for i := range g.Texts {
		g.Texts[i].X += dx
		g.Texts[i].Y += dy
	}
	g.W, g.H = x1-x0+2*margin, y1-y0+2*margin
}
