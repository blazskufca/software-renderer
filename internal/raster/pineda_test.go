package raster

import (
	"math"
	"math/rand/v2"
	"testing"

	mesh "github.com/blazskufca/software-renderer/internal/face"
)

const testW, testH = 960, 480

func randomTriangles(rng *rand.Rand, count int, sizes []float32, tex *mesh.TextureData) []Triangle {
	uniform := func(lo, hi float32) float32 { return lo + rng.Float32()*(hi-lo) }
	tris := make([]Triangle, count)
	for i := range tris {
		cx, cy := uniform(-40, testW+40), uniform(-40, testH+40)
		size := sizes[rng.IntN(len(sizes))]
		tri := &tris[i]
		for k := range tri.V {
			tri.V[k] = Vertex{
				X:     cx + uniform(-size/2, size/2),
				Y:     cy + uniform(-size/2, size/2),
				W:     uniform(0.5, 6),
				U:     uniform(-1.5, 2.5),
				V:     uniform(-1.5, 2.5),
				Light: uniform(-0.25, 1.25),
			}
		}
		tri.Color = rng.Uint32()
		tri.Texture = tex
	}
	return tris
}

func randomTexture(rng *rand.Rand, width, height int) *mesh.TextureData {
	pix := make([]uint32, width*height)
	for i := range pix {
		pix[i] = rng.Uint32()
	}
	return &mesh.TextureData{PixelData: pix, Width: width, Height: height}
}

func TestSpanMatchesEdgeFunctions(t *testing.T) {
	target, err := NewTarget(testW, testH)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(1, 2))
	tris := randomTriangles(rng, 1000, []float32{3, 20, 200, 2000}, nil)
	accepted := 0
	for i := range tris {
		tri := &tris[i]
		s, ok := target.setup(tri)
		if !ok {
			continue
		}
		accepted++
		ax, ay := int(tri.V[0].X), int(tri.V[0].Y)
		bx, by := int(tri.V[1].X), int(tri.V[1].Y)
		cx, cy := int(tri.V[2].X), int(tri.V[2].Y)
		bias0, bias1, bias2 := topLeftBias(bx, by, cx, cy), topLeftBias(cx, cy, ax, ay), topLeftBias(ax, ay, bx, by)
		xMin, xMax := max(0, min(ax, bx, cx)), min(testW-1, max(ax, bx, cx))
		yMin, yMax := max(0, min(ay, by, cy)), min(testH-1, max(ay, by, cy))
		edges := s.edges
		for y := yMin; y <= yMax; y++ {
			lo, hi := 1, 0
			if s.yMin <= y && y <= s.yMax {
				lo, hi = edges.next(s.xMin, s.xMax)
			}
			for x := xMin; x <= xMax; x++ {
				inside := edgeCross(bx, by, cx, cy, x, y)+bias0 >= 0 &&
					edgeCross(cx, cy, ax, ay, x, y)+bias1 >= 0 &&
					edgeCross(ax, ay, bx, by, x, y)+bias2 >= 0
				if inSpan := lo <= x && x <= hi; inSpan != inside {
					t.Fatalf("triangle %d %+v: pixel (%d,%d) inside=%v but span=[%d,%d] rows=[%d,%d]", i, tri.V, x, y, inside, lo, hi, s.yMin, s.yMax)
				}
			}
		}
	}
	t.Logf("%d of %d triangles accepted", accepted, len(tris))
}

func newClearedTarget(t *testing.T, w, h int) *Target {
	t.Helper()
	target, err := NewTarget(w, h)
	if err != nil {
		t.Fatal(err)
	}
	target.Clear(0xFF000000)
	return target
}

