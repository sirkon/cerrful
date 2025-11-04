package tracing

import (
	"go/token"
	"sync"
	"testing"

	"github.com/sirkon/cerrful/internal/cerrules"
)

func TestReporter_ReportPhases(t *testing.T) {
	tests := []struct {
		name     string
		phase    ReportPhase
		rule     cerrules.Rule
		message  string
		filename string
		line     int
	}{
		{
			name:     "source-phase basic",
			phase:    ReportSource,
			rule:     cerrules.AnnotateExternal(),
			message:  "Wrap errors when crossing a semantic boundary",
			filename: "main.go",
			line:     10,
		},
		{
			name:     "trace-phase no silent drop",
			phase:    ReportTrace,
			rule:     cerrules.NoSilentDrop(),
			message:  "Error must never be ignored",
			filename: "trace.go",
			line:     20,
		},
		{
			name:     "state-phase fix before use",
			phase:    ReportState,
			rule:     cerrules.FixBeforeUse(),
			message:  "variable errFoo used before fixation",
			filename: "file.go",
			line:     42,
		},
	}

	var r Reporter

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := r.Phase(tt.phase)
			phase.Report(tt.rule, tt.message, token.Position{
				Filename: tt.filename,
				Line:     tt.line,
			})
		})
	}

	reps := r.Reports()
	if len(reps) != len(tests) {
		t.Fatalf("expected %d reports, got %d", len(tests), len(reps))
	}

	for i, rep := range reps {
		want := tests[i]
		if rep.Phase != want.phase {
			t.Errorf("[%s] phase mismatch: got %v, want %v", want.name, rep.Phase, want.phase)
		}
		if rep.RuleCode != want.rule {
			t.Errorf("[%s] rule mismatch: got %v, want %v", want.name, rep.RuleCode, want.rule)
		}
		if rep.Message != want.message {
			t.Errorf("[%s] message mismatch: got %q, want %q", want.name, rep.Message, want.message)
		}
		if rep.Pos.Filename != want.filename || rep.Pos.Line != want.line {
			t.Errorf("[%s] position mismatch: got %s:%d, want %s:%d",
				want.name, rep.Pos.Filename, rep.Pos.Line, want.filename, want.line)
		}
	}
}

func TestReporter_ConcurrencySafety(t *testing.T) {
	const n = 500
	var (
		r    Reporter
		wg   sync.WaitGroup
		fset token.FileSet
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Report(Report{
				Phase:    ReportTrace,
				RuleCode: cerrules.NoSilentDrop(),
				Message:  "parallel add",
				Pos:      fset.Position(token.Pos(i)),
			})
		}(i)
	}
	wg.Wait()

	reps := r.Reports()
	if len(reps) != n {
		t.Fatalf("expected %d reports, got %d", n, len(reps))
	}
	reps[0].Message = "changed"
	reps2 := r.Reports()
	if reps2[0].Message == "changed" {
		t.Fatalf("Reports() returned shared slice, expected copy")
	}
}
