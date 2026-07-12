# Localized Messages on Rules, Overrides, and Defaults

**Date:** 2026-07-11
**Status:** Design — pending review

## Summary

Add an optional, localized **message** to every rule, override rule, and segment
default. A message is a template string that supports merged variables and
expressions via `${ … }` interpolation (evaluated with expr-lang, the engine
already in use). Messages are keyed by human-language locale (i18n). At
evaluation time the caller requests one or more languages (or "render all"),
and the winning rule/override/default renders its message(s) into the result.

## Motivating example

The existing CT fee-override segment computes `TransferFee` and then matches
`fee-waived` / `fee-partial` rules. With this feature those rules can carry:

```json
{
  "ruleName": "fee-partial",
  "successEvent": "fee-partial",
  "expression": { "field": "CTTotal", "operator": "gt", "value": 26 },
  "messages": {
    "en": "You'll pay a ${TransferFee} fee on your ${CTTotal} transfer.",
    "es": "Pagarás una tarifa de ${TransferFee} en tu transferencia de ${CTTotal}."
  }
}
```

Because `TransferFee` is a computed expression field, message rendering must run
against the **enriched** context that only exists inside the strategy after
expressions are evaluated.

## Decisions (resolved during brainstorming)

1. **Language means human locale (i18n)**, not template dialect.
2. **Multiple messages per rule** — a map of locale → template.
3. **Request specifies one or more languages**, plus a **`render_all`** mode that
   returns every defined locale (a testing aid).
4. **Missing-locale fallback:** fall back to the **layer's default language**,
   which is auto-set to `"en"` when unset.
5. **Interpolation syntax:** `${ … }`, inner content is any expr-lang expression.
6. **Render error behavior:** leave the raw `${…}` token untouched in the output
   **and** emit a warning in the result (reusing the existing warnings surface).
7. Only the **winning** rule/override/default renders (bounded cost).
8. **Engine evaluation order is unchanged.** Overrides continue to evaluate
   before expressions, so override messages resolve against the raw context
   only. Rather than reorder the engine, the **UI** is reordered so the
   override-first behavior is visually obvious (see UI section).

## Data model (`internal/domain/model`)

- `Rule` — add `Messages map[string]string` (`json:"messages,omitempty"`).
  Serves both rules and overrides (same struct).
- `Segment` — add `DefaultMessages map[string]string`
  (`json:"defaultMessages,omitempty"`) for the segment's `Default` fallback.
- `Layer` — add `DefaultLanguage string` (`json:"defaultLanguage,omitempty"`);
  treated as `"en"` when empty (defaulted at evaluation time).

## Message renderer (new file `internal/domain/strategy/message.go`)

Lives in the `strategy` package so it reuses the existing expr-lang setup —
`mathOptions` (so `${pow(x,2)}` etc. work) and the compile-cache pattern from
`ExpressionStrategy.compiled` — rather than duplicating expr wiring in a new
package.

```
RenderResult { Rendered map[string]string; Errors []RenderError }
RenderError  { Language string; Token string; Err string }

Render(raw map[string]string, env map[string]interface{},
       languages []string, renderAll bool, defaultLang string) RenderResult
```

Behavior:

- **Locale selection:**
  - `renderAll == true` → every key in `raw`.
  - else, for each requested language: use `raw[lang]` if present; else
    `raw[defaultLang]` if present; else omit that language.
- **Interpolation:** scan the selected template for `${ … }` tokens; for each,
  compile+run the inner expression via expr-lang against `env`; substitute the
  stringified result.
- **On token error:** leave the literal `${…}` token in place and append a
  `RenderError{Language, Token, Err}`.

## Evaluation plumbing

- `strategy.EvalContext` gains: `Languages []string`, `RenderAll bool`,
  `DefaultLanguage string`.
- `strategy.Result` gains: `Messages map[string]string` (rendered) and
  `RenderErrors []RenderError`.
- `RuleStrategy.Evaluate` — on a rule match, render `rule.Messages`; on default,
  render `seg.DefaultMessages`. Uses the (possibly enriched) `ctx.Context`.
- `EvalOverrides` — on an override match, render `rule.Messages` (base context;
  overrides run before computed expressions, so only raw context vars resolve).
