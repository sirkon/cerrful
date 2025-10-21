package main

import "os"

func panicError() {
	_, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
}