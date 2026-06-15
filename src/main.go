package main

import (
	"cmp"
	"fmt"
	"log"
	"math"
	"slices"

	"github.com/veandco/go-sdl2/sdl"
)

// TODO: Move global state into a central structure

const (
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
	renderMethod             = RENDER_WIRE
	cullMethod               = CULL_BACKFACE
	projectionMatrix   mat4  = mat4{}
	globalLight        light = light{direction: vec3{0, 0, 1}}
)

func setup(r *sdl.Renderer) error {
	var err error
	colorBuffer = make([]uint32, windowWidth*windowHeight)
	colorBufferTexture, err = r.CreateTexture(uint32(sdl.PIXELFORMAT_RGBA32), sdl.TEXTUREACCESS_STREAMING, int32(windowWidth), int32(windowHeight))
	if err != nil {
		return fmt.Errorf("failed to initialize SDL2 texture with error: %w", err)
	}

	FOV := math.Pi / 3 // FOV needs to be in radians -> math.pi/3 = 60deg
	aspectRatio := float64(windowHeight) / float64(windowWidth)
	// We've randomly picked zNear and zFar as these valeues
	zNear, zFar := 0.1, 100.0
	//meshTexture = make([]uint32, len(REDBRICK_TEXTURE)/4)
	//for i := 0; i < len(REDBRICK_TEXTURE); i += 4 {
	//	// Extract the individual bytes and cast them to uint32 so we can shift them
	//	b := uint32(REDBRICK_TEXTURE[i])
	//	g := uint32(REDBRICK_TEXTURE[i+1])
	//	r := uint32(REDBRICK_TEXTURE[i+2])
	//	a := uint32(REDBRICK_TEXTURE[i+3])
	//	pixelColor := (a << 24) | (r << 16) | (g << 8) | b
	//	meshTexture[i/4] = pixelColor
	//}
	err = loadPngTextureData("./assets/cube.png")
	if err != nil {
		return err
	}
	// Initialize the perspective projection matrix
	projectionMatrix = mat4MakePerspective(FOV, aspectRatio, zNear, zFar)
	//currentMesh, err = newMeshFromFromFile("./assets/f22.obj")
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
			drawFilledTriangle(int(triangle.points[0].X), int(triangle.points[0].Y),
				int(triangle.points[1].X), int(triangle.points[1].Y),
				int(triangle.points[2].X), int(triangle.points[2].Y), triangle.color)
		}

		if renderMethod == RENDER_TEXTURED || renderMethod == RENDER_TEXTURED_WIRE {
			drawTexturedTriangle(
				triangle.points[0].X, triangle.points[0].Y, triangle.points[0].Z, triangle.points[0].W, triangle.textCoords[0].u, triangle.textCoords[0].v,
				triangle.points[1].X, triangle.points[1].Y, triangle.points[1].Z, triangle.points[1].W, triangle.textCoords[1].u, triangle.textCoords[1].v,
				triangle.points[2].X, triangle.points[2].Y, triangle.points[2].Z, triangle.points[2].W, triangle.textCoords[2].u, triangle.textCoords[2].v,
				meshTexture,
			)
		}

		if renderMethod == RENDER_WIRE || renderMethod == RENDER_WIRE_VERTEX || renderMethod == RENDER_FILL_TRIANGLE_WIRE || renderMethod == RENDER_TEXTURED_WIRE {
			drawTriangle(int(triangle.points[0].X), int(triangle.points[0].Y),
				int(triangle.points[1].X), int(triangle.points[1].Y),
				int(triangle.points[2].X), int(triangle.points[2].Y), 0xFFFFFFFF)
		}
		if renderMethod == RENDER_WIRE_VERTEX {
			drawRectangle(int(triangle.points[0].X)-3, int(triangle.points[0].Y)-3, 6, 6, 0xFFFFFF00)
			drawRectangle(int(triangle.points[1].X)-3, int(triangle.points[1].Y)-3, 6, 6, 0xFFFFFF00)
			drawRectangle(int(triangle.points[2].X)-3, int(triangle.points[2].Y)-3, 6, 6, 0xFFFFFF00)
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
//func project(point vec3) (projectedPoint vec2) {
//	// To get perspective projection you follow the formula
//	// P'y = Py/Pz and P'x = Px/Pz, where P'y and P'x are the projected points
//	projectedPoint = vec2{X: int((point.X * fovFactor) / point.Z), Y: int(point.Y * fovFactor / point.Z)}
//	return projectedPoint
//}

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
	//currentMesh.rotation.X += 0.005
	//currentMesh.rotation.Z += 0.01

	//currentMesh.scale.X += 0.002
	//currentMesh.scale.Y += 0.001
	//currentMesh.translation.X += 0.01
	currentMesh.translation.Z = 5
	scaleMatrix := mat4MakeScale(currentMesh.scale.X, currentMesh.scale.Y, currentMesh.scale.Z)
	translationMatrix := mat4MakeTranslation(currentMesh.translation.X, currentMesh.translation.Y, currentMesh.translation.Z)
	rotationMatrixX, rotationMatrixY, rotationMatrixZ := mat4MakeRotationX(currentMesh.rotation.X), mat4MakeRotationY(currentMesh.rotation.Y), mat4MakeRotationZ(currentMesh.rotation.Z)

	for i := 0; i < len(currentMesh.faces); i++ {
		// Face index into the vertices array
		meshFace := currentMesh.faces[i]
		facesVertices := make([]vec3, 3)
		facesVertices[0] = currentMesh.vertices[meshFace.a-1]
		facesVertices[1] = currentMesh.vertices[meshFace.b-1]
		facesVertices[2] = currentMesh.vertices[meshFace.c-1]
		// order matters
		// Scale -> rotate -> translate

		worldMatrix := mat4Identity()
		worldMatrix = scaleMatrix.mulMat4(worldMatrix)
		worldMatrix = rotationMatrixZ.mulMat4(worldMatrix)
		worldMatrix = rotationMatrixY.mulMat4(worldMatrix)
		worldMatrix = rotationMatrixX.mulMat4(worldMatrix)
		worldMatrix = translationMatrix.mulMat4(worldMatrix)
		var transformedVertexArr [3]vec4
		for j := 0; j < 3; j++ {
			transformedVertex := vec4FromVec3(facesVertices[j])
			transformedVertex = worldMatrix.mulVec4(transformedVertex)
			transformedVertexArr[j] = transformedVertex // Save current vertex in arr of transformedVertex
		}
		vectorA, vectorB, vectorC := vec3FromVec4(transformedVertexArr[0]), vec3FromVec4(transformedVertexArr[1]), vec3FromVec4(transformedVertexArr[2])
		BAVector, CAVector := vectorB.sub(vectorA), vectorC.sub(vectorA)
		// For vectors BA and CA we also only care about the direction and not the magnitude
		BAVector.normalize()
		CAVector.normalize()

		// We are using a left-handed coordinate system (i.e Z grows inside the monitor)
		// Face normal vector can be normalized
		faceNormal := BAVector.cross(CAVector)
		faceNormal.normalize()

		if cullMethod == CULL_BACKFACE {
			// Backface culling
			cameraRayVec := cameraPosition.sub(vectorA)

			cameraRayFaceNormalDot := faceNormal.dot(cameraRayVec)

			// If dot product is less than 0 we should hide the face
			if cameraRayFaceNormalDot < 0 {
				continue
			}
		}

		var projectedPoints [3]vec4
		for j := 0; j < len(transformedVertexArr); j++ {
			projectedPoints[j] = projectionMatrix.mat4MulVec4Project(transformedVertexArr[j])

			// Scale into the view
			projectedPoints[j].X *= float64(windowWidth / 2)
			projectedPoints[j].Y *= float64(windowHeight / 2)

			// Invert Y values to account for difference between Y0 of color buffer (top left) and obj files
			projectedPoints[j].Y *= -1
			// Translate the projected point into the middle of the screen
			projectedPoints[j].X += float64(windowWidth / 2)
			projectedPoints[j].Y += float64(windowHeight / 2)
		}
		var projectedTriangle triangle
		// We have to invert the light ray, otehrwise it will be shaded when face is away from camera
		flatShadePercent := -faceNormal.dot(globalLight.direction)
		projectedTriangle.color = lightApplyIntensity(meshFace.color, flatShadePercent)
		projectedTriangle.points[0] = projectedPoints[0]
		projectedTriangle.points[1] = projectedPoints[1]
		projectedTriangle.points[2] = projectedPoints[2]
		projectedTriangle.textCoords[0] = meshFace.AUV
		projectedTriangle.textCoords[1] = meshFace.BUV
		projectedTriangle.textCoords[2] = meshFace.CUV
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
				case sdl.K_5:
					renderMethod = RENDER_TEXTURED
				case sdl.K_6:
					renderMethod = RENDER_TEXTURED_WIRE
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
