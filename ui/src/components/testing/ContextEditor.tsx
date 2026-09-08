import { useState } from 'react';
import type { InputSchema } from '../../api/types';
import { displayContextValue, parseContextValue } from '../../utils/parse';
import styles from './ContextEditor.module.css';

/** What a boolean field's select offers, beyond an unparseable current value. */
const BOOL_CHOICES = ['', 'true', 'false'];

interface Props {
  value: Record<string, unknown>;
  onChange: (ctx: Record<string, unknown>) => void;
  schemas: InputSchema[];
}

export default function ContextEditor({ value, onChange, schemas }: Props) {
  const [mode, setMode] = useState<'structured' | 'json'>('structured');
  const [jsonText, setJsonText] = useState(JSON.stringify(value, null, 2));

  // Merge all schemas for structured mode
  const allFields = new Map<string, string>();
  for (const schema of schemas) {
    for (const [field, sf] of Object.entries(schema)) {
      allFields.set(field, sf.type);
    }
  }

  const handleJsonBlur = () => {
    try {
      onChange(JSON.parse(jsonText));
    } catch {
      // keep invalid JSON in textarea, don't update
    }
  };

  /**
   * What was actually typed into each text field.
   *
   * A controlled box that shows `display(parse(text))` can only be typed into
   * while that composition returns what was typed, and for these types it does
   * not: `"a,"` parses to `["a"]` and renders as `"a"`, so an array's separator
   * vanishes under the cursor; `"0.0"` parses to 0 and renders as `"0"`, so a
   * decimal with a zero after the point is unreachable. Rather than chase each
   * case with a smarter parse — parseNumericInput's raw-passthrough is that
   * attempt, and it does not cover trailing zeros — the box shows what was
   * typed and the parsed form goes to the context beside it.
   */
  const [drafts, setDrafts] = useState<Record<string, string>>({});

  /**
   * The context this editor last accounted for — either what arrived as
   * `value` or what it is about to emit.
   *
   * A context it did not produce is a different test being selected, or JSON
   * mode replacing everything, and the raw text above belongs to neither. So
   * every emission registers itself here first; anything else that arrives is
   * external and clears the drafts.
   *
   * Adjusted during render rather than in an effect: this is React's own
   * pattern for resetting state when a prop changes, and it avoids the frame
   * where the new test is on screen holding the previous one's text.
   */
  const [seen, setSeen] = useState(value);
  if (value !== seen) {
    setSeen(value);
    setDrafts({});
  }

  const emit = (next: Record<string, unknown>) => {
    setSeen(next);
    onChange(next);
  };

  const setField = (field: string, parsed: unknown) => emit({ ...value, [field]: parsed });

  // Removes the key rather than setting a falsy value. A field that is absent
  // from the context is a distinct case worth testing — it is what is_null
  // matches, and what a required-input warning reports.
  const clearField = (field: string) => {
    const next = { ...value };
    delete next[field];
    emit(next);
  };

  const updateField = (field: string, raw: string) => {
    setDrafts((d) => ({ ...d, [field]: raw }));
    setField(field, parseContextValue(allFields.get(field), raw));
  };

  return (
    <div>
      <div className={styles.toggle}>
        <button type="button"
          className={mode === 'structured' ? 'btn-primary btn-sm' : 'btn-ghost btn-sm'}
          onClick={() => setMode('structured')}
        >
          Structured
        </button>
        <button type="button"
          className={mode === 'json' ? 'btn-primary btn-sm' : 'btn-ghost btn-sm'}
          onClick={() => {
            setJsonText(JSON.stringify(value, null, 2));
            setMode('json');
          }}
        >
          JSON
        </button>
      </div>

      {mode === 'structured' ? (
        <div className={styles.fields}>
          {[...allFields.entries()].map(([field, type]) => {
            // A boolean has two values, so it gets a picker rather than a text
            // box. As free text it was unusable: the parse was `raw === 'true'`,
            // so the first letter of "true" became false, the box repainted as
            // "false", and no further typing could reach true again.
            const current = value[field] === undefined ? '' : String(value[field]);
            return (
              <div key={field} className="form-group">
                <label>{field} <span className={styles.type}>({type})</span></label>
                {type === 'boolean' ? (
                  <select
                    value={current}
                    onChange={(e) =>
                      e.target.value === ''
                        ? clearField(field)
                        : setField(field, e.target.value === 'true')
                    }
                  >
                    <option value="">(not set)</option>
                    <option value="true">true</option>
                    <option value="false">false</option>
                    {/* JSON mode can leave a value here that is not a boolean
                        at all. Offering it keeps the select honest: one whose
                        value matches no option displays the first instead, so
                        the field would read "(not set)" while holding "T" —
                        and picking that option fires no change event, leaving
                        it uncorrectable. */}
                    {!BOOL_CHOICES.includes(current) && (
                      <option value={current}>{current} — not a boolean</option>
                    )}
                  </select>
                ) : (
                  <input
                    value={drafts[field] ?? displayContextValue(type, value[field])}
                    onChange={(e) => updateField(field, e.target.value)}
                    placeholder={type === 'array' ? 'val1, val2, ...' : type}
                  />
                )}
              </div>
            );
          })}
          {allFields.size === 0 && (
            <p className={styles.hint}>No input schemas defined. Use JSON mode to set context.</p>
          )}
        </div>
      ) : (
        <textarea
          className={styles.json}
          value={jsonText}
          onChange={(e) => setJsonText(e.target.value)}
          onBlur={handleJsonBlur}
          rows={10}
        />
      )}
    </div>
  );
}
