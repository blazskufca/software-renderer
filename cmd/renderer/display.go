package main

import (
	"encoding/binary"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// initializeWindow configures the ebiten window and its render resolution.
func (r *Renderer[T]) initializeWindow() {
	ebiten.SetWindowTitle("3D renderer")
	ebiten.SetWindowSize(r.config.WindowWidth, r.config.WindowHeight)
}

// Pixels are stored as four bytes in R,G,B,A order (matching ebiten's
// WritePixels). Our colors are packed 0xAABBGGRR, so a little-endian write lays
// the bytes down as R,G,B,A.
//
// We seed the first pixel and then replicate it across the buffer by repeatedly
// copying the already-filled prefix onto the remainder. Each copy doubles the
// filled region, so the runtime turns the clear into a handful of bulk memmoves
// instead of one PutUint32 per pixel.
func (r *Renderer[T]) clearColorBuffer(newColor uint32) {
	if len(r.colorBuffer) < 4 {
		return
	}
	binary.LittleEndian.PutUint32(r.colorBuffer[:4], newColor)
	for filled := 4; filled < len(r.colorBuffer); filled *= 2 {
		copy(r.colorBuffer[filled:], r.colorBuffer[:filled])
	}
}

// clearZBuffer resets every depth sample to newZ using the same copy-doubling
// trick as clearColorBuffer: seed element 0, then replicate the filled prefix.
func (r *Renderer[T]) clearZBuffer(newZ T) {
	if len(r.zBuffer) == 0 {
		return
	}
	r.zBuffer[0] = newZ
	for filled := 1; filled < len(r.zBuffer); filled *= 2 {
		copy(r.zBuffer[filled:], r.zBuffer[:filled])
	}
}

// renderColorBuffer hands the color buffer straight to ebiten. The buffer is
// already a flat R,G,B,A byte slice, so there is nothing to convert or copy.
func (r *Renderer[T]) renderColorBuffer(screen *ebiten.Image) {
	screen.WritePixels(r.colorBuffer)
}

func (r *Renderer[T]) drawGrid() {
	for y := 0; y < r.config.WindowHeight; y++ {
		for x := 0; x < r.config.WindowWidth; x++ {
			if y%10 == 0 || x%10 == 0 {
				r.drawPixel(x, y, 0xFF333333)
			}
		}
	}
}

func (r *Renderer[T]) drawRectangle(x, y, w, h int, color uint32) {
	for currentY := y; currentY < (h + y); currentY++ {
		for currentX := x; currentX < (w + x); currentX++ {
			r.drawPixel(currentX, currentY, color)
		}
	}
}

func (r *Renderer[T]) drawPixel(x, y int, color uint32) {
	if x >= 0 && y >= 0 && x < r.config.WindowWidth && y < r.config.WindowHeight {
		idx := ((y * r.config.WindowWidth) + x) * 4
		binary.LittleEndian.PutUint32(r.colorBuffer[idx:], color)
	}
}

// drawLine implements a DDA algorithm for drawing lines
func (r *Renderer[T]) drawLine(x0, y0, x1, y1 int, color uint32) {
	deltaX := x1 - x0
	deltaY := y1 - y0

	var sideLength float64

	if math.Abs(float64(deltaX)) > math.Abs(float64(deltaY)) {
		sideLength = math.Abs(float64(deltaX))
	} else {
		sideLength = math.Abs(float64(deltaY))
	}
	yInc := float64(deltaY) / sideLength
	xInc := float64(deltaX) / sideLength
	yCur := float64(y0)
	xCur := float64(x0)
	for i := 0.0; i <= sideLength; i++ {
		r.drawPixel(int(math.Round(xCur)), int(math.Round(yCur)), color)
		yCur += yInc
		xCur += xInc
	}
}

func (r *Renderer[T]) drawTriangle(x0, y0, x1, y1, x2, y2 int, color uint32) {
	r.drawLine(x0, y0, x1, y1, color)
	r.drawLine(x1, y1, x2, y2, color)
	r.drawLine(x2, y2, x0, y0, color)
}
