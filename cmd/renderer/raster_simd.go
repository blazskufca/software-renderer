//go:build goexperiment.simd

// This file holds the SIMD rewrites of the two Pineda rasterizers. It is only
// compiled when the binary is built with GOEXPERIMENT=simd (see the Makefile);
// raster_simd_stub.go supplies the forwarding counterparts otherwise.
//
// Why the Pineda rasterizers and nothing else: they are the only hot loops that
// are both branch-light and index-linear. Their inner loop already steps three
// edge functions by a constant per column, which is exactly the shape a vector
// unit wants — no gather, no dependency between neighbouring pixels, and the
// per-pixel work (barycentric normalize, 1/w interpolation, Gouraud intensity,
// light modulation) is pure arithmetic. The scanline rasterizers, by contrast,
// recompute barycentric weights from scratch per pixel through
// drawTexel/drawTrianglePixel, and the clipping and matrix code works on 3- and
// 4-component vectors, which is narrower than a single vector register.
//
// Three things about these loops are not obvious, and all three were worth
// several times the speedup the vectorization itself buys:
//
//  1. A vector loop cannot reject a pixel the way a scalar one can, so the
//     uncovered part of the bounding box has to be removed before the loop
//     runs. See simdEdgeSetup.rowSpan.
//
//  2. Deriving the barycentric weights per chunk needs more live vectors than
//     the machine has registers. Interpolating each attribute directly does
//     not. See simdAttrVectors.
//
//  3. NO SCALAR FLOATING-POINT ARITHMETIC MAY APPEAR IN THE INNER LOOP, and
//     every broadcast of a float constant must be hoisted out of it. Go compiles
//     scalar float code to legacy SSE (MOVSS/ADDSD/XORPS/CVT*), and a legacy SSE
//     instruction inside a loop that also touches 256-bit registers triggers the
//     AVX/SSE transition penalty. Measured on Zen 3, a single `base += step` in
//     float64 took one iteration of the fill loop from 4.3ns to 65ns — fifteen
//     times slower, and enough to lose to the scalar rasterizer outright. That
//     is why the attributes are carried as vector accumulators and why the light
//     modulation takes its clamp constants as parameters. Integer scalar work
//     and integer broadcasts are fine; only floating point is affected.
//
// Everything that touches a simd value also lives in a plain (non-generic)
// function: the compiler's SIMD rewrite pass does not run inside generic bodies,
// so the Renderer[T] methods here only widen their arguments to float32 and
// forward. For the same reason no simd value is ever stored in a struct field —
// the exported vector types are declared 16 bytes wide whatever length the
// program actually runs at, so a wider vector cannot stay in the field.
package main

import (
	"math"
	"simd"

	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/vector"
)

const (
	// simdCompiledIn reports that this build has the simd package linked in.
	simdCompiledIn = true

	// simdMaxLanes is the largest number of float32 lanes any supported vector
	// length can hold (512-bit / 32-bit). It bounds the scratch arrays used for
	// the texture gather and, via simdPadPixels, the framebuffer padding.
	simdMaxLanes = 16

	// simdPadPixels is the number of extra pixels appended to the colour and
	// depth buffers. A vector store always writes a full register, so the last
	// chunk of the last scanline can reach up to simdMaxLanes-1 pixels past the
	// visible buffer. Padding the allocation lets the inner loop use plain
	// (full-width) loads and stores with no tail branch; the lanes that land in
	// the padding are masked off and simply write back what they read.
	simdPadPixels = simdMaxLanes

	// simdCoordLimit bounds the screen-space vertex coordinates the SIMD path
	// accepts. Every edge value is a difference of products of coordinate
	// deltas, so |edge| <= 8*L^2; L = 16384 keeps that comfortably inside the
	// int32 lanes. Frustum clipping already keeps projected vertices on screen,
	// so this only trips on pathological input, where we defer to the scalar
	// rasterizer rather than silently wrapping.
	simdCoordLimit = 16384

	// simdMinFillArea and simdMinTextureArea are the smallest screen-clamped
	// bounding boxes, in pixels, for which the vector path is worth taking.
	//
	// Vectorizing trades a fixed per-triangle cost — bounding box and edge
	// setup, the interpolation coefficients, and a broadcast for each of them —
	// for a cheaper per-pixel one, and below some size that trade is a loss.
	// Models built from small triangles sit squarely in the losing region: on the
	// shipped race.obj, whose triangles average about thirteen covered pixels,
	// the textured vector path measured 1.6x SLOWER than the scalar one before
	// this cutoff existed. BenchmarkTriangleSizeSweep is the tool for re-checking
	// these numbers; on Zen 3 the crossover is a bounding box of roughly 16x16
	// for the fill path and 24x24 for the textured one, which does more setup.
	simdMinFillArea    = 16 * 16
	simdMinTextureArea = 24 * 24

	// Bit patterns of the float32 constants the rasterizers broadcast, spelled
	// out so simdBroadcastF32 can take the integer path. Written as constants
	// rather than math.Float32bits calls so they are unmistakably folded.
	simdF32Zero = 0x00000000
	simdF32One  = 0x3F800000
)

