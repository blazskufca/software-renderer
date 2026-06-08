package main

import (
	"fmt"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
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
