package main

import (
	"math"

	"github.com/blazskufca/software-renderer/internal/light"
	"github.com/blazskufca/software-renderer/internal/raster"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

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

func (r *Renderer[T]) drawFilledTriangle(tri *raster.Triangle) {
	// Snap screen coords to integers, like the C reference (keep w as float for depth)
	x0, y0 := int(tri.V[0].X), int(tri.V[0].Y)
	x1, y1 := int(tri.V[1].X), int(tri.V[1].Y)
	x2, y2 := int(tri.V[2].X), int(tri.V[2].Y)
	w0, w1, w2 := tri.V[0].W, tri.V[1].W, tri.V[2].W
	i0, i1, i2 := tri.V[0].Light, tri.V[1].Light, tri.V[2].Light
	color := tri.Color

	if y0 > y1 {
		utils.SwapNumericPtr(&x0, &x1)
		utils.SwapNumericPtr(&y0, &y1)
		utils.SwapNumericPtr(&w0, &w1)
		utils.SwapNumericPtr(&i0, &i1)
	}
	if y1 > y2 {
		utils.SwapNumericPtr(&x1, &x2)
		utils.SwapNumericPtr(&y1, &y2)
		utils.SwapNumericPtr(&w1, &w2)
		utils.SwapNumericPtr(&i1, &i2)
	}
	if y0 > y1 {
		utils.SwapNumericPtr(&x0, &x1)
		utils.SwapNumericPtr(&y0, &y1)
		utils.SwapNumericPtr(&w0, &w1)
		utils.SwapNumericPtr(&i0, &i1)
	}

	pointA := vector.Vec4[float32]{X: float32(x0), Y: float32(y0), W: w0}
	pointB := vector.Vec4[float32]{X: float32(x1), Y: float32(y1), W: w1}
	pointC := vector.Vec4[float32]{X: float32(x2), Y: float32(y2), W: w2}

	// top half
	var invSlope1, invSlope2 float32
	if y1-y0 != 0 {
		invSlope1 = float32(x1-x0) / float32(math.Abs(float64(y1-y0)))
	}
	if y2-y0 != 0 {
		invSlope2 = float32(x2-x0) / float32(math.Abs(float64(y2-y0)))
	}
	if y1-y0 != 0 {
		for y := y0; y <= y1; y++ {
			xStart := int(float32(x1) + float32(y-y1)*invSlope1) // single truncation, no Round
			xEnd := int(float32(x0) + float32(y-y0)*invSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTrianglePixel(x, y, color, pointA, pointB, pointC, i0, i1, i2)
			}
		}
	}
	// bottom half
	invSlope1, invSlope2 = 0, 0
	if y2-y1 != 0 {
		invSlope1 = float32(x2-x1) / float32(math.Abs(float64(y2-y1)))
	}
	if y2-y0 != 0 {
		invSlope2 = float32(x2-x0) / float32(math.Abs(float64(y2-y0)))
	}
	if y2-y1 != 0 {
		for y := y1; y <= y2; y++ {
			xStart := int(float32(x1) + float32(y-y1)*invSlope1)
			xEnd := int(float32(x0) + float32(y-y0)*invSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTrianglePixel(x, y, color, pointA, pointB, pointC, i0, i1, i2)
			}
		}
	}
}

func (r *Renderer[T]) drawTrianglePixel(x, y int, color uint32, a, b, c vector.Vec4[float32], i0, i1, i2 float32) {
	if x < 0 || x >= r.config.WindowWidth || y < 0 || y >= r.config.WindowHeight {
		return
	}
	p := vector.Vec2[float32]{X: float32(x), Y: float32(y)}
	aVec2 := a.ToVec2()
	bVec2 := b.ToVec2()
	cVec2 := c.ToVec2()

	weights := barycentricWeights(aVec2, bVec2, cVec2, p)
	alpha, beta, gamma := weights.X, weights.Y, weights.Z
	interpolatedReciprocalW := (1/a.W)*alpha + (1/b.W)*beta + (1/c.W)*gamma
	interpolatedReciprocalW = 1 - interpolatedReciprocalW

	if interpolatedReciprocalW < r.target.Depth[(r.config.WindowWidth*y)+x] {
		// Interpolate the per-vertex light intensity across the face (Gouraud);
		// for flat shading all three are equal so this is a constant.
		intensity := i0*alpha + i1*beta + i2*gamma
		r.drawPixel(x, y, light.LightApplyIntensity(color, intensity))
		// Update the Z-buffer with 1/w for current pixel
		r.target.Depth[(r.config.WindowWidth*y)+x] = interpolatedReciprocalW
	}

}

func (r *Renderer[T]) drawTexturedTriangle(tri *raster.Triangle) {
	t := tri.Texture
	ix0, iy0 := int(tri.V[0].X), int(tri.V[0].Y)
	ix1, iy1 := int(tri.V[1].X), int(tri.V[1].Y)
	ix2, iy2 := int(tri.V[2].X), int(tri.V[2].Y)
	u0, v0, w0, i0 := tri.V[0].U, tri.V[0].V, tri.V[0].W, tri.V[0].Light
	u1, v1, w1, i1 := tri.V[1].U, tri.V[1].V, tri.V[1].W, tri.V[1].Light
	u2, v2, w2, i2 := tri.V[2].U, tri.V[2].V, tri.V[2].W, tri.V[2].Light

	// Triangles vertices need to be sorted by the Y component here too
	// But when we swap not only do we need X and Y, we also have to swap the UV coordinates
	if iy0 > iy1 {
		utils.SwapNumericPtr(&iy0, &iy1)
		utils.SwapNumericPtr(&ix0, &ix1)
		utils.SwapNumericPtr(&u0, &u1)
		utils.SwapNumericPtr(&v0, &v1)
		utils.SwapNumericPtr(&w0, &w1)
		utils.SwapNumericPtr(&i0, &i1)

	}
	if iy1 > iy2 {
		utils.SwapNumericPtr(&iy1, &iy2)
		utils.SwapNumericPtr(&ix1, &ix2)
		utils.SwapNumericPtr(&u1, &u2)
		utils.SwapNumericPtr(&v1, &v2)
		utils.SwapNumericPtr(&w1, &w2)
		utils.SwapNumericPtr(&i1, &i2)
	}
	if iy0 > iy1 {
		utils.SwapNumericPtr(&iy0, &iy1)
		utils.SwapNumericPtr(&ix0, &ix1)
		utils.SwapNumericPtr(&u0, &u1)
		utils.SwapNumericPtr(&v0, &v1)
		utils.SwapNumericPtr(&w0, &w1)
		utils.SwapNumericPtr(&i0, &i1)

	}

	// Flip the V to account for inverted UV coordinates, V0 is top left for us
	v0 = 1 - v0
	v1 = 1 - v1
	v2 = 1 - v2

	// Vector points
	pointA := vector.Vec4[float32]{X: float32(ix0), Y: float32(iy0), W: w0}
	pointB := vector.Vec4[float32]{X: float32(ix1), Y: float32(iy1), W: w1}
	pointC := vector.Vec4[float32]{X: float32(ix2), Y: float32(iy2), W: w2}
	AUV := texture.Tex2[float32]{U: u0, V: v0}
	BUV := texture.Tex2[float32]{U: u1, V: v1}
	CUV := texture.Tex2[float32]{U: u2, V: v2}

	// Render the flat-bottom triangle
	// Inverse slope is dX/dy
	var inverseSlope1 float32
	var inverseSlope2 float32
	if (iy1 - iy0) != 0 {
		inverseSlope1 = float32(ix1-ix0) / float32(math.Abs(float64(iy1-iy0))) // "Left" leg of the triangle
	}
	if (iy2 - iy0) != 0 {
		inverseSlope2 = float32(ix2-ix0) / float32(math.Abs(float64(iy2-iy0))) // "Right" leg of the triangle
	}
	// For every scanline
	if iy1-iy0 != 0 {
		for y := iy0; y <= iy1; y++ {
			// The xStart and xEnd come from point-slope formula
			// y-y1 = m(x-x1)
			// (y-y1)/m = x-x1
			// x =  x1 + (y-y1)/m
			xStart := int(float32(ix1) + float32(y-iy1)*inverseSlope1)
			xEnd := int(float32(ix0) + float32(y-iy0)*inverseSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTexel(x, y, t, pointA, pointB, pointC, AUV, BUV, CUV, i0, i1, i2)
				//drawPixel(x, y, 0xFFFF00FF)
			}

		}
	}
	// Render the flat top triangle
	inverseSlope1, inverseSlope2 = 0.0, 0.0
	if (iy2 - iy1) != 0 {
		inverseSlope1 = float32(ix2-ix1) / float32(math.Abs(float64(iy2-iy1))) // "Left" leg of the triangle
	}
	if (iy2 - iy0) != 0 {
		inverseSlope2 = float32(ix2-ix0) / float32(math.Abs(float64(iy2-iy0))) // "Right" leg of the triangle
	}
	// For every scanline
	if iy2-iy1 != 0 {
		for y := iy1; y <= iy2; y++ {
			xStart := int(float32(ix1) + float32(y-iy1)*inverseSlope1)
			xEnd := int(float32(ix0) + float32(y-iy0)*inverseSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTexel(x, y, t, pointA, pointB, pointC, AUV, BUV, CUV, i0, i1, i2)
				//drawPixel(x, y, 0xFFFF00FF)
			}

		}
	}
}

func barycentricWeights[T utils.Numeric](a, b, c, p vector.Vec2[T]) vector.Vec3[T] {
	ac := c.Sub(a)
	ab := b.Sub(a)
	pc := c.Sub(p)
	pb := b.Sub(p)
	ap := p.Sub(a)
	// This is the Z component of cross product ACxAB
	ABCParallerogram := (T(ac.X) * T(ab.Y)) - (T(ac.Y) * T(ab.X))
	// Again the Z component for the cross product
	alpha := ((T(pc.X) * T(pb.Y)) - (T(pc.Y) * T(pb.X))) / ABCParallerogram
	beta := ((T(ac.X) * T(ap.Y)) - (T(ac.Y) * T(ap.X))) / ABCParallerogram
	gamma := 1.0 - alpha - beta
	return vector.Vec3[T]{
		X: alpha,
		Y: beta,
		Z: gamma,
	}

}
