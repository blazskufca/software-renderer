package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/blazskufca/software-renderer/internal/config"
	mesh "github.com/blazskufca/software-renderer/internal/face"
)

// testTri is one rasterizer input: screen-space vertices, the W each vertex
// carries out of the projection (used for depth and perspective-correct UV), UV
// coordinates and per-vertex light intensities. Z is carried for signature
// compatibility only — neither Pineda rasterizer reads it.
type testTri struct {
	x, y, z, w [3]float32
	u, v       [3]float32
	i          [3]float32
	color      uint32
}

const (
	testWidth  = 640
	testHeight = 360
)

func newTestRenderer(tb testing.TB) *Renderer[float32] {
	tb.Helper()
	r := &Renderer[float32]{config: Config{WindowWidth: testWidth, WindowHeight: testHeight}}
	r.allocateFramebuffers()
	r.resetBuffers()
	return r
}

func (r *Renderer[T]) resetBuffers() {
	r.clearColorBuffer(0xFF000000)
	r.clearZBuffer(1)
}

// makeTexture builds a deterministic w*h texture. Sizes that are a power of two
// take the vector-AND wrap in the SIMD rasterizer; others take the scalar
// modulo, so tests should cover both.
func makeTexture(w, h int) *mesh.TextureData {
	pixels := make([]uint32, w*h)
	for i := range pixels {
		x, y := uint32(i%w), uint32(i/w)
		pixels[i] = 0xFF000000 | (x&0xFF)<<16 | (y&0xFF)<<8 | uint32(i&0xFF)
	}
	return &mesh.TextureData{PixelData: pixels, Width: w, Height: h}
}

// triLayout selects how makeTriangles places its triangles.
type triLayout int

const (
	// layoutTiled confines each triangle to its own screen tile, so no two
	// overlap and the depth test never has to break a tie. That is what lets the
	// equivalence tests assert on colour directly.
	layoutTiled triLayout = iota
	// layoutMesh scatters small triangles across the screen with heavy overlap,
	// which is what a real model looks like once projected: the benchmark case.
	layoutMesh
	// layoutLarge spreads each triangle's vertices over the whole screen, giving
	// a few very large overlapping triangles.
	layoutLarge
)

// benchTriangleExtent is how many pixels across a layoutMesh triangle is in the
// standard benchmarks. It sits above the SIMD size cutoffs, so those benchmarks
// measure the vector path rather than the fallback; BenchmarkTriangleSizeSweep
// covers the range around and below the cutoffs.
const benchTriangleExtent = 48.0

// makeTriangles generates n triangles with a fixed seed, laid out per layout.
// extent is the size in pixels of a layoutMesh triangle and is ignored by the
// other layouts.
func makeTriangles(n int, layout triLayout, extent float64) []testTri {
	rng := rand.New(rand.NewPCG(0x5EED, 0xBEEF))
	tris := make([]testTri, 0, n)

	// Tile the screen into a grid big enough to hold n triangles.
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	tileW := float64(testWidth) / float64(cols)
	tileH := float64(testHeight) / float64(cols)

	for len(tris) < n {
		var t testTri
		originX, originY, spanX, spanY := 0.0, 0.0, float64(testWidth), float64(testHeight)
		switch layout {
		case layoutTiled:
			cell := len(tris)
			originX, originY = float64(cell%cols)*tileW, float64(cell/cols)*tileH
			spanX, spanY = tileW, tileH
		case layoutMesh:
			originX = rng.Float64() * (float64(testWidth) - extent)
			originY = rng.Float64() * (float64(testHeight) - extent)
			spanX, spanY = extent, extent
		}
		for k := range 3 {
			t.x[k] = float32(originX + rng.Float64()*spanX)
			t.y[k] = float32(originY + rng.Float64()*spanY)
			t.z[k] = float32(rng.Float64())
			t.w[k] = float32(1 + rng.Float64()*9)
			t.u[k] = float32(rng.Float64())
			t.v[k] = float32(rng.Float64())
			t.i[k] = float32(rng.Float64())
		}
		t.color = 0xFF000000 | uint32(rng.Uint32()&0x00FFFFFF)

		// Both Pineda rasterizers cull area <= 0, so flip the winding rather
		// than emitting triangles that would draw nothing.
		area := (t.x[1]-t.x[0])*(t.y[2]-t.y[0]) - (t.y[1]-t.y[0])*(t.x[2]-t.x[0])
		if area == 0 {
			continue
		}
		if area < 0 {
			t.x[1], t.x[2] = t.x[2], t.x[1]
			t.y[1], t.y[2] = t.y[2], t.y[1]
			t.w[1], t.w[2] = t.w[2], t.w[1]
			t.u[1], t.u[2] = t.u[2], t.u[1]
			t.v[1], t.v[2] = t.v[2], t.v[1]
			t.i[1], t.i[2] = t.i[2], t.i[1]
		}
		tris = append(tris, t)
	}
	return tris
}

