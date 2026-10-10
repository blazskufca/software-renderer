//go:build goexperiment.simd

package raster

import (
	"simd"

	mesh "github.com/blazskufca/software-renderer/internal/face"
)

var laneIndex = [maxLanes]int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

func useSIMD() bool { return !simd.Emulated() && simd.VectorBitSize() <= 32*maxLanes }

// lanes is the float32 vector width.
var lanes = simd.VectorBitSize() / 32

// narrow reports whether every row of the triangle fits in one vector. The
// default view is mostly such triangles, a few pixels wide.
func narrow(s *triSetup) bool { return s.xMax-s.xMin < lanes }

func fillRows(t *Target, s *triSetup, color uint32) {
	switch {
	case !useSIMD():
		fillRowsScalar(t, s, color)
	case narrow(s):
		fillNarrowSIMD(t, s, color)
	default:
		fillRowsSIMD(t, s, color)
	}
}

// Narrow textured triangles stay scalar: a vector kernel pays a full texel
// gather per row (through memory, see gatherMemory) for a few covered pixels,
// which measured 10% slower in the default view than the scalar loop.
func textureRows(t *Target, s *triSetup, tex *mesh.TextureData) {
	if useSIMD() && !narrow(s) {
		textureRowsSIMD(t, s, tex)
		return
	}
	textureRowsScalar(t, s, tex)
}

func wrapLanes(i, n, nMinus1 simd.Int32s, invN simd.Float32s, pow2 bool) simd.Int32s {
	if pow2 {
		return i.And(nMinus1)
	}
	zero := n.Sub(n)
	r := i.Sub(i.ConvertToFloat32().Mul(invN).ConvertToInt32().Mul(n))
	r = r.Sub(n).IfElse(r.GreaterEqual(n), r)
	r = r.Add(n).IfElse(r.Less(zero), r)
	return r.Max(zero).Min(nMinus1)
}

// same fixed-point math as shade; clamping after the conversion avoids float min/max
func shadeLanes(c simd.Uint32s, f simd.Float32s) simd.Uint32s {
	full := simd.BroadcastInt32s(256)
	l := f.Mul(full.ConvertToFloat32()).ConvertToInt32()
	l = l.Max(full.Sub(full)).Min(full)
	rb, g, lb := simd.BroadcastUint32s(0x00FF00FF), simd.BroadcastUint32s(0x0000FF00), l.ToBits()
	return c.And(simd.BroadcastUint32s(0xFF000000)).
		Or(c.And(rb).Mul(lb).ShiftAllRight(8).And(rb)).
		Or(c.And(g).Mul(lb).ShiftAllRight(8).And(g))
}

func gatherMemory(idx simd.Int32s, pix []uint32, idxBuf []int32, texelBuf []uint32) simd.Uint32s {
	idx.Store(idxBuf)
	for i := 0; i < len(idxBuf); i += 4 {
		d, s := (*[4]uint32)(texelBuf[i:i+4]), (*[4]int32)(idxBuf[i:i+4])
		d[0], d[1], d[2], d[3] = pix[s[0]], pix[s[1]], pix[s[2]], pix[s[3]]
	}
	return simd.LoadUint32s(texelBuf)
}

func anyLaneMemory(m simd.Mask32s, buf []int32) bool {
	m.ToInt32s().Store(buf)
	var any int32
	for _, v := range buf {
		any |= v
	}
	return any != 0
}

