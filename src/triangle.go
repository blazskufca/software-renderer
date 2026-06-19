package main

import (
	"math"

	"github.com/blazskufca/software-renderer/internal/utils"
)

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
	points           [3]vec4
	textCoords       [3]tex2
	color            uint32
	avgDepth         float64
	flatShadePercent float64
	texture          *MeshTextureData
}

//	func drawFilledTriangle(x0, y0, x1, y1, x2, y2 int, color uint32) {
//		// Sort vertices y0 < y1 < y2
//		if y0 > y1 {
//			intSwap(&y0, &y1)
//			intSwap(&x0, &x1)
//		}
//		if y1 > y2 {
//			intSwap(&y1, &y2)
//			intSwap(&x1, &x2)
//		}
//		if y0 > y1 {
//			intSwap(&y0, &y1)
//			intSwap(&x0, &x1)
//		}
//
//		// There is no bottom triangle if the ys are equal
//		if y1 == y2 {
//			fillFlatBottomTriangle(x0, y0, x1, y1, x2, y2, color)
//		} else if y0 == y1 {
//			fillFlatTopTriangle(x0, y0, x1, y1, x2, y2, color)
//		} else {
//			// Calculate Mx, My using triangle similarity
//			My := y1
//			Mx := (((x2 - x0) * (y1 - y0)) / (y2 - y0)) + x0
//
//			fillFlatBottomTriangle(x0, y0, x1, y1, Mx, My, color)
//			fillFlatTopTriangle(x1, y1, Mx, My, x2, y2, color)
//		}
//	}
//func fillFlatBottomTriangle(x0, y0, x1, y1, x2, y2 int, color uint32) {
//	// Find the slop for both triangle legs
//	// Because we use scanline fill, Y is our independent so we are really looking for dX/dY, inverse of the slope
//	inverseSlope1 := float64(x1-x0) / float64(y1-y0)
//	inverseSlope2 := float64(x2-x0) / float64(y2-y0)
//
//	xStart, xEnd := float64(x0), float64(x0)
//	// Loop thorough all the scanlines
//	for y := y0; y <= y2; y++ {
//		drawLine(int(math.Round(xStart)), y, int(math.Round(xEnd)), y, color)
//		xStart += inverseSlope1
//		xEnd += inverseSlope2
//	}
//}
//
//func fillFlatTopTriangle(x0, y0, x1, y1, x2, y2 int, color uint32) {
//	inverseSlope1 := float64(x2-x0) / float64(y2-y0)
//	inverseSlope2 := float64(x2-x1) / float64(y2-y1)
//	xStart, xEnd := float64(x2), float64(x2)
//	for y := y2; y >= y0; y-- {
//		drawLine(int(math.Round(xStart)), y, int(math.Round(xEnd)), y, color)
//		xStart -= inverseSlope1
//		xEnd -= inverseSlope2
//	}
//}

