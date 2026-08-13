// this package is the fast static pass. runs a set of high-signal, near
// zero-false-positive Rules that catch the footguns an LLM is most likely to
// produce when generating MQL5 — without invoking a compiler.
package lint

import (
	"github.com/AndochBonin/mqls/internal/finding"
	"sort"
)

// a Rule is one self-contained diagnostic check. Adding a footgun to the harness
// is a matter of implementing this interface in one file and adding it to the registry.
type Rule interface {
	// ID is the stable rule identifier prefix, e.g. "mql4-api". Individual
	// findings may extend it, e.g. "mql4-api/order-close".
	ID() string
	Check(src *Source) []finding.Finding // inspects the (preprocessed) source and returns any findings.
}

// registry holds every rule the static pass runs.
var registry = []Rule{
	structuralRule{},
	mql4APIRule{},
	eventHandlerRule{},
	predefinedVarRule{},
}

func Rules() []Rule { return registry } // returns the registered rules (mainly for testing / introspection).

// Run executes an array of rules against the source and returns a Report.
// Findings are sorted by position so the agent reads them top-to-bottom.
func Run(path, raw string) finding.Report {
	src := NewSource(path, raw)
	var findings []finding.Finding
	for _, r := range registry {
		findings = append(findings, r.Check(src)...)
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Column < findings[j].Column
	})
	return finding.NewReport(path, findings)
}
