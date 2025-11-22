package tracing

import (
	"go/token"
	"sync"
	"testing"

	"github.com/sirkon/cerrful/internal/cerrules"
)

func TestReporter_ReportPhases(t *testing.T) {
	tests := []struct {
		name    string
		phase   ReportPhase
		rule    cerrules.Rule
		message string
		pos     token.Pos
	}{
		{
			name:    "source-phase basic",
			phase:   ReportScrap,
			rule:    cerrules.AnnotateExternal(),
			message: "Wrap errors when crossing a semantic boundary",
		},
		{
			name:    "trace-phase no silent drop",
			phase:   ReportTrace,
			rule:    cerrules.NoSilentDrop(),
			message: "Error must never be ignored",
			pos:     10,
		},
		{
			name:    "state-phase fix before use",
			phase:   ReportState,
			rule:    cerrules.FixBeforeUse(),
			message: "variable errFoo used before fixation",
			pos:     20,
		},
	}

	var r ReportEngine

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := r.Phase(tt.phase)
			phase.Report(tt.rule, tt.message, tt.pos)
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
		if rep.Pos != want.pos {
			t.Errorf("[%s] position mismatch: got %d, want %d", want.name, rep.Pos, want.pos)
		}
	}
}

func TestReporter_ConcurrencySafety(t *testing.T) {
	const n = 500
	var (
		r  ReportEngine
		wg sync.WaitGroup
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Report(Report{
				Phase:    ReportTrace,
				RuleCode: cerrules.NoSilentDrop(),
				Message:  "parallel add",
				Pos:      token.Pos(i),
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
