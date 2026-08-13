package lint 

import (
	"regexp"

	"github.com/AndochBonin/mqls/internal/finding"
)

// predefinedVarRule flags MQL4 predefined variables that are NOT predefined in
// MQL5. In MQL4 you can write bare `Bid`, `Ask`, `Point`, `Digits`; in MQL5
// `Bid`/`Ask` do not exist at all, and `Point`/`Digits` became the macros
// `_Point`/`_Digits` (bare `Point`/`Digits` are undeclared). LLMs reproduce the
// MQL4 spellings constantly.
//
// These are Warnings, not Errors: a user could legitimately declare their own
// variable named `Bid`. The message tells the agent how to disambiguate.
type predefinedVarRule struct{}

func (predefinedVarRule) ID() string { return "predefined-var" }

var mql4Predefined = map[string]struct {
	slug    string
	suggest string
}{
	"Bid":    {"bid", "MQL5 has no predefined Bid. Use SymbolInfoDouble(_Symbol, SYMBOL_BID) or read a MqlTick via SymbolInfoTick()."},
	"Ask":    {"ask", "MQL5 has no predefined Ask. Use SymbolInfoDouble(_Symbol, SYMBOL_ASK) or read a MqlTick via SymbolInfoTick()."},
	"Point":  {"point", "In MQL5 use the predefined macro _Point (or SymbolInfoDouble(_Symbol, SYMBOL_POINT)). Bare 'Point' is undeclared."},
	"Digits": {"digits", "In MQL5 use the predefined macro _Digits (or SymbolInfoInteger(_Symbol, SYMBOL_DIGITS)). Bare 'Digits' is undeclared."},
}

func (r predefinedVarRule) Check(src *Source) []finding.Finding {
	var out []finding.Finding
	code := src.Code
	for name, meta := range mql4Predefined {
		// A bare identifier reference: the name not immediately followed by '('
		// (that would be a function/macro call, e.g. a user's own Point()) and
		// not a member/scope access.
		re := regexp.MustCompile(`\b` + name + `\b`)
		for _, loc := range re.FindAllStringIndex(code, -1) {
			start, end := loc[0], loc[1]
			// Skip if used as a call `name(`.
			if n := nextNonSpace(code, end); n >= 0 && code[n] == '(' {
				continue
			}
			// Skip member/scope access.
			if p := prevNonSpace(code, start-1); p >= 0 {
				if code[p] == '.' {
					continue
				}
				if code[p] == ':' && p >= 1 && code[p-1] == ':' {
					continue
				}
			}
			line, col := src.posAt(start)
			out = append(out, finding.Finding{
				File: src.Path, Line: line, Column: col,
				Severity: finding.Warning,
				RuleID:   "predefined-var/" + meta.slug,
				Message:  "'" + name + "' is an MQL4 predefined variable and is not predefined in MQL5.",
				Suggest:  meta.suggest,
				Pass:     finding.PassStatic,
			})
		}
	}
	return out
}

// nextNonSpace returns the index of the nearest non-space byte at or after i,
// or -1 if none.
func nextNonSpace(s string, i int) int {
	for ; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			return i
		}
	}
	return -1
}
