package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AndochBonin/mqls/internal/finding"
)

// hasRule reports whether any finding has the given rule id.
func hasRule(fs []finding.Finding, id string) bool {
	for _, f := range fs {
		if f.RuleID == id {
			return true
		}
	}
	return false
}

func TestFootgunFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "footguns.mq5")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rep := Run(path, string(raw))

	want := []string{
		"mql4-api/order-select",
		"mql4-api/order-lots",
		"mql4-api/order-close",
		"mql4-api/order-ticket",
		"mql4-api/ordersend-signature",
		"event-handler/oninit-params",
		"event-handler/ondeinit-signature",
		"predefined-var/bid",
		"predefined-var/ask",
	}
	for _, id := range want {
		if !hasRule(rep.Findings, id) {
			t.Errorf("expected a finding with rule_id %q, none found", id)
		}
	}
	if rep.OK {
		t.Error("report should not be OK: fixture contains error-severity findings")
	}
}

func TestNoFalsePositivesInCommentsAndStrings(t *testing.T) {
	src := `
void OnTick()
{
   // OrderClose OrderSelect OrderModify — all in a comment
   string s = "OrderDelete(1); OrderClose(2);";
   /* OrderLots() block comment */
}
`
	rep := Run("x.mq5", src)
	for _, f := range rep.Findings {
		if strings.HasPrefix(f.RuleID, "mql4-api/") {
			t.Errorf("false positive in comment/string: %s at %d:%d", f.RuleID, f.Line, f.Column)
		}
	}
}

func TestCleanFileHasNoFindings(t *testing.T) {
	src := `
int OnInit()          { return(INIT_SUCCEEDED); }
void OnDeinit(const int reason) {}
void OnTick()
{
   double bid = SymbolInfoDouble(_Symbol, SYMBOL_BID);
   MqlTradeRequest req; MqlTradeResult res;
   OrderSend(req, res);
}
`
	rep := Run("clean.mq5", src)
	if len(rep.Findings) != 0 {
		t.Errorf("clean file produced %d finding(s): %+v", len(rep.Findings), rep.Findings)
	}
	if !rep.OK {
		t.Error("clean file should be OK")
	}
}

// A class method sharing a handler name must not be mistaken for the global
// event handler (top-level-only detection).
func TestEventHandlerIgnoresClassMethod(t *testing.T) {
	src := `
class Strategy
{
public:
   void OnTick(int x) { }
};

void OnTick() { }
`
	rep := Run("m.mq5", src)
	if hasRule(rep.Findings, "event-handler/ontick-params") {
		t.Errorf("class method OnTick(int) wrongly flagged as the global handler: %+v", rep.Findings)
	}
}

// OnDeinit must take exactly one parameter; a second one is an error.
func TestEventHandlerDeinitArity(t *testing.T) {
	bad := Run("d.mq5", "void OnDeinit(const int reason, double extra) { }")
	if !hasRule(bad.Findings, "event-handler/ondeinit-signature") {
		t.Errorf("OnDeinit with two params should be flagged, got %+v", bad.Findings)
	}

	good := Run("d.mq5", "void OnDeinit(const int reason) { }")
	if hasRule(good.Findings, "event-handler/ondeinit-signature") {
		t.Errorf("correctly-signed OnDeinit should not be flagged, got %+v", good.Findings)
	}
}

// The parameter list is extracted with balanced parens, not truncated at the
// first ')', so a nested paren in the args is captured whole.
func TestEventHandlerNestedParens(t *testing.T) {
	// The bad OnInit arg list contains inner parens; the full list must be
	// reported, proving the capture did not stop at the first ')'.
	rep := Run("n.mq5", "int OnInit(int a = (1 + 2)) { return 0; }")
	var msg string
	for _, f := range rep.Findings {
		if f.RuleID == "event-handler/oninit-params" {
			msg = f.Message
		}
	}
	if msg == "" {
		t.Fatalf("expected event-handler/oninit-params finding, got %+v", rep.Findings)
	}
	if !strings.Contains(msg, "(1 + 2)") {
		t.Errorf("nested-paren arg list truncated; message was %q", msg)
	}
}

// The OnTester* tester callbacks all take no parameters; a param is an error.
func TestEventHandlerTesterParams(t *testing.T) {
	bad := Run("t.mq5", "double OnTester(int x) { return 0; }")
	if !hasRule(bad.Findings, "event-handler/ontester-params") {
		t.Errorf("OnTester with a param should be flagged, got %+v", bad.Findings)
	}
	good := Run("t.mq5", "double OnTester() { return 0; }")
	if hasRule(good.Findings, "event-handler/ontester-params") {
		t.Errorf("param-less OnTester should not be flagged, got %+v", good.Findings)
	}
}

func TestStructuralBalance(t *testing.T) {
	rep := Run("b.mq5", "void OnTick() { if(x) { ")
	if !hasRule(rep.Findings, "struct/unclosed-brace") {
		t.Errorf("expected unclosed-brace finding, got %+v", rep.Findings)
	}
}