func drawFilledTriangle(x0f, y0f, z0, w0, x1f, y1f, z1, w1, x2f, y2f, z2, w2 float64, color uint32) {
	// Snap screen coords to integers, like the C reference (keep z/w as float for depth)
	x0, y0 := int(x0f), int(y0f)
	x1, y1 := int(x1f), int(y1f)
	x2, y2 := int(x2f), int(y2f)

	if y0 > y1 {
		utils.SwapNumericPtr(&x0, &x1)
		utils.SwapNumericPtr(&y0, &y1)
		utils.SwapNumericPtr(&z0, &z1)
		utils.SwapNumericPtr(&w0, &w1)
	}
	if y1 > y2 {
		utils.SwapNumericPtr(&x1, &x2)
		utils.SwapNumericPtr(&y1, &y2)
		utils.SwapNumericPtr(&z1, &z2)
		utils.SwapNumericPtr(&w1, &w2)
	}
	if y0 > y1 {
		utils.SwapNumericPtr(&x0, &x1)
		utils.SwapNumericPtr(&y0, &y1)
		utils.SwapNumericPtr(&z0, &z1)
		utils.SwapNumericPtr(&w0, &w1)
	}

	pointA := vec4{float64(x0), float64(y0), z0, w0}
	pointB := vec4{float64(x1), float64(y1), z1, w1}
	pointC := vec4{float64(x2), float64(y2), z2, w2}

	// top half
	var invSlope1, invSlope2 float64
	if y1-y0 != 0 {
		invSlope1 = float64(x1-x0) / math.Abs(float64(y1-y0))
	}
	if y2-y0 != 0 {
		invSlope2 = float64(x2-x0) / math.Abs(float64(y2-y0))
	}
	if y1-y0 != 0 {
		for y := y0; y <= y1; y++ {
			xStart := int(float64(x1) + float64(y-y1)*invSlope1) // single truncation, no Round
			xEnd := int(float64(x0) + float64(y-y0)*invSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				drawTrianglePixel(x, y, color, pointA, pointB, pointC)
			}
		}
	}
	// bottom half
	invSlope1, invSlope2 = 0, 0
	if y2-y1 != 0 {
		invSlope1 = float64(x2-x1) / math.Abs(float64(y2-y1))
	}
	if y2-y0 != 0 {
		invSlope2 = float64(x2-x0) / math.Abs(float64(y2-y0))
	}
	if y2-y1 != 0 {
		for y := y1; y <= y2; y++ {
			xStart := int(float64(x1) + float64(y-y1)*invSlope1)
			xEnd := int(float64(x0) + float64(y-y0)*invSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				drawTrianglePixel(x, y, color, pointA, pointB, pointC)
			}
		}
	}
}

func drawTrianglePixel(x, y int, color uint32, a, b, c vec4) {
	if x < 0 || x >= windowWidth || y < 0 || y >= windowHeight {
		return
	}
	p := vec2{x, y}
	aVec2 := vec2FromVec4(a)
	bVec2 := vec2FromVec4(b)
	cVec2 := vec2FromVec4(c)

	weights := barycentricWeights(aVec2, bVec2, cVec2, p)
	alpha, beta, gamma := weights.X, weights.Y, weights.Z
	interpolatedReciprocalW := (1/a.W)*alpha + (1/b.W)*beta + (1/c.W)*gamma
	interpolatedReciprocalW = 1 - interpolatedReciprocalW

	if interpolatedReciprocalW < zBuffer[(windowWidth*y)+x] {
		drawPixel(x, y, color)
		// Update the Z-buffer with 1/w for current pixel
		zBuffer[(windowWidth*y)+x] = interpolatedReciprocalW
	}

}

