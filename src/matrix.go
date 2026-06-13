package main

import "math"

type mat4 struct {
	m [4][4]float64
}

// mat4Identity returns a 4x4 identity matrix
// | 1 0 0 0 |
// | 0 1 0 0 |
// | 0 0 1 0 |
// | 0 0 0 1 |
func mat4Identity() mat4 {
	return mat4{
		m: [4][4]float64{
			{1, 0, 0, 0},
			{0, 1, 0, 0},
			{0, 0, 1, 0},
			{0, 0, 0, 1},
		},
	}
}

// mat4MakeScale returns a 4x4 scale matrix
// | SX  0  0  0 |
// | 0  SY  0  0 |
// | 0  0  SZ  0 |
// | 0  0  0   1 |
func mat4MakeScale(sx, sy, sz float64) mat4 {
	eyeMatrix := mat4Identity()
	eyeMatrix.m[0][0] = sx
	eyeMatrix.m[1][1] = sy
	eyeMatrix.m[2][2] = sz
	return eyeMatrix
}

func (m *mat4) mulVec4(v vec4) vec4 {
	result := vec4{}
	result.X = m.m[0][0]*v.X + m.m[0][1]*v.Y + m.m[0][2]*v.Z + m.m[0][3]*v.W
	result.Y = m.m[1][0]*v.X + m.m[1][1]*v.Y + m.m[1][2]*v.Z + m.m[1][3]*v.W
	result.Z = m.m[2][0]*v.X + m.m[2][1]*v.Y + m.m[2][2]*v.Z + m.m[2][3]*v.W
	result.W = m.m[3][0]*v.X + m.m[3][1]*v.Y + m.m[3][2]*v.Z + m.m[3][3]*v.W
	return result
}

// mat4MakeScale returns a 4x4 translation matrix
// | 0  0  0  tx |
// | 0  0  0  ty |
// | 0  0  0  tz |
// | 0  0  0   1 |
func mat4MakeTranslation(x, y, z float64) mat4 {
	identityMatrix := mat4Identity()
	identityMatrix.m[0][3] = x
	identityMatrix.m[1][3] = y
	identityMatrix.m[2][3] = z
	return identityMatrix
}

// mat4MakeRotationX returns a 4x4 translation matrix
// | 0  0  			0  		0 |
// | 0  cos(l)  -sin(l)  	0 |
// | 0  sin(l)  cos(l)  	0 |
// | 0  0  			0  		0 |
func mat4MakeRotationX(angle float64) mat4 {
	c := math.Cos(angle)
	s := math.Sin(angle)
	identityMatrix := mat4Identity()
	identityMatrix.m[1][1] = c
	identityMatrix.m[1][2] = -s
	identityMatrix.m[2][1] = s
	identityMatrix.m[2][2] = c

	return identityMatrix
}

// mat4MakeRotationY returns a 4x4 translation matrix
// | cos(l)  0	sin(l)		0 |
// | 0  	 1    0			0 |
// | -sin(l) 0  cos(l)  	0 |
// | 0		 0	  0  		1 |
func mat4MakeRotationY(angle float64) mat4 {
	c := math.Cos(angle)
	s := math.Sin(angle)
	identityMatrix := mat4Identity()
	identityMatrix.m[0][0] = c
	identityMatrix.m[0][2] = s
	identityMatrix.m[2][0] = -s
	identityMatrix.m[2][2] = c
	return identityMatrix
}

// mat4MakeRotationY returns a 4x4 translation matrix
// | cos(l)  	-sin(l)			0			0 |
// | sin(l) 	cos(l)			0			0 |
// | 0			 0				1			0 |
// | 0			 0				0			1 |
func mat4MakeRotationZ(angle float64) mat4 {
	c := math.Cos(angle)
	s := math.Sin(angle)
	identityMatrix := mat4Identity()
	identityMatrix.m[0][0] = c
	identityMatrix.m[0][1] = -s
	identityMatrix.m[1][0] = s
	identityMatrix.m[1][1] = c
	return identityMatrix
}

func (m mat4) mulMat4(other mat4) mat4 {
	var result mat4
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
