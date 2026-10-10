package main

import (
	"math"
	"testing"
)

const lineColor = 0xFFFFFFFF

func lit(r *Renderer[float32], x, y int) bool {
	return pixelAt(r, y*r.config.WindowWidth+x) == lineColor
}

func TestDrawLine(t *testing.T) {
	tests := []struct {
		name           string
		x0, y0, x1, y1 int
	}{
		{name: "x-major", x0: 10, y0: 20, x1: 110, y1: 57},
		{name: "x-major reversed", x0: 110, y0: 57, x1: 10, y1: 20},
		{name: "y-major", x0: 300, y0: 10, x1: 320, y1: 200},
		{name: "steep negative", x0: 500, y0: 400, x1: 450, y1: 100},
		{name: "point", x0: 5, y0: 5, x1: 5, y1: 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRenderer(t)
			r.drawLine(tc.x0, tc.y0, tc.x1, tc.y1, lineColor)
			if !lit(r, tc.x0, tc.y0) || !lit(r, tc.x1, tc.y1) {
				t.Fatalf("endpoints (%d,%d) lit=%v, (%d,%d) lit=%v",
					tc.x0, tc.y0, lit(r, tc.x0, tc.y0), tc.x1, tc.y1, lit(r, tc.x1, tc.y1))
			}
			dx, dy := tc.x1-tc.x0, tc.y1-tc.y0
			xMajor := abs(dx) >= abs(dy)
			perMajor := map[int]int{}
			count := 0
			for y := range r.config.WindowHeight {
				for x := range r.config.WindowWidth {
					if !lit(r, x, y) {
						continue
					}
					count++
					major, minor := x, float64(y)
					ideal := float64(tc.y0)
					if xMajor {
						if dx != 0 {
							ideal += float64(x-tc.x0) * float64(dy) / float64(dx)
						}
					} else {
						major, minor = y, float64(x)
						ideal = float64(tc.x0) + float64(y-tc.y0)*float64(dx)/float64(dy)
					}
					perMajor[major]++
					if d := math.Abs(minor - ideal); d > 0.5+1e-6 {
						t.Fatalf("pixel (%d,%d) is %.3f px from the ideal line", x, y, d)
					}
				}
			}
			if want := max(abs(dx), abs(dy)) + 1; count != want {
				t.Fatalf("lit %d pixels, want %d", count, want)
			}
			for major, n := range perMajor {
				if n != 1 {
					t.Fatalf("major-axis coordinate %d has %d lit pixels, want 1", major, n)
				}
			}
		})
	}

	t.Run("off-screen", func(t *testing.T) {
		const x0, y0, x1, y1 = -50, -20, 2000, 900
		r := newTestRenderer(t)
		r.drawLine(x0, y0, x1, y1, lineColor)
		for x := range r.config.WindowWidth {
			ideal := y0 + float64(x-x0)*float64(y1-y0)/float64(x1-x0)
			if ideal < 0.5 || ideal > 478.5 {
				continue
			}
			n := 0
			for y := range r.config.WindowHeight {
				if !lit(r, x, y) {
					continue
				}
				n++
				if d := math.Abs(float64(y) - ideal); d > 0.5+1e-6 {
					t.Fatalf("pixel (%d,%d) is %.3f px from the ideal line", x, y, d)
				}
			}
			if n != 1 {
				t.Fatalf("column %d has %d lit pixels, want 1", x, n)
			}
		}
	})
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
