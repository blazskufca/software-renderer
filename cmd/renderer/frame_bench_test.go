package main

// Benchmarks that compare the renderer builds on identical work. The same file
// compiles into every build, so each one measures the code the app runs:
//
//	go test -bench . ./cmd/renderer                               # master's renderer (render_legacy.go)
//	go test -tags simd -bench . ./cmd/renderer                    # new pipeline + rasterizer, scalar kernels
//	GOEXPERIMENT=simd go test -tags simd -bench . ./cmd/renderer  # the same with SIMD kernels
//
// scripts/bench-compare.sh builds all three, runs them interleaved on a pinned
// core and compares them with benchstat.
//
// A frame is what the app does every tick: update (geometry), drawTriangles
// (rasterizing) and clearBuffers. Uploading the frame to ebiten and drawing the
// UI are left out: they need a window, and they are the same in every build.
//
// Every benchmark turns the car through benchAngles poses, one per op, so a
// result is the mean over a full turn rather than one lucky or unlucky pose.
// The pose comes from the frame counter, not the clock, so every build renders
// exactly the same sequence of frames.

import (
	"math"
	"slices"
	"testing"

	"github.com/blazskufca/software-renderer/internal/config"
)

const benchAngles = 36 // car poses per turn, 10° apart

// benchViews move the camera along its start-up view direction: the default
// view (small car, triangles of a few pixels), and two close-ups where the car
// fills much of the screen and triangles overlap a lot.
var benchViews = []struct {
	name    string
	forward float32
}{
	{"default", 0},
	{"near", 3},
	{"close", 3.6},
}

var benchModes = []struct {
	name string
	mode config.RenderMode
}{
	{"textured", config.RENDER_TEXTURED},
	{"filled", config.RENDER_FILL_TRIANGLE},
}

// BenchmarkFrame measures whole frames, the number that decides the frame rate.
func BenchmarkFrame(b *testing.B) {
	for _, v := range benchViews {
		for _, m := range benchModes {
			b.Run("view="+v.name+"/mode="+m.name, func(b *testing.B) {
				r := newBenchRenderer(b, v.forward, m.mode)
				w := measureWorkload(b, r, true)
				b.ReportAllocs()
				frame := 0
				for b.Loop() {
					poseCar(r, frame)
					r.update()
					r.drawTriangles()
					r.clearBuffers()
					frame++
				}
				w.report(b)
			})
		}
	}
}

// BenchmarkGeometry measures update alone: transform, cull, light, clip and
// project the mesh into a triangle list.
func BenchmarkGeometry(b *testing.B) {
	for _, v := range benchViews {
		b.Run("view="+v.name, func(b *testing.B) {
			r := newBenchRenderer(b, v.forward, config.RENDER_TEXTURED)
			w := measureWorkload(b, r, false)
			b.ReportAllocs()
			frame := 0
			for b.Loop() {
				poseCar(r, frame)
				r.update()
				frame++
			}
			w.report(b)
		})
	}
}

// BenchmarkRasterize measures drawing precomputed triangle lists. It includes
// clearBuffers, because drawing the same frame again into full buffers would
// fail the depth test almost everywhere and measure nothing; BenchmarkClear
// measures the clear alone.
func BenchmarkRasterize(b *testing.B) {
	for _, v := range benchViews {
		for _, m := range benchModes {
			b.Run("view="+v.name+"/mode="+m.name, func(b *testing.B) {
				r := newBenchRenderer(b, v.forward, m.mode)
				w := measureWorkload(b, r, true)
				poses := trianglesPerPose(r, &r.trianglesToDraw)
				b.ReportAllocs()
				frame := 0
				for b.Loop() {
					r.trianglesToDraw = poses[frame%benchAngles]
					r.drawTriangles()
					r.clearBuffers()
					frame++
				}
				w.report(b)
			})
		}
	}
}

// BenchmarkClear measures resetting the color and depth buffers.
func BenchmarkClear(b *testing.B) {
	r := newBenchRenderer(b, 0, config.RENDER_TEXTURED)
	b.ReportAllocs()
	for b.Loop() {
		r.clearBuffers()
	}
}

// newBenchRenderer sets up the app's renderer with its default scene, moved
// forward along the view direction, with every mode that affects the work set
// explicitly instead of relying on each build's defaults.
func newBenchRenderer(b *testing.B, forward float32, mode config.RenderMode) *Renderer[float32] {
	b.Helper()
	r := NewRenderer[float32]()
	if err := r.Setup(); err != nil {
		b.Fatalf("Setup: %v", err)
	}
	r.renderMethod = mode
	r.setTextureRaster(config.TEXTURE_PINEDA)
	r.setFillRaster(config.FILL_PINEDA)
	r.shadingMode = config.SHADING_GOURAUD
	r.cullMode = config.CULL_BACKFACE
	r.deltaTime = 0 // the per-frame rotation in Setup is replaced by poseCar
	r.update()      // computes camera.Direction
	r.camera.Position = r.camera.Position.Add(r.camera.Direction.Multiply(forward))
	r.clearBuffers() // the legacy initBuffers leaves the z-buffer at 0
	return r
}

func poseCar(r *Renderer[float32], frame int) {
	r.meshes[0].Rotation.Y = float32(frame%benchAngles) * (2 * math.Pi / benchAngles)
}

// workload is what one frame of a benchmark has to do, averaged over all poses.
type workload struct {
	triangles float64 // triangles handed to the rasterizer
	pixels    float64 // pixels covered after drawing, 0 if not drawn
}

// measureWorkload renders every pose once, which also warms up caches and
// branch predictors before timing starts. It fails the benchmark if nothing
// would be drawn, so a broken scene can't pass as a fast one.
func measureWorkload(b *testing.B, r *Renderer[float32], draw bool) workload {
	b.Helper()
	var w workload
	for k := range benchAngles {
		poseCar(r, k)
		r.update()
		w.triangles += float64(len(r.trianglesToDraw))
		if draw {
			r.drawTriangles()
			w.pixels += float64(coveredPixels(r))
			r.clearBuffers()
		}
	}
	w.triangles /= benchAngles
	w.pixels /= benchAngles
	if w.triangles == 0 || draw && w.pixels == 0 {
		b.Fatalf("empty frames: %.0f triangles, %.0f pixels per frame", w.triangles, w.pixels)
	}
	return w
}

// report attaches the workload to the result, so benchstat shows next to the
// timings whether the builds did the same work. It must run after the b.Loop
// loop: b.Loop resets user metrics when timing starts.
func (w workload) report(b *testing.B) {
	b.ReportMetric(w.triangles, "tris/frame")
	if w.pixels > 0 {
		b.ReportMetric(w.pixels, "pixels/frame")
	}
}

// coveredPixels counts pixels whose depth was written. Every build clears
// depth to 1 and stores smaller values for drawn pixels; unlike color, depth
// can't be mistaken for the background when a pixel is lit black.
func coveredPixels(r *Renderer[float32]) int {
	n := 0
	for i := range r.config.WindowWidth * r.config.WindowHeight {
		if depthAt(r, i) < 1 {
			n++
		}
	}
	return n
}

// trianglesPerPose runs geometry for every pose and keeps a copy of each
// triangle list. It's generic because each build has its own triangle type.
func trianglesPerPose[S ~[]E, E any](r *Renderer[float32], tris *S) []S {
	poses := make([]S, benchAngles)
	for k := range poses {
		poseCar(r, k)
		r.update()
		poses[k] = slices.Clone(*tris)
	}
	return poses
}