func drawFill(r *Renderer[float32], f config.FillRasterizer[float32], tris []testTri) {
	for _, t := range tris {
		f(t.x[0], t.y[0], t.z[0], t.w[0],
			t.x[1], t.y[1], t.z[1], t.w[1],
			t.x[2], t.y[2], t.z[2], t.w[2],
			t.color, t.i[0], t.i[1], t.i[2])
	}
}

func drawTextured(r *Renderer[float32], f config.TexturizedRasterizer[float32], tris []testTri, tex *mesh.TextureData) {
	for _, t := range tris {
		f(t.x[0], t.y[0], t.z[0], t.w[0], t.u[0], t.v[0],
			t.x[1], t.y[1], t.z[1], t.w[1], t.u[1], t.v[1],
			t.x[2], t.y[2], t.z[2], t.w[2], t.u[2], t.v[2],
			t.i[0], t.i[1], t.i[2], tex)
	}
}

// snapshot copies the visible part of both buffers so a second render can be
// compared against it.
func snapshot(r *Renderer[float32]) (color []byte, depth []float32) {
	n := testWidth * testHeight
	color = append(color, r.colorBuffer[:n*4]...)
	depth = append(depth, r.zBufferF32[:n]...)
	return color, depth
}

// compareBuffers reports how far the SIMD render drifted from the scalar one:
// the number of pixels whose colour differs at all, the largest per-channel
// difference among them, and the largest depth difference.
func compareBuffers(wantC, gotC []byte, wantZ, gotZ []float32) (differing int, maxChannel int, maxDepth float64) {
	for i := range wantZ {
		if d := math.Abs(float64(wantZ[i]) - float64(gotZ[i])); d > maxDepth {
			maxDepth = d
		}
		same := true
		for c := range 4 {
			w, g := int(wantC[i*4+c]), int(gotC[i*4+c])
			if w != g {
				same = false
				if d := w - g; d > maxChannel {
					maxChannel = d
				} else if -d > maxChannel {
					maxChannel = -d
				}
			}
		}
		if !same {
			differing++
		}
	}
	return differing, maxChannel, maxDepth
}

func coveredPixels(depth []float32) int {
	n := 0
	for _, z := range depth {
		if z != 1 {
			n++
		}
	}
	return n
}

// TestSIMDFillMatchesScalar renders non-overlapping triangles through the scalar
// and SIMD Pineda fill rasterizers and requires the results to agree. The SIMD
// path interpolates in float32 where the scalar one uses float64, so a colour
// channel is allowed to land one step apart where the intensity sits on a
// truncation boundary; coverage and depth must still agree.
func TestSIMDFillMatchesScalar(t *testing.T) {
	if !simdCompiledIn {
		t.Skip("built without GOEXPERIMENT=simd")
	}
	tris := makeTriangles(64, layoutTiled, 0)

	r := newTestRenderer(t)
	drawFill(r, r.drawFilledTrianglePineda, tris)
	wantC, wantZ := snapshot(r)

	r.resetBuffers()
	drawFill(r, r.drawFilledTrianglePinedaSIMD, tris)
	gotC, gotZ := snapshot(r)

	covered := coveredPixels(wantZ)
	if covered == 0 {
		t.Fatal("scalar rasterizer drew nothing; the fixture is broken")
	}
	if c := coveredPixels(gotZ); c != covered {
		t.Errorf("coverage differs: scalar drew %d pixels, SIMD drew %d", covered, c)
	}

	differing, maxChannel, maxDepth := compareBuffers(wantC, gotC, wantZ, gotZ)
	if maxChannel > 1 {
		t.Errorf("colour differs by up to %d per channel, want <= 1", maxChannel)
	}
	if maxDepth > 1e-5 {
		t.Errorf("depth differs by up to %g, want <= 1e-5", maxDepth)
	}
	if ratio := float64(differing) / float64(covered); ratio > 0.01 {
		t.Errorf("%d of %d covered pixels differ (%.2f%%), want <= 1%%", differing, covered, ratio*100)
	}
	t.Logf("covered=%d differing=%d maxChannelDelta=%d maxDepthDelta=%g", covered, differing, maxChannel, maxDepth)
}

