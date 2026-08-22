package main

import (
	"testing"

	"github.com/blazskufca/software-renderer/internal/config"
)

// TestSIMDMatchesScalarOnRealMesh drives the actual renderer — the shipped model,
// texture, camera and clipping pipeline — and requires the SIMD rasterizers to
// reproduce what the scalar Pineda ones draw. The synthetic fixtures in
// raster_test.go cover the rasterizers in isolation; this covers the geometry
// they actually receive: sliver triangles, clipped polygons, real UVs and the
// 512x512 colormap.
func TestSIMDMatchesScalarOnRealMesh(t *testing.T) {
	if !simdCompiledIn {
		t.Skip("built without GOEXPERIMENT=simd")
	}
	// Setup loads assets by their repository-relative paths.
	t.Chdir("../..")

	r := NewRenderer[float32]()
	if err := r.Setup(); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if !r.simdAvailable {
		t.Fatal("simd compiled in but the renderer did not enable it")
	}

	// One frame of geometry: transform, light, cull, clip and project.
	r.update()
	if len(r.trianglesToDraw) == 0 {
		t.Fatal("update produced no triangles")
	}
	t.Logf("triangles=%d", len(r.trianglesToDraw))

	pixels := r.config.WindowWidth * r.config.WindowHeight

	render := func(fill config.FillRasterMode, tex config.TextureRasterizer) (color []byte, depth []float32) {
		r.setFillRaster(fill)
		r.setTextureRaster(tex)
		for _, mode := range []config.RenderMode{config.RENDER_FILL_TRIANGLE, config.RENDER_TEXTURED} {
			r.renderMethod = mode
			r.clearColorBuffer(0xFF000000)
			r.clearZBuffer(1)
			for _, tri := range r.trianglesToDraw {
				if mode == config.RENDER_FILL_TRIANGLE {
					r.fillRasterFunc(
						tri.points[0].X, tri.points[0].Y, tri.points[0].Z, tri.points[0].W,
						tri.points[1].X, tri.points[1].Y, tri.points[1].Z, tri.points[1].W,
						tri.points[2].X, tri.points[2].Y, tri.points[2].Z, tri.points[2].W,
						tri.color, tri.lightIntensity[0], tri.lightIntensity[1], tri.lightIntensity[2])
				} else {
					r.textureRasterFunc(
						tri.points[0].X, tri.points[0].Y, tri.points[0].Z, tri.points[0].W, tri.textCoords[0].U, tri.textCoords[0].V,
						tri.points[1].X, tri.points[1].Y, tri.points[1].Z, tri.points[1].W, tri.textCoords[1].U, tri.textCoords[1].V,
						tri.points[2].X, tri.points[2].Y, tri.points[2].Z, tri.points[2].W, tri.textCoords[2].U, tri.textCoords[2].V,
						tri.lightIntensity[0], tri.lightIntensity[1], tri.lightIntensity[2], tri.texture)
				}
			}
			// Keep only the last (textured) frame; the fill pass runs first so
			// both rasterizer families get exercised.
			color = append(color[:0], r.colorBuffer[:pixels*4]...)
			depth = append(depth[:0], r.zBufferF32[:pixels]...)
		}
		return color, depth
	}

	wantC, wantZ := render(config.FILL_PINEDA, config.TEXTURE_PINEDA)
	gotC, gotZ := render(config.FILL_PINEDA_SIMD, config.TEXTURE_PINEDA_SIMD)

	covered := coveredPixels(wantZ)
	if covered == 0 {
		t.Fatal("scalar rasterizer drew nothing")
	}
	if c := coveredPixels(gotZ); c != covered {
		t.Errorf("coverage differs: scalar drew %d pixels, SIMD drew %d", covered, c)
	}

	differing, maxChannel, maxDepth := compareBuffers(wantC, gotC, wantZ, gotZ)
	if maxDepth > 1e-4 {
		t.Errorf("depth differs by up to %g, want <= 1e-4", maxDepth)
	}
	// Real geometry produces sliver triangles whose UVs land right on a texel
	// boundary, so a handful of pixels legitimately pick a neighbouring texel.
	if ratio := float64(differing) / float64(covered); ratio > 0.02 {
		t.Errorf("%d of %d covered pixels differ (%.2f%%), want <= 2%%", differing, covered, ratio*100)
	}
	t.Logf("covered=%d differing=%d maxChannelDelta=%d maxDepthDelta=%g", covered, differing, maxChannel, maxDepth)
}

