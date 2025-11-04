// Package main placed in testdata is only "main" to prevent its importing by users, who somehow
// decide this is a good idea – it is not. It lacks "func main() {…}" in order not to be built with
//
//	go build
//
// Because it is not cool either. Its purpose is testing and that's it.
package main
