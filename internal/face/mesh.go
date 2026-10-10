package mesh

import (
	"bufio"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/blazskufca/software-renderer/internal/assets"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

const (
	NCubeFaces = 6 * 2
)

// Face stores vertex indexes which form that specific face
type Face[U utils.Numeric] struct {
	A       int
	B       int
	C       int
	AUV     texture.Tex2[U]
	BUV     texture.Tex2[U]
	CUV     texture.Tex2[U]
	ANormal vector.Vec3[U] // Per-vertex normals, used for Gouraud shading
	BNormal vector.Vec3[U]
	CNormal vector.Vec3[U]
	Color   uint32
}

var (
	CubeMeshVertices = [NCubeFaces]vector.Vec3[float32]{
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
	CubeMeshFaces = [NCubeFaces]Face[float32]{
		// front
		{A: 0, B: 1, C: 2, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{0, 1}, CUV: texture.Tex2[float32]{1, 1}, Color: 0xFFFFFFFF},
		{A: 0, B: 2, C: 3, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{1, 1}, CUV: texture.Tex2[float32]{1, 0}, Color: 0xFFFFFFFF},
		// right
		{A: 3, B: 2, C: 4, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{0, 1}, CUV: texture.Tex2[float32]{1, 1}, Color: 0xFFFFFFFF},
		{A: 3, B: 4, C: 5, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{1, 1}, CUV: texture.Tex2[float32]{1, 0}, Color: 0xFFFFFFFF},
		// back
		{A: 5, B: 4, C: 6, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{0, 1}, CUV: texture.Tex2[float32]{1, 1}, Color: 0xFFFFFFFF},
		{A: 5, B: 6, C: 7, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{1, 1}, CUV: texture.Tex2[float32]{1, 0}, Color: 0xFFFFFFFF},
		// left
		{A: 7, B: 6, C: 1, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{0, 1}, CUV: texture.Tex2[float32]{1, 1}, Color: 0xFFFFFFFF},
		{A: 7, B: 1, C: 0, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{1, 1}, CUV: texture.Tex2[float32]{1, 0}, Color: 0xFFFFFFFF},
		// top
		{A: 1, B: 6, C: 4, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{0, 1}, CUV: texture.Tex2[float32]{1, 1}, Color: 0xFFFFFFFF},
		{A: 1, B: 4, C: 2, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{1, 1}, CUV: texture.Tex2[float32]{1, 0}, Color: 0xFFFFFFFF},
		// bottom
		{A: 5, B: 7, C: 0, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{0, 1}, CUV: texture.Tex2[float32]{1, 1}, Color: 0xFFFFFFFF},
		{A: 5, B: 0, C: 3, AUV: texture.Tex2[float32]{0, 0}, BUV: texture.Tex2[float32]{1, 1}, CUV: texture.Tex2[float32]{1, 0}, Color: 0xFFFFFFFF},
	}
)

// TextureData is a uint32 array of pixels with height and width
type TextureData struct {
	PixelData []uint32
	Width     int
	Height    int
}

// Mesh has a collection of vertices and collection of faces and a vec3 rotation vector
// We seperate vertices from faces, becasue multiple faces can share the same vertices...
type Mesh[T utils.Numeric] struct {
	Vertices          []vector.Vec3[T]
	Faces             []Face[T]
	Texture           TextureData
	Rotation          vector.Vec3[T] // Rotation is also called Euler angle
	Scale             vector.Vec3[T]
	Translation       vector.Vec3[T]
	PerFrameMutations []Opts[T]
}

// MirrorHorizontally mirrors the mesh across the X axis in model space. Some
// assets are UV-unwrapped with the opposite horizontal handedness to their
// texture, so the texture renders mirrored left-to-right.
func (m *Mesh[T]) MirrorHorizontally() {
	for i := range m.Vertices {
		m.Vertices[i].X = -m.Vertices[i].X
	}
	for i := range m.Faces {
		m.Faces[i].B, m.Faces[i].C = m.Faces[i].C, m.Faces[i].B
		m.Faces[i].BUV, m.Faces[i].CUV = m.Faces[i].CUV, m.Faces[i].BUV
		m.Faces[i].BNormal, m.Faces[i].CNormal = m.Faces[i].CNormal, m.Faces[i].BNormal
		m.Faces[i].ANormal.X = -m.Faces[i].ANormal.X
		m.Faces[i].BNormal.X = -m.Faces[i].BNormal.X
		m.Faces[i].CNormal.X = -m.Faces[i].CNormal.X
	}
}

// NewMesh creates a new Mesh
func NewMesh[T utils.Numeric](vertices []vector.Vec3[T], faces []Face[T], rotation, scale, translation vector.Vec3[T]) Mesh[T] {
	if scale == (vector.Vec3[T]{}) {
		scale = vector.Vec3[T]{X: 1.0, Y: 1.0, Z: 1.0}
	}

	return Mesh[T]{Vertices: vertices, Faces: faces, Rotation: rotation, Scale: scale, Translation: translation}
}

// NewCubeMesh creates a new cube Mesh from CubeMeshFaces and CubeMeshVertices.
func NewCubeMesh() Mesh[float32] {
	return NewMesh[float32](CubeMeshVertices[:], CubeMeshFaces[:], vector.Vec3[float32]{}, vector.Vec3[float32]{}, vector.Vec3[float32]{})
}

// Opts specifics an option function on Mesh type.
type Opts[T utils.Numeric] func(mesh *Mesh[T]) error

// NewMeshFromFile creates a new Mesh object from WaveFront .obj file
func NewMeshFromFile[T utils.Numeric](path string, Options ...Opts[T]) (Mesh[T], error) {
	var v []vector.Vec3[T]
	var fa []Face[T]
	var vt []texture.Tex2[T]
	var vn []vector.Vec3[T]

	f, err := assets.OpenAsset(path)
	if err != nil {
		return Mesh[T]{}, err
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
				return Mesh[T]{}, fmt.Errorf("malformed vertex: got %d coordinates", len(fields)-1)
			}

			parsedX, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return Mesh[T]{}, err
			}
			parsedY, err := strconv.ParseFloat(fields[2], 64)
			if err != nil {
				return Mesh[T]{}, err
			}
			parsedZ, err := strconv.ParseFloat(fields[3], 64)
			if err != nil {
				return Mesh[T]{}, err
			}

			v = append(v, vector.Vec3[T]{X: T(parsedX), Y: T(parsedY), Z: T(parsedZ)})

		case "f":
			if len(fields) < 4 {
				return Mesh[T]{}, fmt.Errorf("malformed face: expected at least 3 vertices")
			}
			n := len(fields) - 1
			vIdx := make([]int, n)
			vtIdx := make([]int, n)
			vnIdx := make([]int, n)
			hasNormals := true
			for i := range n {
				parts := strings.Split(fields[i+1], "/")
				if len(parts) < 2 {
					return Mesh[T]{}, fmt.Errorf("malformed face vertex %q: expected v/vt[/vn]", fields[i+1])
				}
				vIdx[i], err = resolveIndex(parts[0], len(v), "vertex")
				if err != nil {
					return Mesh[T]{}, err
				}
				vtIdx[i], err = resolveIndex(parts[1], len(vt), "texture vertex")
				if err != nil {
					return Mesh[T]{}, err
				}
				if len(parts) >= 3 && parts[2] != "" {
					vnIdx[i], err = resolveIndex(parts[2], len(vn), "vertex normal")
					if err != nil {
						return Mesh[T]{}, err
					}
				} else {
					hasNormals = false
				}
			}
			for i := 1; i < n-1; i++ {
				f := Face[T]{
					A:     vIdx[0],
					B:     vIdx[i],
					C:     vIdx[i+1],
					AUV:   vt[vtIdx[0]],
					BUV:   vt[vtIdx[i]],
					CUV:   vt[vtIdx[i+1]],
					Color: 0xFFFFFFFF,
				}
				if hasNormals {
					f.ANormal = vn[vnIdx[0]]
					f.BNormal = vn[vnIdx[i]]
					f.CNormal = vn[vnIdx[i+1]]
				}
				fa = append(fa, f)
			}
		case "vt":
			if n := len(fields) - 1; n < 2 || n > 3 {
				return Mesh[T]{}, fmt.Errorf("malformed texture vertex: got %d coordinates", n)
			}
			parsedU, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return Mesh[T]{}, err
			}
			parsedV, err := strconv.ParseFloat(fields[2], 64)
			if err != nil {
				return Mesh[T]{}, err
			}
			vt = append(vt, texture.Tex2[T]{U: T(parsedU), V: T(parsedV)})

		case "vn":
			if len(fields) < 4 {
				return Mesh[T]{}, fmt.Errorf("malformed vertex normal: got %d coordinates", len(fields)-1)
			}
			parsedX, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return Mesh[T]{}, err
			}
			parsedY, err := strconv.ParseFloat(fields[2], 64)
			if err != nil {
				return Mesh[T]{}, err
			}
			parsedZ, err := strconv.ParseFloat(fields[3], 64)
			if err != nil {
				return Mesh[T]{}, err
			}
			vn = append(vn, vector.Vec3[T]{X: T(parsedX), Y: T(parsedY), Z: T(parsedZ)})

		default:
			log.Printf("Not implemented parsing line type %q", fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return Mesh[T]{}, err
	}

	cMesh := NewMesh(v, fa, vector.Vec3[T]{}, vector.Vec3[T]{}, vector.Vec3[T]{})
	for _, opt := range Options {
		if err := opt(&cMesh); err != nil {
			return Mesh[T]{}, err
		}
	}
	return cMesh, nil
}

// resolveIndex converts a 1-based OBJ index, or a negative index counted back
// from the last of the count elements defined so far, into a 0-based index.
func resolveIndex(field string, count int, kind string) (int, error) {
	idx, err := strconv.Atoi(field)
	if err != nil {
		return 0, err
	}
	resolved := idx - 1
	if idx < 0 {
		resolved = count + idx
	}
	if idx == 0 || resolved < 0 || resolved >= count {
		return 0, fmt.Errorf("face %s index %d out of range (%d defined)", kind, idx, count)
	}
	return resolved, nil
}
