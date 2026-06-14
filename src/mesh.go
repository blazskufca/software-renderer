package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
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
		{a: 1, b: 2, c: 3, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 1, b: 3, c: 4, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
		// right
		{a: 4, b: 3, c: 5, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 4, b: 5, c: 6, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
		// back
		{a: 6, b: 5, c: 7, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 6, b: 7, c: 8, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
		// left
		{a: 8, b: 7, c: 2, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 8, b: 2, c: 1, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
		// top
		{a: 2, b: 7, c: 5, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 2, b: 5, c: 3, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
		// bottom
		{a: 6, b: 8, c: 1, AUV: tex2{0, 0}, BUV: tex2{0, 1}, CUV: tex2{1, 1}, color: 0xFFFFFFFF},
		{a: 6, b: 1, c: 4, AUV: tex2{0, 0}, BUV: tex2{1, 1}, CUV: tex2{1, 0}, color: 0xFFFFFFFF},
	}
	currentMesh mesh
)

// Each mesh has a collection of vertices and collection of faces and a vec3 rotation vector
// We seperate vertices from faces, becasue multiple faces can share the same vertices...
type mesh struct {
	vertices    []vec3
	faces       []face
	rotation    vec3 // rotation is also called Euler angle
	scale       vec3
	translation vec3
}

func newMesh(vertices []vec3, faces []face, rotation, scale, translation vec3) mesh {
	if scale == (vec3{}) {
		scale = vec3{1.0, 1.0, 1.0}
	}

	return mesh{vertices, faces, rotation, scale, translation}
}

func newCubeMesh() mesh {
	return newMesh(cubeMeshVertices[:], cubeMeshFaces[:], vec3{}, vec3{}, vec3{})
}

func newMeshFromFromFile(path string) (mesh, error) {
	var v []vec3
	var fa []face

	f, err := os.Open(path)
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
			xStr := strings.Split(fields[1], "/")[0]
			yStr := strings.Split(fields[2], "/")[0]
			zStr := strings.Split(fields[3], "/")[0]

			xInt, err := strconv.Atoi(xStr)
			if err != nil {
				return mesh{}, err
			}
			yInt, err := strconv.Atoi(yStr)
			if err != nil {
				return mesh{}, err
			}
			zInt, err := strconv.Atoi(zStr)
			if err != nil {
				return mesh{}, err
			}

			fa = append(fa, face{xInt, yInt, zInt, tex2{}, tex2{}, tex2{}, 0xFFFFFFFF})

		default:
			log.Printf("Not implemented parsing line type %q", fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return mesh{}, err
	}
	return newMesh(v, fa, vec3{}, vec3{}, vec3{}), nil
}
