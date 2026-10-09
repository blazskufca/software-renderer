package raster

import (
	"math"

	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/light"
)

func abs32(f float32) float32 { return math.Float32frombits(math.Float32bits(f) &^ (1 << 31)) }

func wrapIndex(i, n int32, invN float32) int32 {
	r := i - int32(float32(i)*invN)*n
	if r >= n {
		r -= n
	}
	if r < 0 {
		r += n
	}
	return min(max(r, 0), n-1)
}

func fillRowsScalar(t *Target, s *triSetup, color uint32) {
	iwX, lX := s.invW.dx, s.light.dx
	edges := s.edges
	for y := s.yMin; y <= s.yMax; y++ {
		lo, hi := edges.next(s.xMin, s.xMax)
		if lo > hi {
			continue
		}
		yf := float32(y - s.yMin)
		// a + float32(b*c) prevents FMA fusion so scalar == SIMD bit-for-bit
		iwRow := s.invW.base + float32(s.invW.dy*yf)
		lRow := s.light.base + float32(s.light.dy*yf)
		dst := t.Color[y*t.Width : (y+1)*t.Width]
		depth := t.Depth[y*t.Width : (y+1)*t.Width]
		for x := lo; x <= hi; x++ {
			xf := float32(x - s.xMin)
			z := 1 - (iwRow + float32(iwX*xf))
			if !(z < depth[x]) {
				continue
			}
			dst[x] = light.LightApplyIntensity(color, lRow+float32(lX*xf))
			depth[x] = z
		}
	}
}

func textureRowsScalar(t *Target, s *triSetup, tex *mesh.TextureData) {
	tw, th := int32(tex.Width), int32(tex.Height)
	twF, thF := float32(tw), float32(th)
	invTW, invTH := 1/twF, 1/thF
	pix := tex.PixelData
	iwX, uwX, vwX, lX := s.invW.dx, s.uw.dx, s.vw.dx, s.light.dx
	edges := s.edges
	for y := s.yMin; y <= s.yMax; y++ {
		lo, hi := edges.next(s.xMin, s.xMax)
		if lo > hi {
			continue
		}
		yf := float32(y - s.yMin)
		iwRow := s.invW.base + float32(s.invW.dy*yf)
		uwRow := s.uw.base + float32(s.uw.dy*yf)
		vwRow := s.vw.base + float32(s.vw.dy*yf)
		lRow := s.light.base + float32(s.light.dy*yf)
		dst := t.Color[y*t.Width : (y+1)*t.Width]
		depth := t.Depth[y*t.Width : (y+1)*t.Width]
		for x := lo; x <= hi; x++ {
			xf := float32(x - s.xMin)
			iw := iwRow + float32(iwX*xf)
			z := 1 - iw
			if !(z < depth[x]) {
				continue
			}
			rw := 1 / iw
			u := (uwRow + float32(uwX*xf)) * rw
			v := (vwRow + float32(vwX*xf)) * rw
			tx := wrapIndex(int32(abs32(u*twF)), tw, invTW)
			ty := wrapIndex(int32(abs32(v*thF)), th, invTH)
			dst[x] = light.LightApplyIntensity(pix[ty*tw+tx], lRow+float32(lX*xf))
			depth[x] = z
		}
	}
}
