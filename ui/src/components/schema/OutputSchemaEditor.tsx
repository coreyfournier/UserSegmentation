import { useState, useRef } from 'react';
import type { FieldType, LookupTable, OutputField, OutputSchema } from '../../api/types';
import { FIELD_TYPES, validateOutputField, type FieldCoverage } from './outputSchemaRules';
import styles from './OutputSchemaEditor.module.css';

interface Props {
  value?: OutputSchema;
  onChange: (s?: OutputSchema) => void;
  lookups: LookupTable[];
  /** Values for fields that do not vary per item, and the segment's coverage. */
  segmentOutputs?: Record<string, string>;
  onSegmentOutputsChange?: (o?: Record<string, string>) => void;
  coverage?: (name: string) => FieldCoverage;
  /** Called instead of the local delete, so the owner can also prune authored values. */
  onRemoveField?: (name: string) => void;
}

export default function OutputSchemaEditor({ value, onChange, lookups, segmentOutputs, onSegmentOutputsChange, coverage, onRemoveField }: Props) {
  const schema = value ?? {};
  const entries = Object.entries(schema);
  const [newField, setNewField] = useState('');
  const [newType, setNewType] = useState<FieldType>('string');
  const addRowRef = useRef<HTMLTableRowElement>(null);

  const write = (next: OutputSchema) => onChange(Object.keys(next).length ? next : undefined);

  const remove = (field: string) => {
    if (onRemoveField) {
      onRemoveField(field);
      return;
    }
    const next = { ...schema };
    delete next[field];
    write(next);
  };

  const patch = (field: string, partial: Partial<OutputField>) => {
    const merged: OutputField = { ...schema[field], ...partial };
    // A lookup binding is only meaningful while the types agree.
    if (merged.lookup) {
      const table = lookups.find((t) => t.id === merged.lookup);
      if (!table || table.keyType !== merged.type) delete merged.lookup;
    }
    write({ ...schema, [field]: merged });
  };

  const add = () => {
    // Trim before comparing: " severity" and "severity" would otherwise be two
    // distinct fields that look identical in the table, and the engine would
    // reject the padded one as an undeclared output key on the first save.
    const name = newField.trim();
    if (!name || schema[name]) return;
    const field: OutputField = { type: newType };
    // Required deliberately defaults to false: declaring a field must never
    // block a save, or the fast authoring flow stops being usable.
    write({ ...schema, [name]: field });
    setNewField('');
    setNewType('string');
  };

  const handleRowBlur = (e: React.FocusEvent) => {
    if (!newField) return;
    if (addRowRef.current?.contains(e.relatedTarget as Node)) return;
    add();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      add();
    }
  };

  return (
    <div>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Field</th><th>Type</th><th>Lookup</th><th>Required</th><th>Segment value</th><th></th>
          </tr>
        </thead>
        <tbody>
          {entries.map(([name, f]) => {
            const err = validateOutputField(name, f, lookups);
            const candidates = lookups.filter((t) => t.keyType === f.type);
            return (
              <tr key={name}>
                <td>
                  {name}
                  {err && (
                    <div style={{ fontSize: 10, color: 'var(--danger)' }}>{err}</div>
                  )}
                </td>
                <td>
                  <select
                    value={f.type}
                    onChange={(e) => patch(name, { type: e.target.value as FieldType })}
                    aria-label={`${name} type`}
                  >
                    {FIELD_TYPES.map((t) => (
                      <option key={t} value={t}>{t}</option>
                    ))}
                  </select>
                </td>
                <td>
                  <select
                    value={f.lookup ?? ''}
                    onChange={(e) => patch(name, { lookup: e.target.value || undefined })}
                    disabled={candidates.length === 0}
                    title={candidates.length === 0 ? `No lookup table has key type "${f.type}"` : undefined}
                    aria-label={`${name} lookup`}
                  >
                    <option value="">—</option>
                    {candidates.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
                  </select>
                </td>
                <td>
                  <input
                    type="checkbox"
                    checked={!!f.required}
                    onChange={() => patch(name, { required: !f.required })}
                    style={{ width: 'auto' }}
                    title="Every reporting rule must author this field, or the segment must set it once. Enforced on save."
                  />
                </td>
                <td>
                  {onSegmentOutputsChange && (
                    <input
                      value={segmentOutputs?.[name] ?? ''}
                      onChange={(e) => {
                        const next = { ...(segmentOutputs ?? {}) };
                        if (e.target.value === '') delete next[name];
                        else next[name] = e.target.value;
                        onSegmentOutputsChange(Object.keys(next).length ? next : undefined);
                      }}
                      placeholder="set once for the segment"
                      title="Satisfies this field for every reporting rule at once."
                      aria-label={`${name} segment value`}
                      style={{ fontSize: 11 }}
                    />
                  )}
                  {coverage && (() => {
                    const c = coverage(name);
                    if (c.segmentLevel) {
                      return <div style={{ fontSize: 10, color: 'var(--text-muted)' }}>set for the whole segment</div>;
                    }
                    const short = c.total - c.authored;
                    const overridesShort = c.overridesTotal - c.overridesAuthored;
                    const extra: string[] = [];
                    if (f.required) {
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
                        <div style={{ fontSize: 10, color: short && f.required ? 'var(--danger)' : 'var(--text-muted)' }}>
                          authored on {c.authored} of {c.total} checks
                          {short && f.required ? ` — ${short} will block saving` : ''}
                        </div>
                        {extra.map((msg) => (
                          <div key={msg} style={{ fontSize: 10, color: 'var(--danger)' }}>{msg}</div>
                        ))}
                      </>
                    );
                  })()}
                </td>
                <td><button className="btn-danger btn-sm" onClick={() => remove(name)}>x</button></td>
              </tr>
            );
          })}
          <tr ref={addRowRef} onBlur={handleRowBlur}>
            <td>
              <input value={newField} onChange={(e) => setNewField(e.target.value)} onKeyDown={handleKeyDown} placeholder="field name" />
            </td>
            <td>
              <select value={newType} onChange={(e) => setNewType(e.target.value as FieldType)}>
                {FIELD_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </td>
            <td colSpan={3} style={{ fontSize: 10, color: 'var(--text-muted)' }}>
              a string's value is a template; every other type is an expression
            </td>
            <td><button className="btn-primary btn-sm" onClick={add}>+</button></td>
          </tr>
        </tbody>
      </table>
      <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
        Declared fields are optional until you tick Required. Values are authored per check,
        below — or once for the whole segment when they do not vary.
      </p>
    </div>
  );
}
