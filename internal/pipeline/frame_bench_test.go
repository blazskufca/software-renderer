package pipeline_test

import (
	"io"
	"log"
	"math"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/blazskufca/software-renderer/internal/clipping"
	"github.com/blazskufca/software-renderer/internal/config"
	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/light"
	"github.com/blazskufca/software-renderer/internal/matrix"
	"github.com/blazskufca/software-renderer/internal/pipeline"
	"github.com/blazskufca/software-renderer/internal/raster"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/vector"
)

const width, height, frames = 960, 480, 36

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	if runtime.GOOS != "js" {
		if err := os.Chdir("../.."); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}

func loadScene(tb testing.TB) (*mesh.Mesh[float32], pipeline.Params[float32]) {
	tb.Helper()
	m, err := mesh.NewMeshFromFile[float32]("assets/race.obj", func(m *mesh.Mesh[float32]) error {
		tw, th, pixels, err := texture.LoadTextureData("assets/colormap.png")
		if err != nil {
			return err
		}
		m.Texture = mesh.TextureData{PixelData: pixels, Width: tw, Height: th}
		m.Translation = vector.Vec3[float32]{Z: 5}
		return nil
	})
	if err != nil {
		tb.Fatal(err)
	}

	fovY := math.Pi / 3
	fovX := 2 * math.Atan(math.Tan(fovY/2)*float64(width)/float64(height))
	p := pipeline.Params[float32]{
		Projection: matrix.Mat4MakePerspective[float32, float32](float32(fovY), float32(height)/float32(width), 0.1, 100),
		Light:      light.Light[float32]{Direction: vector.Vec3[float32]{X: -1, Y: -1, Z: 1}},
		Shading:    config.SHADING_GOURAUD,
		Cull:       config.CULL_BACKFACE,
		Width:      width,
		Height:     height,
	}
	p.Light.Direction.Normalize()
	clipping.InitFrustumPlanes[float32, float32, float32](&p.Frustum, float32(fovX), float32(fovY), 0.1, 100)
	p.View, _ = pipeline.ViewMatrix(cameraPos, cameraYaw, cameraPitch)
	return &m, p
}

var cameraPos = vector.Vec3[float32]{X: -3, Y: 1.5, Z: 1.5}

const cameraYaw, cameraPitch = 0.71, 0.31

func moveCloser(p *pipeline.Params[float32], distance float32) {
	_, dir := pipeline.ViewMatrix(cameraPos, cameraYaw, cameraPitch)
	p.View, _ = pipeline.ViewMatrix(cameraPos.Add(dir.Multiply(distance)), cameraYaw, cameraPitch)
}

func rotate(m *mesh.Mesh[float32], frame int) {
	m.Rotation.Y = float32(frame%frames) * 2 * math.Pi / frames
}

func BenchmarkGeometry(b *testing.B) {
	m, p := loadScene(b)
	var tris []raster.Triangle
	frame := 0
	for b.Loop() {
		rotate(m, frame)
		tris = pipeline.AppendMesh(tris[:0], m, &p)
		frame++
	}
}

func BenchmarkFrame(b *testing.B) {
	for _, name := range []string{"textured", "filled", "textured-near", "filled-near"} {
		mode, near := strings.CutSuffix(name, "-near")
		b.Run(name, func(b *testing.B) {
			m, p := loadScene(b)
			if near {
				moveCloser(&p, 3)
			}
			target, err := raster.NewTarget(width, height)
			if err != nil {
				b.Fatal(err)
			}
			var tris []raster.Triangle
			frame := 0
			for b.Loop() {
				rotate(m, frame)
				tris = pipeline.AppendMesh(tris[:0], m, &p)
				target.Clear(0xFF000000)
				for i := range tris {
					if mode == "textured" {
						target.TextureTriangle(&tris[i])
					} else {
						target.FillTriangle(&tris[i])
					}
				}
				frame++
			}
		})
	}
}
