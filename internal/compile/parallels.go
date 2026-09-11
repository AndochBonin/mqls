package compile

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AndochBonin/mqls/internal/finding"
)

// Parallels compiles MQL5 inside a Parallels Desktop Windows guest via
// `prlctl exec`. It relies on the Mac home dir being auto-shared into the guest
// (default \\Mac\Home), so the source is compiled in place with no copy: the
// path is translated to its guest UNC form, MetaEditor writes its log next to
// the source, and we read that log back on the Mac side.
type Parallels struct {
	vm         string // guest name, e.g. "Windows 11"
	metaeditor string // guest path to metaeditor64.exe
	inc        string // optional guest include dir for /inc
	homeShare  string // guest UNC mapping to $HOME (default \\Mac\Home)
	home       string // $HOME on the Mac side
}

// newParallels builds a Parallels backend from the environment, or returns nil
// when the required config (MQLS_VM, MQLS_METAEDITOR) is unset — keeping
// zero-config machines on the Noop fallback.
func newParallels() *Parallels {
	vm := os.Getenv("MQLS_VM")
	me := os.Getenv("MQLS_METAEDITOR")
	home, _ := os.UserHomeDir()
	if vm == "" || me == "" || home == "" {
		return nil
	}
	share := os.Getenv("MQLS_HOME_SHARE")
	if share == "" {
		share = `\\Mac\Home`
	}
	return &Parallels{
		vm:         vm,
		metaeditor: me,
		inc:        os.Getenv("MQLS_MQL5_INC"),
		homeShare:  share,
		home:       home,
	}
}

func (Parallels) Name() string { return "parallels" }

// Available reports whether the backend can run right now: prlctl on PATH and
// the VM currently running. A suspended/stopped VM is not powered on.
func (p *Parallels) Available() bool {
	if _, err := exec.LookPath("prlctl"); err != nil {
		return false
	}
	out, err := exec.Command("prlctl", "status", p.vm).Output()
	if err != nil {
		return false
	}
	// e.g. "VM "Windows 11" exist running"
	return strings.Contains(string(out), "running")
}

func (p *Parallels) Compile(ctx context.Context, path string) ([]finding.Finding, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(p.home, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return []finding.Finding{{
			File:     path,
			Severity: finding.Error,
			RuleID:   "compile/not-shared",
			Message:  "Source is outside your home directory, which is the only path shared into the Parallels guest.",
			Suggest:  fmt.Sprintf("Move the file under %s so it is reachable at %s in the VM.", p.home, p.homeShare),
			Pass:     finding.PassCompile,
		}}, nil
	}

	logPath := abs + ".log"
	uncSrc := p.toGuest(abs)
	uncLog := p.toGuest(logPath)

	args := []string{"exec", p.vm, p.metaeditor,
		"/compile:" + uncSrc,
		"/log:" + uncLog,
	}
	if p.inc != "" {
		args = append(args, "/inc:"+p.inc)
	}
	// MetaEditor's exit code is unreliable — don't gate on it, parse the log.
	// But keep prlctl's output so a genuine exec failure (bad VM, wrong
	// metaeditor path, share not mounted) can be surfaced if no log appears.
	prlOut, _ := exec.CommandContext(ctx, "prlctl", args...).CombinedOutput()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	raw, readErr := os.ReadFile(logPath)
	defer os.Remove(logPath)
	if readErr != nil {
		return nil, fmt.Errorf("compile produced no log (%s): %w\nprlctl exec output:\n%s\ncommand: prlctl %s",
			logPath, readErr, strings.TrimSpace(string(prlOut)), strings.Join(args, " "))
	}

	findings := parseLog(path, raw)
	if len(findings) == 0 {
		findings = append(findings, finding.Finding{
			File:     path,
			Severity: finding.Info,
			RuleID:   "compile/clean",
			Message:  "Compiled with no diagnostics.",
			Pass:     finding.PassCompile,
		})
	}
	return findings, nil
}

// toGuest translates a Mac path under $HOME to its guest UNC form, e.g.
// /Users/andoch/mqls/x.mq5 -> \\Mac\Home\mqls\x.mq5.
func (p *Parallels) toGuest(abs string) string {
	rel, _ := filepath.Rel(p.home, abs)
	return p.homeShare + `\` + strings.ReplaceAll(rel, "/", `\`)
}
