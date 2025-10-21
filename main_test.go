package main

import (
	"embed"
	_ "embed"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/sirkon/deepequal"

	"github.com/sirkon/cerrful/internal/cerrful"
)

//go:embed testdata
var cirTestCases embed.FS

func TestCerrful(t *testing.T) {
	expected := map[string]*cerrful.CIRProgram{}

	files, err := cirTestCases.ReadDir("testdata/circases")
	if err != nil {
		t.Fatal(fmt.Errorf("list files for CIR checks: %w", err))
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if !strings.HasPrefix(file.Name(), "case_") {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			cir, err := cirTestCases.ReadFile("testdata/circases/" + file.Name())
			if err != nil {
				t.Fatalf("read file %s: %s", file.Name(), err)
			}

			expectedCIR, ok := expected[file.Name()]
			if !ok {
				t.Fatal("no cir found for", file.Name())
			}

			got, err := cerrful.DemoTranslate(string(cir))
			if err != nil {
				t.Fatal("get cir for the case file")
			}

			if !reflect.DeepEqual(expectedCIR, cir) {
				deepequal.SideBySide(t, "cir", expectedCIR, got)
			}

		})
	}
}
