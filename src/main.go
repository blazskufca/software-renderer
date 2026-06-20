package main

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// TODO: Move global state into a central structure
// TODO: Add attribution for models (i.e 3D assets provided by Kenney.nl (CC0 License).)

const (
	cubeNMeshVertices = 8
)

var (
	windowWidth       = 2160
	windowHeight      = 4069
	colorBuffer       []byte
	zBuffer           []float64
	previousFrameTime time.Time
	wasCaptured       bool
	prevMouseX        int
	prevMouseY        int
	trianglesToRender []triangle
	polygonsRendered  int   // clipped polygons that survived culling this frame
	renderMethod            = RENDER_WIRE
	cullMethod              = CULL_BACKFACE
	projectionMatrix  mat4  = mat4{}
	globalLight       light = light{direction: vec3{0, 0, 1}}
	deltaTime         float64
	meshes            []*mesh
)

// Game implements ebiten.Game, driving the software renderer through ebiten's
// Update/Draw loop.
type Game struct{}

func setup() error {
	colorBuffer = make([]byte, windowWidth*windowHeight*4)
	zBuffer = make([]float64, windowWidth*windowHeight)

	FOVY := math.Pi / 3 // FOV needs to be in radians -> math.pi/3 = 60deg
	aspectRatioY := float64(windowHeight) / float64(windowWidth)
	aspectRatioX := float64(windowWidth) / float64(windowHeight)
	FOVX := 2 * (math.Atan(math.Tan(FOVY/2) * aspectRatioX))
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
	// Initialize the perspective projection matrix
	projectionMatrix = mat4MakePerspective(FOVY, aspectRatioY, zNear, zFar)
	initFrustumPlanes(FOVX, FOVY, zNear, zFar)
	currentMesh, err := newMeshFromFromFile("assets/f18-superhornet.obj", func(m *mesh) error {
		tw, th, meshTexture, err := loadTextureData("assets/Diffuse.jpg")
		if err != nil {
			return err
		}
		m.texture = MeshTextureData{
			pixelData: meshTexture,
			width:     tw,
			height:    th,
		}
		return nil
	}, func(m *mesh) error {
		m.translation = vec3{10, 0, 0}
		return nil
	}, func(m *mesh) error {
		m.mirrorHorizontally()
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to load obj: %w", err)
	}
	meshes = append(meshes, &currentMesh)
	currentMesh1, err := newMeshFromFromFile("assets/f22.obj", func(m *mesh) error {
		tw, th, meshTexture, err := loadTextureData("assets/f22.png")
		if err != nil {
			return err
		}
		m.texture = MeshTextureData{
			pixelData: meshTexture,
			width:     tw,
			height:    th,
		}
		m.scale = vec3{X: 1, Y: 1, Z: 1}
		return nil
	}, func(m *mesh) error {
		m.rotation = vec3{0, math.Pi / 2, 0}
		m.translation = vec3{X: -5}
		m.scale = vec3{X: 5, Y: 5, Z: 5}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to load obj: %w", err)
	}
	meshes = append(meshes, &currentMesh1)
	//currentMesh = newCubeMesh()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	for i := 0; i < len(trianglesToRender); i++ {
		triangle := trianglesToRender[i]
		if renderMethod == RENDER_FILL_TRIANGLE || renderMethod == RENDER_FILL_TRIANGLE_WIRE {
			drawFilledTriangle(
				triangle.points[0].X, triangle.points[0].Y, triangle.points[0].Z, triangle.points[0].W,
				triangle.points[1].X, triangle.points[1].Y, triangle.points[1].Z, triangle.points[1].W,
				triangle.points[2].X, triangle.points[2].Y, triangle.points[2].Z, triangle.points[2].W,
				triangle.color)
		}

		if renderMethod == RENDER_TEXTURED || renderMethod == RENDER_TEXTURED_WIRE {
			drawTexturedTriangle(
				triangle.points[0].X, triangle.points[0].Y, triangle.points[0].Z, triangle.points[0].W, triangle.textCoords[0].u, triangle.textCoords[0].v,
				triangle.points[1].X, triangle.points[1].Y, triangle.points[1].Z, triangle.points[1].W, triangle.textCoords[1].u, triangle.textCoords[1].v,
				triangle.points[2].X, triangle.points[2].Y, triangle.points[2].Z, triangle.points[2].W, triangle.textCoords[2].u, triangle.textCoords[2].v,
				triangle.flatShadePercent, triangle.texture)
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
	renderColorBuffer(screen)
	// Overlay the FPS counter on top of the rendered frame
	ebitenutil.DebugPrint(screen, fmt.Sprintf(
		"FPS: %.1f, TPS: %.1f\nPolygons: %d  Triangles: %d",
		ebiten.ActualFPS(), ebiten.ActualTPS(), polygonsRendered, len(trianglesToRender)))
	// Clear color buffer
	clearColorBuffer(0xFF000000)
	// Clear Z-buffer
	clearZBuffer(1)
}

// Layout reports the renderer's internal resolution; ebiten scales this to the
// actual (fullscreen) window.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return windowWidth, windowHeight
}

// project applies perspective projection to vec3 point
//func project(point vec3) (projectedPoint vec2) {
//	// To get perspective projection you follow the formula
//	// P'y = Py/Pz and P'x = Px/Pz, where P'y and P'x are the projected points
//	projectedPoint = vec2{X: int((point.X * fovFactor) / point.Z), Y: int(point.Y * fovFactor / point.Z)}
//	return projectedPoint
//}

func update() {
	// ebiten drives the frame pacing (fixed TPS), so we no longer sleep here;
	// deltaTime is measured from wall-clock time between updates.
	trianglesToRender = trianglesToRender[:0]
	polygonsRendered = 0

	now := time.Now()
	if previousFrameTime.IsZero() {
		previousFrameTime = now
	}
	deltaTime = now.Sub(previousFrameTime).Seconds()
	previousFrameTime = now
	for _, currentMesh := range meshes {
		if currentMesh == nil {
			panic("nil currentMesh")
		}
		for _, mutation := range currentMesh.perFrameMutations {
			err := mutation(currentMesh)
			if err != nil {
				panic(err)
			}
		}
		//currentMesh.rotation.X += math.Pi / 2 * deltaTime
		//currentMesh.rotation.Y += math.Pi / 2 * deltaTime
		//currentMesh.rotation.Y += math.Pi / 2 * deltaTime
		//currentMesh.rotation.Z += math.Pi / 2 * deltaTime
		//currentMesh.translation.Z = 5
		//globalCamera.position.X += 2.0 * deltaTime
		//globalCamera.position.Y += 2.0 * deltaTime

		//currentMesh.scale.X += 0.002
		//currentMesh.scale.Y += 0.001
		//currentMesh.translation.X += 0.01

		//globalCamera.position.X += 0.008 * deltaTime
		//globalCamera.position.Y += 0.008
		//currentMesh.translation.Y = -1.5
		scaleMatrix := mat4MakeScale(currentMesh.scale.X, currentMesh.scale.Y, currentMesh.scale.Z)
		translationMatrix := mat4MakeTranslation(currentMesh.translation.X, currentMesh.translation.Y, currentMesh.translation.Z)
		rotationMatrixX, rotationMatrixY, rotationMatrixZ := mat4MakeRotationX(currentMesh.rotation.X), mat4MakeRotationY(currentMesh.rotation.Y), mat4MakeRotationZ(currentMesh.rotation.Z)
		// Create a view matrix
		// Start the target as normalized Z axis
		target := vec3{Z: 1}

		cameraYaw := mat4MakeRotationY(globalCamera.yawAngle)
		cameraPitch := mat4MakeRotationX(globalCamera.pitchAngle)
		cameraRotation := cameraYaw.mulMat4(cameraPitch)
		globalCamera.direction = vec3FromVec4(cameraRotation.mulVec4(vec4FromVec3(target)))

		target = globalCamera.position.add(globalCamera.direction)

		viewMatrix := mat4LookAt(globalCamera.position, target, vec3{0, 1, 0})

		for i := 0; i < len(currentMesh.faces); i++ {
			// Face index into the vertices array
			//if i != 4 {
			//	continue
			//}
			meshFace := currentMesh.faces[i]
			facesVertices := [3]vec3{}

			facesVertices[0] = currentMesh.vertices[meshFace.a]
			facesVertices[1] = currentMesh.vertices[meshFace.b]
			facesVertices[2] = currentMesh.vertices[meshFace.c]
			// order matters
			// Scale -> rotate -> translate

			worldMatrix := mat4Identity()
			worldMatrix = scaleMatrix.mulMat4(worldMatrix)
			worldMatrix = rotationMatrixZ.mulMat4(worldMatrix)
			worldMatrix = rotationMatrixY.mulMat4(worldMatrix)
			worldMatrix = rotationMatrixX.mulMat4(worldMatrix)
			worldMatrix = translationMatrix.mulMat4(worldMatrix)
			worldMatrix = viewMatrix.mulMat4(worldMatrix)
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
				origin := vec3{} // Camera position can change so we need a real 0, 0, 0
				cameraRayVec := origin.sub(vectorA)

				cameraRayFaceNormalDot := faceNormal.dot(cameraRayVec)

				// If dot product is less than 0 we should hide the face
				if cameraRayFaceNormalDot < 0 {
					continue
				}
			}

			// Clipping stage
			poly := newPolyFromTriangle(&transformedVertexArr, meshFace.AUV, meshFace.BUV, meshFace.CUV)
			poly.clipPolygon()
			// Break the clipped polygon apart into triangles (fan triangulation)
			var clippedTriangles [maximumNumberOfPolygonTriangles]triangle
			var numberOfClippedTriangles int = poly.numberOfVertices - 2
			if numberOfClippedTriangles > 0 {
				polygonsRendered++
			}
			for t := 0; t < numberOfClippedTriangles; t++ {
				tr := triangle{}
				tr.points = [3]vec4{vec4FromVec3(poly.vertices[0]), vec4FromVec3(poly.vertices[t+1]), vec4FromVec3(poly.vertices[t+2])}
				tr.textCoords = [3]tex2{poly.texCoords[0], poly.texCoords[t+1], poly.texCoords[t+2]}
				clippedTriangles[t] = tr
			}
			for i := 0; i < numberOfClippedTriangles; i++ {
				clippedTriangle := clippedTriangles[i]

				var projectedPoints [3]vec4
				for j := 0; j < len(transformedVertexArr); j++ {
					projectedPoints[j] = projectionMatrix.mat4MulVec4Project(clippedTriangle.points[j])

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
				projectedTriangle.flatShadePercent = flatShadePercent
				projectedTriangle.points[0] = projectedPoints[0]
				projectedTriangle.points[1] = projectedPoints[1]
				projectedTriangle.points[2] = projectedPoints[2]
				projectedTriangle.textCoords[0] = clippedTriangle.textCoords[0]
				projectedTriangle.textCoords[1] = clippedTriangle.textCoords[1]
				projectedTriangle.textCoords[2] = clippedTriangle.textCoords[2]
				projectedTriangle.avgDepth = (transformedVertexArr[0].Z + transformedVertexArr[1].Z + transformedVertexArr[2].Z) / 3
				projectedTriangle.texture = &currentMesh.texture
				trianglesToRender = append(trianglesToRender, projectedTriangle)
			}
		}
	}

}

// processInput handles per-frame keyboard and mouse input. It returns true when
// the user has requested to quit.
func processInput() (quit bool) {
	// Capture the cursor for FPS mouse-look. On web, pointer lock is gated behind
	// a user gesture and the browser releases it when Esc is pressed, so we
	// (re)capture on a left click whenever the cursor isn't currently captured.
	if ebiten.CursorMode() != ebiten.CursorModeCaptured &&
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	}

	// On desktop, Esc quits. On web, Esc is the browser's "release pointer lock"
	// gesture, so we leave the demo running and let a click re-capture the cursor.
	if runtime.GOOS != "js" && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return true
	}

	// Render / cull mode toggles (one-shot on key press)
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		renderMethod = RENDER_WIRE_VERTEX
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) {
		renderMethod = RENDER_WIRE
	}
	if inpututil.IsKeyJustPressed(ebiten.Key3) {
		renderMethod = RENDER_FILL_TRIANGLE
	}
	if inpututil.IsKeyJustPressed(ebiten.Key4) {
		renderMethod = RENDER_FILL_TRIANGLE_WIRE
	}
	if inpututil.IsKeyJustPressed(ebiten.Key5) {
		renderMethod = RENDER_TEXTURED
	}
	if inpututil.IsKeyJustPressed(ebiten.Key6) {
		renderMethod = RENDER_TEXTURED_WIRE
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		cullMethod = CULL_BACKFACE
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyX) {
		cullMethod = CULL_NONE
	}

	// Camera movement (continuous while held)
	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		globalCamera.position.Y += 3 * deltaTime
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		globalCamera.position.Y -= 3 * deltaTime
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		right := vec3{Y: 1}.cross(globalCamera.direction) // right = up × forward (left-handed)
		right.normalize()
		globalCamera.rightVelocity = right.multiply(5 * deltaTime)
		globalCamera.position = globalCamera.position.sub(globalCamera.rightVelocity)
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		right := vec3{Y: 1}.cross(globalCamera.direction)
		right.normalize()
		globalCamera.rightVelocity = right.multiply(5 * deltaTime)
		globalCamera.position = globalCamera.position.add(globalCamera.rightVelocity)
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		globalCamera.forwardVelocity = globalCamera.direction.multiply(5 * deltaTime)
		globalCamera.position = globalCamera.position.add(globalCamera.forwardVelocity)
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		globalCamera.forwardVelocity = globalCamera.direction.multiply(5 * deltaTime)
		globalCamera.position = globalCamera.position.sub(globalCamera.forwardVelocity)
	}

	// Relative mouse look (only while captured, so the reported position
	// accumulates motion). While not captured — and on the first captured frame
	// after a transition — we just resync the baseline to the current position so
	// (re)capturing never produces a large delta spike.
	mouseX, mouseY := ebiten.CursorPosition()
	captured := ebiten.CursorMode() == ebiten.CursorModeCaptured
	if !captured || !wasCaptured {
		prevMouseX, prevMouseY = mouseX, mouseY
		wasCaptured = captured
		return false
	}
	yaw := float64(mouseX - prevMouseX)
	pitch := float64(mouseY - prevMouseY)
	prevMouseX, prevMouseY = mouseX, mouseY
	sensitivity := 0.005
	globalCamera.yawAngle += yaw * sensitivity
	globalCamera.pitchAngle -= pitch * sensitivity
	if globalCamera.pitchAngle > 1.55 {
		globalCamera.pitchAngle = 1.55
	}
	if globalCamera.pitchAngle < -1.55 {
		globalCamera.pitchAngle = -1.55
	}
	return false
}

// Update advances simulation state once per tick.
func (g *Game) Update() error {
	if processInput() {
		return ebiten.Termination
	}
	update()
	return nil
}

func main() {
	initializeWindow()
	if err := setup(); err != nil {
		log.Fatal(err)
	}
	if err := ebiten.RunGame(&Game{}); err != nil {
		log.Fatal(err)
	}
}
