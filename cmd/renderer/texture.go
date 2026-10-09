package main

import (
	_ "image/jpeg"
	_ "image/png"
	"math"

	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/light"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/vector"
)

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