- `ExpressionStrategy.Evaluate` — propagate `Languages`, `RenderAll`,
  `DefaultLanguage` into the enriched `EvalContext` it constructs, so messages
  can reference computed fields.
- `engine.Evaluator.evaluateLayer` — compute the effective default language
  (`layer.DefaultLanguage` or `"en"`), populate the new `EvalContext` fields,
  copy `Result.Messages` into `model.Assignment.Messages`, and map each
  `RenderError` into a `model.Warning{Segment: seg.ID, Field: lang,
  Message: err}` appended to the layer warnings.
- `engine.Evaluator.Evaluate` — accept `languages []string, renderAll bool`
  (threaded from the request). Batch passes them per-subject.

## Model additions

- `model.Assignment` — add `Messages map[string]string`
  (`json:"messages,omitempty"`).

## Application / API

- `EvaluateRequest` — add `Languages []string` (`json:"languages,omitempty"`)
  and `RenderAll bool` (`json:"render_all,omitempty"`).
- `LayerResultDTO` — add `Messages map[string]string`
  (`json:"messages,omitempty"`).
- `EvaluateUseCase.Execute` and the batch use case pass the new options into the
  evaluator and copy `Assignment.Messages` into `LayerResultDTO.Messages`.
  Render-error warnings flow through the existing `WarningDTO` path.
- Handlers require no new validation beyond decoding the new optional fields.

## Validation (`internal/domain/validation`)

Light for v1: no hard failure on message content (bad expressions surface as
runtime warnings, per decision 6). Optionally normalize an empty
`Layer.DefaultLanguage` to `"en"` at load, or leave defaulting to eval time
(chosen: eval time, to keep the loader untouched).

## UI (`ui/`)

### Reorder the segment editor to match evaluation order

The engine order is **overrides → expressions → rules → default**, but the
expression-strategy config currently renders **expressions → rules → overrides**
(overrides are bundled last inside `RuleConfig`). This is misleading — it
implies overrides can reference computed expression fields, which they cannot
(they run before expressions).

We do **not** change the engine. Instead, decompose the segment config UI so
its top-to-bottom order mirrors evaluation:

1. **Overrides** (first) — with a caption: *"Evaluated first, before expressions
   and rules. Only raw input fields are available here — computed expression
   fields cannot be referenced."* Its field autocomplete uses the raw
   `inputSchema` (**not** the expression-augmented schema), reinforcing the
   limitation.
2. **Expressions** (expression strategy only).
3. **Rules** — field autocomplete uses the effective schema (raw + computed).
4. **Default Value** (+ its `defaultMessages` editor).

This requires splitting `RuleConfig` so overrides, rules, and the default are
independently placeable (overrides rendered above the expressions block).
For the plain `rule` strategy the order is **Overrides → Rules → Default**.

### Message editing + testing

- `types.ts` — mirror all new fields: `Rule.messages?`,
  `Segment.defaultMessages?`, `Layer.defaultLanguage?`,
  `EvaluateRequest.languages?` / `render_all?`, `LayerResult.messages?`.
- `RuleNode.tsx` — a collapsible "Messages" editor per rule: rows of
  `{ locale, text }`. Applies to rules and overrides automatically (shared
  component).
- Default Value — a `defaultMessages` editor beside it (in the decomposed
  default section).
- `LayerForm.tsx` — a "Default language" field (defaults to `en`).
- `TestingZone.tsx` — a comma-separated languages input and a "Render all"
  checkbox, sent on the evaluate request as `languages` / `render_all`.
- `ResultDisplay.tsx` — render each layer's returned `messages` and surface
  render-error warnings.

## Testing

- **Go unit tests** for the renderer: bare variable, expression, missing-locale
  fallback to default language, `renderAll`, and bad-expression (raw token kept
  + error reported).
- **Go strategy/evaluator tests**: messages attached on rule win, override win,
  and default win; computed-field reference resolves via the enriched context;
  render-error produces a warning.
- **UI**: verified via the Docker build (no local Node toolchain on this
  machine).

## Out of scope

- Message catalogs / external translation files (messages live inline in config).
- Pluralization / ICU MessageFormat.
- Per-request locale negotiation beyond explicit language list + `render_all`.
