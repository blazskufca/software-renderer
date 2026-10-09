package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/blazskufca/software-renderer/internal/camera"
	"github.com/blazskufca/software-renderer/internal/clipping"
	"github.com/blazskufca/software-renderer/internal/config"
	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/light"
	"github.com/blazskufca/software-renderer/internal/matrix"
	"github.com/blazskufca/software-renderer/internal/pipeline"
	"github.com/blazskufca/software-renderer/internal/raster"
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
	config            Config
	target            *raster.Target
	frustumPlanes     [6]clipping.Plane[T]
	projectionMatrix  matrix.Mat4[T]
	meshes            []*mesh.Mesh[T]
	trianglesToDraw   []raster.Triangle
	renderMethod      config.RenderMode
	cullMode          config.CullMode
	previousFrameTime time.Time
	deltaTime         float64
	camera            camera.Camera[T]
	light             light.Light[T]
	prevMouseX        int
	prevMouseY        int
	textureRasterMode config.TextureRasterizer
	fillRasterMode    config.FillRasterMode
	shadingMode       config.ShadingMode
	uiButtonsCache    []uiButton
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

	renderer := &Renderer[T]{config: c, camera: camera.Camera[T]{}}
	for _, opt := range opts {
		opt(renderer)
	}
	renderer.config.AspectRatioY = float64(renderer.config.WindowHeight) / float64(renderer.config.WindowWidth)
	renderer.config.AspectRatioX = float64(renderer.config.WindowWidth) / float64(renderer.config.WindowHeight)
	renderer.config.FovX = 2 * (math.Atan(math.Tan(renderer.config.FovY/2) * renderer.config.AspectRatioX))
	renderer.cullMode = config.CULL_BACKFACE
	renderer.textureRasterMode = config.TEXTURE_PINEDA
	renderer.fillRasterMode = config.FILL_PINEDA
	renderer.renderMethod = config.RENDER_TEXTURED
	renderer.shadingMode = config.SHADING_GOURAUD
	renderer.light = light.Light[T]{Direction: vector.Vec3[T]{X: -1, Y: -1, Z: 1}}
	renderer.light.Direction.Normalize()
	camX, camY, camZ := -3.0, 1.5, 1.5
	renderer.camera.Position = vector.Vec3[T]{X: T(camX), Y: T(camY), Z: T(camZ)}
	renderer.camera.YawAngle = 0.71
	renderer.camera.PitchAngle = 0.31
	return renderer
}

func (r *Renderer[T]) Setup() error {
	r.initializeWindow()
	target, err := raster.NewTarget(r.config.WindowWidth, r.config.WindowHeight)
	if err != nil {
		return err
	}
	r.target = target
	r.target.Clear(0xFF000000)

	// Initialize the perspective projection matrix
	m := matrix.Mat4MakePerspective[T, T](T(r.config.FovY), T(r.config.AspectRatioY), T(r.config.ZNear), T(r.config.ZFar))
	r.projectionMatrix = m
	clipping.InitFrustumPlanes[T, T, T](&r.frustumPlanes, T(r.config.FovX), T(r.config.FovY), T(r.config.ZNear), T(r.config.ZFar))
	currentMesh, err := mesh.NewMeshFromFile[T]("assets/race.obj", func(m *mesh.Mesh[T]) error {
		tw, th, pixels, err := texture.LoadTextureData("assets/colormap.png")
		if err != nil {
			return err
		}
		m.Texture = mesh.TextureData{PixelData: pixels, Width: tw, Height: th}
		return nil
	}, func(m *mesh.Mesh[T]) error {
		m.Translation = vector.Vec3[T]{Z: 5}
		return nil
	}, func(m *mesh.Mesh[T]) error {
		m.PerFrameMutations = append(m.PerFrameMutations, func(m *mesh.Mesh[T]) error {
			rot := (math.Pi / 2) * r.deltaTime
			m.Rotation.Y += T(rot)
			return nil
		})
		return nil
	})
	if err != nil {
		return err
	}
	r.meshes = append(r.meshes, &currentMesh)
	//currentMesh = newCubeMesh()
	return nil
}

