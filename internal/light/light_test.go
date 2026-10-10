package light

import (
	"fmt"
	"testing"
)

func TestLightApplyIntensity(t *testing.T) {
	cases := []struct {
		color  uint32
		factor float32
		want   uint32
	}{
		{color: 0xFF80FF01, factor: -0.5, want: 0xFF000000},
		{color: 0xFF80FF01, factor: 0, want: 0xFF000000},
		{color: 0xFF80FF01, factor: 0.5, want: 0xFF407F00},
		{color: 0xFF80FF01, factor: 1, want: 0xFF80FF01},
		{color: 0xFF80FF01, factor: 1.5, want: 0xFF80FF01},
		{color: 0x80FFFFFF, factor: 0.5, want: 0x807F7F7F},
		{color: 0xFFFF00FF, factor: 0.999, want: 0xFFFE00FE},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%08X*%g", tc.color, tc.factor), func(t *testing.T) {
			if got := LightApplyIntensity(tc.color, tc.factor); got != tc.want {
				t.Fatalf("LightApplyIntensity(%#08x, %g) = %#08x, want %#08x", tc.color, tc.factor, got, tc.want)
			}
		})
	}
}