// simdBroadcastF32 broadcasts a float32 given by its bit pattern.
//
// Broadcasting a float32 value directly compiles to a legacy-SSE load followed
// by VBROADCASTSS, and each of those is a transition into and out of 256-bit
// state — see item 3 of the file comment. Going through the bits keeps the whole
// broadcast on the integer path (MOVL / VMOVD / VPBROADCASTD), and the reinterpret
// back to float costs nothing. The rasterizer prologues do about twenty of these
// per triangle, so it is worth the indirection.
func simdBroadcastF32(bits uint32) simd.Float32s {
	return simd.BroadcastUint32s(bits).BitsToFloat32()
}

// simdLaneOffsets is the constant [0, 1, 2, ...] lane index vector, materialized
// through a plain array because the simd package does not support
// SIMD-dependent package-level variable initializers.
var simdLaneOffsets = [simdMaxLanes]int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// simdTarget is the non-generic view of the render target that the vector code
// writes to: the packed R,G,B,A colour bytes and the float32 depth buffer, both
// including the simdPadPixels of overrun slack described in
// Renderer.allocateFramebuffers.
type simdTarget struct {
	color  []byte
	depth  []float32
	width  int
	height int
}

// simdTarget builds the non-generic render target the vector rasterizers take.
// It is only valid when zBufferF32 is non-nil, i.e. when T is float32.
func (r *Renderer[T]) simdTarget() simdTarget {
	return simdTarget{
		color:  r.colorBuffer,
		depth:  r.zBufferF32,
		width:  r.config.WindowWidth,
		height: r.config.WindowHeight,
	}
}

// simdEdgeSetup carries the per-triangle constants both SIMD rasterizers need:
// the screen-clamped bounding box, the seeds and deltas for the three
// incremental edge functions, the top-left fill-rule biases and 1/area. It is
// the vector-friendly restatement of the prologue in drawFilledTrianglePineda.
type simdEdgeSetup struct {
	xMin, yMin, xMax, yMax int
	// w0Row/w1Row/w2Row are the edge functions evaluated at (xMin, yMin),
	// already biased by the fill rule.
	w0Row, w1Row, w2Row int
	// dCol* step one pixel in +X, dRow* step one scanline in +Y.
	dCol0, dCol1, dCol2 int
	dRow0, dRow1, dRow2 int
	bias0, bias1, bias2 int
	invArea             float64
}

// simdCoordsInRange reports whether the triangle's screen coordinates are small
// enough for the int32 edge functions not to overflow. See simdCoordLimit.
func simdCoordsInRange(ix0, iy0, ix1, iy1, ix2, iy2 int) bool {
	return min(ix0, iy0, ix1, iy1, ix2, iy2) >= -simdCoordLimit &&
		max(ix0, iy0, ix1, iy1, ix2, iy2) <= simdCoordLimit
}

