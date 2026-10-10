package pipeline

import (
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
}

func ViewMatrix[T utils.Numeric](position vector.Vec3[T], yaw, pitch float32) (matrix.Mat4[T], vector.Vec3[T]) {
	rotation := matrix.Mat4MakeRotationY[T](T(yaw)).MulMat4(matrix.Mat4MakeRotationX[T](T(pitch)))
	direction := rotation.MulVec4(vector.Vec3[T]{Z: 1}.ToVec4()).ToVec3()
	return matrix.Mat4LookAt(position, position.Add(direction), vector.Vec3[T]{Y: 1}), direction
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

	for i := range m.Faces {
		face := &m.Faces[i]
		var transformed [3]vector.Vec4[T]
		transformed[0] = world.MulVec4(m.Vertices[face.A].ToVec4())
		transformed[1] = world.MulVec4(m.Vertices[face.B].ToVec4())
		transformed[2] = world.MulVec4(m.Vertices[face.C].ToVec4())
		a, b, c := transformed[0].ToVec3(), transformed[1].ToVec3(), transformed[2].ToVec3()

		ba, ca := b.Sub(a), c.Sub(a)
		ba.Normalize()
		ca.Normalize()
		normal := ba.Cross(ca)
		normal.Normalize()

		if p.Cull == config.CULL_BACKFACE && normal.Dot(vector.Vec3[T]{}.Sub(a)) < 0 {
			continue
		}

		flat := -normal.Dot(p.Light.Direction)
		intensities := [3]T{flat, flat, flat}
		if p.Shading == config.SHADING_GOURAUD {
			vertexNormals := [3]vector.Vec3[T]{face.ANormal, face.BNormal, face.CNormal}
			for j, n := range vertexNormals {
				tn := world.MulVec4(vector.Vec4[T]{X: n.X, Y: n.Y, Z: n.Z}).ToVec3()
				if tn.Length() == 0 {
					tn = normal
				} else {
					tn.Normalize()
				}
				intensities[j] = -tn.Dot(p.Light.Direction)
			}
		}

		poly := clipping.NewPolyFromTriangle(&transformed, &[3]texture.Tex2[T]{face.AUV, face.BUV, face.CUV}, &intensities)
		if !poly.InsideAll(&p.Frustum) {
			poly.ClipPolygon(&p.Frustum)
		}

		for t := range poly.NumberOfVertices - 2 {
			tri := raster.Triangle{Color: face.Color, Texture: &m.Texture}
			for k, vi := range [3]int{0, t + 1, t + 2} {
				pp := p.Projection.Mat4MulVec4Project(poly.Vertices[vi].ToVec4())
				pp.X *= halfW
				pp.Y *= halfH
				pp.Y *= -1
				pp.X += halfW
				pp.Y += halfH
				tri.V[k] = raster.Vertex{
					X:     float32(pp.X),
					Y:     float32(pp.Y),
					W:     float32(pp.W),
					U:     float32(poly.TexCoords[vi].U),
					V:     float32(poly.TexCoords[vi].V),
					Light: float32(poly.LightIntensities[vi]),
				}
			}
			dst = append(dst, tri)
		}
	}
	return dst
}
