import { useState, useRef } from 'react';
import type { EvalMode, FieldType, LookupTable, OutputField, OutputSchema } from '../../api/types';
import { allowedTypesForMode, evalModeOf, validateOutputField } from './outputSchemaRules';
import styles from './OutputSchemaEditor.module.css';

interface Props {
  value?: OutputSchema;
  onChange: (s?: OutputSchema) => void;
  lookups: LookupTable[];
}

const MODES: EvalMode[] = ['literal', 'template', 'expression'];

const MODE_HINT: Record<EvalMode, string> = {
  literal: 'a constant, emitted as the declared type',
  template: 'text with ${ ... } tokens, always a string',
  expression: 'one whole expression, returning a typed value',
};

export default function OutputSchemaEditor({ value, onChange, lookups }: Props) {
  const schema = value ?? {};
  const entries = Object.entries(schema);
  const [newField, setNewField] = useState('');
  const [newMode, setNewMode] = useState<EvalMode>('literal');
  const [newType, setNewType] = useState<FieldType>('string');
  const addRowRef = useRef<HTMLTableRowElement>(null);

  const write = (next: OutputSchema) => onChange(Object.keys(next).length ? next : undefined);

  const remove = (field: string) => {
    const next = { ...schema };
    delete next[field];
    write(next);
  };

  const patch = (field: string, partial: Partial<OutputField>) => {
    const merged: OutputField = { ...schema[field], ...partial };
    // Changing the mode can invalidate the type, so snap it to something legal
    // rather than leaving a declaration the engine will reject at load.
    const allowed = allowedTypesForMode(evalModeOf(merged));
    if (!allowed.includes(merged.type)) merged.type = allowed[0];
    // A lookup binding is only meaningful while the types agree.
    if (merged.lookup) {
      const table = lookups.find((t) => t.id === merged.lookup);
      if (!table || table.keyType !== merged.type) delete merged.lookup;
    }
    write({ ...schema, [field]: merged });
  };

  const add = () => {
    if (!newField || schema[newField]) return;
    const field: OutputField = { type: newType };
    if (newMode !== 'literal') field.eval = newMode;
    // Required deliberately defaults to false: declaring a field must never
    // block a save, or the fast authoring flow stops being usable.
    write({ ...schema, [newField]: field });
    setNewField('');
    setNewMode('literal');
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

  const newAllowed = allowedTypesForMode(newMode);

  return (
    <div>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Field</th><th>Eval</th><th>Type</th><th>Lookup</th><th>Required</th><th></th>
          </tr>
        </thead>
        <tbody>
          {entries.map(([name, f]) => {
            const mode = evalModeOf(f);
            const err = validateOutputField(name, f, lookups);
            const candidates = lookups.filter((t) => t.keyType === f.type);
            return (
              <tr key={name}>
                <td>
                  {name}
                  {err && (
                    <div style={{ fontSize: 10, color: 'var(--danger, #ef4444)' }}>{err}</div>
                  )}
                </td>
                <td>
                  <select value={mode} onChange={(e) => patch(name, { eval: e.target.value as EvalMode })} title={MODE_HINT[mode]}>
                    {MODES.map((m) => <option key={m} value={m}>{m}</option>)}
                  </select>
                </td>
                <td>
                  <select value={f.type} onChange={(e) => patch(name, { type: e.target.value as FieldType })}>
                    {allowedTypesForMode(mode).map((t) => <option key={t} value={t}>{t}</option>)}
                  </select>
                </td>
                <td>
                  <select
                    value={f.lookup ?? ''}
                    onChange={(e) => patch(name, { lookup: e.target.value || undefined })}
                    disabled={candidates.length === 0}
                    title={candidates.length === 0 ? `No lookup table has key type "${f.type}"` : undefined}
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
                <td><button className="btn-danger btn-sm" onClick={() => remove(name)}>x</button></td>
              </tr>
            );
          })}
          <tr ref={addRowRef} onBlur={handleRowBlur}>
            <td>
              <input value={newField} onChange={(e) => setNewField(e.target.value)} onKeyDown={handleKeyDown} placeholder="field name" />
            </td>
            <td>
              <select
                value={newMode}
                onChange={(e) => {
                  const m = e.target.value as EvalMode;
                  setNewMode(m);
                  const allowed = allowedTypesForMode(m);
                  if (!allowed.includes(newType)) setNewType(allowed[0]);
                }}
              >
                {MODES.map((m) => <option key={m} value={m}>{m}</option>)}
              </select>
            </td>
            <td>
              <select value={newType} onChange={(e) => setNewType(e.target.value as FieldType)}>
                {newAllowed.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </td>
            <td colSpan={2} style={{ fontSize: 10, color: 'var(--text-muted)' }}>{MODE_HINT[newMode]}</td>
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
