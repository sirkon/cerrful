// Package main placed in testdata made "main" to avoid being imported by users who somehow
// decide this is a good idea – it is not. It lacks "func main() {…}" in order not to be built with
//
//	go build
//
// Because it is not a good idea either. This "package" is for testing, not for usage.
package main
