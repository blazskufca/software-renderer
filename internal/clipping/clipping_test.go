package clipping

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

const (
	testFovY = math.Pi / 3
	testNear = 0.1
	testFar  = 100
)

var testFovX = 2 * math.Atan(math.Tan(testFovY/2)*960/480)

func frustum[T utils.Numeric]() *[6]Plane[T] {
	var planes [6]Plane[T]
	InitFrustumPlanes(&planes, testFovX, testFovY, testNear, testFar)
	return &planes
}

func pointPoly(p vector.Vec3[float64]) Polygon[float64] {
	v := vector.Vec4[float64]{X: p.X, Y: p.Y, Z: p.Z, W: 1}
	verts := [3]vector.Vec4[float64]{v, v, v}
	var uvs [3]texture.Tex2[float64]
	var lights [3]float64
	return NewPolyFromTriangle(&verts, &uvs, &lights)
}

func TestFrustumBoundaries(t *testing.T) {
	planes := frustum[float64]()
	xe := 10 * math.Tan(testFovX/2)
	ye := 10 * math.Tan(testFovY/2)
	cases := []struct {
		name   string
		p      vector.Vec3[float64]
		inside bool
	}{
		{name: "center", p: vector.Vec3[float64]{Z: 50}, inside: true},
		{name: "just past near", p: vector.Vec3[float64]{Z: 0.2}, inside: true},
		{name: "just before far", p: vector.Vec3[float64]{Z: 99}, inside: true},
		{name: "before near", p: vector.Vec3[float64]{Z: 0.05}, inside: false},
		{name: "past far", p: vector.Vec3[float64]{Z: 150}, inside: false},
		{name: "behind camera", p: vector.Vec3[float64]{Z: -1}, inside: false},
		{name: "right inside", p: vector.Vec3[float64]{X: 0.99 * xe, Z: 10}, inside: true},
		{name: "left inside", p: vector.Vec3[float64]{X: -0.99 * xe, Z: 10}, inside: true},
		{name: "right outside", p: vector.Vec3[float64]{X: 1.01 * xe, Z: 10}, inside: false},
		{name: "left outside", p: vector.Vec3[float64]{X: -1.01 * xe, Z: 10}, inside: false},
		{name: "top inside", p: vector.Vec3[float64]{Y: 0.99 * ye, Z: 10}, inside: true},
		{name: "bottom inside", p: vector.Vec3[float64]{Y: -0.99 * ye, Z: 10}, inside: true},
		{name: "top outside", p: vector.Vec3[float64]{Y: 1.01 * ye, Z: 10}, inside: false},
		{name: "bottom outside", p: vector.Vec3[float64]{Y: -1.01 * ye, Z: 10}, inside: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			poly := pointPoly(tc.p)
			if got := poly.InsideAll(planes); got != tc.inside {
				t.Fatalf("InsideAll(%v) = %v, want %v", tc.p, got, tc.inside)
			}
		})
	}
}

type clipVertex struct {
	p     vector.Vec3[float64]
	uv    texture.Tex2[float64]
	light float64
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func nearVec(a, b vector.Vec3[float64], tol float64) bool {
	return near(a.X, b.X, tol) && near(a.Y, b.Y, tol) && near(a.Z, b.Z, tol)
}

func TestClipPolygonExamples(t *testing.T) {
	planes := frustum[float64]()
	cases := []struct {
		name string
		in   [3]clipVertex
		want []clipVertex
	}{
		{
			name: "far plane cuts two edges",
			in: [3]clipVertex{
				{p: vector.Vec3[float64]{Z: 50}, uv: texture.Tex2[float64]{U: 0, V: 0}, light: 1},
				{p: vector.Vec3[float64]{X: -1, Z: 150}, uv: texture.Tex2[float64]{U: 1, V: 0}, light: 0},
				{p: vector.Vec3[float64]{X: 1, Z: 150}, uv: texture.Tex2[float64]{U: 0, V: 1}, light: 0.5},
			},
			want: []clipVertex{
				{p: vector.Vec3[float64]{Z: 50}, uv: texture.Tex2[float64]{U: 0, V: 0}, light: 1},
				{p: vector.Vec3[float64]{X: -0.5, Z: 100}, uv: texture.Tex2[float64]{U: 0.5, V: 0}, light: 0.5},
				{p: vector.Vec3[float64]{X: 0.5, Z: 100}, uv: texture.Tex2[float64]{U: 0, V: 0.5}, light: 0.75},
			},
		},
		{
			name: "behind camera",
			in: [3]clipVertex{
				{p: vector.Vec3[float64]{Z: -5}},
				{p: vector.Vec3[float64]{X: 1, Z: -5}},
				{p: vector.Vec3[float64]{Y: 1, Z: -5}},
			},
			want: nil,
		},
	}
	const tol = 1e-9
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var verts [3]vector.Vec4[float64]
			var uvs [3]texture.Tex2[float64]
			var lights [3]float64
			for k, v := range tc.in {
				verts[k] = vector.Vec4[float64]{X: v.p.X, Y: v.p.Y, Z: v.p.Z, W: 1}
				uvs[k] = v.uv
				lights[k] = v.light
			}
			poly := NewPolyFromTriangle(&verts, &uvs, &lights)
			poly.ClipPolygon(planes)
			if poly.NumberOfVertices != len(tc.want) {
				t.Fatalf("NumberOfVertices = %d, want %d", poly.NumberOfVertices, len(tc.want))
			}
			used := make([]bool, poly.NumberOfVertices)
			for wi, w := range tc.want {
				found := false
				for i := range poly.NumberOfVertices {
					if used[i] || !nearVec(poly.Vertices[i], w.p, tol) {
						continue
					}
					used[i], found = true, true
					uv, l := poly.TexCoords[i], poly.LightIntensities[i]
					if !near(uv.U, w.uv.U, tol) || !near(uv.V, w.uv.V, tol) || !near(l, w.light, tol) {
						t.Fatalf("vertex %v: uv=%v light=%v, want uv=%v light=%v", w.p, uv, l, w.uv, w.light)
					}
					break
				}
				if !found {
					t.Fatalf("expected vertex %d at %v not found in %v", wi, w.p, poly.Vertices[:poly.NumberOfVertices])
				}
			}
		})
	}
}

