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

func TestCIR(t *testing.T) {
	expected := map[string]*cerrful.CIRFunction{
		// "case_alias_error": {
		// 	Name: "aliasError",
		// 	Nodes: []cerrful.Node{
		// 		cerrful.Assign{
		// 			Name:    "",
		// 			RHS:     "",
		// 			Flavor:  "",
		// 			IsCtor:  false,
		// 			CtorMsg: "",
		// 			CtorVia: "",
		// 		},
		// 	},
		// },
	}

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
			code, err := cirTestCases.ReadFile("testdata/circases/" + file.Name())
			if err != nil {
				t.Fatalf("read file %s: %s", file.Name(), err)
			}

			got, err := cerrful.DemoTranslate(string(code))
			if err != nil {
				t.Fatal("get cir for the case file")
			}

			t.Log("\n" + got.Pretty(true))

			expectedCIR, ok := expected[file.Name()]
			if !ok {
				t.Fatal("no cir found for", file.Name())
			}

			if !reflect.DeepEqual(expectedCIR, code) {
				deepequal.SideBySide(
					t,
					"cir",
					&cerrful.CIRProgram{
						File:      file.Name(),
						Functions: []cerrful.CIRFunction{*expectedCIR},
					},
					got,
				)
			}

		})
	}
}
