//go:build goexperiment.simd

package raster

import (
	"simd"

	mesh "github.com/blazskufca/software-renderer/internal/face"
)

var laneIndex = [maxLanes]int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

func useSIMD() bool { return !simd.Emulated() && simd.VectorBitSize() <= 32*maxLanes }

func fillRows(t *Target, s *triSetup, color uint32) {
	if useSIMD() {
		fillRowsSIMD(t, s, color)
		return
	}
	fillRowsScalar(t, s, color)
}

func textureRows(t *Target, s *triSetup, tex *mesh.TextureData) {
	if useSIMD() {
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

func shadeLanes(c simd.Uint32s, f simd.Float32s) simd.Uint32s {
	channel := func(m uint32) simd.Uint32s {
		mask := simd.BroadcastUint32s(m)
		return c.And(mask).BitsToInt32().ConvertToFloat32().Mul(f).ConvertToInt32().ToBits().And(mask)
	}
	return c.And(simd.BroadcastUint32s(0xFF000000)).Or(channel(0x00FF0000)).Or(channel(0x0000FF00)).Or(channel(0x000000FF))
}

// the portable API has no "any lane set", so spill the mask and OR the lanes
func anyLane(m simd.Mask32s, buf []int32) bool {
	m.ToInt32s().Store(buf)
	var any int32
	for _, v := range buf {
		any |= v
	}
	return any != 0
}

func fillRowsSIMD(t *Target, s *triSetup, color uint32) {
	edges := s.edges
	var probe simd.Float32s
	n := probe.Len()
	lane := simd.LoadInt32s(laneIndex[:n])
	stepI := simd.BroadcastInt32s(int32(n))
	stepF := stepI.ConvertToFloat32()
	one := simd.BroadcastInt32s(1).ConvertToFloat32()
	zero := one.Sub(one)
	yf := zero
	for y := s.yMin; y <= s.yMax; y, yf = y+1, yf.Add(one) {
		lo, hi := edges.next(s.xMin, s.xMax)
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
			lit := shadeLanes(src, lRow.Add(lX.Mul(xf)).Max(zero).Min(one))
			lit.IfElse(write, simd.LoadUint32s(dst[xa:xa+n])).Store(dst[xa : xa+n])
			z.IfElse(write, zOld).Store(depth[xa : xa+n])
			xi, xf = xi.Add(stepI), xf.Add(stepF)
		}
	}
	clearUpperBits()
}

func textureRowsSIMD(t *Target, s *triSetup, tex *mesh.TextureData) {
	edges := s.edges
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
		lo, hi := edges.next(s.xMin, s.xMax)
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
			ty.Mul(tw).Add(tx).Store(idx[:n])
			for i := range n {
				texel[i] = pix[idx[i]]
			}
			lit := shadeLanes(simd.LoadUint32s(texel[:n]), lRow.Add(lX.Mul(xf)).Max(zero).Min(one))
			lit.IfElse(write, simd.LoadUint32s(dst[xa:xa+n])).Store(dst[xa : xa+n])
			z.IfElse(write, zOld).Store(depth[xa : xa+n])
		}
	}
	clearUpperBits()
}
