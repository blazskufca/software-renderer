package raster

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestShade(t *testing.T) {
	tests := []struct {
		name   string
		color  uint32
		factor float32
		want   uint32
	}{
		{name: "negative", color: 0xFF80FF01, factor: -0.5, want: 0xFF000000},
		{name: "zero", color: 0xFF80FF01, factor: 0, want: 0xFF000000},
		{name: "half", color: 0xFF80FF01, factor: 0.5, want: 0xFF407F00},
		{name: "one", color: 0xFF80FF01, factor: 1, want: 0xFF80FF01},
		{name: "saturated", color: 0xFF80FF01, factor: 1.5, want: 0xFF80FF01},
		{name: "alpha preserved", color: 0x80FFFFFF, factor: 0.5, want: 0x807F7F7F},
		{name: "no carry", color: 0xFFFF00FF, factor: 0.999, want: 0xFFFE00FE},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shade(tc.color, tc.factor); got != tc.want {
				t.Fatalf("shade(%#08x, %v) = %#08x, want %#08x", tc.color, tc.factor, got, tc.want)
			}
		})
	}
}

func TestWrapIndex(t *testing.T) {
	for _, n := range []int32{1, 3, 7, 48, 64, 100, 4095, 4096} {
		invN := 1 / float32(n)
		pow2 := isPow2(int(n))
		for i := range int32(1 << 20) {
			if got := wrapIndex(i, n, invN, pow2); got != i%n {
				t.Fatalf("n=%d: wrapIndex(%d) = %d, want %d", n, i, got, i%n)
			}
		}
		rng := rand.New(rand.NewPCG(31, 32))
		for range 100_000 {
			i := int32(1<<20 + rng.IntN(1<<24-1<<20))
			if got := wrapIndex(i, n, invN, pow2); got != i%n {
				t.Fatalf("n=%d: wrapIndex(%d) = %d, want %d", n, i, got, i%n)
			}
		}
		for _, i := range []int32{-1, -n, -(1 << 24), math.MinInt32, math.MaxInt32, 1 << 30} {
			if got := wrapIndex(i, n, invN, pow2); got < 0 || got >= n {
				t.Fatalf("n=%d: wrapIndex(%d) = %d, out of [0,%d)", n, i, got, n)
			}
		}
	}
}
