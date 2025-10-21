package main

import (
	"fmt"
	"io"
)

func logError() {
	err := fmt.Errorf("read stream: %w", io.EOF)
	fmt.Println("fetch data:", err)
}