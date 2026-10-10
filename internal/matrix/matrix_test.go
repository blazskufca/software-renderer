package matrix

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/blazskufca/software-renderer/internal/vector"
)

const tol = 1e-9

func approxMat(t *testing.T, got, want Mat4[float64], tol float64) {
	t.Helper()
	for r := range 4 {
		for c := range 4 {
			if math.Abs(got.m[r][c]-want.m[r][c]) > tol {
				t.Fatalf("m[%d][%d] = %g, want %g\ngot  %v\nwant %v", r, c, got.m[r][c], want.m[r][c], got.m, want.m)
			}
		}
	}
}

func approxVec4(t *testing.T, label string, got, want vector.Vec4[float64], tol float64) {
	t.Helper()
	if math.Abs(got.X-want.X) > tol || math.Abs(got.Y-want.Y) > tol ||
		math.Abs(got.Z-want.Z) > tol || math.Abs(got.W-want.W) > tol {
		t.Fatalf("%s: got %v, want %v", label, got, want)
	}
}

func randVec3(rng *rand.Rand) vector.Vec3[float64] {
	return vector.Vec3[float64]{X: rng.Float64()*20 - 10, Y: rng.Float64()*20 - 10, Z: rng.Float64()*20 - 10}
}

func randAngle(rng *rand.Rand) float64 { return rng.Float64()*2*math.Pi - math.Pi }

func TestRotationsComposeAndPreserveLength(t *testing.T) {
	rots := []struct {
		name string
		make func(float64) Mat4[float64]
	}{
		{name: "X", make: Mat4MakeRotationX[float64]},
		{name: "Y", make: Mat4MakeRotationY[float64]},
		{name: "Z", make: Mat4MakeRotationZ[float64]},
	}
	for _, rot := range rots {
		t.Run(rot.name, func(t *testing.T) {
			rng := rand.New(rand.NewPCG(1, 2))
			for i := range 200 {
				a, b := randAngle(rng), randAngle(rng)
				approxMat(t, rot.make(a).MulMat4(rot.make(b)), rot.make(a+b), tol)
				approxMat(t, rot.make(a).MulMat4(rot.make(-a)), Mat4Identity[float64](), tol)
				v := randVec3(rng)
				r := rot.make(a)
				if got, want := r.MulVec4(v.ToVec4()).ToVec3().Length(), v.Length(); math.Abs(got-want) > tol {
					t.Fatalf("case %d: |R(%g)·%v| = %g, want %g", i, a, v, got, want)
				}
			}
		})
	}
}

func TestTranslationMovesPointsNotDirections(t *testing.T) {
	m := Mat4MakeTranslation(1.0, 2.0, 3.0)
	if got, want := m.MulVec4(vector.Vec4[float64]{X: 4, Y: 5, Z: 6, W: 1}), (vector.Vec4[float64]{X: 5, Y: 7, Z: 9, W: 1}); got != want {
		t.Fatalf("point: got %v, want %v", got, want)
	}
	if got, want := m.MulVec4(vector.Vec4[float64]{X: 4, Y: 5, Z: 6, W: 0}), (vector.Vec4[float64]{X: 4, Y: 5, Z: 6, W: 0}); got != want {
		t.Fatalf("direction: got %v, want %v", got, want)
	}
}

func TestMulMat4MatchesSequentialApplication(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	randM := func() Mat4[float64] {
		s := Mat4MakeScale(rng.Float64()*2+0.5, rng.Float64()*2+0.5, rng.Float64()*2+0.5)
		tr := randVec3(rng)
		return s.MulMat4(Mat4MakeRotationX(randAngle(rng))).
			MulMat4(Mat4MakeRotationY(randAngle(rng))).
			MulMat4(Mat4MakeTranslation(tr.X, tr.Y, tr.Z))
	}
	a, b := randM(), randM()
	ab := a.MulMat4(b)
	for i := range 200 {
		p := randVec3(rng).ToVec4()
		approxVec4(t, fmt.Sprintf("point %d %v", i, p), ab.MulVec4(p), a.MulVec4(b.MulVec4(p)), tol)
	}
}

func TestPerspectiveMapsFrustumToNDC(t *testing.T) {
	const (
		fov    = math.Pi / 3
		aspect = 0.5
		near   = 0.1
		far    = 100.0
	)
	m := Mat4MakePerspective[float64](fov, aspect, near, far)
	th := math.Tan(fov / 2)
	cases := []struct {
		name  string
		in    vector.Vec4[float64]
		check func(vector.Vec4[float64]) bool
	}{
		{name: "near", in: vector.Vec4[float64]{Z: near, W: 1}, check: func(v vector.Vec4[float64]) bool {
			return math.Abs(v.Z) < tol && math.Abs(v.W-near) < tol
		}},
		{name: "far", in: vector.Vec4[float64]{Z: far, W: 1}, check: func(v vector.Vec4[float64]) bool {
			return math.Abs(v.Z-1) < tol && math.Abs(v.W-far) < tol
		}},
		{name: "right", in: vector.Vec4[float64]{X: 7 * th / aspect, Z: 7, W: 1}, check: func(v vector.Vec4[float64]) bool {
			return math.Abs(v.X-1) < tol
		}},
		{name: "left", in: vector.Vec4[float64]{X: -7 * th / aspect, Z: 7, W: 1}, check: func(v vector.Vec4[float64]) bool {
			return math.Abs(v.X+1) < tol
		}},
		{name: "top", in: vector.Vec4[float64]{Y: 7 * th, Z: 7, W: 1}, check: func(v vector.Vec4[float64]) bool {
			return math.Abs(v.Y-1) < tol
		}},
		{name: "W=0 finite", in: vector.Vec4[float64]{X: 1, Y: 1, Z: 0, W: 1}, check: func(v vector.Vec4[float64]) bool {
			for _, c := range []float64{v.X, v.Y, v.Z} {
				if math.IsInf(c, 0) || math.IsNaN(c) {
					return false
				}
			}
			return true
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.Mat4MulVec4Project(tc.in); !tc.check(got) {
				t.Fatalf("project(%v) = %v", tc.in, got)
			}
		})
	}
}

func TestLookAt(t *testing.T) {
	eye := vector.Vec3[float64]{X: 1, Y: 2, Z: 3}
	target := vector.Vec3[float64]{X: 4, Y: 6, Z: 3}
	up := vector.Vec3[float64]{Y: 1}
	m := Mat4LookAt(eye, target, up)
	pt := vector.Vec3[float64].ToVec4
	approxVec4(t, "eye", m.MulVec4(pt(eye)), vector.Vec4[float64]{W: 1}, tol)
	approxVec4(t, "target", m.MulVec4(pt(target)), vector.Vec4[float64]{Z: 5, W: 1}, tol)
	if got := m.MulVec4(pt(eye.Add(up))); got.Y <= 0 {
		t.Fatalf("eye+up maps to %v, want Y > 0", got)
	}
	rng := rand.New(rand.NewPCG(5, 6))
	for i := range 100 {
		a, b := randVec3(rng), randVec3(rng)
		want := a.Sub(b).Length()
		got := m.MulVec4(pt(a)).ToVec3().Sub(m.MulVec4(pt(b)).ToVec3()).Length()
		if math.Abs(got-want) > tol {
			t.Fatalf("pair %d: distance %g after view, want %g (a=%v b=%v)", i, got, want, a, b)
		}
	}
}
