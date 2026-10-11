package clipping

import (
	"math"

	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

const (
	leftFrustumPlane = iota
	rightFrustumPlane
	topFrustumPlane
	bottomFrustumPlane
	nearFrustumPlane
	farFrustumPlane
	maxPolygonVertices              = 10
	maximumNumberOfPolygonTriangles = 10
)

// Plane represents a mathematical plane dividing a space in two parts
type Plane[T utils.Numeric] struct {
	point  vector.Vec3[T]
	normal vector.Vec3[T]
}

func (p *Plane[T]) Point() vector.Vec3[T]  { return p.point }
func (p *Plane[T]) Normal() vector.Vec3[T] { return p.normal }

// InitFrustumPlanes inits all 6 frustum planes with correct Point and normal
// ///////////////////////////////////////////////////////////////////////////
// Frustum planes are defined by a point and a normal vector
// ///////////////////////////////////////////////////////////////////////////
// Near plane   : P=(0, 0, znear), N=(0, 0,  1)
// Far plane    : P=(0, 0, zfar),  N=(0, 0, -1)
// Top plane    : P=(0, 0, 0),     N=(0, -cos(fov/2), sin(fov/2))
// Bottom plane : P=(0, 0, 0),     N=(0, cos(fov/2), sin(fov/2))
// Left plane   : P=(0, 0, 0),     N=(cos(fov/2), 0, sin(fov/2))
// Right plane  : P=(0, 0, 0),     N=(-cos(fov/2), 0, sin(fov/2))
// ///////////////////////////////////////////////////////////////////////////
func InitFrustumPlanes[T utils.Numeric, U utils.Numeric, K utils.Numeric](frustumPlanes *[6]Plane[T], fovx, fovy U, Znear, Zfar K) {
	cosHalfFovY := math.Cos(float64(fovy) / 2)
	sinHalfFovY := math.Sin(float64(fovy) / 2)
	cosHalfFovX := math.Cos(float64(fovx) / 2)
	sinHalfFovX := math.Sin(float64(fovx) / 2)

	frustumPlanes[leftFrustumPlane] = Plane[T]{
		normal: vector.Vec3[T]{X: T(cosHalfFovX), Z: T(sinHalfFovX)},
	}

	frustumPlanes[rightFrustumPlane] = Plane[T]{
		normal: vector.Vec3[T]{T(-cosHalfFovX), 0, T(sinHalfFovX)},
	}

	frustumPlanes[topFrustumPlane] = Plane[T]{
		normal: vector.Vec3[T]{0, T(-cosHalfFovY), T(sinHalfFovY)},
	}

	frustumPlanes[bottomFrustumPlane] = Plane[T]{
		normal: vector.Vec3[T]{0, T(cosHalfFovY), T(sinHalfFovY)},
	}

	frustumPlanes[nearFrustumPlane] = Plane[T]{
		point:  vector.Vec3[T]{Z: T(Znear)},
		normal: vector.Vec3[T]{Z: 1},
	}

	frustumPlanes[farFrustumPlane] = Plane[T]{
		point:  vector.Vec3[T]{Z: T(Zfar)},
		normal: vector.Vec3[T]{Z: T(-1)},
	}

}

// Polygon represents a n-sided polygon (maximum n=10)
type Polygon[T utils.Numeric] struct {
	Vertices         [maxPolygonVertices]vector.Vec3[T]
	TexCoords        [maxPolygonVertices]texture.Tex2[T]
	LightIntensities [maxPolygonVertices]T
	NumberOfVertices int
}

// NewPolyFromTriangle constructs a new Polygon[T] from a triangle's vertices, UV coordinates and per-vertex light intensities.
func NewPolyFromTriangle[T utils.Numeric](vertices *[3]vector.Vec4[T], uvs *[3]texture.Tex2[T], intensities *[3]T) Polygon[T] {
	p := Polygon[T]{}
	for i, vert := range vertices {
		p.Vertices[i] = vert.ToVec3()
		p.NumberOfVertices++
	}
	for i, u := range uvs {
		p.TexCoords[i] = u
	}
	for i, in := range intensities {
		p.LightIntensities[i] = in
	}
	return p
}

// ClipPolygon clips the polygon against all six planes with Sutherland–Hodgman algorithm
func (p *Polygon[T]) ClipPolygon(frustumPlanes *[6]Plane[T]) {
	p.clipPolygonAgainstPlane(frustumPlanes, leftFrustumPlane)
	p.clipPolygonAgainstPlane(frustumPlanes, rightFrustumPlane)
	p.clipPolygonAgainstPlane(frustumPlanes, topFrustumPlane)
	p.clipPolygonAgainstPlane(frustumPlanes, bottomFrustumPlane)
	p.clipPolygonAgainstPlane(frustumPlanes, nearFrustumPlane)
	p.clipPolygonAgainstPlane(frustumPlanes, farFrustumPlane)
}

// InsideAll reports whether every vertex is strictly inside all six planes, in
// which case ClipPolygon would leave the polygon unchanged.
func (p *Polygon[T]) InsideAll(planes *[6]Plane[T]) bool {
	for v := range p.NumberOfVertices {
		for i := range planes {
			d := p.Vertices[v].Sub(planes[i].point).Dot(planes[i].normal)
			if !(d > 0) {
				return false
			}
		}
	}
	return true
}

// Inside reports whether v is strictly inside all six planes. Each product is
// rounded on its own (no fused multiply-add), so it agrees bit for bit with the
// SIMD frustum test in internal/pipeline on every target.
func Inside[T utils.Numeric](v vector.Vec3[T], planes *[6]Plane[T]) bool {
	for i := range planes {
		p, n := planes[i].point, planes[i].normal
		d := T((v.X-p.X)*n.X) + T((v.Y-p.Y)*n.Y) + T((v.Z-p.Z)*n.Z)
		if !(d > 0) {
			return false
		}
	}
	return true
}

// clipPolygonAgainstPlane clips the polygon against plane according to Sutherland–Hodgman algorithm
// https://en.wikipedia.org/wiki/Sutherland%E2%80%93Hodgman_algorithm
func (p *Polygon[T]) clipPolygonAgainstPlane(frustumPlanes *[6]Plane[T], plane int) {
	if p.NumberOfVertices < 3 {
		return
	}
	planePoint := frustumPlanes[plane].point
	planeNormal := frustumPlanes[plane].normal

	var insideVertices = [maxPolygonVertices]vector.Vec3[T]{}
	var insideTextureCoordinates = [maxPolygonVertices]texture.Tex2[T]{}
	var insideLightIntensities = [maxPolygonVertices]T{}
	var numInsideVertices int

	previousVertex := p.Vertices[p.NumberOfVertices-1]
	previousTextureCoord := p.TexCoords[p.NumberOfVertices-1]
	previousLightIntensity := p.LightIntensities[p.NumberOfVertices-1]

	PB := previousVertex.Sub(planePoint)
	previousDot := PB.Dot(planeNormal)

	for i := 0; i < p.NumberOfVertices; i++ {
		PA := p.Vertices[i].Sub(planePoint) // PA is a vector between point P on plane and a current vertex
		currentTextCord := p.TexCoords[i]
		currentLightIntensity := p.LightIntensities[i]
		currentDot := PA.Dot(planeNormal) // Current dot product tells us if a vertex is inside, outside or on the plane

		// Product is only negative if there is a change in sign between the dot products
		// I.e we went inside -> outside or outside -> inside
		if (currentDot * previousDot) < 0 {
			// Always interpolate from the inside vertex towards the outside one.
			// Two triangles that share an edge walk it in opposite directions,
			// and interpolating in walking order would round their intersection
			// points differently, which opens cracks along the shared edge.
			inVertex, outVertex := previousVertex, p.Vertices[i]
			inTextCord, outTextCord := previousTextureCoord, currentTextCord
			inLightIntensity, outLightIntensity := previousLightIntensity, currentLightIntensity
			inDot, outDot := previousDot, currentDot
			if currentDot > 0 {
				inVertex, outVertex = outVertex, inVertex
				inTextCord, outTextCord = outTextCord, inTextCord
				inLightIntensity, outLightIntensity = outLightIntensity, inLightIntensity
				inDot, outDot = outDot, inDot
			}
			//Calculate the factor T for linear interpolation equation -
			// t = Q1 / (Q1 - Q2)
			factorT := inDot / (inDot - outDot)
			// Interpolation equation - I = Q1 + t(Q2-Q1)
			intersectionPoint := vector.Vec3[T]{
				X: utils.Lerp[T, T](inVertex.X, outVertex.X, factorT),
				Y: utils.Lerp[T, T](inVertex.Y, outVertex.Y, factorT),
				Z: utils.Lerp[T, T](inVertex.Z, outVertex.Z, factorT),
			}

			// We also do the linear interpolation on UV texure coordinates
			interpolatedTextCord := texture.Tex2[T]{
				U: utils.Lerp[T, T](inTextCord.U, outTextCord.U, factorT),
				V: utils.Lerp[T, T](inTextCord.V, outTextCord.V, factorT),
			}
			interpolatedLightIntensity := utils.Lerp[T, T](inLightIntensity, outLightIntensity, factorT)
			insideVertices[numInsideVertices] = intersectionPoint
			insideTextureCoordinates[numInsideVertices] = interpolatedTextCord
			insideLightIntensities[numInsideVertices] = interpolatedLightIntensity
			numInsideVertices++
		}

		// If the current vertex is inside the plane or on the plane!
		if currentDot > 0 {
			insideVertices[numInsideVertices] = p.Vertices[i]
			insideTextureCoordinates[numInsideVertices] = currentTextCord
			insideLightIntensities[numInsideVertices] = currentLightIntensity
			numInsideVertices++
		}
		// Move along the polygon so a current vertex and dot become previous vertex and dot product
		previousDot = currentDot
		previousVertex = p.Vertices[i]
		previousTextureCoord = currentTextCord
		previousLightIntensity = currentLightIntensity
	}
	p.Vertices = insideVertices
	p.NumberOfVertices = numInsideVertices
	p.TexCoords = insideTextureCoordinates
	p.LightIntensities = insideLightIntensities
}
