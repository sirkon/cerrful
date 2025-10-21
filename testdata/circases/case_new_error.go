package main

import "errors"

func newError() error {
	return errors.New("error")
}