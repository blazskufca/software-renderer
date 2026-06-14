package main

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
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

func initializeWindow() (*sdl.Window, *sdl.Renderer, error) {
	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		return nil, nil, fmt.Errorf("failed to initialize SDL2 with error: %w", err)
	}
	dm, err := sdl.GetCurrentDisplayMode(0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get the current display mode: %w", err)
	}
	windowHeight = int(dm.H)
	windowWidth = int(dm.W)
	window, err := sdl.CreateWindow("3D renderer", sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED, int32(windowWidth), int32(windowHeight), sdl.WINDOW_BORDERLESS)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize SDL2 window with error: %w", err)
	}
	if window == nil {
		return nil, nil, fmt.Errorf("failed to initialize SDL2 window")
	}

	// Index -1 tells SDL2 to get a default screen device
	// Flags 0 means no flags
	r, err := sdl.CreateRenderer(window, -1, 0)
	if err != nil {
		return window, nil, fmt.Errorf("failed to initialize SDL2 render with error: %w", err)
	}
	if r == nil {
		return window, nil, fmt.Errorf("failed to initialize SDL2 render")
	}
	err = window.SetFullscreen(sdl.WINDOW_FULLSCREEN)
	if err != nil {
		return window, nil, fmt.Errorf("failed to initialize SDL2 fullscreen with error: %w", err)
	}
	return window, r, nil
}

func clearColorBuffer(newColor uint32) {
	for y := 0; y < windowHeight; y++ {
		for x := 0; x < windowWidth; x++ {
			colorBuffer[(y*windowWidth)+x] = newColor
		}
	}
}

func renderColorBuffer(r *sdl.Renderer) error {
	err := colorBufferTexture.Update(
		nil,
		unsafe.Pointer(&colorBuffer[0]),
		windowWidth*int(unsafe.Sizeof(uint32(0))),
	)
	if err != nil {
		return err
	}

	err = r.Copy(colorBufferTexture, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func drawGrid() {
	for y := 0; y < windowHeight; y++ {
		for x := 0; x < windowWidth; x++ {
			if y%10 == 0 || x%10 == 0 {
				colorBuffer[(y*windowWidth)+x] = 0xFF333333
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
		colorBuffer[(y*windowWidth)+x] = color
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