// newSimdEdgeSetup computes the bounding box, edge seeds/deltas and fill-rule
// biases for the triangle. It reports false when there is nothing to rasterize:
// a back-facing or degenerate triangle (area <= 0), or a bounding box that is
// entirely off screen.
func newSimdEdgeSetup(ix0, iy0, ix1, iy1, ix2, iy2, width, height int) (simdEdgeSetup, bool) {
	a := vector.Vec2[int]{X: ix0, Y: iy0}
	b := vector.Vec2[int]{X: ix1, Y: iy1}
	c := vector.Vec2[int]{X: ix2, Y: iy2}

	// Signed area doubles as the backface test, exactly as in the scalar path.
	area := edgeCross(a, b, c)
	if area <= 0 {
		return simdEdgeSetup{}, false
	}

	s := simdEdgeSetup{
		xMin:    max(0, min(ix0, ix1, ix2)),
		yMin:    max(0, min(iy0, iy1, iy2)),
		xMax:    min(width-1, max(ix0, ix1, ix2)),
		yMax:    min(height-1, max(iy0, iy1, iy2)),
		dCol0:   b.Y - c.Y,
		dCol1:   c.Y - a.Y,
		dCol2:   a.Y - b.Y,
		dRow0:   c.X - b.X,
		dRow1:   a.X - c.X,
		dRow2:   b.X - a.X,
		invArea: 1 / float64(area),
	}
	if s.xMin > s.xMax || s.yMin > s.yMax {
		return simdEdgeSetup{}, false
	}

	// Top-left fill rule: 0 bias keeps edge pixels, -1 bias drops them.
	s.bias0, s.bias1, s.bias2 = -1, -1, -1
	if isTopLeft(b, c) {
		s.bias0 = 0
	}
	if isTopLeft(c, a) {
		s.bias1 = 0
	}
	if isTopLeft(a, b) {
		s.bias2 = 0
	}

	p0 := vector.Vec2[int]{X: s.xMin, Y: s.yMin}
	s.w0Row = edgeCross(b, c, p0) + s.bias0
	s.w1Row = edgeCross(c, a, p0) + s.bias1
	s.w2Row = edgeCross(a, b, p0) + s.bias2
	return s, true
}

// simdWorthVectorizing reports whether a triangle is big enough for the vector
// path, given the smallest bounding box that path is worth taking for.
//
// It works straight off the vertex coordinates rather than from a simdEdgeSetup
// so that a triangle the vector path is going to decline costs almost nothing to
// decline: below the cutoff the caller falls back to the scalar rasterizer,
// which computes its own edge setup, and building one here first would be pure
// waste on exactly the triangles that are already too small to absorb it.
func simdWorthVectorizing(ix0, iy0, ix1, iy1, ix2, iy2, width, height, minArea int) bool {
	w := min(width-1, max(ix0, ix1, ix2)) - max(0, min(ix0, ix1, ix2)) + 1
	h := min(height-1, max(iy0, iy1, iy2)) - max(0, min(iy0, iy1, iy2)) + 1
	return w > 0 && h > 0 && w*h >= minArea
}

// rowSpan narrows a scanline to the exact range of x where all three edge
// functions are non-negative, given their (biased) values w0/w1/w2 at x == xMin.
//
// This is what makes vectorizing worthwhile at all. The scalar rasterizer walks
// the whole bounding box and rejects an uncovered pixel with three compares,
// which is so cheap that a vector loop doing full per-pixel work over the same
// box loses badly — a triangle covers well under half of its bounding box. There
// is no per-lane early-out to win that back (and no horizontal reduction in the
// portable API with which to ask "did any lane survive"), so the uncovered
// pixels have to go before the loop starts: a triangle is convex, so its
// intersection with a scanline is a single interval, and each edge function is
// linear in x, so solving w + (x-xMin)*d >= 0 per edge yields that interval
// exactly. Three integer divisions per scanline replace a row of rejected
// pixels, and every lane inside the returned span is covered by construction —
// only the tail of the last chunk still needs masking.
func (s *simdEdgeSetup) rowSpan(w0, w1, w2 int) (lo, hi int, ok bool) {
	lo, hi = s.xMin, s.xMax
	for _, e := range [3]struct{ w, d int }{{w0, s.dCol0}, {w1, s.dCol1}, {w2, s.dCol2}} {
		switch {
		case e.d > 0:
			// w + (x-xMin)*d >= 0  =>  x >= xMin + ceil(-w/d)
			lo = max(lo, s.xMin+ceilDiv(-e.w, e.d))
		case e.d < 0:
			// x <= xMin + floor(w/-d)
			hi = min(hi, s.xMin+floorDiv(e.w, -e.d))
		case e.w < 0:
			// Constant along the row and negative: nothing on this scanline.
			return 0, 0, false
		}
	}
	return lo, hi, lo <= hi
}

// spanEdges returns the three unbiased edge values at the first pixel of the
// span starting at lo, given the biased row values. The fill-rule bias is a
// coverage-only offset: it decides rowSpan's endpoints, but the barycentric
// weights must come from the unbiased values or they would not sum to 1.
func (s *simdEdgeSetup) spanEdges(w0, w1, w2, lo int) (e0, e1, e2 int) {
	span := lo - s.xMin
	return w0 - s.bias0 + span*s.dCol0,
		w1 - s.bias1 + span*s.dCol1,
		w2 - s.bias2 + span*s.dCol2
}