func TestFillTriangleMatchesReference(t *testing.T) {
	target := newClearedTarget(t, testW, testH)
	rng := rand.New(rand.NewPCG(5, 6))
	tris := randomTriangles(rng, 400, []float32{3, 20, 200, 2000}, nil)
	checked := 0
	for i := range tris {
		tri := &tris[i]
		tri.Color = 0xFFFFFFFF
		target.Clear(0xFF000000)
		target.FillTriangle(tri)

		ax, ay := int(tri.V[0].X), int(tri.V[0].Y)
		bx, by := int(tri.V[1].X), int(tri.V[1].Y)
		cx, cy := int(tri.V[2].X), int(tri.V[2].Y)
		area := edgeCross(ax, ay, bx, by, cx, cy)
		bias0, bias1, bias2 := topLeftBias(bx, by, cx, cy), topLeftBias(cx, cy, ax, ay), topLeftBias(ax, ay, bx, by)
		xMin, xMax := max(0, min(ax, bx, cx)), min(testW-1, max(ax, bx, cx))
		yMin, yMax := max(0, min(ay, by, cy)), min(testH-1, max(ay, by, cy))
		insideCount := 0
		for y := yMin; y <= yMax; y++ {
			for x := xMin; x <= xMax; x++ {
				e0 := edgeCross(bx, by, cx, cy, x, y)
				e1 := edgeCross(cx, cy, ax, ay, x, y)
				e2 := edgeCross(ax, ay, bx, by, x, y)
				inside := area > 0 && e0+bias0 >= 0 && e1+bias1 >= 0 && e2+bias2 >= 0
				idx := y*testW + x
				if drawn := target.Depth[idx] != 1; drawn != inside {
					t.Fatalf("triangle %d %+v: pixel (%d,%d) inside=%v drawn=%v", i, tri.V, x, y, inside, drawn)
				}
				if !inside {
					continue
				}
				insideCount++
				b0 := float64(e0) / float64(area)
				b1 := float64(e1) / float64(area)
				b := [3]float64{b0, b1, 1 - b0 - b1}
				invW, light := 0.0, 0.0
				for k := range 3 {
					invW += b[k] / float64(tri.V[k].W)
					light += b[k] * float64(tri.V[k].Light)
				}
				want := 1 - invW
				if d := math.Abs(float64(target.Depth[idx]) - want); d > 1e-4*max(1, math.Abs(want)) {
					t.Fatalf("triangle %d: pixel (%d,%d) depth %v, want %v", i, x, y, target.Depth[idx], want)
				}
				l := min(max(int(math.Floor(light*256)), 0), 256)
				wantR := 255 * l >> 8
				if gotR := int(target.Color[idx] & 0xFF); gotR-wantR > 1 || wantR-gotR > 1 {
					t.Fatalf("triangle %d: pixel (%d,%d) red %d, want %d±1", i, x, y, gotR, wantR)
				}
				checked++
			}
		}
		total := 0
		for _, d := range target.Depth {
			if d != 1 {
				total++
			}
		}
		if total != insideCount {
			t.Fatalf("triangle %d %+v: %d pixels drawn, want %d", i, tri.V, total, insideCount)
		}
	}
	if checked == 0 {
		t.Fatal("no pixels checked")
	}
}

func TestSharedEdgesCoveredOnce(t *testing.T) {
	target := newClearedTarget(t, testW, testH)
	rng := rand.New(rand.NewPCG(11, 12))
	const cell = 40
	nx, ny := (testW+80)/cell, (testH+80)/cell
	pts := make([][2]float32, (nx+1)*(ny+1))
	for j := range ny + 1 {
		for i := range nx + 1 {
			x, y := float32(-40+i*cell), float32(-40+j*cell)
			if i > 0 && i < nx && j > 0 && j < ny {
				x += rng.Float32()*20 - 10
				y += rng.Float32()*20 - 10
			}
			pts[j*(nx+1)+i] = [2]float32{x, y}
		}
	}
	vert := func(i, j int) Vertex {
		p := pts[j*(nx+1)+i]
		return Vertex{X: p[0], Y: p[1], W: 1}
	}
	count := make([]int, testW*testH)
	draw := func(tri Triangle) {
		a, b, c := tri.V[0], tri.V[1], tri.V[2]
		if edgeCross(int(a.X), int(a.Y), int(b.X), int(b.Y), int(c.X), int(c.Y)) < 0 {
			tri.V[1], tri.V[2] = tri.V[2], tri.V[1]
		}
		s, ok := target.setup(&tri)
		if !ok {
			return
		}
		edges := s.edges
		for y := s.yMin; y <= s.yMax; y++ {
			lo, hi := edges.next(s.xMin, s.xMax)
			for x := lo; x <= hi; x++ {
				count[y*testW+x]++
			}
		}
	}
	for j := range ny {
		for i := range nx {
			p00, p10, p11, p01 := vert(i, j), vert(i+1, j), vert(i+1, j+1), vert(i, j+1)
			if rng.IntN(2) == 0 {
				draw(Triangle{V: [3]Vertex{p00, p10, p11}})
				draw(Triangle{V: [3]Vertex{p00, p11, p01}})
			} else {
				draw(Triangle{V: [3]Vertex{p00, p10, p01}})
				draw(Triangle{V: [3]Vertex{p10, p11, p01}})
			}
		}
	}
	for idx, c := range count {
		if c != 1 {
			t.Fatalf("pixel (%d,%d) covered %d times, want 1", idx%testW, idx/testW, c)
		}
	}
}

