//go:build simd

// Opt-in geometry (-tags simd): each vertex is transformed, frustum-tested and
// projected once, and faces are culled and lit in blocks. With GOEXPERIMENT=simd
// too, both stages use the vector kernels in geometry_simd.go.

package pipeline

import (
	"math"
	"slices"

	"github.com/blazskufca/software-renderer/internal/clipping"
	"github.com/blazskufca/software-renderer/internal/config"
	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/light"
	"github.com/blazskufca/software-renderer/internal/matrix"
	"github.com/blazskufca/software-renderer/internal/raster"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

type Params[T utils.Numeric] struct {
	View, Projection matrix.Mat4[T]
	Frustum          [6]clipping.Plane[T]
	Light            light.Light[T]
	Shading          config.ShadingMode
	Cull             config.CullMode
	Width, Height    int

	// Scratch reused across calls: keep the Params between frames, and don't
	// use copies of one from several goroutines at once.
	verts vertexData[T]
	faces *faceData[T]
}

// scalarOnly disables the SIMD paths, for tests.
var scalarOnly bool

// vertexData holds per-vertex results as structure of arrays, so the SIMD
// path can load and store a vector's worth of vertices at once.
type vertexData[T utils.Numeric] struct {
	x, y, z    []T       // view space
	sx, sy, sw []float32 // screen pixels, W = view depth
	inside     []int32   // -1 if strictly inside every frustum plane, else 0
	mx, my, mz []float32 // model space input, SIMD path only
	consts     []float32 // broadcast constants of the running SIMD kernel
}

func (v *vertexData[T]) resize(n int) {
	for _, s := range []*[]T{&v.x, &v.y, &v.z} {
		*s = slices.Grow((*s)[:0], n)[:n]
	}
	for _, s := range []*[]float32{&v.sx, &v.sy, &v.sw} {
		*s = slices.Grow((*s)[:0], n)[:n]
	}
	v.inside = slices.Grow(v.inside[:0], n)[:n]
}

func (v *vertexData[T]) view(i int) vector.Vec3[T] {
	return vector.Vec3[T]{X: v.x[i], Y: v.y[i], Z: v.z[i]}
}

type screenVertex struct{ X, Y, W float32 }

func (v *vertexData[T]) screen(i int) screenVertex {
	return screenVertex{X: v.sx[i], Y: v.sy[i], W: v.sw[i]}
}

// faceBlock is how many faces are culled and lit before their triangles are
// written, so the SIMD path can work on whole vectors of faces.
const faceBlock = 128

// faceData holds cull and lighting results for one block of faces.
type faceData[T utils.Numeric] struct {
	keep   [faceBlock]int32       // -1 if the face survives culling, else 0
	light  [3][faceBlock]T        // corner intensities
	gather [18][faceBlock]float32 // corners and vertex normals, SIMD path only
}

func AppendMesh[T utils.Numeric](dst []raster.Triangle, m *mesh.Mesh[T], p *Params[T]) []raster.Triangle {
	world := matrix.Mat4Identity[T]()
	world = matrix.Mat4MakeScale[T](m.Scale.X, m.Scale.Y, m.Scale.Z).MulMat4(world)
	world = matrix.Mat4MakeRotationZ[T](m.Rotation.Z).MulMat4(world)
	world = matrix.Mat4MakeRotationY[T](m.Rotation.Y).MulMat4(world)
	world = matrix.Mat4MakeRotationX[T](m.Rotation.X).MulMat4(world)
	world = matrix.Mat4MakeTranslation[T](m.Translation.X, m.Translation.Y, m.Translation.Z).MulMat4(world)
	world = p.View.MulMat4(world)

	halfW, halfH := T(p.Width/2), T(p.Height/2)

	// Faces share vertices, so transform, frustum-test and project each vertex
	// once here instead of once per face corner.
	vd := &p.verts
	vd.resize(len(m.Vertices))
	done := transformVerticesSIMD(vd, m.Vertices, &world, p)
	for i := done; i < len(m.Vertices); i++ {
		v := world.MulPoint(m.Vertices[i])
		vd.x[i], vd.y[i], vd.z[i] = v.X, v.Y, v.Z
		vd.inside[i] = 0
		if clipping.Inside(v, &p.Frustum) {
			vd.inside[i] = -1
		}
		s := project(&p.Projection, v, halfW, halfH)
		vd.sx[i], vd.sy[i], vd.sw[i] = s.X, s.Y, s.W
	}

	if p.faces == nil {
		p.faces = new(faceData[T])
	}
	fd := p.faces
	for start := 0; start < len(m.Faces); start += faceBlock {
		faces := m.Faces[start:min(start+faceBlock, len(m.Faces))]
		for i := shadeFacesSIMD(fd, faces, vd, &world, p); i < len(faces); i++ {
			shadeFace(fd, i, &faces[i], vd, &world, p)
		}

		for i := range faces {
			if fd.keep[i] == 0 {
				continue
			}
			face := &faces[i]
			intensities := [3]T{fd.light[0][i], fd.light[1][i], fd.light[2][i]}

			if vd.inside[face.A]&vd.inside[face.B]&vd.inside[face.C] != 0 {
				dst = append(dst, raster.Triangle{})
				tri := &dst[len(dst)-1]
				tri.Color, tri.Texture = face.Color, &m.Texture
				setVertex(&tri.V[0], vd.screen(face.A), face.AUV, intensities[0])
				setVertex(&tri.V[1], vd.screen(face.B), face.BUV, intensities[1])
				setVertex(&tri.V[2], vd.screen(face.C), face.CUV, intensities[2])
				continue
			}

			corners := [3]vector.Vec4[T]{vd.view(face.A).ToVec4(), vd.view(face.B).ToVec4(), vd.view(face.C).ToVec4()}
			poly := clipping.NewPolyFromTriangle(&corners, &[3]texture.Tex2[T]{face.AUV, face.BUV, face.CUV}, &intensities)
			poly.ClipPolygon(&p.Frustum)

			for t := range poly.NumberOfVertices - 2 {
				dst = append(dst, raster.Triangle{})
				tri := &dst[len(dst)-1]
				tri.Color, tri.Texture = face.Color, &m.Texture
				for k, vi := range [3]int{0, t + 1, t + 2} {
					s := project(&p.Projection, poly.Vertices[vi], halfW, halfH)
					setVertex(&tri.V[k], s, poly.TexCoords[vi], poly.LightIntensities[vi])
				}
			}
		}
	}
	return dst
}

// project maps a view-space point to screen pixels, keeping view depth in W.
func project[T utils.Numeric](proj *matrix.Mat4[T], v vector.Vec3[T], halfW, halfH T) screenVertex {
	pp := proj.Mat4MulVec4Project(v.ToVec4())
	pp.X *= halfW
	pp.Y *= halfH
	pp.Y *= -1
	pp.X += halfW
	pp.Y += halfH
	return screenVertex{X: float32(pp.X), Y: float32(pp.Y), W: float32(pp.W)}
}

// setVertex writes v field by field. Building a raster.Vertex (or Triangle)
// and copying it in reads the value back with wider loads than the stores
// that built it, which stalls store-to-load forwarding; for the same reason
// triangles are appended zeroed and filled in place.
func setVertex[T utils.Numeric](v *raster.Vertex, s screenVertex, uv texture.Tex2[T], light T) {
	v.X, v.Y, v.W = s.X, s.Y, s.W
	v.U, v.V, v.Light = float32(uv.U), float32(uv.V), float32(light)
}

// shadeFace culls face i of the block and computes its corner intensities.
// shadeFaces32 must stay in step with it.
func shadeFace[T utils.Numeric](fd *faceData[T], i int, face *mesh.Face[T], vd *vertexData[T], world *matrix.Mat4[T], p *Params[T]) {
	a, b, c := vd.view(face.A), vd.view(face.B), vd.view(face.C)

	// Edge lengths change neither the sign of the cull test nor the unit
	// normal, so the edges stay unnormalized.
	normal := b.Sub(a).Cross(c.Sub(a))
	if p.Cull == config.CULL_BACKFACE && normal.Dot(vector.Vec3[T]{}.Sub(a)) < 0 {
		fd.keep[i] = 0
		return
	}
	fd.keep[i] = -1

	if p.Shading != config.SHADING_GOURAUD {
		flat := flatLight(normal, p.Light.Direction)
		fd.light[0][i], fd.light[1][i], fd.light[2][i] = flat, flat, flat
		return
	}
	for j, vn := range [3]vector.Vec3[T]{face.ANormal, face.BNormal, face.CNormal} {
		tn := world.MulDirection(vn)
		if l := length(tn); l != 0 {
			fd.light[j][i] = -tn.Dot(p.Light.Direction) / l
		} else {
			fd.light[j][i] = flatLight(normal, p.Light.Direction)
		}
	}
}

// flatLight is the intensity for the face normal n, which may be unnormalized.
func flatLight[T utils.Numeric](n, lightDir vector.Vec3[T]) T {
	l := length(n)
	n = vector.Vec3[T]{X: n.X / l, Y: n.Y / l, Z: n.Z / l}
	return -n.Dot(lightDir)
}

// length sums the squares in T rather than float64 (unlike Vec3.Length), so
// for float32 it matches the SIMD path's vector square root bit for bit.
func length[T utils.Numeric](v vector.Vec3[T]) T {
	return T(math.Sqrt(float64(v.Dot(v))))
}
