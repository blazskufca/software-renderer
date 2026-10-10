package vector

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestCross(t *testing.T) {
	x := Vec3[float64]{X: 1}
	y := Vec3[float64]{Y: 1}
	z := Vec3[float64]{Z: 1}
	basis := []struct {
		name       string
		a, b, want Vec3[float64]
	}{
		{name: "XxY=Z", a: x, b: y, want: z},
		{name: "YxZ=X", a: y, b: z, want: x},
		{name: "ZxX=Y", a: z, b: x, want: y},
	}
	for _, tc := range basis {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Cross(tc.b); got != tc.want {
				t.Fatalf("%v x %v = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}

	rng := rand.New(rand.NewPCG(1, 2))
	rv := func() Vec3[float64] {
		return Vec3[float64]{X: rng.Float64()*20 - 10, Y: rng.Float64()*20 - 10, Z: rng.Float64()*20 - 10}
	}
	for i := range 1000 {
		a, b := rv(), rv()
		c := a.Cross(b)
		if d := c.Dot(a); math.Abs(d) > 1e-9 {
			t.Fatalf("pair %d: (a×b)·a = %g, a=%v b=%v", i, d, a, b)
		}
		if d := c.Dot(b); math.Abs(d) > 1e-9 {
			t.Fatalf("pair %d: (a×b)·b = %g, a=%v b=%v", i, d, a, b)
		}
		if n := b.Cross(a).Multiply(-1); c != n {
			t.Fatalf("pair %d: a×b = %v, -(b×a) = %v", i, c, n)
		}
	}
}
