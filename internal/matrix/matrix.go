package matrix

import (
	"math"

	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

type Mat4[T utils.Numeric] struct {
	m [4][4]T
}

// Mat4Identity returns a 4x4 identity matrix
//
// | 1 0 0 0 |
// | 0 1 0 0 |
// | 0 0 1 0 |
// | 0 0 0 1 |
func Mat4Identity[T utils.Numeric]() Mat4[T] {
	return Mat4[T]{
		m: [4][4]T{
			{1, 0, 0, 0},
			{0, 1, 0, 0},
			{0, 0, 1, 0},
			{0, 0, 0, 1},
		},
	}
}

// Mat4MakeScale returns a 4x4 scale matrix
// | SX  0  0  0 |
// | 0  SY  0  0 |
// | 0  0  SZ  0 |
// | 0  0  0   1 |
func Mat4MakeScale[T utils.Numeric](sx, sy, sz T) Mat4[T] {
	eyeMatrix := Mat4Identity[T]()
	eyeMatrix.m[0][0] = sx
	eyeMatrix.m[1][1] = sy
	eyeMatrix.m[2][2] = sz
	return eyeMatrix
}

// MulVec4 multiplies 4x4 vector with a Vec4[T] and returns a resulting vector Vec4[T]
func (m *Mat4[T]) MulVec4(v vector.Vec4[T]) vector.Vec4[T] {
	result := vector.Vec4[T]{}
	result.X = m.m[0][0]*v.X + m.m[0][1]*v.Y + m.m[0][2]*v.Z + m.m[0][3]*v.W
	result.Y = m.m[1][0]*v.X + m.m[1][1]*v.Y + m.m[1][2]*v.Z + m.m[1][3]*v.W
	result.Z = m.m[2][0]*v.X + m.m[2][1]*v.Y + m.m[2][2]*v.Z + m.m[2][3]*v.W
	result.W = m.m[3][0]*v.X + m.m[3][1]*v.Y + m.m[3][2]*v.Z + m.m[3][3]*v.W
	return result
}

// At returns the element at row, col.
func (m *Mat4[T]) At(row, col int) T { return m.m[row][col] }

// MulPoint transforms the point v (W = 1) by an affine matrix (last row 0 0 0 1),
// skipping the W row that MulVec4 would compute.
func (m *Mat4[T]) MulPoint(v vector.Vec3[T]) vector.Vec3[T] {
	return vector.Vec3[T]{
		X: m.m[0][0]*v.X + m.m[0][1]*v.Y + m.m[0][2]*v.Z + m.m[0][3],
		Y: m.m[1][0]*v.X + m.m[1][1]*v.Y + m.m[1][2]*v.Z + m.m[1][3],
		Z: m.m[2][0]*v.X + m.m[2][1]*v.Y + m.m[2][2]*v.Z + m.m[2][3],
	}
}

// MulDirection transforms the direction v (W = 0), so translation is ignored.
func (m *Mat4[T]) MulDirection(v vector.Vec3[T]) vector.Vec3[T] {
	return vector.Vec3[T]{
		X: m.m[0][0]*v.X + m.m[0][1]*v.Y + m.m[0][2]*v.Z,
		Y: m.m[1][0]*v.X + m.m[1][1]*v.Y + m.m[1][2]*v.Z,
		Z: m.m[2][0]*v.X + m.m[2][1]*v.Y + m.m[2][2]*v.Z,
	}
}

// Mat4MakeTranslation returns a 4x4 translation matrix
// | 0  0  0  tx |
// | 0  0  0  ty |
// | 0  0  0  tz |
// | 0  0  0   1 |
func Mat4MakeTranslation[T utils.Numeric](x, y, z T) Mat4[T] {
	identityMatrix := Mat4Identity[T]()
	identityMatrix.m[0][3] = x
	identityMatrix.m[1][3] = y
	identityMatrix.m[2][3] = z
	return identityMatrix
}

// Mat4MakeRotationX returns a 4x4 translation matrix
// | 0  0  			0  		0 |
// | 0  cos(l)  -sin(l)  	0 |
// | 0  sin(l)  cos(l)  	0 |
// | 0  0  			0  		0 |
func Mat4MakeRotationX[T utils.Numeric](angle T) Mat4[T] {
	c := T(math.Cos(float64(angle)))
	s := T(math.Sin(float64(angle)))
	identityMatrix := Mat4Identity[T]()
	identityMatrix.m[1][1] = c
	identityMatrix.m[1][2] = -s
	identityMatrix.m[2][1] = s
	identityMatrix.m[2][2] = c

	return identityMatrix
}

// Mat4MakeRotationY returns a 4x4 translation matrix
// | cos(l)  0	sin(l)		0 |
// | 0  	 1    0			0 |
// | -sin(l) 0  cos(l)  	0 |
// | 0		 0	  0  		1 |
func Mat4MakeRotationY[T utils.Numeric](angle T) Mat4[T] {
	c := T(math.Cos(float64(angle)))
	s := T(math.Sin(float64(angle)))
	identityMatrix := Mat4Identity[T]()
	identityMatrix.m[0][0] = c
	identityMatrix.m[0][2] = s
	identityMatrix.m[2][0] = -s
	identityMatrix.m[2][2] = c
	return identityMatrix
}

// Mat4MakeRotationZ returns a 4x4 translation matrix
// | cos(l)  	-sin(l)			0			0 |
// | sin(l) 	cos(l)			0			0 |
// | 0			 0				1			0 |
// | 0			 0				0			1 |
func Mat4MakeRotationZ[T utils.Numeric](angle T) Mat4[T] {
	c := T(math.Cos(float64(angle)))
	s := T(math.Sin(float64(angle)))
	identityMatrix := Mat4Identity[T]()
	identityMatrix.m[0][0] = c
	identityMatrix.m[0][1] = -s
	identityMatrix.m[1][0] = s
	identityMatrix.m[1][1] = c
	return identityMatrix
}

// MulMat4 multiplies the current matrix with other matrix and returns a new Mat4[T]
func (m Mat4[T]) MulMat4(other Mat4[T]) Mat4[T] {
	var result Mat4[T]
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			result.m[i][j] = m.m[i][0]*other.m[0][j] +
				m.m[i][1]*other.m[1][j] +
				m.m[i][2]*other.m[2][j] +
				m.m[i][3]*other.m[3][j]
		}
	}
	return result
}

