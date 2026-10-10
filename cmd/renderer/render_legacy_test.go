//go:build !simd

package main

import (
	"encoding/binary"
	"math"
	"math/rand/v2"
	"testing"

	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/raster"
)

func depthAt(r *Renderer[float32], i int) float32 { return r.zBuffer[i] }

func pixelAt(r *Renderer[float32], i int) uint32 {
	return binary.LittleEndian.Uint32(r.colorBuffer[i*4:])
}

func legacyFill(draw func(r *Renderer[float32], x0, y0, z0, w0, x1, y1, z1, w1, x2, y2, z2, w2 float32, color uint32, i0, i1, i2 float32)) func(r *Renderer[float32], tri *raster.Triangle) {
	return func(r *Renderer[float32], tri *raster.Triangle) {
		v := &tri.V
		draw(r, v[0].X, v[0].Y, 0, v[0].W, v[1].X, v[1].Y, 0, v[1].W, v[2].X, v[2].Y, 0, v[2].W,
			tri.Color, v[0].Light, v[1].Light, v[2].Light)
	}
}

func legacyTexture(draw func(r *Renderer[float32], x0, y0, z0, w0, u0, v0, x1, y1, z1, w1, u1, v1, x2, y2, z2, w2, u2, v2, i0, i1, i2 float32, t *mesh.TextureData)) func(r *Renderer[float32], tri *raster.Triangle) {
	return func(r *Renderer[float32], tri *raster.Triangle) {
		v := &tri.V
		draw(r, v[0].X, v[0].Y, 0, v[0].W, v[0].U, v[0].V,
			v[1].X, v[1].Y, 0, v[1].W, v[1].U, v[1].V,
			v[2].X, v[2].Y, 0, v[2].W, v[2].U, v[2].V,
			v[0].Light, v[1].Light, v[2].Light, tri.Texture)
	}
}

var fillRasterizers = []rasterizer{
	{name: "scanline", draw: legacyFill((*Renderer[float32]).drawFilledTriangle)},
	{name: "pineda", draw: legacyFill((*Renderer[float32]).drawFilledTrianglePineda)},
}

var textureRasterizers = []rasterizer{
	{name: "scanline", draw: legacyTexture((*Renderer[float32]).drawTexturedTriangle)},
	{name: "pineda", draw: legacyTexture((*Renderer[float32]).drawTexturedTrianglePineda)},
}

// TestLegacyPinedaMatchesRaster checks that the legacy Pineda rasterizers cover
// exactly the pixels raster.Target covers, at the same depth. Colors are not
// compared: LightApplyIntensity and raster's shade round differently.
func TestLegacyPinedaMatchesRaster(t *testing.T) {
	rng := rand.New(rand.NewPCG(21, 22))
	tex := &mesh.TextureData{Width: 48, Height: 32, PixelData: make([]uint32, 48*32)}
	for i := range tex.PixelData {
		tex.PixelData[i] = 0xFF000000 | rng.Uint32()&0xFFFFFF
	}
	in := func(lo, hi float32) float32 { return lo + rng.Float32()*(hi-lo) }
	tris := make([]raster.Triangle, 300)
	for i := range tris {
		for k := range 3 {
			tris[i].V[k] = raster.Vertex{
				X: in(-100, 1060), Y: in(-100, 580), W: in(0.5, 5.5),
				U: rng.Float32(), V: rng.Float32(), Light: rng.Float32(),
			}
		}
		if rng.IntN(2) == 0 {
			tris[i].V[1], tris[i].V[2] = tris[i].V[2], tris[i].V[1]
		}
		tris[i].Color = 0xFF000000 | rng.Uint32()&0xFFFFFF
		tris[i].Texture = tex
	}

	modes := []struct {
		name   string
		legacy func(r *Renderer[float32], tri *raster.Triangle)
		ref    func(tg *raster.Target, tri *raster.Triangle)
	}{
		{name: "fill", legacy: fillRasterizers[1].draw, ref: (*raster.Target).FillTriangle},
		{name: "texture", legacy: textureRasterizers[1].draw, ref: (*raster.Target).TextureTriangle},
	}
	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			r := newTestRenderer(t)
			tg, err := raster.NewTarget(r.config.WindowWidth, r.config.WindowHeight)
			if err != nil {
				t.Fatal(err)
			}
			tg.Clear(0xFF000000)
			for i := range tris {
				mode.legacy(r, &tris[i])
				mode.ref(tg, &tris[i])
			}
			covered := 0
			for i := range tg.Depth {
				got, want := depthAt(r, i), tg.Depth[i]
				if (got != 1) != (want != 1) {
					t.Fatalf("pixel (%d,%d): legacy depth %v, raster depth %v (coverage differs)",
						i%tg.Width, i/tg.Width, got, want)
				}
				if want != 1 {
					covered++
				}
				if d := math.Abs(float64(got - want)); d > 1e-5 {
					t.Fatalf("pixel (%d,%d): legacy depth %v, raster depth %v, diff %g",
						i%tg.Width, i/tg.Width, got, want, d)
				}
			}
			if covered == 0 {
				t.Fatal("no pixels covered")
			}
		})
	}
}
