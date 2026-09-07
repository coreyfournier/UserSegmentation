import { useState } from 'react';
import type { ComputedField, FieldType, OutputField, OutputSchema } from '../../api/types';
import {
  availableOutputFields,
  FIELD_TYPES,
  matchingComputedField,
  outputValueRows,
  placeholderFor,
  renameOutputKey,
  type FieldCoverage,
} from '../schema/outputSchemaRules';
import ExpandableField from '../common/ExpandableField';
import styles from './OutputValuesEditor.module.css';

interface Props {
  outputs?: Record<string, string>;
  schema: OutputSchema;
  onChange: (o?: Record<string, string>) => void;
  /** Declares a new field on the segment's schema, so it can be authored inline. */
  onDeclare: (name: string, field: OutputField) => void;
  /** The segment's computed fields, so a field a computed one could supply
   *  can offer it. Omitted where there are none to offer. */
  computed?: ComputedField[];
  /** How completely each field is authored elsewhere on the segment (its
   *  rules and overrides). Only meaningful when this editor represents a
   *  segment's own values rather than one rule's — omitted by every
   *  per-rule caller. */
  coverage?: (name: string) => FieldCoverage;
}

/**
 * Sentinel select value for "declare new…". Never collides with a real field
 * name — those come from JSON object keys, which cannot contain NUL.
 */
const DECLARE_NEW = '\0declare-new';

export default function OutputValuesEditor({ outputs, schema, onChange, onDeclare, coverage, computed }: Props) {
  // Shared by every select's "declare new…" option and the add-value picker:
  // whichever triggers it first reveals the same name input below the grid.
  const [showDeclare, setShowDeclare] = useState(false);
  const [newName, setNewName] = useState('');
  const [newType, setNewType] = useState<FieldType>('string');

  const set = (name: string, raw: string) => {
    const next = { ...(outputs ?? {}) };
    if (raw === '') delete next[name];
    else next[name] = raw;
    onChange(Object.keys(next).length ? next : undefined);
  };

  // A required row's remove control clears its value but the row stays
  // (outputValueRows always includes required fields); an optional or
  // orphaned row's key disappears entirely, so the row does too.
  const remove = (name: string) => set(name, '');

  const declare = () => {
    // Trim to match OutputSchemaEditor's add path. Without it " severity" and
    // "severity" slip past the duplicate check as two visually identical
    // fields, and the engine rejects the padded one as an undeclared output
    // key on the first save.
    const name = newName.trim();
    if (!name || schema[name]) return;
    const field: OutputField = { type: newType };
    // Required stays false so declaring a field mid-edit invalidates nothing.
    onDeclare(name, field);
    setNewName('');
    setNewType('string');
    // Collapse again, or the panel stays open for the life of the component
    // with no way to dismiss it.
    setShowDeclare(false);
  };

  const rows = outputValueRows(schema, outputs);
  // Fields with no row yet. Deliberately not availableOutputFields, which
  // excludes by presence in `outputs` — a required field is always shown as a
  // row while being absent from `outputs`, so that would offer it here too and
  // picking it would appear to do nothing.
  const shown = new Set(rows.map((r) => r.name));
  const remaining = Object.keys(schema)
    .filter((n) => !shown.has(n))
    .sort((a, b) => a.localeCompare(b));

  return (
    <div>
      {rows.length > 0 && (
        <div className={styles.grid}>
          {rows.map((row) => {
            const options = availableOutputFields(schema, outputs, row.name);
            return (
              <div key={row.name} style={{ display: 'contents' }}>
                <div>
                  <select
                    value={row.name}
                    aria-label={`output field for ${row.name}`}
                    title={
                      row.field
                        ? `${row.field.type}${row.field.required ? ' · required' : ''}`
                        : 'not declared on the layer'
                    }
                    onChange={(e) => {
                      if (e.target.value === DECLARE_NEW) {
                        setShowDeclare(true);
                        return;
                      }
                      onChange(renameOutputKey(outputs, row.name, e.target.value));
                    }}
                  >
                    {options.map((n) => (
                      <option key={n} value={n}>
                        {n}
                        {schema[n]?.required ? ' *' : ''}
                      </option>
                    ))}
                    <option value={DECLARE_NEW}>declare new…</option>
                  </select>
                  {row.orphaned && (
                    <div className={styles.err}>not declared on the layer; pick a field or remove</div>
                  )}
                </div>
                <div>
                  <ExpandableField
                    value={row.value}
                    onChange={(e) => set(row.name, e.target.value)}
                    placeholder={row.field ? placeholderFor(row.field) : undefined}
                    aria-label={`value for ${row.name}`}
                  />
                  {/* A computed field of the same name and type could supply
                      this. Offered, not applied: clicking writes the
                      expression into the config, so what evaluates is what an
                      author can read here — the engine infers nothing. */}
                  {!row.value && (() => {
                    const match = matchingComputedField(row.name, row.field, computed);
                    if (!match) return null;
                    return (
                      <button
                        type="button"
                        className={styles.useComputed}
                        onClick={() => set(row.name, match.name)}
                        title={`Set this to the computed field ${match.name} (${match.type})`}
                      >
                        use computed <code>{match.name}</code>
                      </button>
                    );
                  })()}
                  {coverage && row.field && (() => {
                    const field = row.field;
                    const c = coverage(row.name);
                    if (c.segmentLevel) {
                      return <div style={{ fontSize: 10, color: 'var(--text-muted)' }}>set for the whole segment</div>;
                    }
                    const short = c.total - c.authored;
                    const overridesShort = c.overridesTotal - c.overridesAuthored;
                    const extra: string[] = [];
                    if (field.required) {
                      if (c.defaultUnauthored) {
                        extra.push('the default path has no value for this — set it in the Default section');
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
                <div>
                  <button
                    type="button"
                    className="btn-danger btn-sm"
                    onClick={() => remove(row.name)}
                    aria-label={`remove ${row.name}`}
                  >
                    x
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}
      <div className={styles.addRow}>
        <select
          value=""
          aria-label="add an output value"
          onChange={(e) => {
            const picked = e.target.value;
            if (!picked) return;
            if (picked === DECLARE_NEW) {
              setShowDeclare(true);
              return;
            }
            onChange({ ...(outputs ?? {}), [picked]: '' });
          }}
        >
          <option value="" disabled>
            + add a value…
          </option>
          {remaining.map((n) => (
            <option key={n} value={n}>
              {n}
            </option>
          ))}
          <option value={DECLARE_NEW}>declare new…</option>
        </select>
      </div>
      {showDeclare && (
        <div className={styles.addRow}>
          <input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                declare();
              }
            }}
            placeholder="new output field"
            aria-label="new output field name"
            style={{ fontSize: 11 }}
          />
          <select
            value={newType}
            aria-label="new output field type"
            onChange={(e) => setNewType(e.target.value as FieldType)}
          >
            {FIELD_TYPES.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
          </select>
          <button type="button"
            className="btn-secondary btn-sm"
            onClick={declare}
            disabled={!newName.trim() || !!schema[newName.trim()]}
          >
            + add to schema
          </button>
          <button
            type="button"
            className="btn-secondary btn-sm"
            onClick={() => {
              setNewName('');
              setShowDeclare(false);
            }}
          >
            cancel
          </button>
        </div>
      )}
    </div>
  );
}
