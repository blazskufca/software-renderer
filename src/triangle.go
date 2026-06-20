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

// edgeCross is the edge function: the Z component of the cross product of edge AB
// and AP. It equals twice the signed area of triangle ABP and is positive when P
// lies to the left of the directed edge A->B (for a positive-area winding). It is
// used both as the inside test and as the (unnormalized) barycentric weight.
func edgeCross(a, b, p vec2) int {
	ab := b.sub(a)
	ap := p.sub(a)
	return ab.X*ap.Y - ab.Y*ap.X
}

// isTopLeft reports whether the directed edge start->end is a top edge or a left
// edge. A top edge is perfectly horizontal and points to the right; a left edge
// points upward (its Y delta is negative). This drives the top-left fill rule:
// pixels exactly on a top or left edge belong to this triangle, pixels on the
// other edges belong to the neighbour, so a shared edge is drawn exactly once.
func isTopLeft(start, end vec2) bool {
	edge := end.sub(start)
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
func drawFilledTrianglePineda(x0f, y0f, z0, w0f, x1f, y1f, z1, w1f, x2f, y2f, z2, w2f float64, color uint32) {
	pointA := vec4{x0f, y0f, z0, w0f}
	pointB := vec4{x1f, y1f, z1, w1f}
	pointC := vec4{x2f, y2f, z2, w2f}

	aVec2 := vec2FromVec4(pointA)
	bVec2 := vec2FromVec4(pointB)
	cVec2 := vec2FromVec4(pointC)

	// Signed area can be used at rasterization step to perform backface culling
	area := edgeCross(aVec2, bVec2, cVec2)
	if area <= 0 {
		return
	}

	// Axis aligned bounding box of the triangle, clamped to the screen bounds
	xMin := max(0, min(aVec2.X, bVec2.X, cVec2.X))
	yMin := max(0, min(aVec2.Y, bVec2.Y, cVec2.Y))
	xMax := min(windowWidth-1, max(aVec2.X, bVec2.X, cVec2.X))
	yMax := min(windowHeight-1, max(aVec2.Y, bVec2.Y, cVec2.Y))

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
	p0 := vec2{xMin, yMin}
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
				interpolatedReciprocalW := (1/pointA.W)*alpha + (1/pointB.W)*beta + (1/pointC.W)*gamma
				interpolatedReciprocalW = 1 - interpolatedReciprocalW

				if interpolatedReciprocalW < zBuffer[(windowWidth*y)+x] {
					drawPixel(x, y, color)
					// Update the Z-buffer with 1/w for current pixel
					zBuffer[(windowWidth*y)+x] = interpolatedReciprocalW
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
func drawTexturedTrianglePineda(x0, y0, z0, w0f, u0, v0, x1, y1, z1, w1f, u1, v1, x2, y2, z2, w2f, u2, v2, flatShadePercent float64, texture *MeshTextureData) {
	// Flip the V to account for inverted UV coordinates, V0 is top left for us
	v0 = 1 - v0
	v1 = 1 - v1
	v2 = 1 - v2

	pointA := vec4{x0, y0, z0, w0f}
	pointB := vec4{x1, y1, z1, w1f}
	pointC := vec4{x2, y2, z2, w2f}
	AUV := tex2{u0, v0}
	BUV := tex2{u1, v1}
	CUV := tex2{u2, v2}

	aVec2 := vec2FromVec4(pointA)
	bVec2 := vec2FromVec4(pointB)
	cVec2 := vec2FromVec4(pointC)

	// Signed area can be used to cull the triangles
	area := edgeCross(aVec2, bVec2, cVec2)
	if area <= 0 {
		return // degenerate triangle, nothing to rasterize
	}

	// Axis aligned bounding box of the triangle, clamped to the screen bounds
	xMin := max(0, min(aVec2.X, bVec2.X, cVec2.X))
	yMin := max(0, min(aVec2.Y, bVec2.Y, cVec2.Y))
	xMax := min(windowWidth-1, max(aVec2.X, bVec2.X, cVec2.X))
	yMax := min(windowHeight-1, max(aVec2.Y, bVec2.Y, cVec2.Y))

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
	p0 := vec2{xMin, yMin}
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
				interpolatedU := (AUV.u/pointA.W)*alpha + (BUV.u/pointB.W)*beta + (CUV.u/pointC.W)*gamma
				interpolatedV := (AUV.v/pointA.W)*alpha + (BUV.v/pointB.W)*beta + (CUV.v/pointC.W)*gamma
				interpolatedReciprocalW := (1/pointA.W)*alpha + (1/pointB.W)*beta + (1/pointC.W)*gamma

				// Divide back both U and V by 1/w
				interpolatedU /= interpolatedReciprocalW
				interpolatedV /= interpolatedReciprocalW

				// UV is always between 0 and 1, scale to the texture resolution.
				// Modulo guards against rounding that lands a border pixel just outside.
				texX := int(math.Abs(interpolatedU*float64(texture.width))) % texture.width
				texY := int(math.Abs(interpolatedV*float64(texture.height))) % texture.height

				// Pixels closer to the viewer need to be bigger than the ones further away
				interpolatedReciprocalW = 1 - interpolatedReciprocalW

				if interpolatedReciprocalW < zBuffer[(windowWidth*y)+x] {
					drawPixel(x, y, lightApplyIntensity(texture.pixelData[(texY*texture.width)+texX], flatShadePercent))
					// Update the Z-buffer with 1/w for current pixel
					zBuffer[(windowWidth*y)+x] = interpolatedReciprocalW
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
