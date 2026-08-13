package lint 

import "github.com/AndochBonin/mqls/internal/finding"

// mql4APIRule flags MQL4-only trade/order functions that do not exist in MQL5.
// This is the single highest-ROI static check: LLMs constantly emit MQL4 order
// functions in "MQL5" code because their training data blends both dialects,
// and the compiler's own error for these ("undeclared identifier") does a poor
// job of explaining the MQL4/MQL5 split.
type mql4APIRule struct{}

func (mql4APIRule) ID() string { return "mql4-api" }

// mql4Only maps an MQL4-only free function to (rule slug, guidance). These are
// the classic order-pool functions removed entirely in MQL5.
var mql4Only = map[string]struct {
	slug    string
	suggest string
}{
	"OrderSelect":      {"order-select", "MQL5 has no order pool; iterate positions with PositionSelectByTicket / PositionGetTicket, or use the CTrade/CPositionInfo classes."},
	"OrderClose":       {"order-close", "Close positions with CTrade::PositionClose(ticket) or an OrderSend() DEAL request."},
	"OrderModify":      {"order-modify", "Modify with CTrade::PositionModify() or an OrderSend() SLTP/MODIFY request."},
	"OrderDelete":      {"order-delete", "Delete pending orders with CTrade::OrderDelete(ticket) or an OrderSend() REMOVE request."},
	"OrderLots":        {"order-lots", "Use PositionGetDouble(POSITION_VOLUME) after selecting the position."},
	"OrderOpenPrice":   {"order-openprice", "Use PositionGetDouble(POSITION_PRICE_OPEN)."},
	"OrderClosePrice":  {"order-closeprice", "MQL5 positions have no 'close price' until closed; use the current price via SymbolInfoDouble."},
	"OrderStopLoss":    {"order-stoploss", "Use PositionGetDouble(POSITION_SL)."},
	"OrderTakeProfit":  {"order-takeprofit", "Use PositionGetDouble(POSITION_TP)."},
	"OrderTicket":      {"order-ticket", "Use PositionGetTicket(i) / PositionGetInteger(POSITION_TICKET)."},
	"OrderType":        {"order-type", "Use PositionGetInteger(POSITION_TYPE) (values POSITION_TYPE_BUY/SELL)."},
	"OrderProfit":      {"order-profit", "Use PositionGetDouble(POSITION_PROFIT)."},
	"OrderMagicNumber": {"order-magic", "Use PositionGetInteger(POSITION_MAGIC)."},
	"OrderSymbol":      {"order-symbol", "Use PositionGetString(POSITION_SYMBOL)."},
	"OrderComment":     {"order-comment", "Use PositionGetString(POSITION_COMMENT)."},
	"OrderOpenTime":    {"order-opentime", "Use PositionGetInteger(POSITION_TIME)."},
	"OrderCloseTime":   {"order-closetime", "MQL5 open positions have no close time; query deal history via HistoryDealGetInteger."},
}

func (r mql4APIRule) Check(src *Source) []finding.Finding {
	var out []finding.Finding
	for name, meta := range mql4Only {
		for _, m := range findCalls(src, name) {
			line, col := src.posAt(m.offset)
			out = append(out, finding.Finding{
				File: src.Path, Line: line, Column: col,
				Severity: finding.Error,
				RuleID:   "mql4-api/" + meta.slug,
				Message:  name + "() is an MQL4-only function and does not exist in MQL5.",
				Suggest:  meta.suggest,
				Pass:     finding.PassStatic,
			})
		}
	}

	// OrderSend exists in MQL5 but with a completely different signature:
	// OrderSend(MqlTradeRequest&, MqlTradeResult&). The MQL4 form passes a
	// symbol string and many scalar args, so a scalar-heavy call is a strong
	// MQL4 signal.
	for _, m := range findCalls(src, "OrderSend") {
		if looksLikeMQL4OrderSend(m.args) {
			line, col := src.posAt(m.offset)
			out = append(out, finding.Finding{
				File: src.Path, Line: line, Column: col,
				Severity: finding.Error,
				RuleID:   "mql4-api/ordersend-signature",
				Message:  "OrderSend() is being called with the MQL4 signature. MQL5 uses OrderSend(MqlTradeRequest &request, MqlTradeResult &result).",
				Suggest:  "Fill an MqlTradeRequest struct (action, symbol, volume, type, price, sl, tp) and call OrderSend(request, result) — or use the CTrade class.",
				Pass:     finding.PassStatic,
			})
		}
	}
	return out
}

// looksLikeMQL4OrderSend heuristically decides whether an OrderSend argument
// list is the MQL4 form. The MQL5 form has exactly two struct args; the MQL4
// form has 7+ comma-separated scalars. We require a comfortably high arg count
// to keep false positives near zero.
func looksLikeMQL4OrderSend(args string) bool {
	if args == "" {
		return false
	}
	depth := 0
	commas := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				commas++
			}
		}
	}
	return commas >= 5 // >= 6 top-level args
}
