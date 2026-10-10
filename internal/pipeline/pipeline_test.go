package pipeline_test

import (
	"math"
	"math/rand/v2"
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

const zNear, zFar = 0.1, 100

// viewSpaceParams renders with an identity view, so mesh coordinates are view space.
func viewSpaceParams() pipeline.Params[float32] {
	fovY := math.Pi / 3
	fovX := 2 * math.Atan(math.Tan(fovY/2)*float64(width)/float64(height))
	p := pipeline.Params[float32]{
		View:       matrix.Mat4Identity[float32](),
		Projection: matrix.Mat4MakePerspective[float32, float32](float32(fovY), float32(height)/float32(width), zNear, zFar),
		Light:      light.Light[float32]{Direction: vector.Vec3[float32]{Z: 1}},
		Shading:    config.SHADING_FLAT,
		Cull:       config.CULL_BACKFACE,
		Width:      width,
		Height:     height,
	}
	clipping.InitFrustumPlanes[float32, float32, float32](&p.Frustum, float32(fovX), float32(fovY), zNear, zFar)
	return p
}

// triangleMesh builds a one-face mesh with UVs (0,0),(1,0),(0,1) and optional A/B/C vertex normals.
func triangleMesh(a, b, c vector.Vec3[float32], normals ...vector.Vec3[float32]) *mesh.Mesh[float32] {
	face := mesh.Face[float32]{
		A: 0, B: 1, C: 2,
		AUV:   texture.Tex2[float32]{U: 0, V: 0},
		BUV:   texture.Tex2[float32]{U: 1, V: 0},
		CUV:   texture.Tex2[float32]{U: 0, V: 1},
		Color: 0xFFFFFFFF,
	}
	if len(normals) == 3 {
		face.ANormal, face.BNormal, face.CNormal = normals[0], normals[1], normals[2]
	}
	m := mesh.NewMesh([]vector.Vec3[float32]{a, b, c}, []mesh.Face[float32]{face},
		vector.Vec3[float32]{}, vector.Vec3[float32]{}, vector.Vec3[float32]{})
	return &m
}

// screenArea is the screen-space cross product (B−A)×(C−A).
func screenArea(t *raster.Triangle) float32 {
	return (t.V[1].X-t.V[0].X)*(t.V[2].Y-t.V[0].Y) - (t.V[1].Y-t.V[0].Y)*(t.V[2].X-t.V[0].X)
}

var (
	frontTri = [3]vector.Vec3[float32]{{X: 0, Y: 0, Z: 5}, {X: 1, Y: 1, Z: 5}, {X: 1, Y: 0, Z: 5}}
	backTri  = [3]vector.Vec3[float32]{{X: 0, Y: 0, Z: 5}, {X: 1, Y: 0, Z: 5}, {X: 1, Y: 1, Z: 5}}
)

func TestAppendMeshCulling(t *testing.T) {
	cases := []struct {
		name  string
		tri   [3]vector.Vec3[float32]
		front bool
		cull  config.CullMode
		want  int
	}{
		{name: "front backface", tri: frontTri, front: true, cull: config.CULL_BACKFACE, want: 1},
		{name: "back backface", tri: backTri, front: false, cull: config.CULL_BACKFACE, want: 0},
		{name: "front none", tri: frontTri, front: true, cull: config.CULL_NONE, want: 1},
		{name: "back none", tri: backTri, front: false, cull: config.CULL_NONE, want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := viewSpaceParams()
			p.Cull = tc.cull
			tris := pipeline.AppendMesh(nil, triangleMesh(tc.tri[0], tc.tri[1], tc.tri[2]), &p)
			if len(tris) != tc.want {
				t.Fatalf("got %d triangles, want %d", len(tris), tc.want)
			}
			if tc.front {
				for i := range tris {
					if a := screenArea(&tris[i]); a <= 0 {
						t.Fatalf("front triangle %d has screen area %v, want > 0", i, a)
					}
				}
			}
		})
	}
}

func TestAppendMeshProjection(t *testing.T) {
	p := viewSpaceParams()
	tris := pipeline.AppendMesh(nil, triangleMesh(frontTri[0], frontTri[1], frontTri[2]), &p)
	if len(tris) != 1 {
		t.Fatalf("got %d triangles, want 1", len(tris))
	}
	uvs := [3][2]float32{{0, 0}, {1, 0}, {0, 1}}
	tanHalf := math.Tan(math.Pi / 6)
	w, h := float64(width), float64(height)
	for k, in := range frontTri {
		x, y, z := float64(in.X), float64(in.Y), float64(in.Z)
		wantX := (1 + (h/w)*x/(z*tanHalf)) * w / 2
		wantY := (1 - y/(z*tanHalf)) * h / 2
		v := tris[0].V[k]
		if math.Abs(float64(v.X)-wantX) > 1e-3 || math.Abs(float64(v.Y)-wantY) > 1e-3 || math.Abs(float64(v.W)-z) > 1e-3 {
			t.Fatalf("vertex %d: got (%v,%v,W=%v), want (%v,%v,W=%v)", k, v.X, v.Y, v.W, wantX, wantY, z)
		}
		if v.U != uvs[k][0] || v.V != uvs[k][1] || v.Light != 1 {
			t.Fatalf("vertex %d: got uv (%v,%v) light %v, want (%v,%v) light 1", k, v.U, v.V, v.Light, uvs[k][0], uvs[k][1])
		}
	}
}