func TestTextureTriangle(t *testing.T) {
	t.Run("orientation", func(t *testing.T) {
		target := newClearedTarget(t, 64, 64)
		tex := &mesh.TextureData{PixelData: []uint32{0xFF0000FF, 0xFF00FF00, 0xFFFF0000, 0xFFFFFFFF}, Width: 2, Height: 2}
		tl := Vertex{X: 0, Y: 0, W: 1, U: 0, V: 1, Light: 2}
		tr := Vertex{X: 64, Y: 0, W: 1, U: 1, V: 1, Light: 2}
		br := Vertex{X: 64, Y: 64, W: 1, U: 1, V: 0, Light: 2}
		bl := Vertex{X: 0, Y: 64, W: 1, U: 0, V: 0, Light: 2}
		target.TextureTriangle(&Triangle{V: [3]Vertex{tl, tr, br}, Texture: tex})
		target.TextureTriangle(&Triangle{V: [3]Vertex{tl, br, bl}, Texture: tex})
		for k, p := range [][2]int{{16, 16}, {48, 16}, {16, 48}, {48, 48}} {
			if got := target.Color[p[1]*64+p[0]]; got != tex.PixelData[k] {
				t.Fatalf("pixel (%d,%d) = %#08x, want tex[%d] = %#08x", p[0], p[1], got, k, tex.PixelData[k])
			}
		}
		for idx, d := range target.Depth {
			if d == 1 {
				t.Fatalf("pixel (%d,%d) not drawn", idx%64, idx/64)
			}
		}
	})
	t.Run("uniform texture equals fill", func(t *testing.T) {
		const color = 0xFF336699
		tex := &mesh.TextureData{PixelData: []uint32{color}, Width: 1, Height: 1}
		filled := newClearedTarget(t, testW, testH)
		textured := newClearedTarget(t, testW, testH)
		rng := rand.New(rand.NewPCG(13, 14))
		tris := randomTriangles(rng, 500, []float32{3, 20, 200, 2000}, tex)
		for i := range tris {
			tris[i].Color = color
			filled.FillTriangle(&tris[i])
			textured.TextureTriangle(&tris[i])
		}
		for idx := range filled.Color {
			if filled.Color[idx] != textured.Color[idx] || filled.Depth[idx] != textured.Depth[idx] {
				t.Fatalf("pixel (%d,%d): fill %#08x/%v, texture %#08x/%v", idx%testW, idx/testW,
					filled.Color[idx], filled.Depth[idx], textured.Color[idx], textured.Depth[idx])
			}
		}
	})
	t.Run("missing texture draws nothing", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			tex  *mesh.TextureData
		}{
			{name: "nil", tex: nil},
			{name: "zero width", tex: &mesh.TextureData{PixelData: []uint32{0xFFFFFFFF}, Width: 0, Height: 1}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				target := newClearedTarget(t, 64, 64)
				target.TextureTriangle(&Triangle{V: [3]Vertex{
					{X: 0, Y: 0, W: 1, Light: 1},
					{X: 64, Y: 0, W: 1, Light: 1},
					{X: 0, Y: 64, W: 1, Light: 1},
				}, Texture: tc.tex})
				for idx := range target.Color {
					if target.Color[idx] != 0xFF000000 || target.Depth[idx] != 1 {
						t.Fatalf("pixel (%d,%d) = %#08x/%v, want cleared", idx%64, idx/64, target.Color[idx], target.Depth[idx])
					}
				}
			})
		}
	})
}

func TestDepthTestKeepsNearest(t *testing.T) {
	const nearColor, farColor = 0xFF0000FF, 0xFF00FF00
	near := Triangle{V: [3]Vertex{
		{X: 100, Y: 100, W: 2, Light: 2}, {X: 400, Y: 120, W: 2, Light: 2}, {X: 150, Y: 400, W: 2, Light: 2},
	}, Color: nearColor, Texture: &mesh.TextureData{PixelData: []uint32{nearColor}, Width: 1, Height: 1}}
	far := Triangle{V: [3]Vertex{
		{X: 90, Y: 90, W: 5, Light: 2}, {X: 420, Y: 110, W: 5, Light: 2}, {X: 140, Y: 420, W: 5, Light: 2},
	}, Color: farColor, Texture: &mesh.TextureData{PixelData: []uint32{farColor}, Width: 1, Height: 1}}
	for _, mode := range []struct {
		name string
		draw func(*Target, *Triangle)
	}{
		{name: "fill", draw: (*Target).FillTriangle},
		{name: "texture", draw: (*Target).TextureTriangle},
	} {
		t.Run(mode.name, func(t *testing.T) {
			a := newClearedTarget(t, testW, testH)
			b := newClearedTarget(t, testW, testH)
			mode.draw(a, &near)
			mode.draw(a, &far)
			mode.draw(b, &far)
			mode.draw(b, &near)
			for idx := range a.Color {
				if a.Color[idx] != b.Color[idx] || a.Depth[idx] != b.Depth[idx] {
					t.Fatalf("pixel (%d,%d): near-first %#08x/%v, far-first %#08x/%v", idx%testW, idx/testW,
						a.Color[idx], a.Depth[idx], b.Color[idx], b.Depth[idx])
				}
			}
			if got := a.Color[200*testW+200]; got != nearColor {
				t.Fatalf("pixel (200,200) = %#08x, want near color %#08x", got, nearColor)
			}
		})
	}
}
