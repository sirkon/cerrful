package main

import (
	"errors"
	"fmt"
	"math/rand"
)

func branchSwitch() error {
	switch v := rand.Int(); v {
	case 0:
		return errors.New("we don't wont to have zero")
	case 1, 2:
		return nil
	default:
		return fmt.Errorf("unexpected value %d", v)
	}
}