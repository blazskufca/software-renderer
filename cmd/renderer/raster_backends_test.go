package main

import (
	"math"
	"math/rand/v2"
	"testing"

	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/raster"
)

// rasterizer adapts one backend triangle routine to a common signature; the
// per-backend lists live in render_legacy_test.go and render_simd_test.go.
type rasterizer struct {
	name string
	draw func(r *Renderer[float32], tri *raster.Triangle)
}

func TestTextureRasterizersOrientation(t *testing.T) {
	const ox, oy = 100, 100
	tex := &mesh.TextureData{
		PixelData: []uint32{0xFF0000FF, 0xFF00FF00, 0xFFFF0000, 0xFFFFFFFF},
		Width:     2,
		Height:    2,
	}
	// Light 2 saturates, so texels pass through unmodified.
	tl := raster.Vertex{X: ox, Y: oy, W: 1, U: 0, V: 1, Light: 2}
	tr := raster.Vertex{X: ox + 64, Y: oy, W: 1, U: 1, V: 1, Light: 2}
	br := raster.Vertex{X: ox + 64, Y: oy + 64, W: 1, U: 1, V: 0, Light: 2}
	bl := raster.Vertex{X: ox, Y: oy + 64, W: 1, U: 0, V: 0, Light: 2}
	tris := []raster.Triangle{
		{V: [3]raster.Vertex{tl, tr, br}, Texture: tex},
		{V: [3]raster.Vertex{tl, br, bl}, Texture: tex},
	}
	probes := [4][2]int{{16, 16}, {48, 16}, {16, 48}, {48, 48}}

	for _, rz := range textureRasterizers {
		t.Run(rz.name, func(t *testing.T) {
			r := newTestRenderer(t)
			for i := range tris {
				rz.draw(r, &tris[i])
			}
			for k, p := range probes {
				x, y := ox+p[0], oy+p[1]
				if got := pixelAt(r, y*r.config.WindowWidth+x); got != tex.PixelData[k] {
					t.Fatalf("pixel (%d,%d) = %#08x, want texel %d %#08x", x, y, got, k, tex.PixelData[k])
				}
			}
		})
	}
}

// depthTestTriangles returns a near triangle that lies entirely in front of a
// larger far one.
func depthTestTriangles() (near, far raster.Triangle) {
	near = raster.Triangle{Color: 0xFF0000FF, V: [3]raster.Vertex{
		{X: 100, Y: 100, W: 2, Light: 2}, {X: 400, Y: 120, W: 2, Light: 2}, {X: 150, Y: 400, W: 2, Light: 2},
	}}
	far = raster.Triangle{Color: 0xFF00FF00, V: [3]raster.Vertex{
		{X: 90, Y: 90, W: 5, Light: 2}, {X: 420, Y: 110, W: 5, Light: 2}, {X: 140, Y: 420, W: 5, Light: 2},
	}}
	return near, far
}

func TestFillRasterizersKeepNearest(t *testing.T) {
	near, far := depthTestTriangles()
	for _, rz := range fillRasterizers {
		t.Run(rz.name, func(t *testing.T) {
			a, b := newTestRenderer(t), newTestRenderer(t)
			rz.draw(a, &near)
			rz.draw(a, &far)
			rz.draw(b, &far)
			rz.draw(b, &near)
			w, h := a.config.WindowWidth, a.config.WindowHeight
			for i := range w * h {
				if pa, pb := pixelAt(a, i), pixelAt(b, i); pa != pb {
					t.Fatalf("pixel (%d,%d): near-then-far %#08x, far-then-near %#08x", i%w, i/w, pa, pb)
				}
			}
			if got := pixelAt(a, 200*w+200); got != near.Color {
				t.Fatalf("pixel (200,200) = %#08x, want near color %#08x", got, near.Color)
			}
		})
	}
}

// segmentDistance returns the distance from p to the segment ab.
func segmentDistance(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	s := 0.0
	if l2 := dx*dx + dy*dy; l2 > 0 {
		s = max(0, min(1, ((px-ax)*dx+(py-ay)*dy)/l2))
	}
	return math.Hypot(px-(ax+s*dx), py-(ay+s*dy))
}

func TestScanlineMatchesPinedaAwayFromEdges(t *testing.T) {
	// The scanline path rounds edges differently by up to a pixel; anything
	// farther from an edge than this is a hole or bleed.
	const edgeSlack = 1.5
	var scanline, pineda rasterizer
	for _, rz := range fillRasterizers {
		switch rz.name {
		case "scanline":
			scanline = rz
		case "pineda":
			pineda = rz
		}
	}

	a, b := newTestRenderer(t), newTestRenderer(t)
	rng := rand.New(rand.NewPCG(7, 8))
	in := func(lo, hi float32) float32 { return lo + rng.Float32()*(hi-lo) }
	for i := range 300 {
		var tri raster.Triangle
		for k := range 3 {
			tri.V[k] = raster.Vertex{X: in(-100, 1060), Y: in(-100, 580), W: in(1, 5), Light: 2}
		}
		tri.Color = 0xFFFFFFFF
		sx := func(k int) float64 { return float64(int(tri.V[k].X)) }
		sy := func(k int) float64 { return float64(int(tri.V[k].Y)) }
		area := (sx(1)-sx(0))*(sy(2)-sy(0)) - (sy(1)-sy(0))*(sx(2)-sx(0))
		if area == 0 {
			continue
		}
		if area < 0 {
			tri.V[1], tri.V[2] = tri.V[2], tri.V[1]
		}

		a.clearBuffers()
		b.clearBuffers()
		scanline.draw(a, &tri)
		pineda.draw(b, &tri)
		w, h := a.config.WindowWidth, a.config.WindowHeight
		for p := range w * h {
			if (depthAt(a, p) != 1) == (depthAt(b, p) != 1) {
				continue
			}
			x, y := float64(p%w), float64(p/w)
			d := math.Inf(1)
			for k := range 3 {
				n := (k + 1) % 3
				d = min(d, segmentDistance(x, y, sx(k), sy(k), sx(n), sy(n)))
			}
			if d > edgeSlack {
				t.Fatalf("triangle %d %+v: pixel (%v,%v) coverage differs (scanline %v, pineda %v) %.3f px from nearest edge",
					i, tri.V, x, y, depthAt(a, p) != 1, depthAt(b, p) != 1, d)
			}
		}
	}
}
