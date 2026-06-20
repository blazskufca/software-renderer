package main

import (
	"bufio"
	"fmt"
	"log"
	"strconv"
	"strings"
)

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
		{a: 0, b: 1, c: 2, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 0, b: 2, c: 3, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
		// right
		{a: 3, b: 2, c: 4, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 3, b: 4, c: 5, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
		// back
		{a: 5, b: 4, c: 6, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 5, b: 6, c: 7, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
		// left
		{a: 7, b: 6, c: 1, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 7, b: 1, c: 0, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
		// top
		{a: 1, b: 6, c: 4, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 1, b: 4, c: 2, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
		// bottom
		{a: 5, b: 7, c: 0, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 5, b: 0, c: 3, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
	}
)

type MeshTextureData struct {
	pixelData []uint32
	width     int
	height    int
}

// Each mesh has a collection of vertices and collection of faces and a vec3 rotation vector
// We seperate vertices from faces, becasue multiple faces can share the same vertices...
type mesh struct {
	vertices          []vec3
	faces             []face
	texture           MeshTextureData
	rotation          vec3 // rotation is also called Euler angle
	scale             vec3
	translation       vec3
	perFrameMutations []Opts
}

// mirrorHorizontally mirrors the mesh across the X axis in model space. Some
// assets are UV-unwrapped with the opposite horizontal handedness to their
// texture, so the texture renders mirrored left-to-right.
func (m *mesh) mirrorHorizontally() {
	for i := range m.vertices {
		m.vertices[i].X = -m.vertices[i].X
	}
	for i := range m.faces {
		m.faces[i].b, m.faces[i].c = m.faces[i].c, m.faces[i].b
		m.faces[i].BUV, m.faces[i].CUV = m.faces[i].CUV, m.faces[i].BUV
	}
}

func newMesh(vertices []vec3, faces []face, rotation, scale, translation vec3) mesh {
	if scale == (vec3{}) {
		scale = vec3{1.0, 1.0, 1.0}
	}

	return mesh{vertices: vertices, faces: faces, rotation: rotation, scale: scale, translation: translation}
}

func newCubeMesh() mesh {
	return newMesh(cubeMeshVertices[:], cubeMeshFaces[:], vec3{}, vec3{}, vec3{})
}

type Opts func(*mesh) error

func newMeshFromFromFile(path string, Options ...Opts) (mesh, error) {
	var v []vec3
	var fa []face
	var vt []tex2

	f, err := openAsset(path)
	if err != nil {
		return mesh{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		switch fields[0] {
		case "v":
			if len(fields) < 4 {
				return mesh{}, fmt.Errorf("malformed vertex: got %d coordinates", len(fields)-1)
			}

			parsedX, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return mesh{}, err
			}
			parsedY, err := strconv.ParseFloat(fields[2], 64)
			if err != nil {
				return mesh{}, err
			}
			parsedZ, err := strconv.ParseFloat(fields[3], 64)
			if err != nil {
				return mesh{}, err
			}

			v = append(v, vec3{parsedX, parsedY, parsedZ})

		case "f":
			if len(fields) < 4 {
				return mesh{}, fmt.Errorf("malformed face: expected at least 3 vertices")
			}
			n := len(fields) - 1
			vIdx := make([]int, n)
			vtIdx := make([]int, n)
			for i := 0; i < n; i++ {
				parts := strings.Split(fields[i+1], "/")
				if len(parts) < 2 {
					return mesh{}, fmt.Errorf("malformed face vertex %q: expected v/vt[/vn]", fields[i+1])
				}
				vi, err := strconv.Atoi(parts[0])
				if err != nil {
					return mesh{}, err
				}
				vti, err := strconv.Atoi(parts[1])
				if err != nil {
					return mesh{}, err
				}
				vIdx[i] = vi - 1
				vtIdx[i] = vti - 1
			}
			for i := 1; i < n-1; i++ {
				fa = append(fa, face{
					vIdx[0], vIdx[i], vIdx[i+1],
					vt[vtIdx[0]], vt[vtIdx[i]], vt[vtIdx[i+1]],
					0xFFFFFFFF,
				})
			}
		case "vt":
			if len(fields[1:]) > 3 {
				return mesh{}, fmt.Errorf("malformed texture vertex: got %d coordinates", len(fields[1:]))
			}
			parsedU, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return mesh{}, err
			}
			parsedV, err := strconv.ParseFloat(fields[2], 64)
			if err != nil {
				return mesh{}, err
			}
			vt = append(vt, tex2{parsedU, parsedV})

		default:
			log.Printf("Not implemented parsing line type %q", fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return mesh{}, err
	}

	cMesh := newMesh(v, fa, vec3{}, vec3{}, vec3{})
	for _, opt := range Options {
		if err := opt(&cMesh); err != nil {
			return mesh{}, err
		}
	}
	return cMesh, nil
}
