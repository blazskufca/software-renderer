//go:build !goexperiment.simd

// Fallback for builds without GOEXPERIMENT=simd (the simd package does not exist
// there, so raster_simd.go cannot compile). The SIMD rasterizer modes are hidden
// from the UI and the key bindings in this configuration — see
// Renderer.simdAvailable — but the entry points still exist and forward to the
// scalar Pineda rasterizers so selecting a mode programmatically stays correct.
package main

import (
	mesh "github.com/blazskufca/software-renderer/internal/face"
)

const (
	simdCompiledIn = false

	// No vector stores happen in this build, so the framebuffers need no
	// overrun padding.
	simdPadPixels = 0
)

func (r *Renderer[T]) drawFilledTrianglePinedaSIMD(x0f, y0f, z0, w0f, x1f, y1f, z1, w1f, x2f, y2f, z2, w2f T, color uint32, i0, i1, i2 T) {
	r.drawFilledTrianglePineda(x0f, y0f, z0, w0f, x1f, y1f, z1, w1f, x2f, y2f, z2, w2f, color, i0, i1, i2)
}

func (r *Renderer[T]) drawTexturedTrianglePinedaSIMD(x0, y0, z0, w0f, u0, v0, x1, y1, z1, w1f, u1, v1, x2, y2, z2, w2f, u2, v2, i0, i1, i2 T, t *mesh.TextureData) {
	r.drawTexturedTrianglePineda(x0, y0, z0, w0f, u0, v0, x1, y1, z1, w1f, u1, v1, x2, y2, z2, w2f, u2, v2, i0, i1, i2, t)
}
