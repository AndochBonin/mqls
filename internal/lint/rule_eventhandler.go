package lint

import (
	"github.com/AndochBonin/mqls/internal/finding"
	"regexp"
	"strings"
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
// between) to distinguish a definition from a call. We consider every
// occurrence and return the first genuine top-level definition, skipping
// nested occurrences such as a class method with the same name.
func handlerDef(code, name string) (args string, offset int, ok bool) {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\(`)
	for _, loc := range re.FindAllStringIndex(code, -1) {
		openParen := loc[1] - 1
		a := extractArgs(code, openParen)
		if a == "" && !balancedEmptyArgs(code, openParen) {
			// Unbalanced parens: not a usable definition.
			continue
		}
		// The byte after the outermost close paren must be `{` (a definition,
		// not a bare call). extractArgs returns code[openParen+1:close].
		closeParen := openParen + 1 + len(a)
		if n := nextNonSpace(code, closeParen+1); n < 0 || code[n] != '{' {
			continue
		}
		// Only top-level definitions count; a class method or nested function
		// with the same name lives at brace depth > 0.
		if braceDepthAt(code, loc[0]) != 0 {
			continue
		}
		return a, loc[0], true
	}
	return "", 0, false
}

// balancedEmptyArgs reports whether the parens opened at openParen close
// immediately (`()`), distinguishing genuine empty args from the unbalanced
// case that extractArgs also signals with "".
func balancedEmptyArgs(code string, openParen int) bool {
	n := nextNonSpace(code, openParen+1)
	return n >= 0 && code[n] == ')'
}

// braceDepthAt returns the net `{` minus `}` count in code[:i]. Because code is
// comment/string-blanked, braces inside comments or literals never count.
func braceDepthAt(code string, i int) int {
	depth := 0
	for j := range i {
		switch code[j] {
		case '{':
			depth++
		case '}':
			depth--
		}
	}
	return depth
}

func (r eventHandlerRule) Check(src *Source) []finding.Finding {
	var out []finding.Finding
	code := src.Code

	// Handlers that take no parameters.
	for _, name := range []string{"OnTick", "OnStart", "OnTester", "OnTesterInit", "OnTesterDeinit", "OnTesterPass"} {
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

// deinitParamOK reports whether the OnDeinit parameter list is exactly one
// `const int <id>` parameter. A second parameter (a comma) or a different type
// is rejected.
func deinitParamOK(args string) bool {
	if strings.Contains(args, ",") {
		return false // more than one parameter
	}
	fields := strings.Fields(args)
	// Expect exactly: const int <name>.
	return len(fields) == 3 && fields[0] == "const" && fields[1] == "int"
}