func TestAppendMeshClipsToFrustum(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	p := viewSpaceParams()
	p.Cull = config.CULL_NONE
	r := func(lo, hi float32) float32 { return lo + rng.Float32()*(hi-lo) }
	const eps = 1e-4
	split := false
	var tris []raster.Triangle
	for i := range 20000 {
		var v [3]vector.Vec3[float32]
		for k := range v {
			v[k] = vector.Vec3[float32]{X: r(-200, 200), Y: r(-200, 200), Z: r(-50, 150)}
		}
		tris = pipeline.AppendMesh(tris[:0], triangleMesh(v[0], v[1], v[2]), &p)
		if len(tris) >= 2 {
			split = true
		}
		for j := range tris {
			for k, vx := range tris[j].V {
				if vx.X < -0.5 || vx.X > width+0.5 || vx.Y < -0.5 || vx.Y > height+0.5 {
					t.Fatalf("input %d tri %d vertex %d off screen: (%v,%v)", i, j, k, vx.X, vx.Y)
				}
				if vx.W < zNear*(1-1e-3) || vx.W > zFar*(1+1e-4) {
					t.Fatalf("input %d tri %d vertex %d W=%v outside [near, far]", i, j, k, vx.W)
				}
				if vx.U < -eps || vx.V < -eps || vx.U+vx.V > 1+eps {
					t.Fatalf("input %d tri %d vertex %d UV (%v,%v) not a convex combination", i, j, k, vx.U, vx.V)
				}
			}
		}
	}
	if !split {
		t.Fatal("no input was clipped into 2 or more triangles")
	}
}

func TestAppendMeshShading(t *testing.T) {
	a := vector.Vec3[float32]{X: 0, Y: 0, Z: 5}
	b := vector.Vec3[float32]{X: 1, Y: 1, Z: 6}
	c := vector.Vec3[float32]{X: 1, Y: 0, Z: 5}
	flat := float32(1 / math.Sqrt2)
	cases := []struct {
		name    string
		shading config.ShadingMode
		normals []vector.Vec3[float32]
		want    [3]float32
	}{
		{name: "flat", shading: config.SHADING_FLAT, want: [3]float32{flat, flat, flat}},
		{name: "gouraud zero normals", shading: config.SHADING_GOURAUD, normals: make([]vector.Vec3[float32], 3), want: [3]float32{flat, flat, flat}},
		{
			name:    "gouraud vertex normals",
			shading: config.SHADING_GOURAUD,
			normals: []vector.Vec3[float32]{{X: 0, Y: 0, Z: -2}, {X: 1, Y: 0, Z: 0}, {X: 0, Y: 0.6, Z: -0.8}},
			want:    [3]float32{1, 0, 0.8},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := viewSpaceParams()
			p.Shading = tc.shading
			tris := pipeline.AppendMesh(nil, triangleMesh(a, b, c, tc.normals...), &p)
			if len(tris) != 1 {
				t.Fatalf("got %d triangles, want 1", len(tris))
			}
			for k := range 3 {
				if got := tris[0].V[k].Light; math.Abs(float64(got-tc.want[k])) > 1e-5 {
					t.Fatalf("vertex %d light %v, want %v", k, got, tc.want[k])
				}
			}
		})
	}
}

func TestViewMatrix(t *testing.T) {
	const tol = 1e-5
	near := func(got vector.Vec4[float32], want vector.Vec3[float32]) bool {
		return math.Abs(float64(got.X-want.X)) <= tol && math.Abs(float64(got.Y-want.Y)) <= tol && math.Abs(float64(got.Z-want.Z)) <= tol
	}
	eye := vector.Vec3[float32]{X: 1, Y: 2, Z: 3}
	for _, yp := range [][2]float32{{0, 0}, {0.71, 0.31}, {-2, 1.2}} {
		view, dir := pipeline.ViewMatrix(eye, yp[0], yp[1])
		if l := dir.Length(); math.Abs(float64(l-1)) > tol {
			t.Fatalf("yaw %v pitch %v: |dir| = %v, want 1", yp[0], yp[1], l)
		}
		if got := view.MulVec4(eye.ToVec4()); !near(got, vector.Vec3[float32]{}) {
			t.Fatalf("yaw %v pitch %v: eye maps to %+v, want origin", yp[0], yp[1], got)
		}
		if got := view.MulVec4(eye.Add(dir).ToVec4()); !near(got, vector.Vec3[float32]{Z: 1}) {
			t.Fatalf("yaw %v pitch %v: eye+dir maps to %+v, want (0,0,1)", yp[0], yp[1], got)
		}
	}
	_, dir := pipeline.ViewMatrix(eye, math.Pi/2, 0)
	if !near(dir.ToVec4(), vector.Vec3[float32]{X: 1}) {
		t.Fatalf("yaw π/2: dir = %+v, want (1,0,0)", dir)
	}
}