func drawTexturedTriangle(x0, y0, z0, w0, u0, v0, x1, y1, z1, w1, u1, v1, x2, y2, z2, w2, u2, v2, flatShadePercent float64, texture *MeshTextureData) {
	ix0, iy0 := int(x0), int(y0)
	ix1, iy1 := int(x1), int(y1)
	ix2, iy2 := int(x2), int(y2)

	// Triangles vertices need to be sorted by the Y component here too
	// But when we swap not only do we need X and Y, we also have to swap the UV coordinates
	if iy0 > iy1 {
		utils.SwapNumericPtr(&iy0, &iy1)
		utils.SwapNumericPtr(&ix0, &ix1)
		utils.SwapNumericPtr(&u0, &u1)
		utils.SwapNumericPtr(&v0, &v1)
		utils.SwapNumericPtr(&z0, &z1)
		utils.SwapNumericPtr(&w0, &w1)

	}
	if iy1 > iy2 {
		utils.SwapNumericPtr(&iy1, &iy2)
		utils.SwapNumericPtr(&ix1, &ix2)
		utils.SwapNumericPtr(&u1, &u2)
		utils.SwapNumericPtr(&v1, &v2)
		utils.SwapNumericPtr(&z1, &z2)
		utils.SwapNumericPtr(&w1, &w2)
	}
	if iy0 > iy1 {
		utils.SwapNumericPtr(&iy0, &iy1)
		utils.SwapNumericPtr(&ix0, &ix1)
		utils.SwapNumericPtr(&u0, &u1)
		utils.SwapNumericPtr(&v0, &v1)
		utils.SwapNumericPtr(&z0, &z1)
		utils.SwapNumericPtr(&w0, &w1)

	}

	// Flip the V to account for inverted UV coordinates, V0 is top left for us
	v0 = 1 - v0
	v1 = 1 - v1
	v2 = 1 - v2

	// Vector points
	pointA := vec4{float64(ix0), float64(iy0), z0, w0}
	pointB := vec4{float64(ix1), float64(iy1), z1, w1}
	pointC := vec4{float64(ix2), float64(iy2), z2, w2}
	AUV := tex2{u0, v0}
	BUV := tex2{u1, v1}
	CUV := tex2{u2, v2}

	// Render the flat-bottom triangle
	// Inverse slope is dX/dy
	var inverseSlope1 float64
	var inverseSlope2 float64
	if (iy1 - iy0) != 0 {
		inverseSlope1 = float64(ix1-ix0) / math.Abs(float64(iy1-iy0)) // "Left" leg of the triangle
	}
	if (iy2 - iy0) != 0 {
		inverseSlope2 = float64(ix2-ix0) / math.Abs(float64(iy2-iy0)) // "Right" leg of the triangle
	}
	// For every scanline
	if iy1-iy0 != 0 {
		for y := iy0; y <= iy1; y++ {
			// The xStart and xEnd come from point-slope formula
			// y-y1 = m(x-x1)
			// (y-y1)/m = x-x1
			// x =  x1 + (y-y1)/m
			xStart := int(float64(ix1) + float64(y-iy1)*inverseSlope1)
			xEnd := int(float64(ix0) + float64(y-iy0)*inverseSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				drawTexel(x, y, texture, pointA, pointB, pointC, AUV, BUV, CUV, flatShadePercent)
				//drawPixel(x, y, 0xFFFF00FF)
			}

		}
	}
	// Render the flat top triangle
	inverseSlope1, inverseSlope2 = 0.0, 0.0
	if (iy2 - iy1) != 0 {
		inverseSlope1 = float64(ix2-ix1) / math.Abs(float64(iy2-iy1)) // "Left" leg of the triangle
	}
	if (iy2 - iy0) != 0 {
		inverseSlope2 = float64(ix2-ix0) / math.Abs(float64(iy2-iy0)) // "Right" leg of the triangle
	}
	// For every scanline
	if iy2-iy1 != 0 {
		for y := iy1; y <= iy2; y++ {
			xStart := int(float64(ix1) + float64(y-iy1)*inverseSlope1)
			xEnd := int(float64(ix0) + float64(y-iy0)*inverseSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				drawTexel(x, y, texture, pointA, pointB, pointC, AUV, BUV, CUV, flatShadePercent)
				//drawPixel(x, y, 0xFFFF00FF)
			}

		}
	}
}

func barycentricWeights(a, b, c, p vec2) vec3 {
	ac := c.sub(a)
	ab := b.sub(a)
	pc := c.sub(p)
	pb := b.sub(p)
	ap := p.sub(a)
	// This is the Z component of cross product ACxAB
	ABCParallerogram := (float64(ac.X) * float64(ab.Y)) - (float64(ac.Y) * float64(ab.X))
	// Again the Z component for the cross product
	alpha := ((float64(pc.X) * float64(pb.Y)) - (float64(pc.Y) * float64(pb.X))) / ABCParallerogram
	beta := ((float64(ac.X) * float64(ap.Y)) - (float64(ac.Y) * float64(ap.X))) / ABCParallerogram
	gamma := 1.0 - alpha - beta
	return vec3{
		X: alpha,
		Y: beta,
		Z: gamma,
	}

}
