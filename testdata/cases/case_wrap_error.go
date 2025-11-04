package main

import (
	"fmt"
	"io"
)

func wrapError() error {
	err := io.ErrNoProgress
	if err != nil {
		return fmt.Errorf("get progress: %w", err)
	}

	return nil
}