// ceilDiv and floorDiv round the quotient toward +inf and -inf respectively, for
// a positive divisor. Go's integer division truncates toward zero, which already
// rounds up for negative numerators and down for positive ones, so each only has
// to correct the other sign.
func ceilDiv(a, b int) int {
	q := a / b
	if a > 0 && q*b != a {
		q++
	}
	return q
}

func floorDiv(a, b int) int {
	q := a / b
	if a < 0 && q*b != a {
		q--
	}
	return q
}

// simdAttrCoeffs and simdAttrVecs together prepare one interpolated vertex
// attribute — per-vertex values pA/pB/pC, plus a constant bias added to every
// sample — for evaluation a chunk of pixels at a time.
//
// The scalar rasterizer derives alpha/beta/gamma from the three edge values at
// every pixel and forms each attribute from them. Doing that per chunk needs the
// three integer edge accumulators, their per-chunk deltas, the three bias
// vectors, 1/area and a coefficient vector per vertex per attribute all live at
// once — well past the sixteen vector registers AVX2 has.
//
// Interpolating the attribute directly avoids that. attribute = (pA*e0 + pB*e1 +
// pC*e2)/area is a fixed linear combination of edge functions that are
// themselves linear in x, so the attribute is affine in x: one seed per scanline
// (simdAttrSeed) and one vector add per chunk. Only two registers per attribute
// stay live in the inner loop, the running value and chunkStep.
//
// The split into two calls is deliberate and load-bearing. simdAttrCoeffs is
// scalar floating point; simdAttrVecs is vector. Callers run every
// simdAttrCoeffs first and only then every simdAttrVecs, so the transition
// between the two instruction sets happens once per triangle rather than once
// per coefficient — see item 3 of the file comment. Fusing them back into one
// function costs about a third of the textured rasterizer's runtime.

// simdAttrCoeffsLen is how many scalar coefficients one attribute needs.
const simdAttrCoeffsLen = 5

// simdAttrSpec describes one attribute to simdAttrCoeffs: its value at each of
// the three vertices, plus a constant added to every sample.
type simdAttrSpec struct{ pA, pB, pC, bias float64 }

// simdAttrCoeffs converts every attribute of a triangle into float32 bit
// patterns (see simdBroadcastF32), five per attribute: the three edge scale
// factors, the constant bias, and the per-pixel step.
//
// This is the entire floating-point half of the setup, and it deliberately runs
// in the scalar half of the rasterizer, never beside vector code — see item 3 of
// the file comment.
func simdAttrCoeffs(dst []uint32, s *simdEdgeSetup, attrs []simdAttrSpec) {
	for i, a := range attrs {
		perPixel := s.invArea * (a.pA*float64(s.dCol0) + a.pB*float64(s.dCol1) + a.pC*float64(s.dCol2))
		c := dst[i*simdAttrCoeffsLen:]
		c[0] = math.Float32bits(float32(s.invArea * a.pA))
		c[1] = math.Float32bits(float32(s.invArea * a.pB))
		c[2] = math.Float32bits(float32(s.invArea * a.pC))
		c[3] = math.Float32bits(float32(a.bias))
		c[4] = math.Float32bits(float32(perPixel))
	}
}

// simdAttrVecs broadcasts the coefficients simdAttrCoeffs produced. cA/cB/cC and
// laneBias feed simdAttrSeed; laneBias carries both the constant bias and the
// [0*step, 1*step, ...] spread across the lanes. chunkStep is derived here
// rather than in simdAttrCoeffs so the scalar side never needs to know the
// vector length. lanesF must be the lane count as a float vector.
func simdAttrVecs(c []uint32, laneF, lanesF simd.Float32s) (cA, cB, cC, laneBias, chunkStep simd.Float32s) {
	perPixel := simdBroadcastF32(c[4])
	return simdBroadcastF32(c[0]),
		simdBroadcastF32(c[1]),
		simdBroadcastF32(c[2]),
		laneF.Mul(perPixel).Add(simdBroadcastF32(c[3])),
		perPixel.Mul(lanesF)
}

// simdAttrSeed evaluates an attribute at the first chunk of a scanline span.
// e0f/e1f/e2f are the span's unbiased edge values broadcast as floats; they are
// shared by every attribute of the triangle. Seeding from the exact integer edge
// values each scanline, rather than carrying the value down from the previous
// one, keeps the accumulated error to a single row's worth of chunks.
func simdAttrSeed(cA, cB, cC, laneBias, e0f, e1f, e2f simd.Float32s) simd.Float32s {
	return cA.Mul(e0f).Add(cB.Mul(e1f)).Add(cC.Mul(e2f)).Add(laneBias)
}

