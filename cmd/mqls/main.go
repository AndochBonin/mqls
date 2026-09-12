// Command mqls is the validation harness CLI for AI-generated MQL5 code.
//
// Usage:
//
//	mqls lint <file.mq5> [--json]   run the fast static lint pass
//	mqls compile  <file.mq5> [--json]   run the authoritative compile pass
//
// Every command emits the same Report schema. With --json the output is a
// single JSON object designed to be consumed directly by an LLM agent; without
// it, a compact human-readable summary is printed. The process exits non-zero
// when the report contains any error-severity finding, so it composes in
// scripts and CI.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/AndochBonin/mqls/internal/compile"
	"github.com/AndochBonin/mqls/internal/finding"
	"github.com/AndochBonin/mqls/internal/lint"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 2 {
		usage()
		return 2
	}
	cmd, rest := args[0], args[1:]

	var path string
	jsonOut := false
	for _, a := range rest {
		switch a {
		case "--json":
			jsonOut = true
		default:
			if path == "" {
				path = a
			}
		}
	}
	if path == "" {
		usage()
		return 2
	}

	switch cmd {
	case "lint":
		return doLint(path, jsonOut)
	case "compile":
		return doCompile(path, jsonOut)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage()
		return 2
	}
}

func doLint(path string, jsonOut bool) int {
	pass := "Lint"
	raw, err := os.ReadFile(path)
	if err != nil {
		return fail(pass, path, err, jsonOut)
	}
	report := lint.Run(path, string(raw))
	return emit(pass, report, jsonOut)
}

func doCompile(path string, jsonOut bool) int {
	pass := "Compile"
	if _, err := os.Stat(path); err != nil {
		return fail(pass, path, err, jsonOut)
	}

	backend := compile.Default()
	if backend.Available() {
		slog.Info(pass, "backend", backend.Name(), "status", "available")
	} else {
		slog.Warn(pass, "backend", backend.Name(), "status", "unavailable")
	}

	findings, err := backend.Compile(context.Background(), path)
	if err != nil {
		return fail(pass, path, err, jsonOut)
	}
	return emit("Compile", finding.NewReport(path, findings), jsonOut)
}

func emit(pass string, report finding.Report, jsonOut bool) int {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		logOut(pass, report)
	}
	if !report.OK {
		return 1
	}
	return 0
}

func logOut(pass string, r finding.Report) {
	for _, f := range r.Findings {
		logLine := []any{
			"pass", f.Pass,
			"severity", fmt.Sprintf("%s", f.Severity),
			"file", f.File,
			"line", fmt.Sprintf("%d", f.Line),
			"column", fmt.Sprintf("%d", f.Column),
			"rule", f.RuleID,
			"message", f.Message,
			"suggestion", f.Suggest}

		switch f.Severity {
		case finding.Info:
			slog.Info("Finding", logLine...)
		case finding.Warning:
			slog.Warn("Finding", logLine...)
		case finding.Error:
			slog.Error("Finding", logLine...)
		}
	}

	if r.OK {
		slog.Info(pass, "status", "OK", "file", r.File, "findings", len(r.Findings))
	} else {
		slog.Warn(pass, "status", "FAIL", "file", r.File, "findings", len(r.Findings))
	}
}

// fail emits a single error finding for an I/O-level problem.
func fail(pass string, path string, err error, jsonOut bool) int {
	report := finding.NewReport(path, []finding.Finding{{
		File:     path,
		Severity: finding.Error,
		RuleID:   "io/read-error",
		Message:  err.Error(),
		Pass:     finding.PassStatic,
	}})
	emit(pass, report, jsonOut)
	return 1
}

func usage() {
	fmt.Fprint(os.Stderr, `mqls — MQL5 validation harness

usage:
  mqls lint <file.mq5> [--json]   fast static lint pass
  mqls compile  <file.mq5> [--json]   authoritative compile pass
`)
}
