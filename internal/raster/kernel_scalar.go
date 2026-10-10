package raster

import (
	"math"

	mesh "github.com/blazskufca/software-renderer/internal/face"
)

func abs32(f float32) float32 { return math.Float32frombits(math.Float32bits(f) &^ (1 << 31)) }

// light as 0..256 fixed point; R and B share one multiply since their products can't overlap
func shade(c uint32, f float32) uint32 {
	l := uint32(min(max(int32(f*256), 0), 256))
	rb := ((c & 0x00FF00FF) * l >> 8) & 0x00FF00FF
	g := ((c & 0x0000FF00) * l >> 8) & 0x0000FF00
	return c&0xFF000000 | rb | g
}

func isPow2(n int) bool { return n > 0 && n&(n-1) == 0 }

func wrapIndex(i, n int32, invN float32, pow2 bool) int32 {
	if pow2 {
		return i & (n - 1)
	}
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
	for y := s.yMin; y <= s.yMax; y++ {
		lo, hi := s.edges.next(s.xMin, s.xMax)
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
			dst[x] = shade(color, lRow+float32(lX*xf))
			depth[x] = z
		}
	}
}

func textureRowsScalar(t *Target, s *triSetup, tex *mesh.TextureData) {
	tw, th := int32(tex.Width), int32(tex.Height)
	twF, thF := float32(tw), float32(th)
	invTW, invTH := 1/twF, 1/thF
	pix := tex.PixelData
	pow2 := isPow2(tex.Width) && isPow2(tex.Height)
	iwX, uwX, vwX, lX := s.invW.dx, s.uw.dx, s.vw.dx, s.light.dx
	for y := s.yMin; y <= s.yMax; y++ {
		lo, hi := s.edges.next(s.xMin, s.xMax)
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
			tx := wrapIndex(int32(abs32(u*twF)), tw, invTW, pow2)
			ty := wrapIndex(int32(abs32(v*thF)), th, invTH, pow2)
			dst[x] = shade(pix[ty*tw+tx], lRow+float32(lX*xf))
			depth[x] = z
		}
	}
}
