package main

import (
	"fmt"
	"log"

	"github.com/veandco/go-sdl2/sdl"
)

// TODO: Move global state into a central structure

const (
	fovFactor                = 640
	FPS                      = 30
	frameTargetTime   uint64 = 1000 / FPS
	cubeNMeshVertices        = 8
)

var (
	windowWidth        = 800
	windowHeight       = 600
	colorBuffer        []uint32
	colorBufferTexture *sdl.Texture
	cameraPosition     vec3   = vec3{0, 0, -5}
	previousFrameTime  uint64 = 0
	trianglesToRender  []triangle
)

func setup(r *sdl.Renderer) error {
	var err error
	colorBuffer = make([]uint32, windowWidth*windowHeight)
	colorBufferTexture, err = r.CreateTexture(sdl.PIXELFORMAT_ARGB8888, sdl.TEXTUREACCESS_STREAMING, int32(windowWidth), int32(windowHeight))
	if err != nil {
		return fmt.Errorf("failed to initialize SDL2 texture with error: %w", err)
	}
	currentMesh, err = newMeshFromFromFile("./assets/f22.obj")
	if err != nil {
		return fmt.Errorf("failed to load obj: %w", err)
	}
	return nil
}

func render(r *sdl.Renderer) error {
	for i := 0; i < len(trianglesToRender); i++ {
		triangle := trianglesToRender[i]
		drawRectangle(triangle.points[0].X, triangle.points[0].Y, 3, 3, 0xFFFFFF00)
		drawRectangle(triangle.points[1].X, triangle.points[1].Y, 3, 3, 0xFFFFFF00)
		drawRectangle(triangle.points[2].X, triangle.points[2].Y, 3, 3, 0xFFFFFF00)

		drawTriangle(triangle.points[0].X, triangle.points[0].Y,
			triangle.points[1].X, triangle.points[1].Y,
			triangle.points[2].X, triangle.points[2].Y, 0xFF00FF00)
	}

	err := renderColorBuffer(r)
	if err != nil {
		return err
	}
	clearColorBuffer(0xFF000000)
	r.Present()
	trianglesToRender = nil
	return nil
}

// project applies perspective projection to vec3 point
func project(point vec3) (projectedPoint vec2) {
	// To get perspective projection you follow the formula
	// P'y = Py/Pz and P'x = Px/Pz, where P'y and P'x are the projected points
	projectedPoint = vec2{X: int((point.X * fovFactor) / point.Z), Y: int(point.Y * fovFactor / point.Z)}
	return projectedPoint
}

func update() {
	timeToWait := frameTargetTime - (sdl.GetTicks64() - previousFrameTime)

	if timeToWait > 0 && timeToWait <= frameTargetTime {
		sdl.Delay(uint32(timeToWait))
	}
	trianglesToRender = make([]triangle, 0, len(currentMesh.faces))
	previousFrameTime = sdl.GetTicks64()
	currentMesh.rotation.Y += 0.01
	currentMesh.rotation.X += 0.01
	currentMesh.rotation.Z += 0.01

	for i := 0; i < len(currentMesh.faces); i++ {
		// Face index into the vertices array
		face := currentMesh.faces[i]
		facesVertices := make([]vec3, 3)
		facesVertices[0] = currentMesh.vertices[face.a-1]
		facesVertices[1] = currentMesh.vertices[face.b-1]
		facesVertices[2] = currentMesh.vertices[face.c-1]
		var projectedTriangle triangle
		for j := 0; j < 3; j++ {
			transformedVertex := facesVertices[j]
			transformedVertex.rotateY(currentMesh.rotation.Y)
			transformedVertex.rotateX(currentMesh.rotation.X)
			transformedVertex.rotateZ(currentMesh.rotation.Z)

			transformedVertex.Z -= cameraPosition.Z

			projectedPoint := project(transformedVertex)

			// Transpose to center of viewport on both axis
			projectedPoint.X += windowWidth / 2
			projectedPoint.Y += windowHeight / 2

			projectedTriangle.points[j] = projectedPoint
		}
		trianglesToRender = append(trianglesToRender, projectedTriangle)

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
