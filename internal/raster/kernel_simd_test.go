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
		a.Clear(0xFF000000)
		b.Clear(0xFF000000)
		for i := range tris {
			tri := &tris[i]
			s, ok := a.setup(tri)
			if !ok {
				continue
			}
			if mode != "filled" {
				textureRowsSIMD(a, &s, tex)
				textureRowsScalar(b, &s, tex)
			} else {
				fillRowsSIMD(a, &s, tri.Color)
				fillRowsScalar(b, &s, tri.Color)
			}
		}
		drawn := 0
		for i := range a.Color {
			if a.Depth[i] != 1 {
				drawn++
			}
			if a.Color[i] != b.Color[i] || a.Depth[i] != b.Depth[i] {
				t.Fatalf("%s: first mismatch at %d (x=%d y=%d): simd color %#08x depth %v, scalar color %#08x depth %v",
					mode, i, i%testW, i/testW, a.Color[i], a.Depth[i], b.Color[i], b.Depth[i])
			}
		}
		if drawn == 0 {
			t.Fatalf("%s: nothing drawn", mode)
		}
		t.Logf("%s: %d pixels drawn", mode, drawn)
	}
}
