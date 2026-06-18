package main

import (
	"math"
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

var (
	frustumPlanes = [6]plane{}
)

type plane struct {
	point  vec3
	normal vec3
}

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
func initFrustumPlanes(fovx, fovy, Znear, Zfar float64) {
	cosHalfFovY := math.Cos(fovy / 2)
	sinHalfFovY := math.Sin(fovy / 2)
	cosHalfFovX := math.Cos(fovx / 2)
	sinHalfFovX := math.Sin(fovx / 2)

	frustumPlanes[leftFrustumPlane] = plane{
		normal: vec3{cosHalfFovX, 0, sinHalfFovX},
	}

	frustumPlanes[rightFrustumPlane] = plane{
		normal: vec3{-cosHalfFovX, 0, sinHalfFovX},
	}

	frustumPlanes[topFrustumPlane] = plane{
		normal: vec3{0, -cosHalfFovY, sinHalfFovY},
	}

	frustumPlanes[bottomFrustumPlane] = plane{
		normal: vec3{0, cosHalfFovY, sinHalfFovY},
	}

	frustumPlanes[nearFrustumPlane] = plane{
		point:  vec3{Z: Znear},
		normal: vec3{Z: 1},
	}

	frustumPlanes[farFrustumPlane] = plane{
		point:  vec3{Z: Zfar},
		normal: vec3{Z: -1},
	}

}

type polygon struct {
	vertices         [maxPolygonVertices]vec3
	texCoords        [maxPolygonVertices]tex2
	numberOfVertices int
}

func newPolyFromTriangle(vertices *[3]vec4, UVs ...tex2) polygon {
	p := polygon{}
	for i, vert := range vertices {
		p.vertices[i] = vec3FromVec4(vert)
		p.numberOfVertices++
	}
	for i, u := range UVs {
		p.texCoords[i] = u
	}
	return p
}

// clipPolygon clips the polygon against all six planes with Sutherland–Hodgman algorithm
func (p *polygon) clipPolygon() {
	p.clipPolygonAgainstPlane(leftFrustumPlane)
	p.clipPolygonAgainstPlane(rightFrustumPlane)
	p.clipPolygonAgainstPlane(topFrustumPlane)
	p.clipPolygonAgainstPlane(bottomFrustumPlane)
	p.clipPolygonAgainstPlane(nearFrustumPlane)
	p.clipPolygonAgainstPlane(farFrustumPlane)
}

// clipPolygonAgainstPlane clips the polygon against plane according to Sutherland–Hodgman algorithm
// https://en.wikipedia.org/wiki/Sutherland%E2%80%93Hodgman_algorithm
func (p *polygon) clipPolygonAgainstPlane(plane int) {
	if p.numberOfVertices < 3 {
		return
	}
	planePoint := frustumPlanes[plane].point
	planeNormal := frustumPlanes[plane].normal

	var insideVertices = [maxPolygonVertices]vec3{}
	var insideTextureCoordinates = [maxPolygonVertices]tex2{}
	var numInsideVertices int

	previousVertex := p.vertices[p.numberOfVertices-1]
	previousTextureCoord := p.texCoords[p.numberOfVertices-1]

	PB := previousVertex.sub(planePoint)
	previousDot := PB.dot(planeNormal)

	for i := 0; i < p.numberOfVertices; i++ {
		PA := p.vertices[i].sub(planePoint) // PA is a vector between point P on plane and a current vertex
		currentTextCord := p.texCoords[i]
		currentDot := PA.dot(planeNormal) // Current dot product tells us if a vertex is inside, outside or on the plane

		// Product is only negative if there is a change in sign between the dot products
		// I.e we went inside -> outside or outside -> inside
		if (currentDot * previousDot) < 0 {
			//Calculate the factor T for linear interpolation equation -
			// t = Q1 / (Q2 - Q1)
			factorT := previousDot / (previousDot - currentDot)
			// Interpolation equation - I = Q1 + t(Q2-Q1)
			// But Q1 and Q2 are vectors
			// Instead of doing all of this we can use LERP here too
			//intersectionPoint := p.vertices[i]
			//intersectionPoint = intersectionPoint.sub(previousVertex) // Q1 - Q1
			//intersectionPoint = intersectionPoint.multiply(factorT)   // t(Q1- Q2)
			//intersectionPoint = intersectionPoint.add(previousVertex) // Q1 + t(Q1 - Q2)
			intersectionPoint := vec3{
				X: lerp[float64, float64](previousVertex.X, p.vertices[i].X, factorT),
				Y: lerp[float64, float64](previousVertex.Y, p.vertices[i].Y, factorT),
				Z: lerp[float64, float64](previousVertex.Z, p.vertices[i].Z, factorT),
			}

			// We also do the linear interpolation on UV texure coordinates
			interpolatedTextCord := tex2{
				u: lerp[float64, float64](previousTextureCoord.u, currentTextCord.u, factorT),
				v: lerp[float64, float64](previousTextureCoord.v, currentTextCord.v, factorT),
			}
			insideVertices[numInsideVertices] = intersectionPoint
			insideTextureCoordinates[numInsideVertices] = interpolatedTextCord
			numInsideVertices++
		}

		// If the current vertex is inside the plane or on the plane!
		if currentDot > 0 {
			insideVertices[numInsideVertices] = p.vertices[i]
			insideTextureCoordinates[numInsideVertices] = currentTextCord
			numInsideVertices++
		}
		// Move along the polygon so a current vertex and dot become previous vertex and dot product
		previousDot = currentDot
		previousVertex = p.vertices[i]
		previousTextureCoord = currentTextCord
	}
	p.vertices = insideVertices
	p.numberOfVertices = numInsideVertices
	p.texCoords = insideTextureCoordinates
}

// lerp calculates linear interpolation between U and V with factor T according to formula I = u + factor(v-u)
func lerp[T, U ~int | ~float64 | ~float32](u, v, factor T) U {
	return U(u + factor*(v-u))
}
