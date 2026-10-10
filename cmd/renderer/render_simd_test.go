//go:build simd

package main

import "github.com/blazskufca/software-renderer/internal/raster"

func depthAt(r *Renderer[float32], i int) float32 { return r.target.Depth[i] }

func pixelAt(r *Renderer[float32], i int) uint32 { return r.target.Color[i] }

var fillRasterizers = []rasterizer{
	{name: "scanline", draw: func(r *Renderer[float32], tri *raster.Triangle) { r.drawFilledTriangle(tri) }},
	{name: "pineda", draw: func(r *Renderer[float32], tri *raster.Triangle) { r.target.FillTriangle(tri) }},
}

var textureRasterizers = []rasterizer{
	{name: "scanline", draw: func(r *Renderer[float32], tri *raster.Triangle) { r.drawTexturedTriangle(tri) }},
	{name: "pineda", draw: func(r *Renderer[float32], tri *raster.Triangle) { r.target.TextureTriangle(tri) }},
}
