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
}

func (t *Target) setup(tri *Triangle) (triSetup, bool) {
	a, b, c := &tri.V[0], &tri.V[1], &tri.V[2]
	ax, ay := int(a.X), int(a.Y)
	bx, by := int(b.X), int(b.Y)
	cx, cy := int(c.X), int(c.Y)

	area := edgeCross(ax, ay, bx, by, cx, cy)
	if area <= 0 {
		return triSetup{}, false
	}

	s := triSetup{
		xMin: max(0, min(ax, bx, cx)),
		yMin: max(0, min(ay, by, cy)),
		xMax: min(t.Width-1, max(ax, bx, cx)),
		yMax: min(t.Height-1, max(ay, by, cy)),
	}

	// edge k is opposite vertex k: b→c, c→a, a→b
	pts := [3][2]int{{ax, ay}, {bx, by}, {cx, cy}}
	var edge, bias, dx, dy [3]int
	for k := range 3 {
		sx, sy := pts[(k+1)%3][0], pts[(k+1)%3][1]
		ex, ey := pts[(k+2)%3][0], pts[(k+2)%3][1]
		edge[k] = edgeCross(sx, sy, ex, ey, s.xMin, s.yMin)
		bias[k] = topLeftBias(sx, sy, ex, ey)
		dx[k], dy[k] = sy-ey, ex-sx
	}

	yMin, yMax := s.yMin, s.yMax
	for k := range 3 {
		if dx[k] == 0 {
			yMin, yMax = clipSpan(yMin, yMax, s.yMin, edge[k]+bias[k], dy[k])
		}
	}
	if s.xMin > s.xMax || yMin > yMax {
		return triSetup{}, false
	}
	for k := range 3 {
		edge[k] += dy[k] * (yMin - s.yMin)
	}
	s.yMin, s.yMax = yMin, yMax

	for k := range 3 {
		e, d := edge[k]+bias[k], abs(dx[k])
		if d == 0 {
			s.edges[k] = edgeWalk{q: math.MaxInt32, d: 1, left: true}
			continue
		}
		q, dq := floorDiv(e, d), floorDiv(dy[k], d)
		s.edges[k] = edgeWalk{q: q, m: e - q*d, dq: dq, dm: dy[k] - dq*d, d: d, left: dx[k] > 0}
	}

	inv := 1 / float64(area)
	planeOf := func(va, vb, vc float64) plane {
		k0, k1 := (va-vc)*inv, (vb-vc)*inv
		return plane{
			base: float32(vc + k0*float64(edge[0]) + k1*float64(edge[1])),
			dx:   float32(k0*float64(dx[0]) + k1*float64(dx[1])),
			dy:   float32(k0*float64(dy[0]) + k1*float64(dy[1])),
		}
	}
	aw, bw, cw := float64(a.W), float64(b.W), float64(c.W)
	s.invW = planeOf(1/aw, 1/bw, 1/cw)
	s.uw = planeOf(float64(a.U)/aw, float64(b.U)/bw, float64(c.U)/cw)
	s.vw = planeOf(float64(1-a.V)/aw, float64(1-b.V)/bw, float64(1-c.V)/cw)
	s.light = planeOf(float64(a.Light), float64(b.Light), float64(c.Light))
	return s, true
}

// next returns the covered [lo, hi] of the current row and advances to the next row.
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
	if s, ok := t.setup(tri); ok {
		fillRows(t, &s, tri.Color)
	}
}

func (t *Target) TextureTriangle(tri *Triangle) {
	if tri.Texture == nil || tri.Texture.Width <= 0 || tri.Texture.Height <= 0 {
		return
	}
	if s, ok := t.setup(tri); ok {
		textureRows(t, &s, tri.Texture)
	}
}
