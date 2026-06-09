package main

const (
	nCubeFaces = 6 * 2
)

var (
	cubeMeshVertices = [cubeNMeshVertices]vec3{
		{X: -1, Y: -1, Z: -1}, // 1
		{X: -1, Y: 1, Z: -1},  // 2
		{X: 1, Y: 1, Z: -1},   // 3
		{X: 1, Y: -1, Z: -1},  // 4
		{X: 1, Y: 1, Z: 1},    // 5
		{X: 1, Y: -1, Z: 1},   // 6
		{X: -1, Y: 1, Z: 1},   // 7
		{X: -1, Y: -1, Z: 1},  // 8
	}
	// Faces index into vertex array
	// The order of the indexes matters...Clockwise orientation meas the face is facing outwards and opposite means its facing inwards
	cubeMeshFaces = [nCubeFaces]face{
		// front
		{a: 1, b: 2, c: 3},
		{a: 1, b: 3, c: 4},
		// right
		{a: 4, b: 3, c: 5},
		{a: 4, b: 5, c: 6},
		// back
		{a: 6, b: 5, c: 7},
		{a: 6, b: 7, c: 8},
		// left
		{a: 8, b: 7, c: 2},
		{a: 8, b: 2, c: 1},
		// top
		{a: 2, b: 7, c: 5},
		{a: 2, b: 5, c: 3},
		// bottom
		{a: 6, b: 8, c: 1},
		{a: 6, b: 1, c: 4},
	}
	currentMesh mesh
)

// Each mesh has a collection of vertices and collection of faces and a vec3 rotation vector
// We seperate vertices from faces, becasue multiple faces can share the same vertices...
type mesh struct {
	vertices []vec3
	faces    []face
	rotation vec3 // rotation is also called Euler angle
}

func newMesh(vertices []vec3, faces []face, rotation vec3) mesh {
	return mesh{vertices, faces, rotation}
}

func newCubeMesh() mesh {
	return newMesh(cubeMeshVertices[:], cubeMeshFaces[:], vec3{})
}
