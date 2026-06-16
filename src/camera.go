package main

var (
	globalCamera = camera{
		position:  vec3{},
		direction: vec3{Z: 1},
	}
)

type camera struct {
	position  vec3
	direction vec3
}
