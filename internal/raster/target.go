package raster

import (
	"fmt"
	"unsafe"

	mesh "github.com/blazskufca/software-renderer/internal/face"
)

// widest vector (AVX-512) in float32 lanes; Width must be a multiple
const maxLanes = 16

type Vertex struct{ X, Y, W, U, V, Light float32 }

type Triangle struct {
	V       [3]Vertex
	Color   uint32
	Texture *mesh.TextureData
}

type Target struct {
	Color  []uint32  // 0xAABBGGRR
	Depth  []float32 // stores 1 - 1/w, cleared to 1
	Width  int
	Height int
	bytes  []byte
}

func NewTarget(width, height int) (*Target, error) {
	if width <= 0 || height <= 0 || width%maxLanes != 0 {
		return nil, fmt.Errorf("raster: invalid size %dx%d (width must be a positive multiple of %d)", width, height, maxLanes)
	}
	color := make([]uint32, width*height)
	t := &Target{
		Color:  color,
		Depth:  make([]float32, width*height),
		Width:  width,
		Height: height,
		// little-endian 0xAABBGGRR is R,G,B,A in memory
		bytes: unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(color))), len(color)*4),
	}
	return t, nil
}

func (t *Target) Clear(color uint32) {
	fill(t.Color, color)
	fill(t.Depth, 1)
}

func (t *Target) Bytes() []byte { return t.bytes }

func fill[E uint32 | float32](s []E, v E) {
	if len(s) == 0 {
		return
	}
	s[0] = v
	for n := 1; n < len(s); n *= 2 {
		copy(s[n:], s[:n])
	}
}
