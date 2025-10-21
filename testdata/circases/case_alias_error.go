package main

import (
	"fmt"
	"os"
)

func aliasError() error {
	_, oldErr := os.UserHomeDir()
	if oldErr != nil {
		newErr := oldErr
		return fmt.Errorf("get user home directory: %w", newErr)
	}

	return nil
}