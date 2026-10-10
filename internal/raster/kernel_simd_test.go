//go:build goexperiment.simd

package raster

import (
	"math/rand/v2"
	"simd"
	"testing"
)

func TestSIMDMatchesScalar(t *testing.T) {
	if simd.VectorBitSize() > 32*maxLanes {
		t.Skipf("vector width %d bits exceeds %d lanes", simd.VectorBitSize(), maxLanes)
	}
	t.Logf("vector bits %d, emulated %v", simd.VectorBitSize(), simd.Emulated())

	rng := rand.New(rand.NewPCG(3, 4))
	tris := randomTriangles(rng, 3000, []float32{3, 12, 60, 300}, nil)
	odd, pow2 := randomTexture(rng, 64, 48), randomTexture(rng, 64, 64)

	// a gets the row kernels, c the narrow fill kernel where a triangle
	// qualifies (the row kernels otherwise), b the scalar kernels; all three
	// must agree.
	for _, mode := range []string{"textured 64x48", "textured 64x64", "filled"} {
		tex := odd
		if mode == "textured 64x64" {
			tex = pow2
		}
		a, err := NewTarget(testW, testH)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := NewTarget(testW, testH)
		c, _ := NewTarget(testW, testH)
		for _, tg := range []*Target{a, b, c} {
			tg.Clear(0xFF000000)
		}
		narrowed := 0
		for i := range tris {
			tri := &tris[i]
			var s triSetup
			if !a.setup(tri, &s) {
				continue
			}
			s2, s3 := s, s // the row kernels advance s.edges
			if narrow(&s) {
				narrowed++
			}
			if mode != "filled" {
				textureRowsSIMD(a, &s, tex)
				textureRowsScalar(b, &s2, tex)
				textureRowsSIMD(c, &s3, tex)
			} else {
				fillRowsSIMD(a, &s, tri.Color)
				fillRowsScalar(b, &s2, tri.Color)
				if narrow(&s3) {
					fillNarrowSIMD(c, &s3, tri.Color)
				} else {
					fillRowsSIMD(c, &s3, tri.Color)
				}
			}
		}
		drawn := 0
		for i := range a.Color {
			if a.Depth[i] != 1 {
				drawn++
			}
			for _, got := range []struct {
				name string
				tg   *Target
			}{{"row kernels", a}, {"narrow kernels", c}} {
				if got.tg.Color[i] != b.Color[i] || got.tg.Depth[i] != b.Depth[i] {
					t.Fatalf("%s, %s: first mismatch at %d (x=%d y=%d): simd color %#08x depth %v, scalar color %#08x depth %v",
						mode, got.name, i, i%testW, i/testW, got.tg.Color[i], got.tg.Depth[i], b.Color[i], b.Depth[i])
				}
			}
		}
		if drawn == 0 || narrowed == 0 {
			t.Fatalf("%s: %d pixels drawn, %d narrow triangles", mode, drawn, narrowed)
		}
		t.Logf("%s: %d pixels drawn, %d narrow triangles", mode, drawn, narrowed)
	}
}
