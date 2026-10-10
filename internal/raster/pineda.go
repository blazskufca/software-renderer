package raster

import "math"

func edgeCross(ax, ay, bx, by, px, py int) int {
	return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
}

func topLeftBias(sx, sy, ex, ey int) int {
	if (ey == sy && ex > sx) || ey < sy {
		return 0
	}
	return -1
}

type plane struct{ base, dx, dy float32 } // value(x, y) = base + dy*(y-yMin) + dx*(x-xMin)

// edgeWalk tracks q = floor(e/d) for an edge value e that grows by r per row, without dividing.
type edgeWalk struct {
	q, m   int // e = q*d + m, 0 <= m < d
	dq, dm int // r = dq*d + dm, 0 <= dm < d
	d      int
	left   bool // edge bounds the span from the left (at xMin-q), else from the right (at xMin+q)
}

type spanWalk [3]edgeWalk

type triSetup struct {
	xMin, xMax, yMin, yMax int
	edges                  spanWalk // spans start at row yMin
	invW, uw, vw, light    plane
	// The edge functions (with the top-left bias) at (xMin, yMin) and their
	// steps per pixel and per row: a pixel is covered when all three are >= 0.
	// The small-triangle SIMD kernels use these instead of the span walk.
	edge, edgeDx, edgeDy [3]int32
}

// setup fills s for tri and reports whether there is anything to draw. It
// writes s field by field, in place: building it as a value and returning or
// copying it reads it back with wider loads than the stores that built it,
// which stalls store-to-load forwarding, several times per triangle.
func (t *Target) setup(tri *Triangle, s *triSetup) bool {
	a, b, c := &tri.V[0], &tri.V[1], &tri.V[2]
	ax, ay := int(a.X), int(a.Y)
	bx, by := int(b.X), int(b.Y)
	cx, cy := int(c.X), int(c.Y)

	area := edgeCross(ax, ay, bx, by, cx, cy)
	if area <= 0 {
		return false
	}

	s.xMin, s.yMin = max(0, min(ax, bx, cx)), max(0, min(ay, by, cy))
	s.xMax, s.yMax = min(t.Width-1, max(ax, bx, cx)), min(t.Height-1, max(ay, by, cy))

	// edge k is opposite vertex k: b→c, c→a, a→b. Three variables rather than
	// arrays, which the compiler keeps in memory.
	e0 := newEdge(bx, by, cx, cy, s.xMin, s.yMin)
	e1 := newEdge(cx, cy, ax, ay, s.xMin, s.yMin)
	e2 := newEdge(ax, ay, bx, by, s.xMin, s.yMin)

	yMin, yMax := e0.clipRows(s.yMin, s.yMax, s.yMin)
	yMin, yMax = e1.clipRows(yMin, yMax, s.yMin)
	yMin, yMax = e2.clipRows(yMin, yMax, s.yMin)
	if s.xMin > s.xMax || yMin > yMax {
		return false
	}
	shift := yMin - s.yMin
	e0.e, e1.e, e2.e = e0.e+e0.dy*shift, e1.e+e1.dy*shift, e2.e+e2.dy*shift
	s.yMin, s.yMax = yMin, yMax
	s.edge = [3]int32{int32(e0.e + e0.bias), int32(e1.e + e1.bias), int32(e2.e + e2.bias)}
	s.edgeDx = [3]int32{int32(e0.dx), int32(e1.dx), int32(e2.dx)}
	s.edgeDy = [3]int32{int32(e0.dy), int32(e1.dy), int32(e2.dy)}
	s.edges[0].init(e0)
	s.edges[1].init(e1)
	s.edges[2].init(e2)

	inv := 1 / float64(area)
	fe0, fe1 := float64(e0.e), float64(e1.e)
	fdx0, fdx1, fdy0, fdy1 := float64(e0.dx), float64(e1.dx), float64(e0.dy), float64(e1.dy)
	setPlane := func(p *plane, va, vb, vc float64) {
		k0, k1 := (va-vc)*inv, (vb-vc)*inv
		p.base = float32(vc + k0*fe0 + k1*fe1)
		p.dx = float32(k0*fdx0 + k1*fdx1)
		p.dy = float32(k0*fdy0 + k1*fdy1)
	}
	aw, bw, cw := float64(a.W), float64(b.W), float64(c.W)
	setPlane(&s.invW, 1/aw, 1/bw, 1/cw)
	setPlane(&s.uw, float64(a.U)/aw, float64(b.U)/bw, float64(c.U)/cw)
	setPlane(&s.vw, float64(1-a.V)/aw, float64(1-b.V)/bw, float64(1-c.V)/cw)
	setPlane(&s.light, float64(a.Light), float64(b.Light), float64(c.Light))
	t.dirty.add(s.xMin, s.yMin, s.xMax, s.yMax)
	return true
}

// next returns the covered [lo, hi] of the current row and advances to the next
// row. The kernels walk triSetup.edges in place; copying it first would stall
// on the stores setup just made (see setup).
func (w *spanWalk) next(xMin, xMax int) (lo, hi int) {
	lo, hi = xMin, xMax
	for k := range w {
		e := &w[k]
		if e.left {
			lo = max(lo, xMin-e.q)
		} else {
			hi = min(hi, xMin+e.q)
		}
		m := e.m + e.dm - e.d
		carry := ^(m >> 63) & 1 // 1 when e.m+e.dm >= e.d, without a branch
		e.q += e.dq + carry
		e.m = m + e.d&^-carry
	}
	return lo, hi
}

// edgeEq is one edge function: its value at the box corner, its top-left
// bias, and its steps per pixel (dx) and per row (dy).
type edgeEq struct{ e, bias, dx, dy int }

func newEdge(sx, sy, ex, ey, x, y int) edgeEq {
	return edgeEq{e: edgeCross(sx, sy, ex, ey, x, y), bias: topLeftBias(sx, sy, ex, ey), dx: sy - ey, dy: ex - sx}
}

// clipRows narrows the rows [lo, hi] to those a horizontal edge leaves inside.
func (q edgeEq) clipRows(lo, hi, origin int) (int, int) {
	if q.dx != 0 {
		return lo, hi
	}
	return clipSpan(lo, hi, origin, q.e+q.bias, q.dy)
}

// init sets up the walk of the edge's span bound, starting at row yMin.
func (w *edgeWalk) init(q edgeEq) {
	e, d := q.e+q.bias, abs(q.dx)
	if d == 0 {
		w.q, w.m, w.dq, w.dm, w.d, w.left = math.MaxInt32, 0, 0, 0, 1, true
		return
	}
	qq, dq := floorDiv(e, d), floorDiv(q.dy, d)
	w.q, w.m, w.dq, w.dm, w.d, w.left = qq, e-qq*d, dq, q.dy-dq*d, d, q.dx > 0
}

func clipSpan(lo, hi, origin, e, c int) (int, int) {
	switch {
	case c > 0:
		lo = max(lo, origin-floorDiv(e, c))
	case c < 0:
		hi = min(hi, origin+floorDiv(e, -c))
	case e < 0:
		return 1, 0
	}
	return lo, hi
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

func abs(v int) int { return max(v, -v) }

func (t *Target) FillTriangle(tri *Triangle) {
	var s triSetup
	if t.setup(tri, &s) {
		fillRows(t, &s, tri.Color)
	}
}

func (t *Target) TextureTriangle(tri *Triangle) {
	if tri.Texture == nil || tri.Texture.Width <= 0 || tri.Texture.Height <= 0 {
		return
	}
	var s triSetup
	if t.setup(tri, &s) {
		textureRows(t, &s, tri.Texture)
	}
}
