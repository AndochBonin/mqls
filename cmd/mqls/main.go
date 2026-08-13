// Command mqls is the validation harness CLI for AI-generated MQL5 code.
//
// Usage:
//
//	mqls validate <file.mq5> [--json]   run the fast static lint pass
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
	case "validate":
		return doValidate(path, jsonOut)
	case "compile":
		return doCompile(path, jsonOut)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage()
		return 2
	}
}

func doValidate(path string, jsonOut bool) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fail(path, err, jsonOut)
	}
	report := lint.Run(path, string(raw))
	return emit(report, jsonOut)
}

func doCompile(path string, jsonOut bool) int {
	if _, err := os.Stat(path); err != nil {
		return fail(path, err, jsonOut)
	}
	backend := compile.Default()
	findings, err := backend.Compile(context.Background(), path)
	if err != nil {
		return fail(path, err, jsonOut)
	}
	return emit(finding.NewReport(path, findings), jsonOut)
}

func emit(report finding.Report, jsonOut bool) int {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		printHuman(report)
	}
	if !report.OK {
		return 1
	}
	return 0
}

func printHuman(r finding.Report) {
	if len(r.Findings) == 0 {
		fmt.Printf("OK  %s — no findings\n", r.File)
		return
	}
	for _, f := range r.Findings {
		fmt.Printf("%-7s %s:%d:%d  [%s] %s\n",
			f.Severity, f.File, f.Line, f.Column, f.RuleID, f.Message)
		if f.Suggest != "" {
			fmt.Printf("        ↳ %s\n", f.Suggest)
		}
	}
	status := "PASS"
	if !r.OK {
		status = "FAIL"
	}
	fmt.Printf("%s  %s — %d finding(s)\n", status, r.File, len(r.Findings))
}

// fail emits a single error finding for an I/O-level problem.
func fail(path string, err error, jsonOut bool) int {
	report := finding.NewReport(path, []finding.Finding{{
		File:     path,
		Severity: finding.Error,
		RuleID:   "io/read-error",
		Message:  err.Error(),
		Pass:     finding.PassStatic,
	}})
	emit(report, jsonOut)
	return 1
}

func usage() {
	fmt.Fprint(os.Stderr, `mqls — MQL5 validation harness

usage:
  mqls validate <file.mq5> [--json]   fast static lint pass
  mqls compile  <file.mq5> [--json]   authoritative compile pass
`)
}
