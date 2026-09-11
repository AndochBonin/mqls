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

// returnTypeOf returns the return-type token immediately preceding the handler
// name at nameOffset (e.g. "void", "int", "double"), or "" if it can't be
// determined — the byte before the name is punctuation (}, ;, {, )) rather than
// an identifier. code is comment/string-blanked, so this never trips on comments.
func returnTypeOf(code string, nameOffset int) string {
	end := prevNonSpace(code, nameOffset-1)
	if end < 0 || !isIdentByte(code[end]) {
		return ""
	}
	start := end
	for start >= 0 && isIdentByte(code[start]) {
		start--
	}
	return code[start+1 : end+1]
}

type handlerMatch struct {
	args        string
	trimmedArgs string
	offset      int
	ok          bool
}

func (r eventHandlerRule) Check(src *Source) []finding.Finding {
	var out []finding.Finding
	code := src.Code
	handlers := make(map[string]handlerMatch)
	getHandler := func(name string) handlerMatch {
		if match, ok := handlers[name]; ok {
			return match
		}
		args, offset, ok := handlerDef(code, name)
		match := handlerMatch{
			args:        args,
			trimmedArgs: strings.TrimSpace(args),
			offset:      offset,
			ok:          ok,
		}
		handlers[name] = match
		return match
	}

	// Handlers that take no parameters.
	for _, name := range []string{"OnTick", "OnStart", "OnTester", "OnTesterInit", "OnTesterDeinit", "OnTesterPass"} {
		match := getHandler(name)
		if match.ok && (match.trimmedArgs != "" && match.trimmedArgs != "void") {
			line, col := src.posAt(match.offset)
			out = append(out, finding.Finding{
				File: src.Path, Line: line, Column: col,
				Severity: finding.Error,
				RuleID:   "event-handler/" + strings.ToLower(name) + "-params",
				Message:  name + "() must take no parameters; found \"" + match.trimmedArgs + "\".",
				Suggest:  "Declare it as void " + name + "() / void " + name + "(void)",
				Pass:     finding.PassStatic,
			})
		}
	}

	// OnInit takes no parameters and should return int (or void).
	onInit := getHandler("OnInit")
	if onInit.ok && (onInit.trimmedArgs != "" && onInit.trimmedArgs != "void") {
		line, col := src.posAt(onInit.offset)
		out = append(out, finding.Finding{
			File: src.Path, Line: line, Column: col,
			Severity: finding.Error,
			RuleID:   "event-handler/oninit-params",
			Message:  "OnInit() must take no parameters or void parameter; found \"" + onInit.trimmedArgs + "\".",
			Suggest:  "Declare it as int OnInit() / int OnInit(void).",
			Pass:     finding.PassStatic,
		})
	}

	// OnDeinit requires exactly one `const int reason` parameter.
	onDeinit := getHandler("OnDeinit")
	if onDeinit.ok {
		if !deinitParamOK(onDeinit.args) {
			line, col := src.posAt(onDeinit.offset)
			out = append(out, finding.Finding{
				File: src.Path, Line: line, Column: col,
				Severity: finding.Error,
				RuleID:   "event-handler/ondeinit-signature",
				Message:  "OnDeinit() must take a single 'const int' parameter; found \"" + onDeinit.trimmedArgs + "\".",
				Suggest:  "Declare it as void OnDeinit(const int reason).",
				Pass:     finding.PassStatic,
			})
		}
	}

	// Each handler has a fixed expected return type. A wrong return type, like a
	// wrong parameter list, makes the terminal skip the handler silently.
	for _, h := range []struct{ name, want string }{
		{"OnTick", "void"}, {"OnStart", "void"}, {"OnInit", "int"}, {"OnDeinit", "void"},
		{"OnTester", "double"},
		{"OnTesterInit", "void"}, {"OnTesterDeinit", "void"}, {"OnTesterPass", "void"},
	} {
		match := getHandler(h.name)
		if !match.ok {
			continue
		}
		got := returnTypeOf(code, match.offset)
		if got == "" || got == h.want {
			continue // undeterminable → skip (no false positive); or already correct
		}
		if h.name == "OnInit" && got == "void" {
			continue // int or void both valid for OnInit
		}
		line, col := src.posAt(match.offset)
		want := h.want
		suggest := "Declare it as " + h.want + " " + h.name + "(...)."
		if h.name == "OnInit" {
			want = "int (or void)"
			suggest = "Declare it as int " + h.name + "(...) or void " + h.name + "(...)."
		}
		out = append(out, finding.Finding{
			File: src.Path, Line: line, Column: col,
			Severity: finding.Error,
			RuleID:   "event-handler/" + strings.ToLower(h.name) + "-return",
			Message:  h.name + "() must return " + want + "; found \"" + got + "\".",
			Suggest:  suggest,
			Pass:     finding.PassStatic,
		})
	}
	return out
}

// deinitParamOK reports whether the OnDeinit parameter list is exactly one
// `const int <id>` parameter, optionally with a default value. A second
// parameter (a comma) or a different type is rejected.
func deinitParamOK(args string) bool {
	if strings.Contains(args, ",") {
		return false // more than one parameter
	}
	parts := strings.SplitN(args, "=", 2)
	fields := strings.Fields(parts[0])
	// Expect exactly: const int <name>.
	if len(fields) != 3 || fields[0] != "const" || fields[1] != "int" {
		return false
	}
	if len(parts) == 1 {
		return true
	}
	defaultValue := strings.TrimSpace(parts[1])
	return defaultValue != "" && !strings.Contains(defaultValue, "=")
}
