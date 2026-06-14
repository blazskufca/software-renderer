package main

import "math"

// face stores vertex indexes
type face struct {
	a     int
	b     int
	c     int
	AUV   tex2
	BUV   tex2
	CUV   tex2
	color uint32
}

// triangle stores actual vec2 points of the triangle projected onto the screen
type triangle struct {
	points     [3]vec2
	textCoords [3]tex2
	color      uint32
	avgDepth   float64
}

func intSwap(a, b *int) {
	tmp := *a
	*a = *b
	*b = tmp
}

func floatSwap(a, b *float64) {
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

func drawTexturedTriangle(x0, y0 int, u0, v0 float64, x1, y1 int, u1, v1 float64, x2, y2 int, u2, v2 float64, texture *[]uint32) {
	// Triangles vertices need to be sorted by the Y component here too
	// But when we swap not only do we need X and Y, we also have to swap the UV coordinates
	if y0 > y1 {
		intSwap(&y0, &y1)
		intSwap(&x0, &x1)
		floatSwap(&u0, &u1)
		floatSwap(&v0, &v1)

	}
	if y1 > y2 {
		intSwap(&y1, &y2)
		intSwap(&x1, &x2)
		floatSwap(&u1, &u2)
		floatSwap(&v1, &v2)
	}
	if y0 > y1 {
		intSwap(&y0, &y1)
		intSwap(&x0, &x1)
		floatSwap(&u0, &u1)
		floatSwap(&v0, &v1)
	}

	// Render the flat-bottom triangle
	// Inverse slope is dX/dy
	var inverseSlope1 float64
	var inverseSlope2 float64
	if (y1 - y0) != 0 {
		inverseSlope1 = (float64(x1) - float64(x0)) / math.Abs(float64(y1)-float64(y0)) // "Left" leg of the triangle
	}
	if (y2 - y0) != 0 {
		inverseSlope2 = (float64(x2) - float64(x0)) / math.Abs(float64(y2)-float64(y0)) // "Right" leg of the triangle
	}
	// For every scanline
	if y1-y0 != 0 {
		for y := y0; y < y1; y++ {
			// The xStart and xEnd come from point-slope formula
			// y-y1 = m(x-x1)
			// (y-y1)/m = x-x1
			// x =  x1 + (y-y1)/m
			xStart := x1 + int(math.Round(float64(y-y1)*inverseSlope1))
			xEnd := x0 + int(math.Round(float64(y-y0)*inverseSlope2))
			if xEnd < xStart {
				intSwap(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				drawPixel(x, y, 0xFFFF00FF)
			}

		}
	}
	// Render the flat top triangle
	inverseSlope1, inverseSlope1 = 0.0, 0.0
	if (y2 - y1) != 0 {
		inverseSlope1 = (float64(x2) - float64(x1)) / math.Abs(float64(y2)-float64(y1)) // "Left" leg of the triangle
	}
	if (y2 - y0) != 0 {
		inverseSlope2 = (float64(x2) - float64(x0)) / math.Abs(float64(y2)-float64(y0)) // "Right" leg of the triangle
	}
	// For every scanline
	if y2-y1 != 0 {
		for y := y1; y < y2; y++ {
			xStart := x1 + int(math.Round(float64(y-y1)*inverseSlope1))
			xEnd := x0 + int(math.Round(float64(y-y0)*inverseSlope2))
			if xEnd < xStart {
				intSwap(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				drawPixel(x, y, 0xFFFF00FF)
			}

		}
	}
}
