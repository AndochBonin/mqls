package compile

import (
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/AndochBonin/mqls/internal/finding"
)

var severityMap = map[string]finding.Severity{
	"error":       finding.Error,
	"warning":     finding.Warning,
	"information": finding.Info,
}

// parseDiag parses one MetaEditor diagnostic line, e.g.
//
//	strategy.mq5(22,7) : error 145: 'foo' - undeclared identifier
//
// into "location : severity code: message". It reports ok=false for any line
// that isn't a diagnostic (compile progress, "Result:" summaries, blanks). The
// logged path is discarded; findings are anchored to the caller's source arg.
func parseDiag(source, line string) (finding.Finding, bool) {
	loc, rest, ok := strings.Cut(line, " : ")
	if !ok {
		return finding.Finding{}, false
	}

	// loc ends with "(row,col)"; pull the numbers out of the last parens.
	open := strings.LastIndexByte(loc, '(')
	if open < 0 || !strings.HasSuffix(loc, ")") {
		return finding.Finding{}, false
	}
	rowStr, colStr, ok := strings.Cut(loc[open+1:len(loc)-1], ",")
	if !ok {
		return finding.Finding{}, false
	}
	row, err1 := strconv.Atoi(rowStr)
	col, err2 := strconv.Atoi(colStr)
	if err1 != nil || err2 != nil {
		return finding.Finding{}, false
	}

	// rest is "severity code: message".
	head, msg, ok := strings.Cut(rest, ": ")
	if !ok {
		return finding.Finding{}, false
	}
	severity, code, ok := strings.Cut(head, " ")
	sev, known := severityMap[severity]
	if !ok || !known {
		return finding.Finding{}, false
	}

	return finding.Finding{
		File:     source,
		Line:     row,
		Column:   col,
		Severity: sev,
		RuleID:   "compile/" + code,
		Message:  strings.TrimSpace(msg),
		Pass:     finding.PassCompile,
	}, true
}

// parseLog decodes a MetaEditor /log file (UTF-16 with BOM) and turns its
// diagnostic lines into Findings anchored to source. Summary lines ("Result: N
// errors, M warnings"), blank lines, and anything that isn't a diagnostic are
// ignored.
func parseLog(source string, raw []byte) []finding.Finding {
	text := decodeUTF16(raw)
	var out []finding.Finding
	for _, ln := range strings.Split(text, "\n") {
		if f, ok := parseDiag(source, strings.TrimRight(ln, "\r")); ok {
			out = append(out, f)
		}
	}
	return out
}

// decodeUTF16 converts a MetaEditor log to a Go string. The log is UTF-16LE with
// a BOM; if no UTF-16 BOM is present the bytes are treated as UTF-8 (also covers
// the empty case).
// ponytail: UTF-16LE only, add BE handling if a locale ever emits it.
func decodeUTF16(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE { // UTF-16LE BOM
		b = b[2:]
		u16 := make([]uint16, 0, len(b)/2)
		for i := 0; i+1 < len(b); i += 2 {
			u16 = append(u16, uint16(b[i])|uint16(b[i+1])<<8)
		}
		return string(utf16.Decode(u16))
	}
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF { // UTF-8 BOM
		b = b[3:]
	}
	return string(b)
}
