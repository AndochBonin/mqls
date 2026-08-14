# mqlv

a validation harness for AI agents generating MQL5 code.

The consumer is an **LLM agent**, not a human in an editor, so every result is
emitted as a structured `Finding` (file, line, column, severity, `rule_id`,
message, suggestion) that the agent can act on directly in its next turn.

## Two passes

1. **Static pass** (`mqls validate`) — fast, no compiler. A curated set of
   high-signal, near-zero-false-positive rules for the footguns an LLM produces
   most: MQL4↔MQL5 API confusion, wrong event-handler signatures, and
   structural (brace/bracket/paren) balance. Runs in-process, no spawn cost.
2. **Compile pass** (`mqls compile`) — the authoritative oracle. Shells out to a
   real MetaEditor compiler and normalizes its log **into the same `Finding`
   schema**, so the agent sees one contract regardless of which pass fired. The
   compile host (remote Windows / local VM / Wine) is pluggable behind a
   `Backend` interface and currently stubbed (`noop`).

## Usage

```
mqls validate <file.mq5> [--json]   fast static lint pass
mqls compile  <file.mq5> [--json]   authoritative compile pass
```

`--json` emits a single `Report` object for agent consumption; without it a
compact human summary is printed. Exit code is non-zero when the report contains
any error-severity finding.

```
go build -o mqls ./cmd/mqls
./mqls validate strategy.mq5 --json
```

## Report schema

```json
{
  "file": "strategy.mq5",
  "ok": false,
  "findings": [
    {
      "file": "strategy.mq5",
      "line": 22, "column": 7,
      "severity": "error",
      "rule_id": "mql4-api/order-close",
      "message": "OrderClose() is an MQL4-only function and does not exist in MQL5.",
      "suggestion": "Close positions with CTrade::PositionClose(ticket) or an OrderSend() DEAL request.",
      "pass": "static"
    }
  ]
}
```

## Layout

```
cmd/mqls            CLI entry (validate | compile)
internal/finding    shared Finding / Report schema (both passes)
internal/lint       static pass: Rule interface, engine, rule packs
internal/compile    compile pass: Backend interface + noop stub
testdata            footgun fixtures
```

## Adding a rule

Implement `lint.Rule` (`ID()` + `Check(*Source) []finding.Finding`) in one file
under `internal/lint`, register it in `registry` (`lint.go`), and add a fixture
case. Rules match against `Source.Code`, which has comments and string/char
literals blanked to spaces (positions preserved) — so a match is always real
code, never text inside a comment or literal.

### Current rule packs

| Pack | rule_id prefix | Catches |
|------|----------------|---------|
| Structural | `struct/*` | Unbalanced / mismatched `()[]{}` |
| MQL4 API | `mql4-api/*` | MQL4-only order functions & the MQL4 `OrderSend` signature |
| Event handlers | `event-handler/*` | Wrong `OnTick`/`OnInit`/`OnStart`/`OnDeinit` signatures |
| Predefined vars | `predefined-var/*` | MQL4 `Bid`/`Ask`/`Point`/`Digits` used bare in MQL5 |
```