func fillRowsSIMD(t *Target, s *triSetup, color uint32) {
	var probe simd.Float32s
	n := probe.Len()
	lane := simd.LoadInt32s(laneIndex[:n])
	stepI := simd.BroadcastInt32s(int32(n))
	stepF := stepI.ConvertToFloat32()
	one := simd.BroadcastInt32s(1).ConvertToFloat32()
	zero := one.Sub(one)
	yf := zero
	for y := s.yMin; y <= s.yMax; y, yf = y+1, yf.Add(one) {
		lo, hi := s.edges.next(s.xMin, s.xMax)
		if lo > hi {
			continue
		}
		// broadcast float params here, not in the prologue: avoids legacy-SSE spills (AVX↔SSE penalty)
		iwX, lX := simd.BroadcastFloat32s(s.invW.dx), simd.BroadcastFloat32s(s.light.dx)
		iwRow := simd.BroadcastFloat32s(s.invW.dy).Mul(yf).Add(simd.BroadcastFloat32s(s.invW.base))
		lRow := simd.BroadcastFloat32s(s.light.dy).Mul(yf).Add(simd.BroadcastFloat32s(s.light.base))
		src := simd.BroadcastUint32s(color)
		dst := t.Color[y*t.Width : (y+1)*t.Width]
		depth := t.Depth[y*t.Width : (y+1)*t.Width]
		xa := lo - lo%n
		loV, hiV := simd.BroadcastInt32s(int32(lo)), simd.BroadcastInt32s(int32(hi))
		xi := lane.Add(simd.BroadcastInt32s(int32(xa)))
		xf := lane.Add(simd.BroadcastInt32s(int32(xa - s.xMin))).ConvertToFloat32()
		for ; xa <= hi; xa += n {
			covered := xi.GreaterEqual(loV).And(xi.LessEqual(hiV))
			z := one.Sub(iwRow.Add(iwX.Mul(xf)))
			zOld := simd.LoadFloat32s(depth[xa : xa+n])
			write := covered.And(z.Less(zOld))
			lit := shadeLanes(src, lRow.Add(lX.Mul(xf)))
			lit.IfElse(write, simd.LoadUint32s(dst[xa:xa+n])).Store(dst[xa : xa+n])
			z.IfElse(write, zOld).Store(depth[xa : xa+n])
			xi, xf = xi.Add(stepI), xf.Add(stepF)
		}
	}
	clearUpperBits()
}

func textureRowsSIMD(t *Target, s *triSetup, tex *mesh.TextureData) {
	pow2 := isPow2(tex.Width) && isPow2(tex.Height)
	var idx, hit [maxLanes]int32
	var texel [maxLanes]uint32
	var probe simd.Float32s
	n := probe.Len()
	lane := simd.LoadInt32s(laneIndex[:n]).ConvertToFloat32()
	stepF := simd.BroadcastInt32s(int32(n)).ConvertToFloat32()
	oneI := simd.BroadcastInt32s(1)
	one := oneI.ConvertToFloat32()
	zero := one.Sub(one)
	tw, th := simd.BroadcastInt32s(int32(tex.Width)), simd.BroadcastInt32s(int32(tex.Height))
	twM1, thM1 := tw.Sub(oneI), th.Sub(oneI)
	twF, thF := tw.ConvertToFloat32(), th.ConvertToFloat32()
	invTW, invTH := one.Div(twF), one.Div(thF)
	pix := tex.PixelData
	yf := zero
	for y := s.yMin; y <= s.yMax; y, yf = y+1, yf.Add(one) {
		lo, hi := s.edges.next(s.xMin, s.xMax)
		if lo > hi {
			continue
		}
		// broadcast float params here, not in the prologue: avoids legacy-SSE spills (AVX↔SSE penalty)
		iwX, uwX := simd.BroadcastFloat32s(s.invW.dx), simd.BroadcastFloat32s(s.uw.dx)
		vwX, lX := simd.BroadcastFloat32s(s.vw.dx), simd.BroadcastFloat32s(s.light.dx)
		iwRow := simd.BroadcastFloat32s(s.invW.dy).Mul(yf).Add(simd.BroadcastFloat32s(s.invW.base))
		uwRow := simd.BroadcastFloat32s(s.uw.dy).Mul(yf).Add(simd.BroadcastFloat32s(s.uw.base))
		vwRow := simd.BroadcastFloat32s(s.vw.dy).Mul(yf).Add(simd.BroadcastFloat32s(s.vw.base))
		lRow := simd.BroadcastFloat32s(s.light.dy).Mul(yf).Add(simd.BroadcastFloat32s(s.light.base))
		dst := t.Color[y*t.Width : (y+1)*t.Width]
		depth := t.Depth[y*t.Width : (y+1)*t.Width]
		xa := lo - lo%n
		loF := simd.BroadcastInt32s(int32(lo - s.xMin)).ConvertToFloat32()
		hiF := simd.BroadcastInt32s(int32(hi - s.xMin)).ConvertToFloat32()
		xf := lane.Add(simd.BroadcastInt32s(int32(xa - s.xMin)).ConvertToFloat32())
		for ; xa <= hi; xa, xf = xa+n, xf.Add(stepF) {
			iw := iwRow.Add(iwX.Mul(xf))
			z := one.Sub(iw)
			zOld := simd.LoadFloat32s(depth[xa : xa+n])
			write := xf.GreaterEqual(loF).And(xf.LessEqual(hiF)).And(z.Less(zOld))
			if !anyLane(write, hit[:n]) {
				continue
			}
			rw := one.Div(iw)
			u := uwRow.Add(uwX.Mul(xf)).Mul(rw)
			v := vwRow.Add(vwX.Mul(xf)).Mul(rw)
			tx := wrapLanes(u.Mul(twF).Abs().ConvertToInt32(), tw, twM1, invTW, pow2)
			ty := wrapLanes(v.Mul(thF).Abs().ConvertToInt32(), th, thM1, invTH, pow2)
			texels := gatherLanes(ty.Mul(tw).Add(tx), pix, idx[:n], texel[:n])
			lit := shadeLanes(texels, lRow.Add(lX.Mul(xf)))
			lit.IfElse(write, simd.LoadUint32s(dst[xa:xa+n])).Store(dst[xa : xa+n])
			z.IfElse(write, zOld).Store(depth[xa : xa+n])
		}
	}
	clearUpperBits()
}

