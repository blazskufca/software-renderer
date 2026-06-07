package main

import (
	"fmt"
	"log"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

// TODO: Move global state into a central structure

var (
	windowWidth        = 800
	windowHeight       = 600
	colorBuffer        []uint32
	colorBufferTexture *sdl.Texture
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

func setup(r *sdl.Renderer) error {
	var err error
	colorBuffer = make([]uint32, windowWidth*windowHeight)
	colorBufferTexture, err = r.CreateTexture(sdl.PIXELFORMAT_ARGB8888, sdl.TEXTUREACCESS_STREAMING, int32(windowWidth), int32(windowHeight))
	if err != nil {
		return fmt.Errorf("failed to initialize SDL2 texture with error: %w", err)
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
			colorBuffer[currentY*windowWidth+currentX] = color
		}
	}
}

func update() {}

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

func render(r *sdl.Renderer) error {
	err := r.SetDrawColor(255, 0, 0, 0)
	if err != nil {
		return err
	}
	err = r.Clear()
	if err != nil {
		return err
	}
	drawRectangle(0, 50, 20000, 50, 0xFFFF0000)
	err = renderColorBuffer(r)
	if err != nil {

	}
	clearColorBuffer(0xFF000000)
	r.Present()
	return nil
}

func processInput(running *bool) {
	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		switch t := event.(type) {
		case *sdl.QuitEvent:
			*running = false
			break
		case *sdl.KeyboardEvent:
			// Check if it's a keydown event and specifically the ESCAPE key
			if t.Type == sdl.KEYDOWN && t.Keysym.Sym == sdl.K_ESCAPE {
				*running = false
			}
		}
	}
}

func main() {
	defer sdl.Quit()
	window, r, err := initializeWindow()
	if err != nil {
		log.Fatal(err)
	}
	defer window.Destroy()
	defer r.Destroy()
	err = setup(r)
	if err != nil {
		log.Fatal(err)
	}
	running := true
	for running {
		processInput(&running)
		err := render(r)
		if err != nil {
			log.Fatal(err)
		}
	}
}
