// this package defines the uniform result schema (a Finding) shared by every pass of
// the validation harness. Both static lint and the compile pass emit Findings,
// so the consuming agent sees one contract regardless of which pass produced
// the result.
package finding

type Severity string // ranks how much a finding should block the agent.

const (
	Error   Severity = "error"   // code is (almost certainly) broken and must be fixed.
	Warning Severity = "warning" // likely bug or a strong MQL4/MQL5 smell worth reviewing.
	Info    Severity = "info"    // advisory context, never blocking.
)

type Pass string // identifies which stage of the harness produced a finding.

const (
	PassStatic  Pass = "static"
	PassCompile Pass = "compile"
)

// A Finding is a single, agent-actionable diagnostic. It is designed to be
// consumed by an AI agent in its next generation turn, so every field is
// stable and self-explanatory.
type Finding struct {
	File     string   `json:"file"`
	Line     int      `json:"line"`   // 1-based; 0 if unknown
	Column   int      `json:"column"` // 1-based; 0 if unknown
	Severity Severity `json:"severity"`
	RuleID   string   `json:"rule_id"` // stable id, e.g. "mql4-api/order-close"
	Message  string   `json:"message"`
	Suggest  string   `json:"suggestion,omitempty"`
	Pass     Pass     `json:"pass"`
}

// Report is the top-level envelope emitted for one validated file.
type Report struct {
	File     string    `json:"file"`
	OK       bool      `json:"ok"` // true when no error-severity findings
	Findings []Finding `json:"findings"`
}

// NewReport builds a Report and derives OK from the findings.
func NewReport(file string, findings []Finding) Report {
	if findings == nil {
		findings = []Finding{}
	}
	ok := true
	for _, f := range findings {
		if f.Severity == Error {
			ok = false
			break
		}
	}
	return Report{File: file, OK: ok, Findings: findings}
}
