// this package is the second, authoritative pass: it shells out to a real
// MetaEditor/mql compiler and normalizes its log into the shared Finding
// schema. The compile host (remote Windows, local VM, Wine, ...) is not decided
// yet, so this package is defined entirely in terms of a Backend interface. A
// concrete backend can be slotted in later without touching callers.
package compile

import (
	"context"
	"github.com/AndochBonin/mqls/internal/finding"
)

// Backend is a compiler implementation. Whatever the eventual host, it takes a
// source path and returns findings already normalized into the shared schema.
type Backend interface {
	Name() string    // identifies the backend, e.g. "remote-windows", "wine", "noop".
	Available() bool // reports whether this backend can actually run right now.
	// I may want to do some fancier things later like calculating "cost of compilation"

	// compile builds the file and returns normalized compiler diagnostics.
	Compile(ctx context.Context, path string) ([]finding.Finding, error)
}

// Noop is the placeholder backend used until a real compile host is chosen. It
// is always "unavailable" and returns a single informational finding so the
// agent knows verification was skipped rather than silently passing.
type Noop struct{}

func (Noop) Name() string    { return "noop" }
func (Noop) Available() bool { return false }

func (Noop) Compile(_ context.Context, path string) ([]finding.Finding, error) {
	return []finding.Finding{{
		File:     path,
		Severity: finding.Info,
		RuleID:   "compile/no-backend",
		Message:  "No compile backend is configured; the authoritative compile pass was skipped.",
		Suggest:  "Configure a MetaEditor backend (remote Windows host, local VM, or Wine) to enable ground-truth verification.",
		Pass:     finding.PassCompile,
	}}, nil
}

// Default returns the backend to use: the Parallels backend when it is
// configured (MQLS_VM/MQLS_METAEDITOR set) and the VM is reachable, otherwise
// the Noop stub. Backend selection grows here.
func Default() Backend {
	if p := newParallels(); p != nil && p.Available() {
		return p
	}
	return Noop{}
}
