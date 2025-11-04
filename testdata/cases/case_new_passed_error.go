package main

import "fmt"

func newPassedError() error {
	err := fmt.Errorf("hello %s", "world")
	return err
}