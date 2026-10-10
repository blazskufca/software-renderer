package main

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// initializeWindow configures the ebiten window and its render resolution.
func (r *Renderer[T]) initializeWindow() {
	ebiten.SetWindowTitle("3D renderer")
	ebiten.SetWindowSize(r.config.WindowWidth, r.config.WindowHeight)
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
