package main

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/blazskufca/software-renderer/internal/camera"
	"github.com/blazskufca/software-renderer/internal/clipping"
	"github.com/blazskufca/software-renderer/internal/config"
	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/light"
	"github.com/blazskufca/software-renderer/internal/matrix"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// TODO: Move global state into a central structure
// TODO: Add attribution for models (i.e 3D assets provided by Kenney.nl (CC0 License).)

// Game implements ebiten.Game, driving the software renderer through ebiten's
// Update/Draw loop.
type Config struct {
	WindowWidth             int
	WindowHeight            int
	FovY                    float64
	FovX                    float64
	AspectRatioY            float64
	AspectRatioX            float64
	ZNear                   float64
	ZFar                    float64
	MaximumNumberOfPolygons int
	MouseWasCaptured        bool
}

type Renderer[T utils.Numeric] struct {
	Config            Config
	ColorBuffer       []byte
	ZBuffer           []T
	frustumPlanes     [6]clipping.Plane[T]
	projectionMatrix  matrix.Mat4[T]
	meshes            []*mesh.Mesh[T]
	trianglesToDraw   []triangle[T]
	renderMethod      config.RenderMode
	cullMode          config.CullMode
	previousFrameTime time.Time
	deltaTime         float64
	camera            camera.Camera[T]
	light             light.Light[T]
	prevMouseX        int
	prevMouseY        int
	textureRasterFunc config.TexturizedRasterizer[T]
	fillRasterFunc    config.FillRasterizer[T]
	textureRasterMode config.TextureRasterizer
	fillRasterMode    config.FillRasterMode
}

// setFillRaster selects the triangle fill rasterizer and records the choice so
// the UI can reflect which backend is active (func values aren't comparable, so
// we track the mode enum alongside the func).
func (r *Renderer[T]) setFillRaster(mode config.FillRasterMode) {
	r.fillRasterMode = mode
	switch mode {
	case config.FILL_PINEDA:
		r.fillRasterFunc = r.drawFilledTrianglePineda
	default:
		r.fillRasterFunc = r.drawFilledTriangle
	}
}

// setTextureRaster selects the texture-mapping rasterizer and records the choice
// so the UI can reflect which backend is active.
func (r *Renderer[T]) setTextureRaster(mode config.TextureRasterizer) {
	r.textureRasterMode = mode
	switch mode {
	case config.TEXTURE_PINEDA:
		r.textureRasterFunc = r.drawTexturedTrianglePineda
	default:
		r.textureRasterFunc = r.drawTexturedTriangle
	}
}

type RendererOpts[T utils.Numeric] func(*Renderer[T])

func NewRenderer[T utils.Numeric](opts ...RendererOpts[T]) *Renderer[T] {
	c := Config{
		WindowWidth:             960,
		WindowHeight:            480,
		FovY:                    math.Pi / 3,
		ZNear:                   0.1,
		ZFar:                    100.0,
		MaximumNumberOfPolygons: 10,
	}

	renderer := &Renderer[T]{Config: c, camera: camera.Camera[T]{}}
	for _, opt := range opts {
		opt(renderer)
	}
	renderer.Config.AspectRatioY = float64(renderer.Config.WindowHeight) / float64(renderer.Config.WindowWidth)
	renderer.Config.AspectRatioX = float64(renderer.Config.WindowWidth) / float64(renderer.Config.WindowHeight)
	renderer.Config.FovX = 2 * (math.Atan(math.Tan(renderer.Config.FovY/2) * renderer.Config.AspectRatioX))
	renderer.light = light.Light[T]{Direction: vector.Vec3[T]{Y: 10}}
	renderer.camera.Direction = vector.Vec3[T]{Z: 1} // forward, +Z, normalized
	renderer.camera.Position = vector.Vec3[T]{}      // origin, looking down +Z at the mesh
	renderer.cullMode = config.CULL_BACKFACE
	renderer.setTextureRaster(config.TEXTURE_PINEDA)
	renderer.setFillRaster(config.FILL_PINEDA)
	renderer.renderMethod = config.RENDER_TEXTURED
	return renderer
}

