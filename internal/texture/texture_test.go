//go:build !js

package texture_test

import (
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/blazskufca/software-renderer/internal/texture"
)

func writeImage(t *testing.T, name string, encode func(f *os.File) error) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := encode(f); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadTextureDataPNG(t *testing.T) {
	const w, h = 3, 2
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(10 + x), G: uint8(20 + y), B: uint8(30 + x + y), A: 255})
		}
	}
	path := writeImage(t, "tex.png", func(f *os.File) error { return png.Encode(f, img) })

	gw, gh, pixels, err := texture.LoadTextureData(path)
	if err != nil {
		t.Fatalf("LoadTextureData: %v", err)
	}
	if gw != w || gh != h {
		t.Fatalf("size = %dx%d, want %dx%d", gw, gh, w, h)
	}
	if len(pixels) != w*h {
		t.Fatalf("len(pixels) = %d, want %d", len(pixels), w*h)
	}
	for y := range h {
		for x := range w {
			r, g, b := uint32(10+x), uint32(20+y), uint32(30+x+y)
			want := 0xFF000000 | b<<16 | g<<8 | r
			if got := pixels[y*w+x]; got != want {
				t.Fatalf("pixel (%d,%d) = %#08x, want %#08x", x, y, got, want)
			}
		}
	}
}

func TestLoadTextureDataJPEG(t *testing.T) {
	const w, h = 4, 2
	src := color.RGBA{R: 200, G: 100, B: 50, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, src)
		}
	}
	path := writeImage(t, "tex.jpg", func(f *os.File) error {
		return jpeg.Encode(f, img, &jpeg.Options{Quality: 100})
	})

	gw, gh, pixels, err := texture.LoadTextureData(path)
	if err != nil {
		t.Fatalf("LoadTextureData: %v", err)
	}
	if gw != w || gh != h {
		t.Fatalf("size = %dx%d, want %dx%d", gw, gh, w, h)
	}
	near := func(got uint32, want uint8) bool {
		d := int(got) - int(want)
		return d >= -4 && d <= 4
	}
	for i, p := range pixels {
		if a := p >> 24; a != 0xFF {
			t.Fatalf("pixel %d alpha = %#02x, want 0xff", i, a)
		}
		r, g, b := p&0xFF, p>>8&0xFF, p>>16&0xFF
		if !near(r, src.R) || !near(g, src.G) || !near(b, src.B) {
			t.Fatalf("pixel %d = (%d,%d,%d), want (%d,%d,%d) ±4", i, r, g, b, src.R, src.G, src.B)
		}
	}
}

func TestLoadTextureDataErrors(t *testing.T) {
	dir := t.TempDir()
	notImage := filepath.Join(dir, "bad.png")
	if err := os.WriteFile(notImage, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	gifPath := writeImage(t, "tex.gif", func(f *os.File) error {
		return gif.Encode(f, image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White}), nil)
	})

	tests := []struct {
		name string
		path string
	}{
		{name: "missing file", path: filepath.Join(dir, "does-not-exist.png")},
		{name: "not an image", path: notImage},
		{name: "unsupported gif", path: gifPath},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := texture.LoadTextureData(tc.path)
			if err == nil {
				t.Fatalf("LoadTextureData(%q) succeeded, want error", tc.path)
			}
		})
	}
}
