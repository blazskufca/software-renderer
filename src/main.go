package main

import (
	"fmt"
	"log"

	"github.com/veandco/go-sdl2/sdl"
)

// TODO: Move global state into a central structure

const (
	numberOfCubePoints        = 9 * 9 * 9
	fovFactor                 = 640
	FPS                       = 30
	frameTargetTime    uint64 = 1000 / FPS
)

var (
	windowWidth        = 800
	windowHeight       = 600
	colorBuffer        []uint32
	colorBufferTexture *sdl.Texture
	cubePoints         [numberOfCubePoints]vec3
	projectedPoints    [numberOfCubePoints]vec2
	cameraPosition     vec3   = vec3{0, 0, -5}
	cubeRotation       vec3   = vec3{0, 0, 0}
	previousFrameTime  uint64 = 0
)

func setup(r *sdl.Renderer) error {
	var err error
	colorBuffer = make([]uint32, windowWidth*windowHeight)
	colorBufferTexture, err = r.CreateTexture(sdl.PIXELFORMAT_ARGB8888, sdl.TEXTUREACCESS_STREAMING, int32(windowWidth), int32(windowHeight))
	if err != nil {
		return fmt.Errorf("failed to initialize SDL2 texture with error: %w", err)
	}
	pointCount := 0
	cubePoints = [numberOfCubePoints]vec3{}
	for x := -1.0; x <= 1; x += 0.25 {
		for y := -1.0; y <= 1; y += 0.25 {
			for z := -1.0; z <= 1.0; z += 0.25 {
				newPoint := vec3{x, y, z}
				cubePoints[pointCount] = newPoint
				pointCount++
			}
		}
	}
	return nil
}

func render(r *sdl.Renderer) error {
	for i := 0; i < numberOfCubePoints; i++ {
		projectedPoint := projectedPoints[i]
		drawRectangle(int(projectedPoint.X)+int(windowWidth/2), int(projectedPoint.Y)+int(windowHeight/2), 4, 4, 0xFFFFFF00)
	}
	err := renderColorBuffer(r)
	if err != nil {
		return err
	}
	clearColorBuffer(0xFF000000)
	r.Present()
	return nil
}

func project(point vec3) (projectedPoint vec2) {
	// To get perspective projection you follow the formula
	// P'y = Py/Pz and P'x = Px/Pz, where P'y and P'x are the projected points
	projectedPoint = vec2{X: (point.X * fovFactor) / point.Z, Y: point.Y * fovFactor / point.Z}
	return projectedPoint
}

func update() {
	timeToWait := frameTargetTime - (sdl.GetTicks64() - previousFrameTime)

	if timeToWait > 0 && timeToWait <= frameTargetTime {
		sdl.Delay(uint32(timeToWait))
	}
	previousFrameTime = sdl.GetTicks64()
	cubeRotation.Y += 0.01
	cubeRotation.X += 0.01
	cubeRotation.Z += 0.01

	for i := 0; i < numberOfCubePoints; i++ {
		point := cubePoints[i]
		// Simulate moving away from the camera
		point.rotateY(cubeRotation.Y)
		point.rotateX(cubeRotation.X)
		point.rotateZ(cubeRotation.Z)
		point.Z -= cameraPosition.Z
		projectedPoint := project(point)
		projectedPoints[i] = projectedPoint

	}
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
		update()
		err := render(r)
		if err != nil {
			log.Fatal(err)
		}

	}
}
