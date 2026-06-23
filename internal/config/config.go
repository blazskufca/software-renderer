package config

import (
	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/utils"
)

type CullMode int

type RenderMode int

type TextureRasterizer int

type FillRasterMode int

type ShadingMode int

const (
	CULL_NONE CullMode = iota
	CULL_BACKFACE
)

const (
	SHADING_FLAT ShadingMode = iota
	SHADING_GOURAUD
)

const (
	RENDER_WIRE RenderMode = iota
	RENDER_WIRE_VERTEX
	RENDER_FILL_TRIANGLE
	RENDER_FILL_TRIANGLE_WIRE
	RENDER_TEXTURED
	RENDER_TEXTURED_WIRE
)

const (
	TEXTURE_SCANLINE TextureRasterizer = iota
	TEXTURE_PINEDA
)

const (
	FILL_SCANLINE FillRasterMode = iota
	FILL_PINEDA
)

type TexturizedRasterizer[T utils.Numeric] func(x0, y0, z0, w0, u0, v0, x1, y1, z1, w1, u1, v1, x2, y2, z2, w2, u2, v2, i0, i1, i2 T, t *mesh.TextureData)

type FillRasterizer[T utils.Numeric] func(x0f, y0f, z0, w0, x1f, y1f, z1, w1, x2f, y2f, z2, w2 T, color uint32, i0, i1, i2 T)
