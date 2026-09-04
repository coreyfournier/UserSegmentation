import { useState } from 'react';
import type { OutputField, OutputSchema } from '../../api/types';
import { evalModeOf, validateLiteralValue } from '../schema/outputSchemaRules';
import styles from './OutputValuesEditor.module.css';

interface Props {
  outputs?: Record<string, string>;
  schema: OutputSchema;
  onChange: (o?: Record<string, string>) => void;
  /** Declares a new field on the segment's schema, so it can be authored inline. */
  onDeclare: (name: string, field: OutputField) => void;
}

const PLACEHOLDER: Record<string, string> = {
  literal: 'constant value',
  template: 'text with ${ tokens }',
  expression: 'one whole expression',
};

export default function OutputValuesEditor({ outputs, schema, onChange, onDeclare }: Props) {
  const [newName, setNewName] = useState('');

  const set = (name: string, raw: string) => {
    const next = { ...(outputs ?? {}) };
    if (raw === '') delete next[name];
    else next[name] = raw;
    onChange(Object.keys(next).length ? next : undefined);
  };

  const declareAndFocus = () => {
    if (!newName || schema[newName]) return;
    // Required stays false so declaring a field mid-edit invalidates nothing.
    onDeclare(newName, { type: 'string' });
    setNewName('');
  };

  const names = Object.keys(schema);

  return (
    <div>
      {names.length > 0 && (
        <div className={styles.grid}>
          {names.map((name) => {
            const field = schema[name];
            const raw = outputs?.[name] ?? '';
            const err = validateLiteralValue(field, raw);
            return (
              <div key={name} style={{ display: 'contents' }}>
                <div className={styles.name} title={`${evalModeOf(field)} · ${field.type}${field.required ? ' · required' : ''}`}>
                  {name}{field.required ? ' *' : ''}
                </div>
                <div>
                  <input
                    value={raw}
                    onChange={(e) => set(name, e.target.value)}
                    placeholder={PLACEHOLDER[evalModeOf(field)]}
                  />
                  {err && <div className={styles.err}>{err}</div>}
                </div>
              </div>
            );
          })}
        </div>
      )}
      <div className={styles.addRow}>
        <input
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              declareAndFocus();
            }
          }}
          placeholder="new output field"
          style={{ fontSize: 11 }}
        />
        <button className="btn-secondary btn-sm" onClick={declareAndFocus} disabled={!newName || !!schema[newName]}>
          + add to schema
        </button>
      </div>
    </div>
  );
}