// simdEdgeFloats broadcasts the three integer edge values as float vectors, via
// the integer broadcast so that no scalar float conversion enters the loop.
func simdEdgeFloats(e0, e1, e2 int) (simd.Float32s, simd.Float32s, simd.Float32s) {
	return simd.BroadcastInt32s(int32(e0)).ConvertToFloat32(),
		simd.BroadcastInt32s(int32(e1)).ConvertToFloat32(),
		simd.BroadcastInt32s(int32(e2)).ConvertToFloat32()
}

// simdScaleChannel multiplies one packed colour channel by f, mirroring
// light.LightApplyIntensity: the channel is scaled in place (still shifted into
// its byte position) and re-masked afterwards. Every masked channel value is
// below 2^24, so it round-trips through float32 exactly and the int32
// reinterprets are lossless.
func simdScaleChannel(c simd.Uint32s, channelMask uint32, f simd.Float32s) simd.Uint32s {
	m := simd.BroadcastUint32s(channelMask)
	scaled := c.And(m).BitsToInt32().ConvertToFloat32().Mul(f)
	return scaled.ConvertToInt32().ToBits().And(m)
}

// simdLightApplyIntensity is the vector form of light.LightApplyIntensity: it
// clamps the intensity to [0,1] and scales the R, G and B channels of each lane
// while leaving alpha untouched.
//
// zeroF and oneF are the clamp bounds, passed in rather than broadcast here.
// Broadcasting a float constant compiles to legacy SSE, which must not appear
// inside a 256-bit loop — see item 3 of the file comment. The integer mask
// broadcasts in simdScaleChannel are unaffected and stay where they are.
func simdLightApplyIntensity(color simd.Uint32s, intensity, zeroF, oneF simd.Float32s) simd.Uint32s {
	f := intensity.Max(zeroF).Min(oneF)
	return color.And(simd.BroadcastUint32s(0xFF000000)).
		Or(simdScaleChannel(color, 0x00FF0000, f)).
		Or(simdScaleChannel(color, 0x0000FF00, f)).
		Or(simdScaleChannel(color, 0x000000FF, f))
}

// simdTailMask marks the lanes of the chunk starting at x that are still within
// a span ending at hi. Every other lane of a chunk is covered by construction
// (see rowSpan), so this is the only coverage test left in the inner loop.
func simdTailMask(laneIdx simd.Int32s, x, hi int) simd.Mask32s {
	return laneIdx.Less(simd.BroadcastInt32s(int32(hi - x + 1)))
}

// drawFilledTrianglePinedaSIMD forwards to rasterFilledPinedaSIMD, falling back
// to the scalar Pineda rasterizer when the vector path cannot take the triangle.
func (r *Renderer[T]) drawFilledTrianglePinedaSIMD(x0f, y0f, z0, w0f, x1f, y1f, z1, w1f, x2f, y2f, z2, w2f T, color uint32, i0, i1, i2 T) {
	if r.zBufferF32 != nil && rasterFilledPinedaSIMD(r.simdTarget(),
		float32(x0f), float32(y0f), float32(w0f),
		float32(x1f), float32(y1f), float32(w1f),
		float32(x2f), float32(y2f), float32(w2f),
		color, float32(i0), float32(i1), float32(i2)) {
		return
	}
	r.drawFilledTrianglePineda(x0f, y0f, z0, w0f, x1f, y1f, z1, w1f, x2f, y2f, z2, w2f, color, i0, i1, i2)
}