type affine struct {
	c vector.Vec3[float64]
	d float64
}

func (f affine) at(p vector.Vec3[float64]) float64 { return f.c.Dot(p) + f.d }

func randAffine(rng *rand.Rand) affine {
	r := func() float64 { return rng.Float64()*2 - 1 }
	return affine{c: vector.Vec3[float64]{X: r(), Y: r(), Z: r()}, d: r()}
}

func randomTriangle(rng *rand.Rand, xy, zMin, zMax float64) [3]vector.Vec4[float64] {
	var verts [3]vector.Vec4[float64]
	for k := range verts {
		verts[k] = vector.Vec4[float64]{
			X: (rng.Float64()*2 - 1) * xy,
			Y: (rng.Float64()*2 - 1) * xy,
			Z: zMin + rng.Float64()*(zMax-zMin),
			W: 1,
		}
	}
	return verts
}

func TestClipPolygonProperties(t *testing.T) {
	planes := frustum[float64]()
	rng := rand.New(rand.NewPCG(1, 2))
	const tol = 1e-9
	for i := range 20000 {
		verts := randomTriangle(rng, 150, -50, 150)
		fU, fV, fL := randAffine(rng), randAffine(rng), randAffine(rng)
		var uvs [3]texture.Tex2[float64]
		var lights [3]float64
		for k, v := range verts {
			p := v.ToVec3()
			uvs[k] = texture.Tex2[float64]{U: fU.at(p), V: fV.at(p)}
			lights[k] = fL.at(p)
		}
		poly := NewPolyFromTriangle(&verts, &uvs, &lights)
		poly.ClipPolygon(planes)
		if poly.NumberOfVertices > 9 {
			t.Fatalf("triangle %d: %d vertices after clipping, want <= 9", i, poly.NumberOfVertices)
		}
		for j := range poly.NumberOfVertices {
			p := poly.Vertices[j]
			for pi, pl := range planes {
				if d := p.Sub(pl.point).Dot(pl.normal); d < -tol {
					t.Fatalf("triangle %d vertex %d %v: distance %g to plane %d", i, j, p, d, pi)
				}
			}
			uv, l := poly.TexCoords[j], poly.LightIntensities[j]
			if e := math.Abs(uv.U - fU.at(p)); e > tol {
				t.Fatalf("triangle %d vertex %d %v: U error %g", i, j, p, e)
			}
			if e := math.Abs(uv.V - fV.at(p)); e > tol {
				t.Fatalf("triangle %d vertex %d %v: V error %g", i, j, p, e)
			}
			if e := math.Abs(l - fL.at(p)); e > tol {
				t.Fatalf("triangle %d vertex %d %v: light error %g", i, j, p, e)
			}
		}
	}
}

func TestInsideAllMeansClipIsNoop(t *testing.T) {
	planes := frustum[float64]()
	rng := rand.New(rand.NewPCG(3, 4))
	inside := 0
	for i := range 20000 {
		verts := randomTriangle(rng, 20, 0, 120)
		uvs := [3]texture.Tex2[float64]{{U: rng.Float64(), V: rng.Float64()}, {U: rng.Float64(), V: rng.Float64()}, {U: rng.Float64(), V: rng.Float64()}}
		lights := [3]float64{rng.Float64(), rng.Float64(), rng.Float64()}
		poly := NewPolyFromTriangle(&verts, &uvs, &lights)
		if !poly.InsideAll(planes) {
			continue
		}
		inside++
		before := poly
		poly.ClipPolygon(planes)
		if poly != before {
			t.Fatalf("triangle %d: ClipPolygon changed an InsideAll polygon: before %+v, after %+v", i, before, poly)
		}
	}
	if inside < 1000 {
		t.Fatalf("only %d triangles were inside the frustum, want >= 1000", inside)
	}
}

// Two triangles that share an edge walk it in opposite directions. Both have to
// clip it to the very same points, bit for bit, or the rasterizer leaves cracks
// along the edge.
func TestClipPolygonIgnoresWinding(t *testing.T) {
	planes := frustum[float32]()
	rng := rand.New(rand.NewPCG(3, 4))
	var uvs [3]texture.Tex2[float32]
	var lights [3]float32
	for range 2000 {
		tri := randomTriangle(rng, 40, -5, 120)
		var forward, backward [3]vector.Vec4[float32]
		for i, v := range tri {
			forward[i] = vector.Vec4[float32]{X: float32(v.X), Y: float32(v.Y), Z: float32(v.Z), W: 1}
			backward[2-i] = forward[i]
		}
		a := NewPolyFromTriangle(&forward, &uvs, &lights)
		b := NewPolyFromTriangle(&backward, &uvs, &lights)
		a.ClipPolygon(planes)
		b.ClipPolygon(planes)
		if a.NumberOfVertices != b.NumberOfVertices {
			t.Fatalf("%v: clipped to %d vertices, reversed to %d", tri, a.NumberOfVertices, b.NumberOfVertices)
		}
		for _, v := range a.Vertices[:a.NumberOfVertices] {
			if !slices.Contains(b.Vertices[:b.NumberOfVertices], v) {
				t.Fatalf("%v: vertex %v is missing when the triangle is reversed", tri, v)
			}
		}
	}
}
