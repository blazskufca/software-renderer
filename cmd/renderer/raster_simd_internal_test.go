//go:build goexperiment.simd

package main

import (
	"math"
	"simd"
	"testing"

	"github.com/blazskufca/software-renderer/internal/light"
)

// TestSIMDLightApplyIntensity pins the vector colour modulation to the scalar
// light.LightApplyIntensity it replaces. This is the subtlest piece of the SIMD
// rasterizers — it relies on masked channels staying below 2^24 so the float32
// round trip is exact, on float-to-int conversion truncating toward zero, and on
// the intensity clamp matching — so it is checked exhaustively over a grid
// rather than only through the rasterizer output.
func TestSIMDLightApplyIntensity(t *testing.T) {
	lanes := simd.Float32s{}.Len()

	colors := []uint32{
		0x00000000, 0xFFFFFFFF, 0xFF000000, 0x00FFFFFF,
		0xFF010203, 0xFFFEFDFC, 0x80112233, 0xFF7F8081,
		0xFFFF0000, 0xFF00FF00, 0xFF0000FF, 0xFF808080,
		0xFFC0FFEE, 0xFFDEADBE, 0xFF123456, 0xFFABCDEF,
	}
	intensities := []float32{
		-1, -0.0001, 0, 0.0001, 0.1, 0.25, 1.0 / 3.0, 0.5,
		0.75, 0.9999, 1, 1.0001, 2, 1e6,
		float32(math.Inf(-1)), float32(math.Inf(1)),
	}

	in := make([]uint32, lanes)
	out := make([]uint32, lanes)
	fs := make([]float32, lanes)

	for _, f := range intensities {
		for base := 0; base < len(colors); base += lanes {
			for l := range lanes {
				in[l] = colors[(base+l)%len(colors)]
				fs[l] = f
			}
			simdLightApplyIntensity(simd.LoadUint32s(in), simd.LoadFloat32s(fs),
				simd.BroadcastFloat32s(0), simd.BroadcastFloat32s(1)).Store(out)
			for l := range lanes {
				want := light.LightApplyIntensity(in[l], f)
				if out[l] != want {
					t.Errorf("intensity %v, color %#08x: got %#08x, want %#08x", f, in[l], out[l], want)
				}
			}
		}
	}
}