// rasterFilledPinedaSIMD is the scalar half of the SIMD rewrite of
// drawFilledTrianglePineda: it works out the bounding box, the edge functions
// and the interpolation coefficients, then hands them to rasterFilledSpansSIMD,
// which does the vector work. The split is not cosmetic — this half is all
// floating-point scalar code, which must not sit next to 256-bit vector code
// (item 3 of the file comment), and keeping it in its own function is what keeps
// the two apart.
//
// It reports false, having drawn nothing, when it declines the triangle: either
// the coordinates are too large for the int32 edge functions (simdCoordLimit) or
// it is too small to be worth vectorizing (simdMinFillArea). The caller must
// then fall back to the scalar rasterizer.
//
// Note that z0/z1/z2 are absent: like the scalar version, depth comes from the
// interpolated 1/w and the vertex Z is never read.
func rasterFilledPinedaSIMD(tgt simdTarget, x0, y0, wA, x1, y1, wB, x2, y2, wC float32, color uint32, i0, i1, i2 float32) bool {
	ix0, iy0 := int(x0), int(y0)
	ix1, iy1 := int(x1), int(y1)
	ix2, iy2 := int(x2), int(y2)

	if !simdCoordsInRange(ix0, iy0, ix1, iy1, ix2, iy2) ||
		!simdWorthVectorizing(ix0, iy0, ix1, iy1, ix2, iy2, tgt.width, tgt.height, simdMinFillArea) {
		return false
	}
	s, ok := newSimdEdgeSetup(ix0, iy0, ix1, iy1, ix2, iy2, tgt.width, tgt.height)
	if !ok {
		return true // nothing to draw: degenerate, back-facing or off screen
	}

	// Depth is 1 minus the interpolated 1/w, so interpolating -1/w with a bias
	// of 1 folds that into one affine attribute and saves a subtract per chunk.
	// Intensity is the Gouraud term; flat shading feeds three equal values, so
	// it collapses to a constant.
	specs := [2]simdAttrSpec{
		{float64(-1 / wA), float64(-1 / wB), float64(-1 / wC), 1},
		{float64(i0), float64(i1), float64(i2), 0},
	}
	var coeffs [2 * simdAttrCoeffsLen]uint32
	simdAttrCoeffs(coeffs[:], &s, specs[:])

	rasterFilledSpansSIMD(tgt, &s, coeffs[:], color)
	return true
}

// rasterFilledSpansSIMD shades every covered pixel of the triangle described by
// s and coeffs. It covers the same pixels as drawFilledTrianglePineda under the
// same top-left fill rule, but a whole vector of them per iteration.
//
// There is no masked store in the portable simd API, so instead of skipping
// lanes we blend: every iteration reads the existing depth and colour, computes
// the new values, selects per lane with IfElse and writes the full vector back.
// Lanes that fail the depth test or that ran past the end of the span therefore
// store exactly what they loaded.
//
// Unlike the scalar rasterizer, which narrows to float32 only when storing, this
// path shades at float32 — the width the depth buffer and the vertex attributes
// are already stored at. Results can differ in the last bits of a colour
// channel; they do not differ visibly. Every operation below is either vector or
// integer: no scalar floating point appears anywhere in this function.
func rasterFilledSpansSIMD(tgt simdTarget, s *simdEdgeSetup, coeffs []uint32, color uint32) {
	lanes := simd.Float32s{}.Len()
	laneIdx := simd.LoadInt32s(simdLaneOffsets[:])
	laneF := laneIdx.ConvertToFloat32()
	lanesF := simd.BroadcastInt32s(int32(lanes)).ConvertToFloat32()

	zeroF := simdBroadcastF32(simdF32Zero)
	oneF := simdBroadcastF32(simdF32One)
	vColor := simd.BroadcastUint32s(color)
	zCA, zCB, zCC, zBias, zStep := simdAttrVecs(coeffs[0:], laneF, lanesF)
	iCA, iCB, iCC, iBias, iStep := simdAttrVecs(coeffs[simdAttrCoeffsLen:], laneF, lanesF)

	w0Row, w1Row, w2Row := s.w0Row, s.w1Row, s.w2Row
	for y := s.yMin; y <= s.yMax; y++ {
		if lo, hi, covered := s.rowSpan(w0Row, w1Row, w2Row); covered {
			e0f, e1f, e2f := simdEdgeFloats(s.spanEdges(w0Row, w1Row, w2Row, lo))
			z := simdAttrSeed(zCA, zCB, zCC, zBias, e0f, e1f, e2f)
			intensity := simdAttrSeed(iCA, iCB, iCC, iBias, e0f, e1f, e2f)

			idx := y*tgt.width + lo
			for x := lo; x <= hi; x += lanes {
				oldZ := simd.LoadFloat32s(tgt.depth[idx:])
				write := simdTailMask(laneIdx, x, hi).And(z.Less(oldZ))

				shaded := simdLightApplyIntensity(vColor, intensity, zeroF, oneF)
				oldC := simd.LoadUint8s(tgt.color[idx*4:]).ReshapeToUint32s()
				shaded.IfElse(write, oldC).ReshapeToUint8s().Store(tgt.color[idx*4:])
				z.IfElse(write, oldZ).Store(tgt.depth[idx:])

				z = z.Add(zStep)
				intensity = intensity.Add(iStep)
				idx += lanes
			}
		}

		w0Row += s.dRow0
		w1Row += s.dRow1
		w2Row += s.dRow2
	}
}