// benchRealMesh times one frame's worth of rasterization for the shipped model,
// with the geometry stage hoisted out so only the rasterizer is measured. This
// is the number that matters for the app, and it is a harsher test than the
// synthetic benchmarks: the model's triangles are small, so per-triangle setup
// carries more weight than the inner loop does.
func benchRealMesh(b *testing.B, mode config.RenderMode, fill config.FillRasterMode, tex config.TextureRasterizer) {
	b.Chdir("../..")
	r := NewRenderer[float32]()
	if err := r.Setup(); err != nil {
		b.Fatalf("Setup: %v", err)
	}
	r.update()
	if len(r.trianglesToDraw) == 0 {
		b.Fatal("update produced no triangles")
	}
	r.renderMethod = mode
	r.setFillRaster(fill)
	r.setTextureRaster(tex)
	b.ReportAllocs()
	for b.Loop() {
		r.clearColorBuffer(0xFF000000)
		r.clearZBuffer(1)
		for _, tri := range r.trianglesToDraw {
			if mode == config.RENDER_FILL_TRIANGLE {
				r.fillRasterFunc(
					tri.points[0].X, tri.points[0].Y, tri.points[0].Z, tri.points[0].W,
					tri.points[1].X, tri.points[1].Y, tri.points[1].Z, tri.points[1].W,
					tri.points[2].X, tri.points[2].Y, tri.points[2].Z, tri.points[2].W,
					tri.color, tri.lightIntensity[0], tri.lightIntensity[1], tri.lightIntensity[2])
			} else {
				r.textureRasterFunc(
					tri.points[0].X, tri.points[0].Y, tri.points[0].Z, tri.points[0].W, tri.textCoords[0].U, tri.textCoords[0].V,
					tri.points[1].X, tri.points[1].Y, tri.points[1].Z, tri.points[1].W, tri.textCoords[1].U, tri.textCoords[1].V,
					tri.points[2].X, tri.points[2].Y, tri.points[2].Z, tri.points[2].W, tri.textCoords[2].U, tri.textCoords[2].V,
					tri.lightIntensity[0], tri.lightIntensity[1], tri.lightIntensity[2], tri.texture)
			}
		}
	}
}

func BenchmarkRealMeshFillPineda(b *testing.B) {
	benchRealMesh(b, config.RENDER_FILL_TRIANGLE, config.FILL_PINEDA, config.TEXTURE_PINEDA)
}

func BenchmarkRealMeshFillPinedaSIMD(b *testing.B) {
	if !simdCompiledIn {
		b.Skip("built without GOEXPERIMENT=simd")
	}
	benchRealMesh(b, config.RENDER_FILL_TRIANGLE, config.FILL_PINEDA_SIMD, config.TEXTURE_PINEDA)
}

func BenchmarkRealMeshTexturedPineda(b *testing.B) {
	benchRealMesh(b, config.RENDER_TEXTURED, config.FILL_PINEDA, config.TEXTURE_PINEDA)
}

func BenchmarkRealMeshTexturedPinedaSIMD(b *testing.B) {
	if !simdCompiledIn {
		b.Skip("built without GOEXPERIMENT=simd")
	}
	benchRealMesh(b, config.RENDER_TEXTURED, config.FILL_PINEDA, config.TEXTURE_PINEDA_SIMD)
}
