import { useState } from 'react';
import type { OutputField, OutputSchema } from '../../api/types';
import { evalModeOf, validateLiteralValue, type FieldCoverage } from '../schema/outputSchemaRules';
import styles from './OutputValuesEditor.module.css';

interface Props {
  outputs?: Record<string, string>;
  schema: OutputSchema;
  onChange: (o?: Record<string, string>) => void;
  /** Declares a new field on the segment's schema, so it can be authored inline. */
  onDeclare: (name: string, field: OutputField) => void;
  /** How completely each field is authored elsewhere on the segment (its
   *  rules and overrides). Only meaningful when this editor represents a
   *  segment's own values rather than one rule's — omitted by every
   *  per-rule caller. */
  coverage?: (name: string) => FieldCoverage;
}

const PLACEHOLDER: Record<string, string> = {
  literal: 'constant value',
  template: 'text with ${ tokens }',
  expression: 'one whole expression',
};

export default function OutputValuesEditor({ outputs, schema, onChange, onDeclare, coverage }: Props) {
  const [newName, setNewName] = useState('');

  const set = (name: string, raw: string) => {
    const next = { ...(outputs ?? {}) };
    if (raw === '') delete next[name];
    else next[name] = raw;
    onChange(Object.keys(next).length ? next : undefined);
  };

  const declareAndFocus = () => {
    // Trim to match OutputSchemaEditor's add path. Without it " severity" and
    // "severity" slip past the duplicate check as two visually identical
    // fields, and the engine rejects the padded one as an undeclared output
    // key on the first save.
    const name = newName.trim();
    if (!name || schema[name]) return;
    // Required stays false so declaring a field mid-edit invalidates nothing.
    onDeclare(name, { type: 'string' });
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
                    aria-label={name}
                  />
                  {err && <div className={styles.err}>{err}</div>}
                  {coverage && (() => {
                    const c = coverage(name);
                    if (c.segmentLevel) {
                      return <div style={{ fontSize: 10, color: 'var(--text-muted)' }}>set for the whole segment</div>;
                    }
                    const short = c.total - c.authored;
                    const overridesShort = c.overridesTotal - c.overridesAuthored;
                    const extra: string[] = [];
                    if (field.required) {
                      if (c.defaultNeedsSegmentValue) {
                        extra.push('a default is declared, so this must be set for the segment');
                      }
                      if (overridesShort > 0) {
                        extra.push(
                          `${overridesShort} override(s) also need this — only a segment value can satisfy them here`,
                        );
                      }
                    }
                    return (
                      <>
                        <div style={{ fontSize: 10, color: short && field.required ? 'var(--danger)' : 'var(--text-muted)' }}>
                          authored on {c.authored} of {c.total} checks
                          {short && field.required ? ` — ${short} will block saving` : ''}
                        </div>
                        {extra.map((msg) => (
                          <div key={msg} style={{ fontSize: 10, color: 'var(--danger)' }}>{msg}</div>
                        ))}
                      </>
                    );
                  })()}
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
        <button
          className="btn-secondary btn-sm"
          onClick={declareAndFocus}
          disabled={!newName.trim() || !!schema[newName.trim()]}
        >
          + add to schema
        </button>
      </div>
    </div>
  );
}
