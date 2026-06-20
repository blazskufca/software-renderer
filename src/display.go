package main

import (
	"encoding/binary"
	"math"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
)

type cullMode int

type renderMode int

const (
	CULL_NONE cullMode = iota
	CULL_BACKFACE
)

const (
	RENDER_WIRE renderMode = iota
	RENDER_WIRE_VERTEX
	RENDER_FILL_TRIANGLE
	RENDER_FILL_TRIANGLE_WIRE
	RENDER_TEXTURED
	RENDER_TEXTURED_WIRE
)

// initializeWindow configures the ebiten window and its render resolution.
func initializeWindow() {
	windowHeight = 720
	windowWidth = 1280
	ebiten.SetWindowTitle("3D renderer")
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetFullscreen(true)

	// Capture the cursor for continuous relative mouse motion (FPS camera).
	// Desktop can capture immediately. Browsers gate pointer lock behind a user
	// gesture, so on wasm we defer capture to the first click (see processInput).
	if runtime.GOOS != "js" {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	}
}

// Pixels are stored as four bytes in R,G,B,A order (matching ebiten's
// WritePixels). Our colors are packed 0xAABBGGRR, so a little-endian write lays
// the bytes down as R,G,B,A.
func clearColorBuffer(newColor uint32) {
	for i := 0; i < len(colorBuffer); i += 4 {
		binary.LittleEndian.PutUint32(colorBuffer[i:], newColor)
	}
}

func clearZBuffer(newZ float64) {
	for y := 0; y < windowHeight; y++ {
		for x := 0; x < windowWidth; x++ {
			zBuffer[(y*windowWidth)+x] = newZ
		}
	}
}

// renderColorBuffer hands the color buffer straight to ebiten. The buffer is
// already a flat R,G,B,A byte slice, so there is nothing to convert or copy.
func renderColorBuffer(screen *ebiten.Image) {
	screen.WritePixels(colorBuffer)
}

func drawGrid() {
	for y := 0; y < windowHeight; y++ {
		for x := 0; x < windowWidth; x++ {
			if y%10 == 0 || x%10 == 0 {
				drawPixel(x, y, 0xFF333333)
			}
		}
	}
}

func drawRectangle(x, y, w, h int, color uint32) {
	for currentY := y; currentY < (h + y); currentY++ {
		for currentX := x; currentX < (w + x); currentX++ {
			drawPixel(currentX, currentY, color)
		}
	}
}

func drawPixel(x, y int, color uint32) {
	if x >= 0 && y >= 0 && x < windowWidth && y < windowHeight {
		idx := ((y * windowWidth) + x) * 4
		binary.LittleEndian.PutUint32(colorBuffer[idx:], color)
	}
}

// drawLine implements a DDA algorithm for drawing lines
func drawLine(x0, y0, x1, y1 int, color uint32) {
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
		drawPixel(int(math.Round(xCur)), int(math.Round(yCur)), color)
		yCur += yInc
		xCur += xInc
	}
}

func drawTriangle(x0, y0, x1, y1, x2, y2 int, color uint32) {
	drawLine(x0, y0, x1, y1, color)
	drawLine(x1, y1, x2, y2, color)
	drawLine(x2, y2, x0, y0, color)
}
