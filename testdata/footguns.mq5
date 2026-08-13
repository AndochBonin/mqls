//+------------------------------------------------------------------+
//| Deliberately buggy MQL5 file exercising the static lint rules.    |
//| The string "OrderClose" here in a comment must NOT be flagged.    |
//+------------------------------------------------------------------+
#property strict

int OnInit(int badParam)          // event-handler: OnInit takes no params
{
   return(INIT_SUCCEEDED);
}

void OnTick()
{
   string note = "OrderClose is fine inside a string literal";  // must NOT flag

   double price = Bid;            // predefined-var: Bid not in MQL5
   double spread = Ask - Bid;     // predefined-var x2

   if(OrderSelect(0, SELECT_BY_POS))   // mql4-api: order-select
   {
      double lots = OrderLots();       // mql4-api: order-lots
      OrderClose(OrderTicket(), lots, Bid, 3);  // mql4-api: order-close + order-ticket
   }

   // MQL4-style OrderSend signature (many scalar args):
   OrderSend(_Symbol, OP_BUY, 0.1, Ask, 3, 0, 0, "c", 0, 0, clrGreen);
}

void OnDeinit()                   // event-handler: missing const int reason
{
}