// Mat4MakePerspective accepts a FOV angle, aspect ratio (h/w), zNear and zFar
// and returns a matrix
// | aspect*(1/tan(fov/2)	0					0						0								|
// | 0						1/(tan(fov/2))		0						0								|
// | 0						0					zFar/(zFar - zNear)		(-zFar)/(zFar-zNear)*zNear)		|
// | 0						0					1						0								|
// | 											^
// | 											|
// | 											-- This 1 here stores the original Z in the matrix W
func Mat4MakePerspective[T utils.Numeric, K utils.Numeric](fov, aspect T, znear, zfar K) Mat4[T] {
	// Start from a zero matrix, NOT the identity: a perspective matrix must have
	// m[3][3] == 0 so the perspective divide divides by the original Z (stored in
	// W via m[3][2]). Mat4Identity would leave m[3][3] == 1, making W = Z + W.
	var m Mat4[T]
	m.m[0][0] = aspect * T(1/(math.Tan(float64(fov)/2)))
	m.m[1][1] = T(1 / (math.Tan(float64(fov) / 2)))
	m.m[2][2] = T(zfar) / (T(zfar) - T(znear))
	m.m[2][3] = (T(-zfar) * T(znear)) / (T(zfar) - T(znear))
	m.m[3][2] = 1 // Store the original Z in the matrix W
	return m
}

// Mat4MulVec4Project applies perspective matrix and does perspective divide
func (m *Mat4[T]) Mat4MulVec4Project(vec vector.Vec4[T]) vector.Vec4[T] {
	result := m.MulVec4(vec)
	// Original Z is stored in result W
	// After doing aspect ratio, FOV and normalization we still need to perform perspective divide
	if result.W != 0 {
		result.X /= result.W
		result.Y /= result.W
		result.Z /= result.W
	}
	return result
}

// Mat4LookAt creates and returns a camera matrix
func Mat4LookAt[T utils.Numeric](eye, target, up vector.Vec3[T]) Mat4[T] {
	Z := target.Sub(eye) // Forward vector Z pointing at the target
	Z.Normalize()        // We only care about the direction not magnitude
	X := up.Cross(Z)     // Right vector X
	X.Normalize()
	Y := Z.Cross(X) // Up vector Z - Already normalized because X and Y are already normalized

	return Mat4[T]{m: [4][4]T{
		{X.X, X.Y, X.Z, -X.Dot(eye)},
		{Y.X, Y.Y, Y.Z, -Y.Dot(eye)},
		{Z.X, Z.Y, Z.Z, -Z.Dot(eye)},
		{0, 0, 0, 1},
	}}
}
