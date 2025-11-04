package tracing

import (
	"fmt"
	"go/token"
	"sync"

	"github.com/sirkon/cerrful/internal/cerrules"
)

// Reporter collects and classifies inconsistencies discovered during tracing.
type Reporter struct {
	mu      sync.Mutex
	reports []Report
}

// Report represents a single diagnostic entry.
type Report struct {
	Phase    ReportPhase
	RuleCode cerrules.Rule
	Pos      token.Position
	Message  string
	Details  any
}

// ReportPhase marks the tracing stage where a report was generated.
type ReportPhase int

const (
	reportPhaseInvalid ReportPhase = iota
	ReportSource                   // AST collection phase
	ReportTrace                    // SSA scanning / path interpretation
	ReportState                    // post-trace error state analysis
)

func (p ReportPhase) String() string {
	switch p {
	case ReportSource:
		return "source"
	case ReportTrace:
		return "trace"
	case ReportState:
		return "state"
	default:
		return fmt.Sprintf("unknown-phase(%d)", p)
	}
}

// ReporterPhase binds a Reporter to a fixed phase.
// It is used during an entire analysis pass to record rule violations
// without specifying the phase repeatedly.
type ReporterPhase struct {
	parent *Reporter
	phase  ReportPhase
}

// Phase returns a pointer to a phase-bound reporter that automatically
// sets the given phase for all reports produced through it.
func (r *Reporter) Phase(p ReportPhase) *ReporterPhase {
	return &ReporterPhase{parent: r, phase: p}
}

// Report adds a new record to the reporter.
func (r *Reporter) Report(rep Report) {
	r.mu.Lock()
	r.reports = append(r.reports, rep)
	r.mu.Unlock()
}

// Report records a new rule violation under the bound phase.
// It accepts a cerrules.Rule, human-readable message, and source position.
func (rp *ReporterPhase) Report(rule cerrules.Rule, message string, pos token.Position) {
	rp.parent.Report(Report{
		Phase:    rp.phase,
		RuleCode: rule,
		Message:  message,
		Pos:      pos,
	})
}

// Reports returns a snapshot of all collected records.
func (r *Reporter) Reports() []Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Report, len(r.reports))
	copy(out, r.reports)
	return out
}

// PrintSummary prints all collected reports in a compact, human-readable form.
func (r *Reporter) PrintSummary() {
	for _, rep := range r.Reports() {
		fmt.Printf("[%s] %s — %s (%s:%d)\n",
			rep.Phase,
			rep.RuleCode,
			rep.Message,
			rep.Pos.Filename,
			rep.Pos.Line)
	}
}