// TestSIMDTexturedMatchesScalar is the textured counterpart of
// TestSIMDFillMatchesScalar. It runs twice: once with a power-of-two texture
// (vector AND wrap) and once without (scalar modulo wrap), so both texel-fetch
// paths are exercised.
func TestSIMDTexturedMatchesScalar(t *testing.T) {
	if !simdCompiledIn {
		t.Skip("built without GOEXPERIMENT=simd")
	}
	for _, tc := range []struct {
		name string
		w, h int
	}{
		{"pow2", 64, 64},
		{"nonPow2", 50, 37},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tex := makeTexture(tc.w, tc.h)
			tris := makeTriangles(64, layoutTiled, 0)

			r := newTestRenderer(t)
			drawTextured(r, r.drawTexturedTrianglePineda, tris, tex)
			wantC, wantZ := snapshot(r)

			r.resetBuffers()
			drawTextured(r, r.drawTexturedTrianglePinedaSIMD, tris, tex)
			gotC, gotZ := snapshot(r)

			covered := coveredPixels(wantZ)
			if covered == 0 {
				t.Fatal("scalar rasterizer drew nothing; the fixture is broken")
			}
			if c := coveredPixels(gotZ); c != covered {
				t.Errorf("coverage differs: scalar drew %d pixels, SIMD drew %d", covered, c)
			}

			differing, maxChannel, maxDepth := compareBuffers(wantC, gotC, wantZ, gotZ)
			if maxDepth > 1e-5 {
				t.Errorf("depth differs by up to %g, want <= 1e-5", maxDepth)
			}
			// A texel index sits on a truncation boundary far more often than a
			// light intensity does, and one step in either direction picks a
			// completely different texel, so bound how many pixels may differ
			// rather than by how much.
			if ratio := float64(differing) / float64(covered); ratio > 0.02 {
				t.Errorf("%d of %d covered pixels differ (%.2f%%), want <= 2%%", differing, covered, ratio*100)
			}
			t.Logf("covered=%d differing=%d maxChannelDelta=%d maxDepthDelta=%g", covered, differing, maxChannel, maxDepth)
		})
	}
}

// TestSIMDCoverageMatchesScalarWhenOverlapping checks the part that must hold
// exactly even when triangles overlap: the set of pixels the rasterizer touches.
// Which triangle wins a contested pixel can legitimately differ, because the two
// paths compute depth at different precisions, but the union of covered pixels
// is decided purely by the integer edge functions and the top-left fill rule,
// which the SIMD path reproduces exactly.
func TestSIMDCoverageMatchesScalarWhenOverlapping(t *testing.T) {
	if !simdCompiledIn {
		t.Skip("built without GOEXPERIMENT=simd")
	}
	tris := makeTriangles(200, layoutLarge, 0)

	r := newTestRenderer(t)
	drawFill(r, r.drawFilledTrianglePineda, tris)
	_, wantZ := snapshot(r)

	r.resetBuffers()
	drawFill(r, r.drawFilledTrianglePinedaSIMD, tris)
	_, gotZ := snapshot(r)

	mismatched := 0
	for i := range wantZ {
		if (wantZ[i] != 1) != (gotZ[i] != 1) {
			mismatched++
		}
	}
	if mismatched != 0 {
		t.Errorf("%d pixels covered by one rasterizer but not the other", mismatched)
	}
	if covered := coveredPixels(wantZ); covered == 0 {
		t.Fatal("scalar rasterizer drew nothing; the fixture is broken")
	} else {
		t.Logf("covered=%d", covered)
	}
}

// TestSIMDFullScreenTriangleStaysInBounds drives a triangle that covers the
// whole framebuffer, including the final scanline, where a vector store runs off
// the end of the visible buffer into the padding added by allocateFramebuffers.
// It also checks that the overrun never reaches past the padding.
func TestSIMDFullScreenTriangleStaysInBounds(t *testing.T) {
	if !simdCompiledIn {
		t.Skip("built without GOEXPERIMENT=simd")
	}
	r := newTestRenderer(t)
	// Two triangles with positive screen-space area (Y grows downward) covering
	// the entire viewport and overhanging it on every side.
	full := []testTri{
		{x: [3]float32{-50, testWidth + 50, -50}, y: [3]float32{-50, -50, testHeight + 50},
			w: [3]float32{2, 2, 2}, i: [3]float32{1, 1, 1}, color: 0xFFFF0000},
		{x: [3]float32{testWidth + 50, testWidth + 50, -50}, y: [3]float32{-50, testHeight + 50, testHeight + 50},
			w: [3]float32{2, 2, 2}, i: [3]float32{1, 1, 1}, color: 0xFF00FF00},
	}
	drawFill(r, r.drawFilledTrianglePinedaSIMD, full)

	for i := range testWidth * testHeight {
		if r.zBufferF32[i] == 1 {
			t.Fatalf("pixel %d (%d,%d) was not covered by a full-screen triangle", i, i%testWidth, i/testWidth)
		}
	}
	if len(r.zBufferF32) != testWidth*testHeight+simdPadPixels {
		t.Fatalf("depth buffer length %d, want %d", len(r.zBufferF32), testWidth*testHeight+simdPadPixels)
	}
}

