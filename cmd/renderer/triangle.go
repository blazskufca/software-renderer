package main

import (
	"math"

	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/light"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

// triangle stores actual vec2 points of the triangle projected onto the screen
type triangle[T utils.Numeric] struct {
	points           [3]vector.Vec4[T]
	textCoords       [3]texture.Tex2[T]
	color            uint32
	avgDepth         T
	flatShadePercent T
	texture          *mesh.TextureData
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

func (r *Renderer[T]) drawFilledTriangle(x0f, y0f, z0, w0, x1f, y1f, z1, w1, x2f, y2f, z2, w2 T, color uint32) {
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

	pointA := vector.Vec4[T]{T(x0), T(y0), T(z0), T(w0)}
	pointB := vector.Vec4[T]{T(x1), T(y1), T(z1), T(w1)}
	pointC := vector.Vec4[T]{T(x2), T(y2), T(z2), T(w2)}

	// top half
	var invSlope1, invSlope2 T
	if y1-y0 != 0 {
		invSlope1 = T(x1-x0) / T(math.Abs(float64(y1-y0)))
	}
	if y2-y0 != 0 {
		invSlope2 = T(x2-x0) / T(math.Abs(float64(y2-y0)))
	}
	if y1-y0 != 0 {
		for y := y0; y <= y1; y++ {
			xStart := int(T(x1) + T(y-y1)*invSlope1) // single truncation, no Round
			xEnd := int(T(x0) + T(y-y0)*invSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTrianglePixel(x, y, color, pointA, pointB, pointC)
			}
		}
	}
	// bottom half
	invSlope1, invSlope2 = 0, 0
	if y2-y1 != 0 {
		invSlope1 = T(x2-x1) / T(math.Abs(float64(y2-y1)))
	}
	if y2-y0 != 0 {
		invSlope2 = T(x2-x0) / T(math.Abs(float64(y2-y0)))
	}
	if y2-y1 != 0 {
		for y := y1; y <= y2; y++ {
			xStart := int(T(x1) + T(y-y1)*invSlope1)
			xEnd := int(T(x0) + T(y-y0)*invSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTrianglePixel(x, y, color, pointA, pointB, pointC)
			}
		}
	}
}

func (r *Renderer[T]) drawTrianglePixel(x, y int, color uint32, a, b, c vector.Vec4[T]) {
	if x < 0 || x >= r.Config.WindowWidth || y < 0 || y >= r.Config.WindowHeight {
		return
	}
	p := vector.Vec2[T]{T(x), T(y)}
	aVec2 := a.ToVec2()
	bVec2 := b.ToVec2()
	cVec2 := c.ToVec2()

	weights := barycentricWeights(aVec2, bVec2, cVec2, p)
	alpha, beta, gamma := weights.X, weights.Y, weights.Z
	interpolatedReciprocalW := (1/a.W)*alpha + (1/b.W)*beta + (1/c.W)*gamma
	interpolatedReciprocalW = 1 - interpolatedReciprocalW

	if interpolatedReciprocalW < r.ZBuffer[(r.Config.WindowWidth*y)+x] {
		r.drawPixel(x, y, color)
		// Update the Z-buffer with 1/w for current pixel
		r.ZBuffer[(r.Config.WindowWidth*y)+x] = interpolatedReciprocalW
	}

}

func (r *Renderer[T]) drawTexturedTriangle(x0, y0, z0, w0, u0, v0, x1, y1, z1, w1, u1, v1, x2, y2, z2, w2, u2, v2, flatShadePercent T, t *mesh.TextureData) {
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
	pointA := vector.Vec4[T]{T(ix0), T(iy0), T(z0), T(w0)}
	pointB := vector.Vec4[T]{T(ix1), T(iy1), T(z1), T(w1)}
	pointC := vector.Vec4[T]{T(ix2), T(iy2), T(z2), T(w2)}
	AUV := texture.Tex2[T]{u0, v0}
	BUV := texture.Tex2[T]{u1, v1}
	CUV := texture.Tex2[T]{u2, v2}

	// Render the flat-bottom triangle
	// Inverse slope is dX/dy
	var inverseSlope1 T
	var inverseSlope2 T
	if (iy1 - iy0) != 0 {
		inverseSlope1 = T(ix1-ix0) / T(math.Abs(float64(iy1-iy0))) // "Left" leg of the triangle
	}
	if (iy2 - iy0) != 0 {
		inverseSlope2 = T(ix2-ix0) / T(math.Abs(float64(iy2-iy0))) // "Right" leg of the triangle
	}
	// For every scanline
	if iy1-iy0 != 0 {
		for y := iy0; y <= iy1; y++ {
			// The xStart and xEnd come from point-slope formula
			// y-y1 = m(x-x1)
			// (y-y1)/m = x-x1
			// x =  x1 + (y-y1)/m
			xStart := int(T(ix1) + T(y-iy1)*inverseSlope1)
			xEnd := int(T(ix0) + T(y-iy0)*inverseSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTexel(x, y, t, pointA, pointB, pointC, AUV, BUV, CUV, flatShadePercent)
				//drawPixel(x, y, 0xFFFF00FF)
			}

		}
	}
	// Render the flat top triangle
	inverseSlope1, inverseSlope2 = 0.0, 0.0
	if (iy2 - iy1) != 0 {
		inverseSlope1 = T(ix2-ix1) / T(math.Abs(float64(iy2-iy1))) // "Left" leg of the triangle
	}
	if (iy2 - iy0) != 0 {
		inverseSlope2 = T(ix2-ix0) / T(math.Abs(float64(iy2-iy0))) // "Right" leg of the triangle
	}
	// For every scanline
	if iy2-iy1 != 0 {
		for y := iy1; y <= iy2; y++ {
			xStart := int(T(ix1) + T(y-iy1)*inverseSlope1)
			xEnd := int(T(ix0) + T(y-iy0)*inverseSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTexel(x, y, t, pointA, pointB, pointC, AUV, BUV, CUV, flatShadePercent)
				//drawPixel(x, y, 0xFFFF00FF)
			}

		}
	}
}

// edgeCross is the edge function: the Z component of the cross product of edge AB
// and AP. It equals twice the signed area of triangle ABP and is positive when P
// lies to the left of the directed edge A->B (for a positive-area winding). It is
// used both as the inside test and as the (unnormalized) barycentric weight.
func edgeCross[T utils.Numeric](a, b, p vector.Vec2[T]) T {
	ab := b.Sub(a)
	ap := p.Sub(a)
	return ab.X*ap.Y - ab.Y*ap.X
}

// isTopLeft reports whether the directed edge start->end is a top edge or a left
// edge. A top edge is perfectly horizontal and points to the right; a left edge
// points upward (its Y delta is negative). This drives the top-left fill rule:
// pixels exactly on a top or left edge belong to this triangle, pixels on the
// other edges belong to the neighbour, so a shared edge is drawn exactly once.
func isTopLeft[T utils.Numeric](start, end vector.Vec2[T]) bool {
	edge := end.Sub(start)
	isTopEdge := edge.Y == 0 && edge.X > 0
	isLeftEdge := edge.Y < 0
	return isTopEdge || isLeftEdge
}

// drawFilledTrianglePineda rasterizes a flat-colored triangle using Juan Pineda's
// algorithm instead of the flat-top/flat-bottom scanline split. We walk the
// triangle's axis aligned bounding box and use three integer edge functions to
// test which pixels fall inside. The edge functions are evaluated once at the
// bounding box corner and then stepped incrementally (the "iteration by w0/w1/w2"):
// a constant delta is added per column and per row, so there are no multiplies in
// the inner loop. A top-left fill rule biases the edges so shared edges between
// adjacent triangles are drawn exactly once.
// Signature matches drawFilledTriangle so it can be swapped in directly.
func (r *Renderer[T]) drawFilledTrianglePineda(x0f, y0f, z0, w0f, x1f, y1f, z1, w1f, x2f, y2f, z2, w2f T, color uint32) {
	ix0, iy0 := int(x0f), int(y0f)
	ix1, iy1 := int(x1f), int(y1f)
	ix2, iy2 := int(x2f), int(y2f)

	pointA := vector.Vec4[T]{T(ix0), T(iy0), z0, w0f}
	pointB := vector.Vec4[T]{T(ix1), T(iy1), z1, w1f}
	pointC := vector.Vec4[T]{T(ix2), T(iy2), z2, w2f}

	aVec2 := vector.Vec2[int]{X: ix0, Y: iy0}
	bVec2 := vector.Vec2[int]{X: ix1, Y: iy1}
	cVec2 := vector.Vec2[int]{X: ix2, Y: iy2}

	// Signed area can be used at rasterization step to perform backface culling
	area := edgeCross(aVec2, bVec2, cVec2)
	if area <= 0 {
		return
	}

	// Axis aligned bounding box of the triangle, clamped to the screen bounds
	xMin := max(0, min(ix0, ix1, ix2))
	yMin := max(0, min(iy0, iy1, iy2))
	xMax := min(r.Config.WindowWidth-1, max(ix0, ix1, ix2))
	yMax := min(r.Config.WindowHeight-1, max(iy0, iy1, iy2))

	// Per-column (moving in +X) and per-row (moving in +Y) deltas for each edge
	// function. These are constant, which is what lets us step instead of recompute.
	deltaW0Col := bVec2.Y - cVec2.Y
	deltaW1Col := cVec2.Y - aVec2.Y
	deltaW2Col := aVec2.Y - bVec2.Y
	deltaW0Row := cVec2.X - bVec2.X
	deltaW1Row := aVec2.X - cVec2.X
	deltaW2Row := bVec2.X - aVec2.X

	// Top-left fill rule: 0 bias keeps edge pixels, -1 bias drops them
	bias0 := -1
	if isTopLeft(bVec2, cVec2) {
		bias0 = 0
	}
	bias1 := -1
	if isTopLeft(cVec2, aVec2) {
		bias1 = 0
	}
	bias2 := -1
	if isTopLeft(aVec2, bVec2) {
		bias2 = 0
	}

	// Seed the edge functions at the top-left corner of the bounding box
	p0 := vector.Vec2[int]{X: xMin, Y: yMin}
	w0Row := edgeCross(bVec2, cVec2, p0) + bias0
	w1Row := edgeCross(cVec2, aVec2, p0) + bias1
	w2Row := edgeCross(aVec2, bVec2, p0) + bias2

	invArea := 1.0 / float64(area)

	for y := yMin; y <= yMax; y++ {
		w0, w1, w2 := w0Row, w1Row, w2Row
		for x := xMin; x <= xMax; x++ {
			// Pixel is inside the triangle when all three edge functions are >= 0
			if w0 >= 0 && w1 >= 0 && w2 >= 0 {
				// Remove the fill-rule bias before normalizing: the bias is a
				// coverage-only offset, so the barycentric weights used for
				// interpolation must come from the unbiased edge values, otherwise
				// they no longer sum to 1 and depth (especially on small/far
				// triangles, where the bias dominates) comes out wrong.
				alpha := float64(w0-bias0) * invArea
				beta := float64(w1-bias1) * invArea
				gamma := float64(w2-bias2) * invArea

				// Interpolate 1/w to get the per pixel depth, same as the scanline path
				interpolatedReciprocalW := (1/float64(pointA.W))*alpha + (1/float64(pointB.W))*beta + (1/float64(pointC.W))*gamma
				interpolatedReciprocalW = 1 - interpolatedReciprocalW

				if T(interpolatedReciprocalW) < r.ZBuffer[(r.Config.WindowWidth*y)+x] {
					r.drawPixel(x, y, color)
					// Update the Z-buffer with 1/w for current pixel
					r.ZBuffer[(r.Config.WindowWidth*y)+x] = T(interpolatedReciprocalW)
				}
			}
			// Step the edge functions one column to the right
			w0 += deltaW0Col
			w1 += deltaW1Col
			w2 += deltaW2Col
		}
		// Step the edge functions one row down
		w0Row += deltaW0Row
		w1Row += deltaW1Row
		w2Row += deltaW2Row
	}
}

// drawTexturedTrianglePineda is the texture-mapped counterpart of
// drawFilledTrianglePineda: same bounding-box + incremental edge function
// rasterization with the top-left fill rule, but for every inside pixel it
// perspective-correct samples the texture. Signature matches drawTexturedTriangle
// so it can be swapped in directly.
func (r *Renderer[T]) drawTexturedTrianglePineda(x0, y0, z0, w0f, u0, v0, x1, y1, z1, w1f, u1, v1, x2, y2, z2, w2f, u2, v2, flatShadePercent T, t *mesh.TextureData) {
	// Flip the V to account for inverted UV coordinates, V0 is top left for us
	v0 = 1 - v0
	v1 = 1 - v1
	v2 = 1 - v2

	ix0, iy0 := int(x0), int(y0)
	ix1, iy1 := int(x1), int(y1)
	ix2, iy2 := int(x2), int(y2)

	pointA := vector.Vec4[T]{T(ix0), T(iy0), z0, w0f}
	pointB := vector.Vec4[T]{T(ix1), T(iy1), z1, w1f}
	pointC := vector.Vec4[T]{T(ix2), T(iy2), z2, w2f}
	AUV := texture.Tex2[T]{u0, v0}
	BUV := texture.Tex2[T]{u1, v1}
	CUV := texture.Tex2[T]{u2, v2}

	aVec2 := vector.Vec2[int]{X: ix0, Y: iy0}
	bVec2 := vector.Vec2[int]{X: ix1, Y: iy1}
	cVec2 := vector.Vec2[int]{X: ix2, Y: iy2}

	// Signed area can be used to cull the triangles
	area := edgeCross(aVec2, bVec2, cVec2)
	if area <= 0 {
		return // degenerate triangle, nothing to rasterize
	}

	// Axis aligned bounding box of the triangle, clamped to the screen bounds
	xMin := max(0, min(ix0, ix1, ix2))
	yMin := max(0, min(iy0, iy1, iy2))
	xMax := min(r.Config.WindowWidth-1, max(ix0, ix1, ix2))
	yMax := min(r.Config.WindowHeight-1, max(iy0, iy1, iy2))

	// Constant per-column and per-row deltas for the incremental edge functions
	deltaW0Col := bVec2.Y - cVec2.Y
	deltaW1Col := cVec2.Y - aVec2.Y
	deltaW2Col := aVec2.Y - bVec2.Y
	deltaW0Row := cVec2.X - bVec2.X
	deltaW1Row := aVec2.X - cVec2.X
	deltaW2Row := bVec2.X - aVec2.X

	// Top-left fill rule biases
	bias0 := -1
	if isTopLeft(bVec2, cVec2) {
		bias0 = 0
	}
	bias1 := -1
	if isTopLeft(cVec2, aVec2) {
		bias1 = 0
	}
	bias2 := -1
	if isTopLeft(aVec2, bVec2) {
		bias2 = 0
	}

	// Seed the edge functions at the top-left corner of the bounding box
	p0 := vector.Vec2[int]{X: xMin, Y: yMin}
	w0Row := edgeCross(bVec2, cVec2, p0) + bias0
	w1Row := edgeCross(cVec2, aVec2, p0) + bias1
	w2Row := edgeCross(aVec2, bVec2, p0) + bias2

	invArea := 1.0 / float64(area)

	for y := yMin; y <= yMax; y++ {
		w0, w1, w2 := w0Row, w1Row, w2Row
		for x := xMin; x <= xMax; x++ {
			if w0 >= 0 && w1 >= 0 && w2 >= 0 {
				// Use the unbiased edge values for interpolation (the fill-rule
				// bias is for coverage only). Biased weights would not sum to 1 and
				// would corrupt depth/UV on small far triangles.
				alpha := float64(w0-bias0) * invArea
				beta := float64(w1-bias1) * invArea
				gamma := float64(w2-bias2) * invArea

				// Perspective correct interpolation of U/w and V/w
				interpolatedU := float64(AUV.U)/float64(pointA.W)*alpha + float64(BUV.U)/float64(pointB.W)*beta + float64(CUV.U)/float64(pointC.W)*gamma
				interpolatedV := float64(AUV.V)/float64(pointA.W)*alpha + float64(BUV.V)/float64(pointB.W)*beta + float64(CUV.V)/float64(pointC.W)*gamma
				interpolatedReciprocalW := 1/float64(pointA.W)*alpha + 1/float64(pointB.W)*beta + 1/float64(pointC.W)*gamma

				// Divide back both U and V by 1/w
				interpolatedU /= interpolatedReciprocalW
				interpolatedV /= interpolatedReciprocalW

				// UV is always between 0 and 1, scale to the texture resolution.
				// Modulo guards against rounding that lands a border pixel just outside.
				texX := int(math.Abs(interpolatedU*float64(t.Width))) % t.Width
				texY := int(math.Abs(interpolatedV*float64(t.Height))) % t.Height

				// Pixels closer to the viewer need to be bigger than the ones further away
				interpolatedReciprocalW = 1 - interpolatedReciprocalW

				if T(interpolatedReciprocalW) < r.ZBuffer[(r.Config.WindowWidth*y)+x] {
					r.drawPixel(x, y, light.LightApplyIntensity(t.PixelData[(texY*t.Width)+texX], float32(flatShadePercent)))
					// Update the Z-buffer with 1/w for current pixel
					r.ZBuffer[(r.Config.WindowWidth*y)+x] = T(interpolatedReciprocalW)
				}
			}
			// Step the edge functions one column to the right
			w0 += deltaW0Col
			w1 += deltaW1Col
			w2 += deltaW2Col
		}
		// Step the edge functions one row down
		w0Row += deltaW0Row
		w1Row += deltaW1Row
		w2Row += deltaW2Row
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
