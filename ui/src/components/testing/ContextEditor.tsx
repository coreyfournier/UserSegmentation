import { useState } from 'react';
import type { InputSchema } from '../../api/types';
import styles from './ContextEditor.module.css';

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

  const updateField = (field: string, raw: string) => {
    const type = allFields.get(field);
    let parsed: unknown = raw;
    if (type === 'number') parsed = Number(raw) || 0;
    else if (type === 'boolean') parsed = raw === 'true';
    else if (type === 'array') parsed = raw.split(',').map((s) => s.trim()).filter(Boolean);
    onChange({ ...value, [field]: parsed });
  };

  return (
    <div>
      <div className={styles.toggle}>
        <button
          className={mode === 'structured' ? 'btn-primary btn-sm' : 'btn-ghost btn-sm'}
          onClick={() => setMode('structured')}
        >
          Structured
        </button>
        <button
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
          {[...allFields.entries()].map(([field, type]) => (
            <div key={field} className="form-group">
              <label>{field} <span className={styles.type}>({type})</span></label>
              <input
                value={
                  Array.isArray(value[field])
                    ? (value[field] as string[]).join(', ')
                    : String(value[field] ?? '')
                }
                onChange={(e) => updateField(field, e.target.value)}
                placeholder={type === 'array' ? 'val1, val2, ...' : type}
              />
            </div>
          ))}
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
