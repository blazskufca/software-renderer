package main

import (
	"io"
	"log"
	"math"
	"os"
	"testing"

	"github.com/blazskufca/software-renderer/internal/vector"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	// Setup loads assets/ relative to the working directory.
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// newTestRenderer returns a fully set-up renderer with cleared buffers.
func newTestRenderer(t *testing.T) *Renderer[float32] {
	t.Helper()
	r := NewRenderer[float32]()
	if err := r.Setup(); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	// The legacy initBuffers leaves the z-buffer at 0.
	r.clearBuffers()
	return r
}

// forward runs one frame update (which computes camera.Direction) and returns
// the camera's forward direction.
func forward(r *Renderer[float32]) vector.Vec3[float32] {
	r.update()
	return r.camera.Direction
}

func TestLookControls(t *testing.T) {
	// (forward × v).Y > 0 means v is to the right of forward in this
	// left-handed, Y-up frame.
	up := func(before, after vector.Vec3[float32]) bool { return after.Y > before.Y }
	down := func(before, after vector.Vec3[float32]) bool { return after.Y < before.Y }
	right := func(before, after vector.Vec3[float32]) bool { return before.Cross(after).Y > 0 }
	left := func(before, after vector.Vec3[float32]) bool { return before.Cross(after).Y < 0 }

	tests := []struct {
		name   string
		action func(r *Renderer[float32])
		want   func(before, after vector.Vec3[float32]) bool
	}{
		{name: "lookUp", action: (*Renderer[float32]).lookUp, want: up},
		{name: "lookDown", action: (*Renderer[float32]).lookDown, want: down},
		{name: "lookRight", action: (*Renderer[float32]).lookRight, want: right},
		{name: "lookLeft", action: (*Renderer[float32]).lookLeft, want: left},
		{name: "mouse up", action: func(r *Renderer[float32]) { r.applyMouseLook(0, -20) }, want: up},
		{name: "mouse down", action: func(r *Renderer[float32]) { r.applyMouseLook(0, 20) }, want: down},
		{name: "mouse right", action: func(r *Renderer[float32]) { r.applyMouseLook(20, 0) }, want: right},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRenderer(t)
			r.deltaTime = 0.1
			before := forward(r)
			tc.action(r)
			after := forward(r)
			if !tc.want(before, after) {
				t.Fatalf("direction before %+v, after %+v", before, after)
			}
		})
	}
}

func TestMoveControls(t *testing.T) {
	const tol = 1e-5
	near := func(a, b float32) bool { return math.Abs(float64(a-b)) <= tol }
	vecNear := func(a, b vector.Vec3[float32]) bool { return near(a.X, b.X) && near(a.Y, b.Y) && near(a.Z, b.Z) }
	strafe := func(sign float32) func(t *testing.T, dir, d vector.Vec3[float32]) {
		return func(t *testing.T, dir, d vector.Vec3[float32]) {
			if d.Y != 0 || !near(d.Dot(dir), 0) || !near(d.Length(), 2.5) || dir.Cross(d).Y*sign <= 0 {
				t.Fatalf("dir %+v, displacement %+v: want horizontal, perpendicular, length 2.5, cross.Y sign %v", dir, d, sign)
			}
		}
	}

	tests := []struct {
		name   string
		action func(r *Renderer[float32])
		check  func(t *testing.T, dir, d vector.Vec3[float32])
	}{
		{name: "moveForward", action: (*Renderer[float32]).moveForward, check: func(t *testing.T, dir, d vector.Vec3[float32]) {
			if want := dir.Multiply(2.5); !vecNear(d, want) {
				t.Fatalf("displacement %+v, want %+v", d, want)
			}
		}},
		{name: "moveBackward", action: (*Renderer[float32]).moveBackward, check: func(t *testing.T, dir, d vector.Vec3[float32]) {
			if want := dir.Multiply(-2.5); !vecNear(d, want) {
				t.Fatalf("displacement %+v, want %+v", d, want)
			}
		}},
		{name: "strafeRight", action: (*Renderer[float32]).strafeRight, check: strafe(1)},
		{name: "strafeLeft", action: (*Renderer[float32]).strafeLeft, check: strafe(-1)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRenderer(t)
			r.deltaTime = 0.5
			dir := forward(r)
			before := r.camera.Position
			tc.action(r)
			tc.check(t, dir, r.camera.Position.Sub(before))
		})
	}
}

func TestPitchClampKeepsViewUpright(t *testing.T) {
	repeat := func(n int, f func(r *Renderer[float32])) func(r *Renderer[float32]) {
		return func(r *Renderer[float32]) {
			for range n {
				f(r)
			}
		}
	}
	tests := []struct {
		name   string
		action func(r *Renderer[float32])
	}{
		{name: "lookUp x1000", action: repeat(1000, (*Renderer[float32]).lookUp)},
		{name: "lookDown x1000", action: repeat(1000, (*Renderer[float32]).lookDown)},
		{name: "mouse up", action: func(r *Renderer[float32]) { r.applyMouseLook(0, -100000) }},
		{name: "mouse down", action: func(r *Renderer[float32]) { r.applyMouseLook(0, 100000) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRenderer(t)
			r.deltaTime = 0.1
			initial := forward(r)
			tc.action(r)
			if p := r.camera.PitchAngle; p > 1.55 || p < -1.55 {
				t.Fatalf("PitchAngle %v outside [-1.55, 1.55]", p)
			}
			after := forward(r)
			if dot := initial.X*after.X + initial.Z*after.Z; dot <= 0 {
				t.Fatalf("horizontal direction flipped: initial %+v, after %+v, dot %v", initial, after, dot)
			}
		})
	}
}
