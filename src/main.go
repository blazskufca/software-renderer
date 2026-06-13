package main

import (
	"cmp"
	"fmt"
	"log"
	"slices"

	"github.com/veandco/go-sdl2/sdl"
)

// TODO: Move global state into a central structure

const (
	fovFactor                = 640
	FPS                      = 60
	frameTargetTime   uint64 = 1000 / FPS
	cubeNMeshVertices        = 8
)

var (
	windowWidth        = 800
	windowHeight       = 600
	colorBuffer        []uint32
	colorBufferTexture *sdl.Texture
	cameraPosition     vec3   = vec3{0, 0, 0}
	previousFrameTime  uint64 = 0
	trianglesToRender  []triangle
	renderMethod       = RENDER_WIRE
	cullMethod         = CULL_BACKFACE
)

func setup(r *sdl.Renderer) error {
	var err error
	colorBuffer = make([]uint32, windowWidth*windowHeight)
	colorBufferTexture, err = r.CreateTexture(sdl.PIXELFORMAT_ARGB8888, sdl.TEXTUREACCESS_STREAMING, int32(windowWidth), int32(windowHeight))
	if err != nil {
		return fmt.Errorf("failed to initialize SDL2 texture with error: %w", err)
	}
	//currentMesh, err = newMeshFromFromFile("./assets/cube.obj")
	//if err != nil {
	//	return fmt.Errorf("failed to load obj: %w", err)
	//}
	currentMesh = newCubeMesh()
	return nil
}

func render(r *sdl.Renderer) error {
	for i := 0; i < len(trianglesToRender); i++ {
		triangle := trianglesToRender[i]
		if renderMethod == RENDER_FILL_TRIANGLE || renderMethod == RENDER_FILL_TRIANGLE_WIRE {
			drawFilledTriangle(triangle.points[0].X, triangle.points[0].Y,
				triangle.points[1].X, triangle.points[1].Y,
				triangle.points[2].X, triangle.points[2].Y, triangle.color)
		}

		if renderMethod == RENDER_WIRE || renderMethod == RENDER_WIRE_VERTEX || renderMethod == RENDER_FILL_TRIANGLE_WIRE {
			drawTriangle(triangle.points[0].X, triangle.points[0].Y,
				triangle.points[1].X, triangle.points[1].Y,
				triangle.points[2].X, triangle.points[2].Y, 0xFFFFFFFF)
		}

		if renderMethod == RENDER_WIRE_VERTEX {
			drawRectangle(triangle.points[0].X-3, triangle.points[0].Y-3, 6, 6, 0xFFFFFF00)
			drawRectangle(triangle.points[1].X-3, triangle.points[1].Y-3, 6, 6, 0xFFFFFF00)
			drawRectangle(triangle.points[2].X-3, triangle.points[2].Y-3, 6, 6, 0xFFFFFF00)
		}

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

	now := sdl.GetTicks64()
	log.Printf("Last frame took %d ms", now-previousFrameTime)
	previousFrameTime = now
	currentMesh.rotation.Y += 0.01
	currentMesh.rotation.X += 0.01
	currentMesh.rotation.Z += 0.01

	for i := 0; i < len(currentMesh.faces); i++ {
		// Face index into the vertices array
		meshFace := currentMesh.faces[i]
		facesVertices := make([]vec3, 3)
		facesVertices[0] = currentMesh.vertices[meshFace.a-1]
		facesVertices[1] = currentMesh.vertices[meshFace.b-1]
		facesVertices[2] = currentMesh.vertices[meshFace.c-1]
		var transformedVertexArr [3]vec3
		for j := 0; j < 3; j++ {
			transformedVertex := facesVertices[j]
			transformedVertex.rotateY(currentMesh.rotation.Y)
			transformedVertex.rotateX(currentMesh.rotation.X)
			transformedVertex.rotateZ(currentMesh.rotation.Z)
			transformedVertex.Z += 5
			transformedVertexArr[j] = transformedVertex // Save current vertex in arr of transformedVertex
		}

		if cullMethod == CULL_BACKFACE {
			// Backface culling
			vectorA, vectorB, vectorC := transformedVertexArr[0], transformedVertexArr[1], transformedVertexArr[2]
			BAVector, CAVector := vectorB.sub(vectorA), vectorC.sub(vectorA)
			// For vectors BA and CA we also only care about the direction and not the magnitude
			BAVector.normalize()
			CAVector.normalize()

			// We are using a left-handed coordinate system (i.e Z grows inside the monitor)
			// Face normal vector can be normalized
			faceNormal := BAVector.cross(CAVector)
			faceNormal.normalize()

			cameraRayVec := cameraPosition.sub(vectorA)

			cameraRayFaceNormalDot := faceNormal.dot(cameraRayVec)

			// If dot product is less than 0 we should hide the face
			if cameraRayFaceNormalDot < 0 {
				continue
			}
		}

		var projectedPoints [3]vec2
		for j := 0; j < len(transformedVertexArr); j++ {
			projectedPoints[j] = project(transformedVertexArr[j])

			// Transpose to center of viewport on both axis
			projectedPoints[j].X += windowWidth / 2
			projectedPoints[j].Y += windowHeight / 2
		}
		var projectedTriangle triangle
		projectedTriangle.points[0] = projectedPoints[0]
		projectedTriangle.points[1] = projectedPoints[1]
		projectedTriangle.points[2] = projectedPoints[2]
		projectedTriangle.color = meshFace.color
		projectedTriangle.avgDepth = (transformedVertexArr[0].Z + transformedVertexArr[1].Z + transformedVertexArr[2].Z) / 3
		trianglesToRender = append(trianglesToRender, projectedTriangle)
	}
	// Painters algorithm
	slices.SortFunc(trianglesToRender, func(a, b triangle) int {
		return cmp.Compare(b.avgDepth, a.avgDepth)
	})

}

func processInput(running *bool) {
	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		switch t := event.(type) {
		case *sdl.QuitEvent:
			*running = false
			break
		case *sdl.KeyboardEvent:
			// Check if it's a keydown event and specifically the ESCAPE key
			if t.Type == sdl.KEYDOWN {
				switch t.Keysym.Sym {
				case sdl.K_ESCAPE:
					*running = false
				case sdl.K_1:
					renderMethod = RENDER_WIRE_VERTEX
				case sdl.K_2:
					renderMethod = RENDER_WIRE
				case sdl.K_3:
					renderMethod = RENDER_FILL_TRIANGLE
				case sdl.K_4:
					renderMethod = RENDER_FILL_TRIANGLE_WIRE
				case sdl.K_c:
					cullMethod = CULL_BACKFACE
				case sdl.K_d:
					cullMethod = CULL_NONE
				}
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
