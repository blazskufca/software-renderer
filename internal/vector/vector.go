package vector

import (
	"math"

	"github.com/blazskufca/software-renderer/internal/utils"
)

// Vec3 represents a vector with 3 components
type Vec3[T utils.Numeric] struct {
	X T
	Y T
	Z T
}

// RotateX rotates the vector by angle around the X axis.
func (v *Vec3[T]) RotateX(angle T) {
	// Calculate both using original values, then assign simultaneously
	v.Y, v.Z = v.Y*T(math.Cos(float64(angle)))-v.Z*T(math.Sin(float64(angle))), v.Y*T(math.Sin(float64(angle)))+v.Z*T(math.Cos(float64(angle)))
}

// RotateY rotates the vector by angle around the Y axis.
func (v *Vec3[T]) RotateY(angle T) {
	v.X, v.Z = v.X*T(math.Cos(float64(angle)))-v.Z*T(math.Sin(float64(angle))), v.X*T(math.Sin(float64(angle)))+v.Z*T(math.Cos(float64(angle)))
}

// RotateY rotates the vector by angle around the Z axis.
func (v *Vec3[T]) rotateZ(angle T) {
	v.X, v.Y = v.X*T(math.Cos(float64(angle)))-v.Y*T(math.Sin(float64(angle))), v.X*T(math.Sin(float64(angle)))+v.Y*T(math.Cos(float64(angle)))
}

// Normalize normalizes the vector
func (v *Vec3[T]) Normalize() {
	magnitude := v.Length()
	v.X = v.X / magnitude
	v.Y = v.Y / magnitude
	v.Z = v.Z / magnitude
}

// Length returns the magnitude of a vector
func (v Vec3[T]) Length() T {
	return T(math.Sqrt(float64(v.X)*float64(v.X) + float64(v.Y)*float64(v.Y) + float64(v.Z)*float64(v.Z)))
}

// Add adds other to current vector and returns a new vector of Vec3[T]
func (v Vec3[T]) Add(other Vec3[T]) Vec3[T] {
	return Vec3[T]{X: v.X + other.X, Y: v.Y + other.Y, Z: v.Z + other.Z}
}

// Sub subtracts other from current vector and returns a new vector of Vec3[T]
func (v Vec3[T]) Sub(other Vec3[T]) Vec3[T] {
	return Vec3[T]{X: v.X - other.X, Y: v.Y - other.Y, Z: v.Z - other.Z}
}

// Multiply multiplies the current vector by scalar factor and returns a new vector of Vec3[T]
func (v Vec3[T]) Multiply(factor T) Vec3[T] {
	return Vec3[T]{X: v.X * factor, Y: v.Y * factor, Z: v.Z * factor}
}

// Divide divides the current vector by scalar factor and returns a new vector of Vec3[T]
func (v Vec3[T]) Divide(factor T) Vec3[T] {
	return Vec3[T]{X: v.X / factor, Y: v.Y / factor, Z: v.Z / factor}
}

// Cross calculates the vector cross product between the current vector and the other vector and returns a new Vec3[T]
func (v Vec3[T]) Cross(other Vec3[T]) Vec3[T] {
	return Vec3[T]{
		X: v.Y*other.Z - v.Z*other.Y,
		Y: v.Z*other.X - v.X*other.Z,
		Z: v.X*other.Y - v.Y*other.X,
	}
}

// Dot calculates the vector dot product between the current and the other vector and returns a scalar value of type T
func (v Vec3[T]) Dot(other Vec3[T]) T {
	return (v.X * other.X) + (v.Y * other.Y) + (v.Z * other.Z)
}

// ToVec4 casts current Vec3[T] to Vec4[T] and returns it. W is set to 1 so the
// resulting vector represents a point: affine transforms (the translation column
// of a Mat4) only apply when W == 1. With W == 0 the translation is discarded,
// which silently breaks the view-matrix camera translation and mesh translation.
func (v Vec3[T]) ToVec4() Vec4[T] {
	return Vec4[T]{
		X: v.X,
		Y: v.Y,
		Z: v.Z,
		W: 1,
	}
}

// --- 2D Vector ---

// Vec2 represents a vector with two components
type Vec2[T utils.Numeric] struct {
	X T
	Y T
}

// Length returns a vector magnitude of scalar type T
func (v Vec2[T]) Length() T {
	fx, fy := float64(v.X), float64(v.Y)
	return T(math.Sqrt(fx*fx + fy*fy))
}

// Add adds other vector of type Vec2[T] to current vector and returns a new Vec2[T] with the result of the addition
func (v Vec2[T]) Add(other Vec2[T]) Vec2[T] {
	return Vec2[T]{X: v.X + other.X, Y: v.Y + other.Y}
}

// Sub subtracts other vector of type Vec2[T] to current vector and returns a new Vec2[T] with the result of the subtraction
func (v Vec2[T]) Sub(other Vec2[T]) Vec2[T] {
	return Vec2[T]{X: v.X - other.X, Y: v.Y - other.Y}
}

// Multiply multiplies the current vector with scalar value of type T and returns a new Vec2[T] with the result of multiplication
func (v Vec2[T]) Multiply(factor T) Vec2[T] {
	return Vec2[T]{X: v.X * factor, Y: v.Y * factor}
}

// Divide divides the current vector with scalar value of type T and returns a new vec2[T] with the result of the division
func (v Vec2[T]) Divide(factor T) Vec2[T] {
	return Vec2[T]{X: v.X / factor, Y: v.Y / factor}
}

// Dot preforms a vector dot product between the current vector and the other vector and returns a scalar value of type T
func (v Vec2[T]) dot(other Vec2[T]) T {
	return (v.X * other.X) + (v.Y * other.Y)
}

// Normlize normalizes the vector
func (v *Vec2[T]) Normalize() {
	magnitude := v.Length()
	v.X = v.X / magnitude
	v.Y = v.Y / magnitude
}

// Vec4 represents a vector with 4 components
type Vec4[T utils.Numeric] struct {
	X, Y, Z, W T
}

// ToVec2 casts Vec4[T] to Vec2[T] and returns it.
func (v Vec4[T]) ToVec2() Vec2[T] {
	return Vec2[T]{
		X: v.X,
		Y: v.Y,
	}
}

// ToVec3 casts Vec4[T] to Vec3[T] and returns it.
func (v Vec4[T]) ToVec3() Vec3[T] {
	return Vec3[T]{
		X: v.X,
		Y: v.Y,
		Z: v.Z,
	}
}
