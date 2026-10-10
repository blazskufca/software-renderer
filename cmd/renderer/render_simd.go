//go:build simd

// Opt-in backend (-tags simd): geometry goes through internal/pipeline and the
// Pineda modes through internal/raster, whose kernels are vectorized when also
// built with GOEXPERIMENT=simd (scalar otherwise).

package main

import (
	"math"

	"github.com/blazskufca/software-renderer/internal/config"
	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/light"
	"github.com/blazskufca/software-renderer/internal/pipeline"
	"github.com/blazskufca/software-renderer/internal/raster"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
	"github.com/hajimehoshi/ebiten/v2"
)

// rasterState is the backend-specific part of Renderer.
type rasterState[T utils.Numeric] struct {
	target          *raster.Target
	trianglesToDraw []raster.Triangle
	geometry        pipeline.Params[T] // kept between frames for its scratch buffers
}

// drawTriangles dispatches on the mode itself, so the setters only record it.
func (r *Renderer[T]) setFillRaster(mode config.FillRasterMode)       { r.fillRasterMode = mode }
func (r *Renderer[T]) setTextureRaster(mode config.TextureRasterizer) { r.textureRasterMode = mode }

func (r *Renderer[T]) initBuffers() error {
	target, err := raster.NewTarget(r.config.WindowWidth, r.config.WindowHeight)
	if err != nil {
		return err
	}
	r.target = target
	r.target.Clear(0xFF000000)
	return nil
}

func (r *Renderer[T]) drawTriangles() {
	for i := range r.trianglesToDraw {
		tri := &r.trianglesToDraw[i]
		if r.renderMethod == config.RENDER_FILL_TRIANGLE || r.renderMethod == config.RENDER_FILL_TRIANGLE_WIRE {
			if r.fillRasterMode == config.FILL_PINEDA {
				r.target.FillTriangle(tri)
			} else {
				r.drawFilledTriangle(tri)
			}
		}

		if r.renderMethod == config.RENDER_TEXTURED || r.renderMethod == config.RENDER_TEXTURED_WIRE {
			if r.textureRasterMode == config.TEXTURE_PINEDA {
				r.target.TextureTriangle(tri)
			} else {
				r.drawTexturedTriangle(tri)
			}
		}

		if r.renderMethod == config.RENDER_WIRE || r.renderMethod == config.RENDER_WIRE_VERTEX || r.renderMethod == config.RENDER_FILL_TRIANGLE_WIRE || r.renderMethod == config.RENDER_TEXTURED_WIRE {
			r.drawTriangle(int(tri.V[0].X), int(tri.V[0].Y),
				int(tri.V[1].X), int(tri.V[1].Y),
				int(tri.V[2].X), int(tri.V[2].Y), 0xFFFFFFFF)
		}
		if r.renderMethod == config.RENDER_WIRE_VERTEX {
			r.drawRectangle(int(tri.V[0].X)-3, int(tri.V[0].Y)-3, 6, 6, 0xFFFFFF00)
			r.drawRectangle(int(tri.V[1].X)-3, int(tri.V[1].Y)-3, 6, 6, 0xFFFFFF00)
			r.drawRectangle(int(tri.V[2].X)-3, int(tri.V[2].Y)-3, 6, 6, 0xFFFFFF00)
		}

	}
}

func (r *Renderer[T]) renderColorBuffer(screen *ebiten.Image) {
	screen.WritePixels(r.target.Bytes())
}

func (r *Renderer[T]) clearBuffers() {
	r.target.Clear(0xFF000000)
}

func (r *Renderer[T]) drawPixel(x, y int, color uint32) {
	if x >= 0 && y >= 0 && x < r.target.Width && y < r.target.Height {
		r.target.Color[y*r.target.Width+x] = color
	}
}

func (r *Renderer[T]) update() {
	// ebiten drives the frame pacing (fixed TPS), so we no longer sleep here;
	// deltaTime is measured in Update, before input is processed.
	r.trianglesToDraw = r.trianglesToDraw[:0]

	params := &r.geometry
	params.View, r.camera.Direction = pipeline.ViewMatrix(r.camera.Position, r.camera.YawAngle, r.camera.PitchAngle)
	params.Projection = r.projectionMatrix
	params.Frustum = r.frustumPlanes
	params.Light = r.light
	params.Shading = r.shadingMode
	params.Cull = r.cullMode
	params.Width = r.config.WindowWidth
	params.Height = r.config.WindowHeight

	for _, currentMesh := range r.meshes {
		if currentMesh == nil {
			panic("nil currentMesh")
		}
		for _, mutation := range currentMesh.PerFrameMutations {
			err := mutation(currentMesh)
			if err != nil {
				panic(err)
			}
		}
		r.trianglesToDraw = pipeline.AppendMesh(r.trianglesToDraw, currentMesh, params)
	}
}

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

func (r *Renderer[T]) drawTexel(x, y int, t *mesh.TextureData, a, b, c vector.Vec4[float32], AUV, BUV, CUV texture.Tex2[float32], i0, i1, i2 float32) {
	if x < 0 || x >= r.config.WindowWidth || y < 0 || y >= r.config.WindowHeight {
		return
	}
	p := vector.Vec2[float32]{X: float32(x), Y: float32(y)}
	aVec2 := a.ToVec2()
	bVec2 := b.ToVec2()
	cVec2 := c.ToVec2()

	weights := barycentricWeights(aVec2, bVec2, cVec2, p)
	alpha, beta, gamma := weights.X, weights.Y, weights.Z
	// Perform the interpolation of all the U/w and V/w values using barycentric weights and a factor of 1/w
	interpolatedU := (AUV.U/a.W)*alpha + (BUV.U/b.W)*beta + (CUV.U/c.W)*gamma
	interpolatedV := (AUV.V/a.W)*alpha + (BUV.V/b.W)*beta + (CUV.V/c.W)*gamma
	// This is 1/w because W (original Z) is not linear
	interpolatedReciprocalW := (1/a.W)*alpha + (1/b.W)*beta + (1/c.W)*gamma

	// Divide back both U and V by 1/w
	interpolatedU /= interpolatedReciprocalW
	interpolatedV /= interpolatedReciprocalW
	// UV is always between 0 and 1 so we need to scale them to texture resolution
	// The overflows here come from the fact that we truncate to (integer) pixels
	// On the border of a triangle a pixel might be outside a triangle which throws the barycentric weights (become bigger than 1)
	// Which then tries to fetch texel outside of bounds of texture array
	texX := int(math.Abs(float64(interpolatedU)*float64(t.Width))) % t.Width
	texY := int(math.Abs(float64(interpolatedV)*float64(t.Height))) % t.Height

	// Pixels closer to the viewer need to bigger than the ones further away
	interpolatedReciprocalW = 1 - interpolatedReciprocalW

	// Only draw pixels if depth value if less than the one previously stored in the Z-buffer
	if interpolatedReciprocalW < r.target.Depth[(r.config.WindowWidth*y)+x] {
		// Interpolate the per-vertex light intensity across the face (Gouraud);
		// flat shading passes equal vertex values so this is a constant.
		intensity := i0*alpha + i1*beta + i2*gamma
		r.drawPixel(x, y, light.LightApplyIntensity(t.PixelData[(texY*t.Width)+texX], intensity))
		// Update the Z-buffer with 1/w for current pixel
		r.target.Depth[(r.config.WindowWidth*y)+x] = interpolatedReciprocalW

	}
}
