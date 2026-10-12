package main

import (
	"math"
	"path"

	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

const (
	assetsDir   = "assets/"
	groundCells = 16 // quads per side of the ground
)

const skyColor = 0xFFEBCE87 // 0xAABBGGRR

// These are variables rather than constants so they can be converted to any T.
var (
	carX, carY, carZ = 0.6, roadHeight, 5.0
	carYaw           = 0.0

	roadScale  = 3.6 // a road tile is one unit square
	roadHeight = 0.072

	groundX, groundZ = 0.0, 5.0
	groundSize       = 40.0
	// The ground sits just below y=0 so it doesn't z-fight with the flat
	// undersides of the props standing on it.
	groundY = -0.01
	// Texel of the forest colormap that the grass patches are painted with.
	grassU, grassV = 0.15625, 0.125
)

type prop struct {
	// model is the path of the .obj file under assetsDir, without the
	// extension. It is textured with the colormap.png next to it.
	model   string
	x, y, z float64
	yaw     float64
	scale   float64
	// length is the scale along the model's X axis, same as scale when zero.
	length float64
	// flatten squashes the model vertically by this factor before it is scaled,
	// zero leaves it as it is.
	flatten float64
}

var sceneProps = []prop{
	// One road tile stretched from one edge of the ground to the other. Tiles
	// laid end to end would each round the seam they share differently and
	// leave cracks along it.
	{model: "roads/road-straight", x: groundX, z: groundZ, yaw: math.Pi / 2, scale: roadScale, length: groundSize},

	// On the road
	{model: "cars/kart-oobi", x: -0.6, y: roadHeight, z: 7.8, yaw: math.Pi, scale: 0.9},
	{model: "cars/cone", x: 2.1, z: 7.0, scale: 0.5},

	// Camp
	{model: "forest/patch-dirt", x: 4.0, z: 5.9, yaw: 2.1, scale: 3.6, flatten: 0.15},
	{model: "forest/tent", x: 4.8, z: 7.8, yaw: 3.9, scale: 2.2},
	{model: "forest/flag", x: 3.0, z: 8.4, yaw: 3.6, scale: 2.0},
	{model: "camp/campfire-pit", x: 3.6, z: 5.6, scale: 3.0},
	{model: "camp/campfire-stand", x: 3.6, z: 5.6, yaw: 0.7, scale: 3.0},
	{model: "camp/tree-log-small", x: 3.4, z: 4.4, yaw: 1.4, scale: 2.2},
	{model: "camp/tree-log-small", x: 4.9, z: 5.4, yaw: 0.2, scale: 2.2},
	{model: "camp/tree-trunk", x: 5.0, z: 4.0, scale: 3.0},
	{model: "camp/tool-axe", x: 5.0, y: 0.7, z: 4.0, yaw: 0.9, scale: 3.0},
	{model: "camp/barrel", x: 5.9, z: 6.0, scale: 2.4},
	{model: "forest/rocks-low", x: 6.4, z: 3.6, yaw: 0.6, scale: 1.2},
	{model: "pets/animal-fox", x: 2.6, z: 9.4, yaw: 3.6, scale: 0.4},

	// Across the road
	{model: "cars/truck", x: -4.6, z: 6.8, yaw: 2.6, scale: 1.0},
	{model: "forest/character-archer", x: -2.4, z: 4.4, yaw: 3.5, scale: 1.3},
	{model: "forest/target", x: -3.0, z: 9.6, yaw: 3.3, scale: 1.5},
	{model: "pets/animal-deer", x: -3.9, z: 11.0, yaw: 2.6, scale: 0.5},

	// Edge of the clearing
	{model: "forest/tree-high", x: 2.8, z: 10.6, yaw: 0.4, scale: 1.9},
	{model: "forest/tree", x: -3.0, z: 13.4, yaw: 1.9, scale: 1.8},
	{model: "forest/tree-high", x: -5.4, z: 11.8, yaw: 2.7, scale: 2.1},
	{model: "forest/tree", x: 6.8, z: 8.4, yaw: 5.1, scale: 1.7},
	{model: "forest/tree-high", x: 5.4, z: 10.2, yaw: 3.3, scale: 2.0},
	{model: "forest/tree", x: -5.0, z: 9.6, yaw: 0.9, scale: 1.6},
	{model: "forest/tree-high", x: 7.6, z: 5.4, yaw: 1.2, scale: 2.2},
	{model: "forest/tree-high", x: -6.4, z: 3.5, yaw: 2.2, scale: 2.0},
	{model: "forest/tree", x: -5.5, z: -0.5, yaw: 0.3, scale: 1.7},
	{model: "forest/tree", x: -2.6, z: -2.2, yaw: 3.9, scale: 1.8},
	{model: "forest/tree-high", x: 3.0, z: -0.8, yaw: 4.8, scale: 2.0},

	// Deeper forest
	{model: "forest/tree-high", x: -2.8, z: 16.0, yaw: 1.0, scale: 2.4},
	{model: "forest/tree", x: 3.5, z: 13.5, yaw: 4.4, scale: 2.2},
	{model: "forest/tree-high", x: 2.8, z: 17.5, yaw: 3.0, scale: 2.5},
	{model: "forest/tree-high", x: 7.2, z: 12.2, yaw: 2.6, scale: 2.5},
	{model: "forest/tree-high", x: 10.2, z: 4.4, yaw: 5.3, scale: 2.4},
	{model: "forest/tree-high", x: -8.0, z: 11.0, yaw: 1.7, scale: 2.3},
	{model: "forest/tree-high", x: -8.2, z: -3.0, yaw: 0.2, scale: 2.4},
	{model: "forest/tree-high", x: 6.0, z: -4.0, yaw: 4.0, scale: 2.3},
	{model: "forest/tree-high", x: -3.0, z: -7.5, yaw: 5.0, scale: 2.4},

	{model: "forest/patch-grass", x: -3.6, z: 2.6, yaw: 0.0, scale: 1.1, flatten: 0.8},
	{model: "forest/patch-grass", x: 3.2, z: 12.0, yaw: 1.3, scale: 1.3, flatten: 0.8},
	{model: "forest/patch-grass", x: 3.0, z: 1.6, yaw: 2.9, scale: 1.2, flatten: 0.8},
	{model: "forest/patch-grass", x: -2.9, z: 7.8, yaw: 0.8, scale: 1.2, flatten: 0.8},

	{model: "forest/plant", x: -2.3, z: 6.0, yaw: 1.1, scale: 1.5},
	{model: "forest/plant", x: 2.3, z: 11.4, yaw: 3.0, scale: 1.5},
	{model: "forest/plant", x: -2.2, z: 1.4, yaw: 0.2, scale: 1.6},
	{model: "forest/plant", x: -2.1, z: 10.6, yaw: 2.0, scale: 1.5},
}

func loadTexture(path string) (mesh.TextureData, error) {
	tw, th, pixels, err := texture.LoadTextureData(path)
	if err != nil {
		return mesh.TextureData{}, err
	}
	return mesh.TextureData{PixelData: pixels, Width: tw, Height: th}, nil
}

// loadScene fills r.meshes. The car is always the first mesh.
func (r *Renderer[T]) loadScene() error {
	car, err := mesh.NewMeshFromFile[T]("assets/race.obj", func(m *mesh.Mesh[T]) error {
		tex, err := loadTexture("assets/colormap.png")
		m.Texture = tex
		return err
	}, func(m *mesh.Mesh[T]) error {
		m.Translation = vector.Vec3[T]{X: T(carX), Y: T(carY), Z: T(carZ)}
		m.Rotation = vector.Vec3[T]{Y: T(carYaw)}
		return nil
	})
	if err != nil {
		return err
	}
	r.addSolid(&car)

	textures := make(map[string]mesh.TextureData)
	loadKitTexture := func(model string) (mesh.TextureData, error) {
		file := assetsDir + path.Dir(model) + "/colormap.png"
		tex, ok := textures[file]
		if !ok {
			tex, err = loadTexture(file)
			textures[file] = tex
		}
		return tex, err
	}

	forestTexture, err := loadKitTexture("forest/")
	if err != nil {
		return err
	}
	ground := newGroundMesh[T](forestTexture)
	r.meshes = append(r.meshes, &ground)

	models := make(map[string]mesh.Mesh[T])
	for _, p := range sceneProps {
		model, ok := models[p.model]
		if !ok {
			model, err = mesh.NewMeshFromFile[T](assetsDir + p.model + ".obj")
			if err != nil {
				return err
			}
			model.Texture, err = loadKitTexture(p.model)
			if err != nil {
				return err
			}
			models[p.model] = model
		}
		// Instances of a model share its vertices, faces and texels.
		instance := model
		instance.Translation = vector.Vec3[T]{X: T(p.x), Y: T(p.y), Z: T(p.z)}
		instance.Rotation = vector.Vec3[T]{Y: T(p.yaw)}
		length := p.length
		if length == 0 {
			length = p.scale
		}
		instance.Scale = vector.Vec3[T]{X: T(length), Y: T(p.scale), Z: T(p.scale)}
		if p.flatten != 0 {
			flattenMesh(&instance, p.flatten)
		}
		r.addSolid(&instance)
	}
	return nil
}

// addSolid adds m to the scene along with its collider, if it has one.
func (r *Renderer[T]) addSolid(m *mesh.Mesh[T]) {
	r.meshes = append(r.meshes, m)
	if c, ok := meshCollider(m); ok {
		r.colliders = append(r.colliders, c)
	}
}

// flattenMesh squashes m vertically by factor. The squash is baked into a copy
// of the vertices and faces rather than put in m.Scale, because the normals
// have to tilt upright as the surface flattens for it to be lit correctly.
func flattenMesh[T utils.Numeric](m *mesh.Mesh[T], factor float64) {
	vertices := make([]vector.Vec3[T], len(m.Vertices))
	for i, v := range m.Vertices {
		v.Y = T(float64(v.Y) * factor)
		vertices[i] = v
	}
	faces := make([]mesh.Face[T], len(m.Faces))
	for i, f := range m.Faces {
		for _, n := range []*vector.Vec3[T]{&f.ANormal, &f.BNormal, &f.CNormal} {
			n.Y = T(float64(n.Y) / factor)
			n.Normalize()
		}
		faces[i] = f
	}
	m.Vertices, m.Faces = vertices, faces
}

// newGroundMesh is a grid rather than a single quad so that no triangle spans
// the whole scene.
func newGroundMesh[T utils.Numeric](tex mesh.TextureData) mesh.Mesh[T] {
	const side = groundCells + 1
	cell := groundSize / groundCells

	vertices := make([]vector.Vec3[T], 0, side*side)
	for j := range side {
		for i := range side {
			vertices = append(vertices, vector.Vec3[T]{
				X: T(groundX - groundSize/2 + float64(i)*cell),
				Y: T(groundY),
				Z: T(groundZ - groundSize/2 + float64(j)*cell),
			})
		}
	}

	uv := texture.Tex2[T]{U: T(grassU), V: T(grassV)}
	up := vector.Vec3[T]{Y: 1}
	faces := make([]mesh.Face[T], 0, groundCells*groundCells*2)
	for j := range groundCells {
		for i := range groundCells {
			nearLeft := j*side + i
			nearRight := nearLeft + 1
			farLeft := nearLeft + side
			farRight := farLeft + 1
			// Clockwise seen from above, so the grass faces up
			for _, tri := range [2][3]int{{nearLeft, farLeft, farRight}, {nearLeft, farRight, nearRight}} {
				faces = append(faces, mesh.Face[T]{
					A: tri[0], B: tri[1], C: tri[2],
					AUV: uv, BUV: uv, CUV: uv,
					ANormal: up, BNormal: up, CNormal: up,
					Color: 0xFFFFFFFF,
				})
			}
		}
	}

	m := mesh.NewMesh(vertices, faces, vector.Vec3[T]{}, vector.Vec3[T]{}, vector.Vec3[T]{})
	m.Texture = tex
	return m
}
