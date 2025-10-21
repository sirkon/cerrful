package main

import "errors"

func newNamedError() (retErr error) {
	return errors.New("error")
}