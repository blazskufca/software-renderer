package config

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