// drawTexturedTrianglePinedaSIMD forwards to rasterTexturedPinedaSIMD, falling
// back to the scalar Pineda rasterizer when the vector path cannot take the
// triangle.
func (r *Renderer[T]) drawTexturedTrianglePinedaSIMD(x0, y0, z0, w0f, u0, v0, x1, y1, z1, w1f, u1, v1, x2, y2, z2, w2f, u2, v2, i0, i1, i2 T, t *mesh.TextureData) {
	if r.zBufferF32 != nil && rasterTexturedPinedaSIMD(r.simdTarget(),
		float32(x0), float32(y0), float32(w0f), float32(u0), float32(v0),
		float32(x1), float32(y1), float32(w1f), float32(u1), float32(v1),
		float32(x2), float32(y2), float32(w2f), float32(u2), float32(v2),
		float32(i0), float32(i1), float32(i2), t) {
		return
	}
	r.drawTexturedTrianglePineda(x0, y0, z0, w0f, u0, v0, x1, y1, z1, w1f, u1, v1, x2, y2, z2, w2f, u2, v2, i0, i1, i2, t)
}

// rasterTexturedPinedaSIMD is the scalar half of the SIMD rewrite of
// drawTexturedTrianglePineda, mirroring rasterFilledPinedaSIMD: it prepares the
// edge functions and interpolation coefficients and hands them to
// rasterTexturedSpansSIMD. See rasterFilledPinedaSIMD for why the halves are
// separate functions, and for what a false return means.
func rasterTexturedPinedaSIMD(tgt simdTarget, x0, y0, wA, u0, v0, x1, y1, wB, u1, v1, x2, y2, wC, u2, v2, i0, i1, i2 float32, t *mesh.TextureData) bool {
	ix0, iy0 := int(x0), int(y0)
	ix1, iy1 := int(x1), int(y1)
	ix2, iy2 := int(x2), int(y2)

	if !simdCoordsInRange(ix0, iy0, ix1, iy1, ix2, iy2) ||
		!simdWorthVectorizing(ix0, iy0, ix1, iy1, ix2, iy2, tgt.width, tgt.height, simdMinTextureArea) {
		return false
	}
	s, ok := newSimdEdgeSetup(ix0, iy0, ix1, iy1, ix2, iy2, tgt.width, tgt.height)
	if !ok {
		return true
	}

	// Perspective-correct interpolation works on U/w, V/w and 1/w, all three of
	// which are linear in screen space; U and V come back out divided by the
	// interpolated 1/w. Scaling to the texture resolution commutes with that
	// divide, so it folds into the U/V coefficients here instead of costing two
	// more broadcasts in the loop. V is flipped first, for the inverted UV
	// convention (V0 is top left for us).
	invWA, invWB, invWC := float64(1/wA), float64(1/wB), float64(1/wC)
	texW, texH := float64(t.Width), float64(t.Height)
	specs := [4]simdAttrSpec{
		{invWA, invWB, invWC, 0},
		{float64(u0) * invWA * texW, float64(u1) * invWB * texW, float64(u2) * invWC * texW, 0},
		{float64(1-v0) * invWA * texH, float64(1-v1) * invWB * texH, float64(1-v2) * invWC * texH, 0},
		{float64(i0), float64(i1), float64(i2), 0},
	}
	var coeffs [4 * simdAttrCoeffsLen]uint32
	simdAttrCoeffs(coeffs[:], &s, specs[:])

	rasterTexturedSpansSIMD(tgt, &s, coeffs[:], t)
	return true
}

