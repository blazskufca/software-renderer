package main

import (
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
