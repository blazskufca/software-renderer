//go:build simd

package pipeline_test

import (
	"math"
	"testing"

	"github.com/blazskufca/software-renderer/internal/config"
	"github.com/blazskufca/software-renderer/internal/pipeline"
	"github.com/blazskufca/software-renderer/internal/raster"
)

// TestSIMDMatchesScalar requires the SIMD geometry paths (GOEXPERIMENT=simd)
// to produce bit-identical triangles to the scalar ones.
func TestSIMDMatchesScalar(t *testing.T) {
	m, p := loadScene(t)
	var got, want []raster.Triangle
	for _, shading := range []config.ShadingMode{config.SHADING_FLAT, config.SHADING_GOURAUD} {
		for _, cull := range []config.CullMode{config.CULL_BACKFACE, config.CULL_NONE} {
			for _, near := range []float32{0, 3, 3.6} {
				p.Shading, p.Cull = shading, cull
				moveCloser(&p, near)
				for f := range frames {
					rotate(m, f)
					pipeline.SetScalarOnly(true)
					want = pipeline.AppendMesh(want[:0], m, &p)
					pipeline.SetScalarOnly(false)
					got = pipeline.AppendMesh(got[:0], m, &p)
					if len(got) != len(want) {
						t.Fatalf("shading %v cull %v near %v frame %d: %d triangles, want %d", shading, cull, near, f, len(got), len(want))
					}
					for i := range got {
						for k := range 3 {
							g, w := got[i].V[k], want[i].V[k]
							if !sameBits(g, w) {
								t.Fatalf("shading %v cull %v near %v frame %d tri %d vertex %d: %+v, want %+v", shading, cull, near, f, i, k, g, w)
							}
						}
					}
				}
			}
		}
	}
}

func sameBits(a, b raster.Vertex) bool {
	for i, x := range [6]float32{a.X, a.Y, a.W, a.U, a.V, a.Light} {
		y := [6]float32{b.X, b.Y, b.W, b.U, b.V, b.Light}[i]
		if math.Float32bits(x) != math.Float32bits(y) {
			return false
		}
	}
	return true
}
