package main

import "math"

// face stores vertex indexes
type face struct {
	a int
	b int
	c int
}

// triangle stores actual vec2 points of the triangle projected onto the screen
type triangle struct {
	points [3]vec2
}

func intSwap(a, b *int) {
	tmp := *a
	*a = *b
	*b = tmp
}

func drawFilledTriangle(x0, y0, x1, y1, x2, y2 int, color uint32) {
	// Sort vertices y0 < y1 < y2
	if y0 > y1 {
		intSwap(&y0, &y1)
		intSwap(&x0, &x1)
	}
	if y1 > y2 {
		intSwap(&y1, &y2)
		intSwap(&x1, &x2)
	}
	if y0 > y1 {
		intSwap(&y0, &y1)
		intSwap(&x0, &x1)
	}

	// There is no bottom triangle if the ys are equal
	if y1 == y2 {
		fillFlatBottomTriangle(x0, y0, x1, y1, x2, y2, color)
	} else if y0 == y1 {
		fillFlatTopTriangle(x0, y0, x1, y1, x2, y2, color)
	} else {
		// Calculate Mx, My using triangle similarity
		My := y1
		Mx := (((x2 - x0) * (y1 - y0)) / (y2 - y0)) + x0

		fillFlatBottomTriangle(x0, y0, x1, y1, Mx, My, color)
		fillFlatTopTriangle(x1, y1, Mx, My, x2, y2, color)
	}
}

func fillFlatBottomTriangle(x0, y0, x1, y1, x2, y2 int, color uint32) {
	// Find the slop for both triangle legs
	// Because we use scanline fill, Y is our independent so we are really looking for dX/dY, inverse of the slope
	inverseSlope1 := float64(x1-x0) / float64(y1-y0)
	inverseSlope2 := float64(x2-x0) / float64(y2-y0)

	xStart, xEnd := float64(x0), float64(x0)
	// Loop thorough all the scanlines
	for y := y0; y <= y2; y++ {
		drawLine(int(math.Round(xStart)), y, int(math.Round(xEnd)), y, color)
		xStart += inverseSlope1
		xEnd += inverseSlope2
	}
}

func fillFlatTopTriangle(x0, y0, x1, y1, x2, y2 int, color uint32) {
	inverseSlope1 := float64(x2-x0) / float64(y2-y0)
	inverseSlope2 := float64(x2-x1) / float64(y2-y1)
	xStart, xEnd := float64(x2), float64(x2)
	for y := y2; y >= y0; y-- {
		drawLine(int(math.Round(xStart)), y, int(math.Round(xEnd)), y, color)
		xStart -= inverseSlope1
		xEnd -= inverseSlope2
	}
}
