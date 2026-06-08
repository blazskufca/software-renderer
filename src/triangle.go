package main

// face stores vertex indexes
type face struct {
	a int
	b int
	c int
}

// triangle stores actual vec2 points of the triangle projected onto the screen
type triangle struct {
	points [3]vec2
}