// narrowChunk returns the x of the one chunk that holds every row of a narrow
// triangle, and each lane's edge function values for the triangle's first row.
// Coverage comes from the edge functions instead of the span walk: one vector
// add per edge and row, and no per-row bookkeeping.
func narrowChunk(t *Target, s *triSetup, n int) (xa int, x simd.Int32s, e0, e1, e2 simd.Int32s) {
	xa = min(s.xMin, t.Width-n) // Width is a multiple of every n, so the chunk stays in the row
	x = simd.LoadInt32s(laneIndex[:n]).Add(simd.BroadcastInt32s(int32(xa - s.xMin)))
	e0 = simd.BroadcastInt32s(s.edgeDx[0]).Mul(x).Add(simd.BroadcastInt32s(s.edge[0]))
	e1 = simd.BroadcastInt32s(s.edgeDx[1]).Mul(x).Add(simd.BroadcastInt32s(s.edge[1]))
	e2 = simd.BroadcastInt32s(s.edgeDx[2]).Mul(x).Add(simd.BroadcastInt32s(s.edge[2]))
	return
}

// fillNarrowSIMD is fillRowsSIMD for triangles narrower than a vector.
func fillNarrowSIMD(t *Target, s *triSetup, color uint32) {
	var probe simd.Float32s
	n := probe.Len()
	xa, x, e0, e1, e2 := narrowChunk(t, s, n)
	xf := x.ConvertToFloat32()
	zeroI := x.Sub(x)
	one := simd.BroadcastInt32s(1).ConvertToFloat32()
	yf := one.Sub(one)
	for y := s.yMin; y <= s.yMax; y, yf = y+1, yf.Add(one) {
		covered := e0.Or(e1).Or(e2).GreaterEqual(zeroI) // all three edge values >= 0
		e0 = e0.Add(simd.BroadcastInt32s(s.edgeDy[0]))
		e1 = e1.Add(simd.BroadcastInt32s(s.edgeDy[1]))
		e2 = e2.Add(simd.BroadcastInt32s(s.edgeDy[2]))
		iwRow := simd.BroadcastFloat32s(s.invW.dy).Mul(yf).Add(simd.BroadcastFloat32s(s.invW.base))
		lRow := simd.BroadcastFloat32s(s.light.dy).Mul(yf).Add(simd.BroadcastFloat32s(s.light.base))
		row := y*t.Width + xa
		dst, depth := t.Color[row:row+n], t.Depth[row:row+n]
		z := one.Sub(iwRow.Add(simd.BroadcastFloat32s(s.invW.dx).Mul(xf)))
		zOld := simd.LoadFloat32s(depth)
		write := covered.And(z.Less(zOld))
		lit := shadeLanes(simd.BroadcastUint32s(color), lRow.Add(simd.BroadcastFloat32s(s.light.dx).Mul(xf)))
		lit.IfElse(write, simd.LoadUint32s(dst)).Store(dst)
		z.IfElse(write, zOld).Store(depth)
	}
	clearUpperBits()
}
