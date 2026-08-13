package lint 

import (
	"regexp"
	"strings"

	"github.com/AndochBonin/mqls/internal/finding"
)

// eventHandlerRule validates the signatures of the standard MQL5 event
// handlers. Getting these wrong (extra params on OnTick, a param-less OnDeinit)
// is a frequent LLM mistake that produces confusing "no such handler" behaviour
// rather than a clean compile error, because the terminal simply never calls a
// mis-signed handler.
type eventHandlerRule struct{}

func (eventHandlerRule) ID() string { return "event-handler" }

// A handler definition is a top-level `... Name ( args ) {`. We match the name
// followed by parens and an opening brace (allowing a return type / newline in
// between) to distinguish a definition from a call.
func handlerDef(code, name string) (args string, offset int, ok bool) {
	re := regexp.MustCompile(`\b` + name + `\s*\(([^)]*)\)\s*\{`)
	loc := re.FindStringSubmatchIndex(code)
	if loc == nil {
		return "", 0, false
	}
	return code[loc[2]:loc[3]], loc[0], true
}

func (r eventHandlerRule) Check(src *Source) []finding.Finding {
	var out []finding.Finding
	code := src.Code

	// Handlers that take no parameters.
	for _, name := range []string{"OnTick", "OnStart"} {
		if args, off, ok := handlerDef(code, name); ok && strings.TrimSpace(args) != "" {
			line, col := src.posAt(off)
			out = append(out, finding.Finding{
				File: src.Path, Line: line, Column: col,
				Severity: finding.Error,
				RuleID:   "event-handler/" + strings.ToLower(name) + "-params",
				Message:  name + "() must take no parameters; found \"" + strings.TrimSpace(args) + "\".",
				Suggest:  "Declare it as void " + name + "().",
				Pass:     finding.PassStatic,
			})
		}
	}

	// OnInit takes no parameters and should return int (or void).
	if args, off, ok := handlerDef(code, "OnInit"); ok && strings.TrimSpace(args) != "" {
		line, col := src.posAt(off)
		out = append(out, finding.Finding{
			File: src.Path, Line: line, Column: col,
			Severity: finding.Error,
			RuleID:   "event-handler/oninit-params",
			Message:  "OnInit() must take no parameters; found \"" + strings.TrimSpace(args) + "\".",
			Suggest:  "Declare it as int OnInit().",
			Pass:     finding.PassStatic,
		})
	}

	// OnDeinit requires exactly one `const int reason` parameter.
	if args, off, ok := handlerDef(code, "OnDeinit"); ok {
		if !deinitParamOK(args) {
			line, col := src.posAt(off)
			out = append(out, finding.Finding{
				File: src.Path, Line: line, Column: col,
				Severity: finding.Error,
				RuleID:   "event-handler/ondeinit-signature",
				Message:  "OnDeinit() must take a single 'const int' parameter; found \"" + strings.TrimSpace(args) + "\".",
				Suggest:  "Declare it as void OnDeinit(const int reason).",
				Pass:     finding.PassStatic,
			})
		}
	}
	return out
}

// deinitParamOK reports whether the OnDeinit parameter list is `const int <id>`.
func deinitParamOK(args string) bool {
	fields := strings.Fields(args)
	// Expect: const int reason  (3 tokens) — accept const int <name>.
	if len(fields) < 2 {
		return false
	}
	return fields[0] == "const" && fields[1] == "int"
}
