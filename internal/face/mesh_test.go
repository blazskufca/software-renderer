//go:build !js

package mesh_test

import (
	"errors"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/vector"
)

const validOBJ = `# comment
o Quad
v 0 0 0
v 1 0 0
v 1 1 0
v 0 1 0
vt 0 0
vt 1 0
vt 1 1
vt 0 1
vn 0 0 1
s off
f 1/1/1 2/2/1 3/3/1 4/4/1
f -4/-4 -3/-3 -2/-2
`

const tri = "v 0 0 0\nv 1 0 0\nv 0 1 0\nvt 0 0\n"

// malformedOBJ lists inputs that must be rejected with an error. Each case
// defines valid preceding elements so only the named defect is under test.
var malformedOBJ = []struct{ name, src string }{
	{name: "vertex with two coordinates", src: "v 1 2\n"},
	{name: "vertex non-numeric", src: "v a 0 0\n"},
	{name: "normal with two coordinates", src: "vn 0 1\n"},
	{name: "texture vertex with one coordinate", src: "vt 0.5\n"},
	{name: "texture vertex with four coordinates", src: "vt 0 0 0 0\n"},
	{name: "face with two vertices", src: tri + "f 1/1 2/1\n"},
	{name: "face without texture index", src: tri + "f 1 2 3\n"},
	{name: "face with empty texture index", src: tri + "vn 0 0 1\nf 1//1 2//1 3//1\n"},
	{name: "vertex index out of range", src: tri + "f 1/1 2/1 9/1\n"},
	{name: "texture index out of range", src: tri + "f 1/1 2/5 3/1\n"},
	{name: "normal index out of range", src: tri + "f 1/1/3 2/1/3 3/1/3\n"},
	{name: "zero index", src: tri + "f 0/1 1/1 2/1\n"},
	{name: "relative index past start", src: tri + "f -9/1 1/1 2/1\n"},
	{name: "forward reference", src: "f 1/1 2/1 3/1\n" + tri},
}

func writeOBJ(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "m.obj")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func quietLog(t testing.TB) {
	t.Helper()
	prev := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(prev) })
}

func TestNewMeshFromFileParsesFaces(t *testing.T) {
	quietLog(t)
	m, err := mesh.NewMeshFromFile[float32](writeOBJ(t, validOBJ))
	if err != nil {
		t.Fatalf("NewMeshFromFile: %v", err)
	}

	wantV := []vector.Vec3[float32]{{X: 0, Y: 0, Z: 0}, {X: 1, Y: 0, Z: 0}, {X: 1, Y: 1, Z: 0}, {X: 0, Y: 1, Z: 0}}
	if !slices.Equal(m.Vertices, wantV) {
		t.Fatalf("Vertices = %v, want %v", m.Vertices, wantV)
	}

	uv := func(u, v float32) texture.Tex2[float32] { return texture.Tex2[float32]{U: u, V: v} }
	n := vector.Vec3[float32]{X: 0, Y: 0, Z: 1}
	white := uint32(0xFFFFFFFF)
	want := []mesh.Face[float32]{
		{A: 0, B: 1, C: 2, AUV: uv(0, 0), BUV: uv(1, 0), CUV: uv(1, 1), ANormal: n, BNormal: n, CNormal: n, Color: white},
		{A: 0, B: 2, C: 3, AUV: uv(0, 0), BUV: uv(1, 1), CUV: uv(0, 1), ANormal: n, BNormal: n, CNormal: n, Color: white},
		{A: 0, B: 1, C: 2, AUV: uv(0, 0), BUV: uv(1, 0), CUV: uv(1, 1), Color: white},
	}
	if len(m.Faces) != len(want) {
		t.Fatalf("got %d faces, want %d", len(m.Faces), len(want))
	}
	for i := range want {
		if m.Faces[i] != want[i] {
			t.Fatalf("face %d = %+v, want %+v", i, m.Faces[i], want[i])
		}
	}
	if want := (vector.Vec3[float32]{X: 1, Y: 1, Z: 1}); m.Scale != want {
		t.Fatalf("Scale = %v, want %v", m.Scale, want)
	}
}

func TestNewMeshFromFileOptions(t *testing.T) {
	quietLog(t)
	path := writeOBJ(t, validOBJ)

	t.Run("applied in order", func(t *testing.T) {
		m, err := mesh.NewMeshFromFile[float32](path,
			func(m *mesh.Mesh[float32]) error { m.Translation.Z = 5; return nil },
			func(m *mesh.Mesh[float32]) error {
				if m.Translation.Z != 5 {
					t.Fatalf("second option saw Translation.Z = %v, want 5", m.Translation.Z)
				}
				return nil
			},
		)
		if err != nil {
			t.Fatalf("NewMeshFromFile: %v", err)
		}
		if m.Translation.Z != 5 {
			t.Fatalf("Translation.Z = %v, want 5", m.Translation.Z)
		}
	})

	t.Run("error propagates", func(t *testing.T) {
		errOpt := errors.New("option failed")
		_, err := mesh.NewMeshFromFile[float32](path, func(*mesh.Mesh[float32]) error { return errOpt })
		if !errors.Is(err, errOpt) {
			t.Fatalf("err = %v, want %v", err, errOpt)
		}
	})
}