func (r *Renderer[T]) Setup() error {
	r.initializeWindow()
	r.ColorBuffer = make([]byte, r.Config.WindowWidth*r.Config.WindowHeight*4)
	r.ZBuffer = make([]T, r.Config.WindowWidth*r.Config.WindowHeight)
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
	m := matrix.Mat4MakePerspective[T, T](T(r.Config.FovY), T(r.Config.AspectRatioY), T(r.Config.ZNear), T(r.Config.ZFar))
	r.projectionMatrix = m
	clipping.InitFrustumPlanes[T, T, T](&r.frustumPlanes, T(r.Config.FovX), T(r.Config.FovY), T(r.Config.ZNear), T(r.Config.ZFar))
	currentMesh, err := mesh.NewMeshFromFile[T]("assets/race.obj")
	if err != nil {
		return err
	}
	// Load the diffuse texture for the mesh. LoadTextureData returns primitives
	// (it can't import the mesh package), so build the TextureData here.
	tw, th, pixels, err := texture.LoadTextureData("assets/colormap.png")
	if err != nil {
		return err
	}
	currentMesh.Texture = mesh.TextureData{PixelData: pixels, Width: tw, Height: th}
	// Push the mesh in front of the camera (which sits at the origin looking down
	// +Z) so it lands inside the view frustum (ZNear..ZFar).
	currentMesh.Translation = vector.Vec3[T]{Z: 5}
	r.meshes = append(r.meshes, &currentMesh)

	//currentMesh = newCubeMesh()
	return nil
}

func (r *Renderer[T]) Draw(screen *ebiten.Image) {
	for i := 0; i < len(r.trianglesToDraw); i++ {
		triangle := r.trianglesToDraw[i]
		if r.renderMethod == config.RENDER_FILL_TRIANGLE || r.renderMethod == config.RENDER_FILL_TRIANGLE_WIRE {
			r.fillRasterFunc(
				triangle.points[0].X, triangle.points[0].Y, triangle.points[0].Z, triangle.points[0].W,
				triangle.points[1].X, triangle.points[1].Y, triangle.points[1].Z, triangle.points[1].W,
				triangle.points[2].X, triangle.points[2].Y, triangle.points[2].Z, triangle.points[2].W,
				triangle.color)
		}

		if r.renderMethod == config.RENDER_TEXTURED || r.renderMethod == config.RENDER_TEXTURED_WIRE {
			r.textureRasterFunc(
				triangle.points[0].X, triangle.points[0].Y, triangle.points[0].Z, triangle.points[0].W, triangle.textCoords[0].U, triangle.textCoords[0].V,
				triangle.points[1].X, triangle.points[1].Y, triangle.points[1].Z, triangle.points[1].W, triangle.textCoords[1].U, triangle.textCoords[1].V,
				triangle.points[2].X, triangle.points[2].Y, triangle.points[2].Z, triangle.points[2].W, triangle.textCoords[2].U, triangle.textCoords[2].V,
				triangle.flatShadePercent, triangle.texture)
		}

		if r.renderMethod == config.RENDER_WIRE || r.renderMethod == config.RENDER_WIRE_VERTEX || r.renderMethod == config.RENDER_FILL_TRIANGLE_WIRE || r.renderMethod == config.RENDER_TEXTURED_WIRE {
			r.drawTriangle(int(triangle.points[0].X), int(triangle.points[0].Y),
				int(triangle.points[1].X), int(triangle.points[1].Y),
				int(triangle.points[2].X), int(triangle.points[2].Y), 0xFFFFFFFF)
		}
		if r.renderMethod == config.RENDER_WIRE_VERTEX {
			r.drawRectangle(int(triangle.points[0].X)-3, int(triangle.points[0].Y)-3, 6, 6, 0xFFFFFF00)
			r.drawRectangle(int(triangle.points[1].X)-3, int(triangle.points[1].Y)-3, 6, 6, 0xFFFFFF00)
			r.drawRectangle(int(triangle.points[2].X)-3, int(triangle.points[2].Y)-3, 6, 6, 0xFFFFFF00)
		}

	}
	r.renderColorBuffer(screen)
	r.drawRenderModeToolbar(screen)
	r.drawMovementButtons(screen)
	// Overlay the FPS counter on top of the rendered frame
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(
		"FPS: %.1f, TPS: %.1f\nTriangles: %d",
		ebiten.ActualFPS(), ebiten.ActualTPS(), len(r.trianglesToDraw)),
		8, r.renderModeToolbarHeight()+8)
	// Clear color buffer
	r.clearColorBuffer(0xFF000000)
	// Clear Z-buffer
	r.clearZBuffer(1)
}

