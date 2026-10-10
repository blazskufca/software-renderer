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

// Target is a color and depth buffer pair. Code that writes Color or Depth
// directly, instead of through FillTriangle or TextureTriangle, must call
// MarkDirty so that Clear resets those pixels too.
type Target struct {
	Color  []uint32  // 0xAABBGGRR
	Depth  []float32 // stores 1 - 1/w, cleared to 1
	Width  int
	Height int
	bytes  []byte

	// Clear only resets dirty, the area drawn since the last Clear: the rest
	// still holds the clear color and depth 1. In the default view the car
	// covers a few percent of the screen.
	dirty      rect
	clearColor uint32
	blankColor []uint32  // one row of clearColor
	blankDepth []float32 // one row of 1
}

// rect is an inclusive pixel rectangle, empty when x0 > x1.
type rect struct{ x0, y0, x1, y1 int }

func (r *rect) add(x0, y0, x1, y1 int) {
	r.x0, r.y0 = min(r.x0, x0), min(r.y0, y0)
	r.x1, r.y1 = max(r.x1, x1), max(r.y1, y1)
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
		bytes:      unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(color))), len(color)*4),
		blankColor: make([]uint32, width),
		blankDepth: make([]float32, width),
	}
	fill(t.blankDepth, 1)
	t.MarkAllDirty() // new buffers hold zeros, not a clear color
	return t, nil
}

// Clear resets color and depth everywhere something was drawn since the last
// Clear, which leaves the whole target cleared.
func (t *Target) Clear(color uint32) {
	if color != t.clearColor {
		t.clearColor = color
		fill(t.blankColor, color)
		t.MarkAllDirty()
	}
	d := t.dirty
	t.dirty = rect{x0: t.Width, y0: t.Height, x1: -1, y1: -1}
	if d.x0 > d.x1 || d.y0 > d.y1 {
		return
	}
	if d.x0 == 0 && d.x1 == t.Width-1 {
		// whole rows: one contiguous block
		lo, hi := d.y0*t.Width, (d.y1+1)*t.Width
		fill(t.Color[lo:hi], color)
		fill(t.Depth[lo:hi], 1)
		return
	}
	for y := d.y0; y <= d.y1; y++ {
		row := y * t.Width
		copy(t.Color[row+d.x0:row+d.x1+1], t.blankColor[d.x0:d.x1+1])
		copy(t.Depth[row+d.x0:row+d.x1+1], t.blankDepth[d.x0:d.x1+1])
	}
}

// MarkDirty records that pixels in the inclusive rectangle were written
// directly, so the next Clear resets them. It clips to the target.
func (t *Target) MarkDirty(x0, y0, x1, y1 int) {
	t.dirty.add(max(x0, 0), max(y0, 0), min(x1, t.Width-1), min(y1, t.Height-1))
}

// MarkAllDirty makes the next Clear reset the whole target.
func (t *Target) MarkAllDirty() { t.MarkDirty(0, 0, t.Width-1, t.Height-1) }

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