func TestNewMeshFromFileRejectsMalformedInput(t *testing.T) {
	quietLog(t)
	for _, tc := range malformedOBJ {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := mesh.NewMeshFromFile[float32](writeOBJ(t, tc.src)); err == nil {
				t.Fatalf("no error for %q", tc.src)
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "does-not-exist.obj")
		if _, err := mesh.NewMeshFromFile[float32](path); err == nil {
			t.Fatalf("no error for missing file %s", path)
		}
	})
}

func FuzzNewMeshFromFile(f *testing.F) {
	f.Add(validOBJ)
	for _, tc := range malformedOBJ {
		f.Add(tc.src)
	}
	quietLog(f)
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, src string) {
		path := filepath.Join(dir, "fuzz.obj")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
		m, err := mesh.NewMeshFromFile[float32](path)
		if err != nil {
			return
		}
		nv := len(m.Vertices)
		for i, fc := range m.Faces {
			for _, idx := range []int{fc.A, fc.B, fc.C} {
				if idx < 0 || idx >= nv {
					t.Fatalf("face %d index %d out of range [0,%d)", i, idx, nv)
				}
			}
		}
	})
}

func faceNormal(v []vector.Vec3[float32], f mesh.Face[float32]) vector.Vec3[float32] {
	a := v[f.A]
	return v[f.B].Sub(a).Cross(v[f.C].Sub(a))
}

func TestCubeMeshFacesPointOutward(t *testing.T) {
	m := mesh.NewCubeMesh()
	for i, f := range m.Faces {
		n := faceNormal(m.Vertices, f)
		centroid := m.Vertices[f.A].Add(m.Vertices[f.B]).Add(m.Vertices[f.C])
		if d := n.Dot(centroid); d <= 0 {
			t.Fatalf("face %d (%d,%d,%d): normal %v points inward (n·centroid = %v)", i, f.A, f.B, f.C, n, d)
		}
	}
}

type vertUV struct {
	idx int
	uv  texture.Tex2[float32]
}

func vertUVs(f mesh.Face[float32]) []vertUV {
	s := []vertUV{{idx: f.A, uv: f.AUV}, {idx: f.B, uv: f.BUV}, {idx: f.C, uv: f.CUV}}
	slices.SortFunc(s, func(a, b vertUV) int { return a.idx - b.idx })
	return s
}

func TestMirrorHorizontally(t *testing.T) {
	// Clone the package-level arrays: mirroring NewCubeMesh() would mutate them.
	m := mesh.NewMesh(slices.Clone(mesh.CubeMeshVertices[:]), slices.Clone(mesh.CubeMeshFaces[:]),
		vector.Vec3[float32]{}, vector.Vec3[float32]{}, vector.Vec3[float32]{})
	for i := range m.Faces {
		n := faceNormal(m.Vertices, m.Faces[i])
		m.Faces[i].ANormal, m.Faces[i].BNormal, m.Faces[i].CNormal = n, n, n
	}
	origV := slices.Clone(m.Vertices)
	origF := slices.Clone(m.Faces)

	m.MirrorHorizontally()

	near := func(a, b vector.Vec3[float32]) bool {
		return math.Abs(float64(a.X-b.X)) <= 1e-6 && math.Abs(float64(a.Y-b.Y)) <= 1e-6 && math.Abs(float64(a.Z-b.Z)) <= 1e-6
	}
	for i, f := range m.Faces {
		want := faceNormal(origV, origF[i])
		want.X = -want.X
		if got := faceNormal(m.Vertices, f); !near(got, want) {
			t.Fatalf("face %d: mirrored geometric normal %v, want %v", i, got, want)
		}
		for k, vn := range []vector.Vec3[float32]{f.ANormal, f.BNormal, f.CNormal} {
			if !near(vn, want) {
				t.Fatalf("face %d: vertex normal %d = %v, want %v", i, k, vn, want)
			}
		}
		if got, wantUV := vertUVs(f), vertUVs(origF[i]); !slices.Equal(got, wantUV) {
			t.Fatalf("face %d: (vertex, UV) pairs %v, want %v", i, got, wantUV)
		}
	}

	m.MirrorHorizontally()
	if !slices.Equal(m.Vertices, origV) {
		t.Fatalf("double mirror Vertices = %v, want %v", m.Vertices, origV)
	}
	if !slices.Equal(m.Faces, origF) {
		t.Fatalf("double mirror Faces = %v, want %v", m.Faces, origF)
	}
}

func TestBundledAssetsParse(t *testing.T) {
	quietLog(t)
	for _, path := range []string{"../../assets/race.obj", "../../assets/sedan_fan_tri.obj"} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			m, err := mesh.NewMeshFromFile[float32](path)
			if err != nil {
				t.Fatalf("NewMeshFromFile(%s): %v", path, err)
			}
			if len(m.Faces) == 0 {
				t.Fatalf("%s: no faces", path)
			}
			nv := len(m.Vertices)
			var zero vector.Vec3[float32]
			for i, f := range m.Faces {
				for _, idx := range []int{f.A, f.B, f.C} {
					if idx < 0 || idx >= nv {
						t.Fatalf("face %d index %d out of range [0,%d)", i, idx, nv)
					}
				}
				for k, n := range []vector.Vec3[float32]{f.ANormal, f.BNormal, f.CNormal} {
					if n == zero {
						t.Fatalf("face %d vertex normal %d is zero", i, k)
					}
				}
			}
		})
	}
}