// Layout reports the renderer's internal resolution; ebiten scales this to the
// actual (fullscreen) window.
func (r *Renderer[T]) Layout(outsideWidth, outsideHeight int) (int, int) {
	return r.Config.WindowWidth, r.Config.WindowHeight
}

// project applies perspective projection to vec3 point
//func project(point vec3) (projectedPoint vec2) {
//	// To get perspective projection you follow the formula
//	// P'y = Py/Pz and P'x = Px/Pz, where P'y and P'x are the projected points
//	projectedPoint = vec2{X: int((point.X * fovFactor) / point.Z), Y: int(point.Y * fovFactor / point.Z)}
//	return projectedPoint
//}

func (r *Renderer[T]) update() {
	// ebiten drives the frame pacing (fixed TPS), so we no longer sleep here;
	// deltaTime is measured in Update, before input is processed.
	r.trianglesToDraw = r.trianglesToDraw[:0]

	for _, currentMesh := range r.meshes {
		if currentMesh == nil {
			panic("nil currentMesh")
		}
		for _, mutation := range currentMesh.PerFrameMutations {
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

		scaleMatrix := matrix.Mat4MakeScale[T](currentMesh.Scale.X, currentMesh.Scale.Y, currentMesh.Scale.Z)
		translationMatrix := matrix.Mat4MakeTranslation[T](currentMesh.Translation.X, currentMesh.Translation.Y, currentMesh.Translation.Z)
		rotationMatrixX, rotationMatrixY, rotationMatrixZ := matrix.Mat4MakeRotationX[T](currentMesh.Rotation.X), matrix.Mat4MakeRotationY[T](currentMesh.Rotation.Y), matrix.Mat4MakeRotationZ[T](currentMesh.Rotation.Z)
		// Create a view matrix
		// Start the target as normalized Z axis
		target := vector.Vec3[T]{Z: 1}

		cameraYaw := matrix.Mat4MakeRotationY[T](T(r.camera.YawAngle))
		cameraPitch := matrix.Mat4MakeRotationX[T](T(r.camera.PitchAngle))
		cameraRotation := cameraYaw.MulMat4(cameraPitch)
		r.camera.Direction = cameraRotation.MulVec4(target.ToVec4()).ToVec3()

		target = r.camera.Position.Add(r.camera.Direction)

		viewMatrix := matrix.Mat4LookAt(r.camera.Position, target, vector.Vec3[T]{Y: 1})

		for i := 0; i < len(currentMesh.Faces); i++ {
			// Face index into the vertices array
			//if i != 4 {
			//	continue
			//}
			meshFace := currentMesh.Faces[i]
			facesVertices := [3]vector.Vec3[T]{}

			facesVertices[0] = currentMesh.Vertices[meshFace.A]
			facesVertices[1] = currentMesh.Vertices[meshFace.B]
			facesVertices[2] = currentMesh.Vertices[meshFace.C]
			// order matters
			// Scale -> rotate -> translate

			worldMatrix := matrix.Mat4Identity[T]()
			worldMatrix = scaleMatrix.MulMat4(worldMatrix)
			worldMatrix = rotationMatrixZ.MulMat4(worldMatrix)
			worldMatrix = rotationMatrixY.MulMat4(worldMatrix)
			worldMatrix = rotationMatrixX.MulMat4(worldMatrix)
			worldMatrix = translationMatrix.MulMat4(worldMatrix)
			worldMatrix = viewMatrix.MulMat4(worldMatrix)
			var transformedVertexArr [3]vector.Vec4[T]
			for j := 0; j < 3; j++ {
				transformedVertex := facesVertices[j].ToVec4()
				transformedVertex = worldMatrix.MulVec4(transformedVertex)
				transformedVertexArr[j] = transformedVertex // Save current vertex in arr of transformedVertex
			}
			vectorA, vectorB, vectorC := transformedVertexArr[0].ToVec3(), transformedVertexArr[1].ToVec3(), transformedVertexArr[2].ToVec3()
			BAVector, CAVector := vectorB.Sub(vectorA), vectorC.Sub(vectorA)
			// For vectors BA and CA we also only care about the direction and not the magnitude
			BAVector.Normalize()
			CAVector.Normalize()

			// We are using a left-handed coordinate system (i.e Z grows inside the monitor)
			// Face normal vector can be normalized
			faceNormal := BAVector.Cross(CAVector)
			faceNormal.Normalize()

			if r.cullMode == config.CULL_BACKFACE {
				// Backface culling
				origin := vector.Vec3[T]{} // Camera position can change so we need a real 0, 0, 0
				cameraRayVec := origin.Sub(vectorA)

				cameraRayFaceNormalDot := faceNormal.Dot(cameraRayVec)

				// If dot product is less than 0 we should hide the face
				if cameraRayFaceNormalDot < 0 {
					continue
				}
			}

			// Clipping stage
			poly := clipping.NewPolyFromTriangle(&transformedVertexArr, meshFace.AUV, meshFace.BUV, meshFace.CUV)
			poly.ClipPolygon(&r.frustumPlanes)
			// Break the clipped polygon apart into triangles (fan triangulation)
			var clippedTriangles [10]triangle[T]
			var numberOfClippedTriangles int = poly.NumberOfVertices - 2
			for t := 0; t < numberOfClippedTriangles; t++ {
				tr := triangle[T]{}
				tr.points = [3]vector.Vec4[T]{poly.Vertices[0].ToVec4(), poly.Vertices[t+1].ToVec4(), poly.Vertices[t+2].ToVec4()}
				tr.textCoords = [3]texture.Tex2[T]{poly.TexCoords[0], poly.TexCoords[t+1], poly.TexCoords[t+2]}
				clippedTriangles[t] = tr
			}
			for i := 0; i < numberOfClippedTriangles; i++ {
				clippedTriangle := clippedTriangles[i]

				var projectedPoints [3]vector.Vec4[T]
				for j := 0; j < len(transformedVertexArr); j++ {
					projectedPoints[j] = r.projectionMatrix.Mat4MulVec4Project(clippedTriangle.points[j])

					// Scale into the view
					projectedPoints[j].X *= T(r.Config.WindowWidth / 2)
					projectedPoints[j].Y *= T(r.Config.WindowHeight / 2)

					// Invert Y values to account for difference between Y0 of color buffer (top left) and obj files
					projectedPoints[j].Y *= -1
					// Translate the projected point into the middle of the screen
					projectedPoints[j].X += T(r.Config.WindowWidth / 2)
					projectedPoints[j].Y += T(r.Config.WindowHeight / 2)
				}
				var projectedTriangle triangle[T]
				// We have to invert the light ray, otehrwise it will be shaded when face is away from camera
				flatShadePercent := -faceNormal.Dot(r.light.Direction)
				projectedTriangle.color = light.LightApplyIntensity(meshFace.Color, float32(flatShadePercent))
				projectedTriangle.flatShadePercent = flatShadePercent
				projectedTriangle.points[0] = projectedPoints[0]
				projectedTriangle.points[1] = projectedPoints[1]
				projectedTriangle.points[2] = projectedPoints[2]
				projectedTriangle.textCoords[0] = clippedTriangle.textCoords[0]
				projectedTriangle.textCoords[1] = clippedTriangle.textCoords[1]
				projectedTriangle.textCoords[2] = clippedTriangle.textCoords[2]
				projectedTriangle.avgDepth = (transformedVertexArr[0].Z + transformedVertexArr[1].Z + transformedVertexArr[2].Z) / 3
				projectedTriangle.texture = &currentMesh.Texture
				r.trianglesToDraw = append(r.trianglesToDraw, projectedTriangle)
			}
		}
	}

}

// Camera movement primitives. These are shared by the keyboard handlers and the
// on-screen navigation buttons so both paths behave identically. Each applies one
// frame of motion scaled by deltaTime.
func (r *Renderer[T]) moveForward() {
	r.camera.ForwardVelocity = r.camera.Direction.Multiply(5 * T(r.deltaTime))
	r.camera.Position = r.camera.Position.Add(r.camera.ForwardVelocity)
}

func (r *Renderer[T]) moveBackward() {
	r.camera.ForwardVelocity = r.camera.Direction.Multiply(5 * T(r.deltaTime))
	r.camera.Position = r.camera.Position.Sub(r.camera.ForwardVelocity)
}

func (r *Renderer[T]) strafeLeft() {
	right := vector.Vec3[T]{Y: 1}.Cross(r.camera.Direction) // right = up × forward (left-handed)
	right.Normalize()
	r.camera.RightVelocity = right.Multiply(5 * T(r.deltaTime))
	r.camera.Position = r.camera.Position.Sub(r.camera.RightVelocity)
}

func (r *Renderer[T]) strafeRight() {
	right := vector.Vec3[T]{Y: 1}.Cross(r.camera.Direction)
	right.Normalize()
	r.camera.RightVelocity = right.Multiply(5 * T(r.deltaTime))
	r.camera.Position = r.camera.Position.Add(r.camera.RightVelocity)
}

func (r *Renderer[T]) moveUp()   { r.camera.Position.Y += 3 * T(r.deltaTime) }
func (r *Renderer[T]) moveDown() { r.camera.Position.Y -= 3 * T(r.deltaTime) }

func (r *Renderer[T]) lookLeft()  { r.camera.YawAngle -= float32(1.5 * r.deltaTime) }
func (r *Renderer[T]) lookRight() { r.camera.YawAngle += float32(1.5 * r.deltaTime) }
func (r *Renderer[T]) lookUp()    { r.camera.PitchAngle += float32(1.5 * r.deltaTime); r.clampPitch() }
func (r *Renderer[T]) lookDown()  { r.camera.PitchAngle -= float32(1.5 * r.deltaTime); r.clampPitch() }

func (r *Renderer[T]) clampPitch() {
	if r.camera.PitchAngle > 1.55 {
		r.camera.PitchAngle = 1.55
	}
	if r.camera.PitchAngle < -1.55 {
		r.camera.PitchAngle = -1.55
	}
}

// processInput handles per-frame keyboard and mouse input. It returns true when
// the user has requested to quit.
func (r *Renderer[T]) processInput() (quit bool) {
	// On-screen navigation buttons (held via touch or mouse) run every frame.
	r.applyMovementButtons()

	// Touch taps on the toolbar (mobile has no mouse), handled like a click.
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		x, y := ebiten.TouchPosition(id)
		r.handleRenderModeButtonClick(x, y)
	}

	// UI clicks are handled before pointer capture so the toolbar remains usable.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mouseX, mouseY := ebiten.CursorPosition()
		if r.handleRenderModeButtonClick(mouseX, mouseY) {
			return false
		}
		// Don't capture the cursor for mouse-look when the click lands on a
		// navigation button — the user is driving the camera with the buttons.
		if r.pointInMovementButtons(mouseX, mouseY) {
			return false
		}

		// Capture the cursor for FPS mouse-look. Browsers gate pointer lock behind
		// a user gesture and release it on Esc; desktop follows the same click-to-
		// capture flow so the toolbar is clickable before capture.
		if ebiten.CursorMode() != ebiten.CursorModeCaptured {
			ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		}
	}

	// Esc releases pointer capture first so the toolbar remains reachable after
	// using mouse-look. On desktop, pressing Esc again while uncaptured quits.
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if ebiten.CursorMode() == ebiten.CursorModeCaptured {
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
			r.Config.MouseWasCaptured = false
			return false
		}
		if runtime.GOOS != "js" {
			return true
		}
	}

	// Render / cull mode toggles (one-shot on key press)
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		r.renderMethod = config.RENDER_WIRE_VERTEX
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) {
		r.renderMethod = config.RENDER_WIRE
	}
	if inpututil.IsKeyJustPressed(ebiten.Key3) {
		r.renderMethod = config.RENDER_FILL_TRIANGLE
	}
	if inpututil.IsKeyJustPressed(ebiten.Key4) {
		r.renderMethod = config.RENDER_FILL_TRIANGLE_WIRE
	}
	if inpututil.IsKeyJustPressed(ebiten.Key5) {
		r.renderMethod = config.RENDER_TEXTURED
	}
	if inpututil.IsKeyJustPressed(ebiten.Key6) {
		r.renderMethod = config.RENDER_TEXTURED_WIRE
	}
	if inpututil.IsKeyJustPressed(ebiten.Key7) {
		r.setFillRaster(config.FILL_SCANLINE)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key8) {
		r.setFillRaster(config.FILL_PINEDA)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key9) {
		r.setTextureRaster(config.TEXTURE_SCANLINE)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDigit0) {
		r.setTextureRaster(config.TEXTURE_PINEDA)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		r.cullMode = config.CULL_BACKFACE
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyX) {
		r.cullMode = config.CULL_NONE

	}

	// Camera movement (continuous while held)
	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		r.moveUp()
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		r.moveDown()
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		r.strafeLeft()
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		r.strafeRight()
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		r.moveForward()
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		r.moveBackward()
	}

	// Relative mouse look (only while captured, so the reported position
	// accumulates motion). While not captured — and on the first captured frame
	// after a transition — we just resync the baseline to the current position so
	// (re)capturing never produces a large delta spike.
	mouseX, mouseY := ebiten.CursorPosition()
	captured := ebiten.CursorMode() == ebiten.CursorModeCaptured
	if !captured || !r.Config.MouseWasCaptured {
		r.prevMouseX, r.prevMouseY = mouseX, mouseY
		r.Config.MouseWasCaptured = captured
		return false
	}
	yaw := float64(mouseX - r.prevMouseX)
	pitch := float64(mouseY - r.prevMouseY)
	r.prevMouseX, r.prevMouseY = mouseX, mouseY
	sensitivity := 0.005
	r.camera.YawAngle += float32(yaw) * float32(sensitivity)
	r.camera.PitchAngle -= float32(pitch) * float32(sensitivity)
	if r.camera.PitchAngle > 1.55 {
		r.camera.PitchAngle = 1.55
	}
	if r.camera.PitchAngle < -1.55 {
		r.camera.PitchAngle = -1.55
	}
	return false
}

// Update advances simulation state once per tick.
func (r *Renderer[T]) Update() error {
	// deltaTime is measured here, before processInput, so that input applied this
	// tick uses the current frame's delta rather than the previous frame's (and so
	// the first frame isn't multiplied by a zero delta).
	now := time.Now()
	if r.previousFrameTime.IsZero() {
		r.previousFrameTime = now
	}
	r.deltaTime = now.Sub(r.previousFrameTime).Seconds()
	r.previousFrameTime = now

	if r.processInput() {
		return ebiten.Termination
	}
	r.update()
	return nil
}

func main() {
	r := NewRenderer[float32]()
	if err := r.Setup(); err != nil {
		log.Fatal(err)
	}
	if err := ebiten.RunGame(r); err != nil {
		log.Fatal(err)
	}
}