// rasterTexturedSpansSIMD is the vector half of the textured rasterizer. The
// coverage, depth and lighting machinery is the same as rasterFilledSpansSIMD;
// the extra work is the perspective divide and the texel fetch.
//
// The portable simd API has no gather, so the texel fetch is the one scalar step
// left in the loop: the wrapped texture coordinates are stored to a small stack
// array, read back with ordinary indexing and reloaded as a vector. That is all
// integer work, so it does not disturb the vector state. Everything around it —
// both interpolations, the perspective divide, the depth test and the light
// modulation — stays vectorized.
func rasterTexturedSpansSIMD(tgt simdTarget, s *simdEdgeSetup, coeffs []uint32, t *mesh.TextureData) {
	// A power-of-two texture lets the wrap be a vector AND, which is
	// bit-identical to `% size` for the non-negative indices we produce and
	// keeps the whole wrap in the vector unit. Otherwise the modulo has to run
	// scalar next to the gather, because the vector API has no integer division.
	wMask, hMask := t.Width-1, t.Height-1
	pow2 := t.Width > 0 && t.Height > 0 && t.Width&wMask == 0 && t.Height&hMask == 0

	lanes := simd.Float32s{}.Len()
	laneIdx := simd.LoadInt32s(simdLaneOffsets[:])
	laneF := laneIdx.ConvertToFloat32()
	lanesF := simd.BroadcastInt32s(int32(lanes)).ConvertToFloat32()

	zeroI := simd.BroadcastInt32s(0)
	vWMask := simd.BroadcastInt32s(int32(wMask))
	vHMask := simd.BroadcastInt32s(int32(hMask))
	zeroF := simdBroadcastF32(simdF32Zero)
	oneF := simdBroadcastF32(simdF32One)

	rCA, rCB, rCC, rBias, rStep := simdAttrVecs(coeffs[0:], laneF, lanesF)
	uCA, uCB, uCC, uBias, uStep := simdAttrVecs(coeffs[simdAttrCoeffsLen:], laneF, lanesF)
	vCA, vCB, vCC, vBias, vStep := simdAttrVecs(coeffs[2*simdAttrCoeffsLen:], laneF, lanesF)
	iCA, iCB, iCC, iBias, iStep := simdAttrVecs(coeffs[3*simdAttrCoeffsLen:], laneF, lanesF)

	var texX, texY [simdMaxLanes]int32
	var texels [simdMaxLanes]uint32

	w0Row, w1Row, w2Row := s.w0Row, s.w1Row, s.w2Row
	for y := s.yMin; y <= s.yMax; y++ {
		if lo, hi, covered := s.rowSpan(w0Row, w1Row, w2Row); covered {
			e0f, e1f, e2f := simdEdgeFloats(s.spanEdges(w0Row, w1Row, w2Row, lo))
			recipW := simdAttrSeed(rCA, rCB, rCC, rBias, e0f, e1f, e2f)
			uOverW := simdAttrSeed(uCA, uCB, uCC, uBias, e0f, e1f, e2f)
			vOverW := simdAttrSeed(vCA, vCB, vCC, vBias, e0f, e1f, e2f)
			intensity := simdAttrSeed(iCA, iCB, iCC, iBias, e0f, e1f, e2f)

			idx := y*tgt.width + lo
			for x := lo; x <= hi; x += lanes {
				// One reciprocal serves both coordinates; vector divide is by
				// far the most expensive operation in this loop. Abs plus
				// Max(0) keeps the texel index non-negative even when the
				// barycentric weights blow up on a border pixel, or when a
				// degenerate divide yields a NaN (which converts to INT_MIN).
				w := oneF.Div(recipW)
				tx := uOverW.Mul(w).Abs().ConvertToInt32().Max(zeroI)
				ty := vOverW.Mul(w).Abs().ConvertToInt32().Max(zeroI)
				if pow2 {
					tx = tx.And(vWMask)
					ty = ty.And(vHMask)
				}
				tx.Store(texX[:])
				ty.Store(texY[:])

				z := oneF.Sub(recipW)
				oldZ := simd.LoadFloat32s(tgt.depth[idx:])
				write := simdTailMask(laneIdx, x, hi).And(z.Less(oldZ))

				if pow2 {
					for l := range lanes {
						texels[l] = t.PixelData[int(texY[l])*t.Width+int(texX[l])]
					}
				} else {
					for l := range lanes {
						texels[l] = t.PixelData[(int(texY[l])%t.Height)*t.Width+int(texX[l])%t.Width]
					}
				}

				shaded := simdLightApplyIntensity(simd.LoadUint32s(texels[:]), intensity, zeroF, oneF)
				oldC := simd.LoadUint8s(tgt.color[idx*4:]).ReshapeToUint32s()
				shaded.IfElse(write, oldC).ReshapeToUint8s().Store(tgt.color[idx*4:])
				z.IfElse(write, oldZ).Store(tgt.depth[idx:])

				recipW = recipW.Add(rStep)
				uOverW = uOverW.Add(uStep)
				vOverW = vOverW.Add(vStep)
				intensity = intensity.Add(iStep)
				idx += lanes
			}
		}

		w0Row += s.dRow0
		w1Row += s.dRow1
		w2Row += s.dRow2
	}
}
