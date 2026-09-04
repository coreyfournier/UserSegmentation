/**
 * What the evaluation response already carries without any declaration.
 *
 * Shown read-only beside the output schema editor so an author declares only
 * what is missing instead of restating the baseline. These are not values an
 * output expression can read — outputs resolve inside the strategy, before the
 * layer result exists — so this is documentation, not a field picker.
 */
const PER_LAYER = [
  ['status', 'satisfied / violated / unevaluable for a checklist; resolved / unresolved / skipped otherwise'],
  ['segment', 'the segment value a single-value strategy resolved (blank for a checklist)'],
  ['strategy', 'which strategy produced the result, or "override"'],
  ['reason', 'why, e.g. rule:<name> or checklist:<segment>'],
  ['computed', 'the segment’s computed fields, as evaluated'],
  ['messages', 'rendered localized messages for the winning rule'],
  ['outputs', 'the record your declared fields below produce'],
];

const PER_RESPONSE = [
  ['subject_key', 'the subject that was evaluated'],
  ['warnings', 'required inputs missing from context, render errors, absent required outputs'],
  ['evaluated_at', 'RFC3339 timestamp'],
  ['duration_us', 'evaluation time in microseconds'],
];

export default function EmittedFieldsReference() {
  return (
    <details>
      <summary style={{ cursor: 'pointer', fontSize: 12, color: 'var(--text-muted)' }}>
        Always emitted — no declaration needed
      </summary>
      <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 8 }}>
        <p style={{ margin: '0 0 6px' }}>Per layer:</p>
        <ul style={{ margin: '0 0 10px', paddingLeft: 18 }}>
          {PER_LAYER.map(([f, d]) => (
            <li key={f}><code>{f}</code> — {d}</li>
          ))}
        </ul>
        <p style={{ margin: '0 0 6px' }}>On the response:</p>
        <ul style={{ margin: 0, paddingLeft: 18 }}>
          {PER_RESPONSE.map(([f, d]) => (
            <li key={f}><code>{f}</code> — {d}</li>
          ))}
        </ul>
      </div>
    </details>
  );
}
