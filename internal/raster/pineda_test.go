package raster

import (
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
