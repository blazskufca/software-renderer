package main

import "math"

// TODO: Fix this to either have all the pointer receivers or all value receivers
// TODO: Fix the weird casts lol

type vec3 struct {
	X float64
	Y float64
	Z float64
}

func (v *vec3) rotateX(angle float64) {
	// Calculate both using original values, then assign simultaneously
	v.Y, v.Z = v.Y*math.Cos(angle)-v.Z*math.Sin(angle), v.Y*math.Sin(angle)+v.Z*math.Cos(angle)
}

func (v *vec3) rotateY(angle float64) {
	v.X, v.Z = v.X*math.Cos(angle)-v.Z*math.Sin(angle), v.X*math.Sin(angle)+v.Z*math.Cos(angle)
}

func (v *vec3) rotateZ(angle float64) {
	v.X, v.Y = v.X*math.Cos(angle)-v.Y*math.Sin(angle), v.X*math.Sin(angle)+v.Y*math.Cos(angle)
}

func (v *vec3) normalize() {
	magnitude := v.length()
	v.X = v.X / magnitude
	v.Y = v.Y / magnitude
	v.Z = v.Z / magnitude
}

func (v vec3) length() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

func (v vec3) add(other vec3) vec3 {
	return vec3{X: v.X + other.X, Y: v.Y + other.Y, Z: v.Z + other.Z}
}

func (v vec3) sub(other vec3) vec3 {
	return vec3{X: v.X - other.X, Y: v.Y - other.Y, Z: v.Z - other.Z}
}

func (v vec3) multiply(factor float64) vec3 {
	return vec3{X: v.X * factor, Y: v.Y * factor, Z: v.Z * factor}
}

func (v vec3) divide(factor float64) vec3 {
	return vec3{X: v.X / factor, Y: v.Y / factor, Z: v.Z / factor}
}

func (v vec3) cross(other vec3) vec3 {
	return vec3{
		X: v.Y*other.Z - v.Z*other.Y,
		Y: v.Z*other.X - v.X*other.Z,
		Z: v.X*other.Y - v.Y*other.X,
	}
}

func (v vec3) dot(other vec3) float64 {
	return (v.X * other.X) + (v.Y * other.Y) + (v.Z * other.Z)
}

// --- 2D Vector ---

type vec2 struct {
	X int
	Y int
}

func (v vec2) length() float64 {
	fx, fy := float64(v.X), float64(v.Y)
	return math.Sqrt(fx*fx + fy*fy)
}

func (v vec2) add(other vec2) vec2 {
	return vec2{X: v.X + other.X, Y: v.Y + other.Y}
}

// Renamed from subtract to sub to match vec3
func (v vec2) sub(other vec2) vec2 {
	return vec2{X: v.X - other.X, Y: v.Y - other.Y}
}

func (v vec2) multiply(factor float64) vec2 {
	f := int(math.Round(factor))
	return vec2{X: v.X * f, Y: v.Y * f}
}

func (v vec2) divide(factor float64) vec2 {
	f := int(math.Round(factor))
	return vec2{X: v.X / f, Y: v.Y / f}
}

func (v vec2) dot(other vec2) int {
	return (v.X * other.X) + (v.Y * other.Y)
}

func (v *vec2) normalize() {
	magnitude := v.length()
	v.X = v.X / int(magnitude)
	v.Y = v.Y / int(magnitude)
}

type vec4 struct {
	X, Y, Z, W float64
}

func vec4FromVec3(vec vec3) vec4 {
	return vec4{vec.X, vec.Y, vec.Z, 1}
}

type vectorType interface {
	vec2 | vec3 | vec4
}

func vec3FromVec4(vec vec4) vec3 {
	return vec3{
		X: vec.X,
		Y: vec.Y,
		Z: vec.Z,
	}
}

func vec2FromVec4(vec vec4) vec2 {
	return vec2{
		int(math.Round(vec.X)), int(math.Round(vec.Y)),
	}
}