func (r *Renderer[T]) Draw(screen *ebiten.Image) {
	for i := range r.trianglesToDraw {
		tri := &r.trianglesToDraw[i]
		if r.renderMethod == config.RENDER_FILL_TRIANGLE || r.renderMethod == config.RENDER_FILL_TRIANGLE_WIRE {
			if r.fillRasterMode == config.FILL_PINEDA {
				r.target.FillTriangle(tri)
			} else {
				r.drawFilledTriangle(tri)
			}
		}

		if r.renderMethod == config.RENDER_TEXTURED || r.renderMethod == config.RENDER_TEXTURED_WIRE {
			if r.textureRasterMode == config.TEXTURE_PINEDA {
				r.target.TextureTriangle(tri)
			} else {
				r.drawTexturedTriangle(tri)
			}
		}

		if r.renderMethod == config.RENDER_WIRE || r.renderMethod == config.RENDER_WIRE_VERTEX || r.renderMethod == config.RENDER_FILL_TRIANGLE_WIRE || r.renderMethod == config.RENDER_TEXTURED_WIRE {
			r.drawTriangle(int(tri.V[0].X), int(tri.V[0].Y),
				int(tri.V[1].X), int(tri.V[1].Y),
				int(tri.V[2].X), int(tri.V[2].Y), 0xFFFFFFFF)
		}
		if r.renderMethod == config.RENDER_WIRE_VERTEX {
			r.drawRectangle(int(tri.V[0].X)-3, int(tri.V[0].Y)-3, 6, 6, 0xFFFFFF00)
			r.drawRectangle(int(tri.V[1].X)-3, int(tri.V[1].Y)-3, 6, 6, 0xFFFFFF00)
			r.drawRectangle(int(tri.V[2].X)-3, int(tri.V[2].Y)-3, 6, 6, 0xFFFFFF00)
		}

	}
	screen.WritePixels(r.target.Bytes())
	r.drawRenderModeToolbar(screen)
	r.drawMovementButtons(screen)
	// Overlay the FPS counter on top of the rendered frame
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf(
		"FPS: %.1f, TPS: %.1f\nTriangles: %d",
		ebiten.ActualFPS(), ebiten.ActualTPS(), len(r.trianglesToDraw)),
		8, r.renderModeToolbarHeight()+8)
	r.target.Clear(0xFF000000)
}

// Layout reports the renderer's internal resolution; ebiten scales this to the
// actual (fullscreen) window.
func (r *Renderer[T]) Layout(outsideWidth, outsideHeight int) (int, int) {
	return r.config.WindowWidth, r.config.WindowHeight
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

	view, dir := pipeline.ViewMatrix(r.camera.Position, r.camera.YawAngle, r.camera.PitchAngle)
	r.camera.Direction = dir
	params := pipeline.Params[T]{
		View:       view,
		Projection: r.projectionMatrix,
		Frustum:    r.frustumPlanes,
		Light:      r.light,
		Shading:    r.shadingMode,
		Cull:       r.cullMode,
		Width:      r.config.WindowWidth,
		Height:     r.config.WindowHeight,
	}

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
		r.trianglesToDraw = pipeline.AppendMesh(r.trianglesToDraw, currentMesh, &params)
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
			r.config.MouseWasCaptured = false
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
		r.fillRasterMode = config.FILL_SCANLINE
	}
	if inpututil.IsKeyJustPressed(ebiten.Key8) {
		r.fillRasterMode = config.FILL_PINEDA
	}
	if inpututil.IsKeyJustPressed(ebiten.Key9) {
		r.textureRasterMode = config.TEXTURE_SCANLINE
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDigit0) {
		r.textureRasterMode = config.TEXTURE_PINEDA
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		r.cullMode = config.CULL_BACKFACE
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyX) {
		r.cullMode = config.CULL_NONE

	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		r.shadingMode = config.SHADING_FLAT
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		r.shadingMode = config.SHADING_GOURAUD
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
	if !captured || !r.config.MouseWasCaptured {
		r.prevMouseX, r.prevMouseY = mouseX, mouseY
		r.config.MouseWasCaptured = captured
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
	cpuProfile := flag.String("cpuprofile", "", "write a CPU profile to `file`")
	memProfile := flag.String("memprofile", "", "write a memory profile to `file` on exit")
	flag.Parse()

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatalf("could not create CPU profile: %v", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("could not start CPU profile: %v", err)
		}
		defer pprof.StopCPUProfile()
	}

	r := NewRenderer[float32](func(r *Renderer[float32]) {
		isVsynced := ebiten.IsVsyncEnabled()
		log.Printf("Using vsync: %v", isVsynced)
		if isVsynced {
			log.Printf("Trying to disable vsync")
			ebiten.SetVsyncEnabled(false)
			ebiten.SetTPS(ebiten.SyncWithFPS)
			if isVs := ebiten.IsVsyncEnabled(); !isVs {
				log.Printf("Successfully disabled vsync, TPS synced with Framerate - Both uncapped")
			}
		}

	})
	if err := r.Setup(); err != nil {
		log.Fatal(err)
	}
	if err := ebiten.RunGame(r); err != nil {
		// Don't log.Fatal here: os.Exit would skip the deferred profile flushes.
		log.Printf("game exited: %v", err)
	}

	if *memProfile != "" {
		f, err := os.Create(*memProfile)
		if err != nil {
			log.Fatalf("could not create memory profile: %v", err)
		}
		defer f.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatalf("could not write memory profile: %v", err)
		}
	}
}