func benchFill(b *testing.B, pick func(*Renderer[float32]) config.FillRasterizer[float32]) {
	r := newTestRenderer(b)
	tris := makeTriangles(2000, layoutMesh, benchTriangleExtent)
	f := pick(r)
	b.ReportAllocs()
	for b.Loop() {
		r.resetBuffers()
		drawFill(r, f, tris)
	}
}

func benchTextured(b *testing.B, pick func(*Renderer[float32]) config.TexturizedRasterizer[float32]) {
	r := newTestRenderer(b)
	tris := makeTriangles(2000, layoutMesh, benchTriangleExtent)
	tex := makeTexture(512, 512)
	f := pick(r)
	b.ReportAllocs()
	for b.Loop() {
		r.resetBuffers()
		drawTextured(r, f, tris, tex)
	}
}

func BenchmarkFillScanline(b *testing.B) {
	benchFill(b, func(r *Renderer[float32]) config.FillRasterizer[float32] { return r.drawFilledTriangle })
}

func BenchmarkFillPineda(b *testing.B) {
	benchFill(b, func(r *Renderer[float32]) config.FillRasterizer[float32] { return r.drawFilledTrianglePineda })
}

func BenchmarkFillPinedaSIMD(b *testing.B) {
	if !simdCompiledIn {
		b.Skip("built without GOEXPERIMENT=simd")
	}
	benchFill(b, func(r *Renderer[float32]) config.FillRasterizer[float32] { return r.drawFilledTrianglePinedaSIMD })
}

func BenchmarkTexturedScanline(b *testing.B) {
	benchTextured(b, func(r *Renderer[float32]) config.TexturizedRasterizer[float32] {
		return r.drawTexturedTriangle
	})
}

func BenchmarkTexturedPineda(b *testing.B) {
	benchTextured(b, func(r *Renderer[float32]) config.TexturizedRasterizer[float32] {
		return r.drawTexturedTrianglePineda
	})
}

func BenchmarkTexturedPinedaSIMD(b *testing.B) {
	if !simdCompiledIn {
		b.Skip("built without GOEXPERIMENT=simd")
	}
	benchTextured(b, func(r *Renderer[float32]) config.TexturizedRasterizer[float32] {
		return r.drawTexturedTrianglePinedaSIMD
	})
}

// BenchmarkTriangleSizeSweep runs both rasterizer families over a range of
// triangle sizes so the crossover point can be located: below it the vector
// path's fixed per-triangle setup outweighs its cheaper per-pixel work. The
// simdMinFillArea and simdMinTextureArea cutoffs come from this measurement, so
// re-run it (with the cutoffs temporarily lowered to 0) when retuning them on
// different hardware. Triangle count is scaled so every size covers a comparable
// number of pixels.
func BenchmarkTriangleSizeSweep(b *testing.B) {
	tex := makeTexture(512, 512)
	for _, extent := range []float64{8, 12, 16, 24, 32, 48, 96} {
		n := max(20, int(200000/(extent*extent)))
		tris := makeTriangles(n, layoutMesh, extent)

		for _, kind := range []struct {
			name string
			run  func(*Renderer[float32], bool)
		}{
			{"fill", func(r *Renderer[float32], useSIMD bool) {
				f := r.drawFilledTrianglePineda
				if useSIMD {
					f = r.drawFilledTrianglePinedaSIMD
				}
				drawFill(r, f, tris)
			}},
			{"textured", func(r *Renderer[float32], useSIMD bool) {
				f := r.drawTexturedTrianglePineda
				if useSIMD {
					f = r.drawTexturedTrianglePinedaSIMD
				}
				drawTextured(r, f, tris, tex)
			}},
		} {
			for _, useSIMD := range []bool{false, true} {
				backend := "pineda"
				if useSIMD {
					backend = "simd"
				}
				b.Run(fmt.Sprintf("%s/extent%03.0f/%s", kind.name, extent, backend), func(b *testing.B) {
					if useSIMD && !simdCompiledIn {
						b.Skip("built without GOEXPERIMENT=simd")
					}
					r := newTestRenderer(b)
					for b.Loop() {
						r.resetBuffers()
						kind.run(r, useSIMD)
					}
				})
			}
		}
	}
}
