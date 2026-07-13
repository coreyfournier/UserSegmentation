import { useState } from 'react';
import type { LookupTable, LookupEntry, FieldType } from '../../api/types';
import styles from './LookupForm.module.css';

interface Props {
  initial?: LookupTable;
  onSubmit: (table: Omit<LookupTable, 'id'> & { id?: string }) => void;
  onCancel: () => void;
  submitLabel?: string;
}

const KEY_TYPES: FieldType[] = ['string', 'number'];

// Client-side preview of the server's slug (server remains authoritative).
function slugify(s: string): string {
  return s
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

// Coerce a raw input string to the declared key type so JSON carries the right type.
function coerceKey(raw: string, keyType: FieldType): unknown {
  if (keyType === 'number') {
    const n = Number(raw);
    return raw.trim() === '' || Number.isNaN(n) ? raw : n;
  }
  return raw;
}

export default function LookupForm({ initial, onSubmit, onCancel, submitLabel = 'Create' }: Props) {
  const isEdit = !!initial;
  const [name, setName] = useState(initial?.name ?? '');
  const [keyType, setKeyType] = useState<FieldType>(initial?.keyType ?? 'string');
  const [entries, setEntries] = useState<LookupEntry[]>(initial?.entries ?? []);

  const setKey = (i: number, raw: string) =>
    setEntries((es) => es.map((e, idx) => (idx === i ? { ...e, key: coerceKey(raw, keyType) } : e)));
  const setValue = (i: number, value: string) =>
    setEntries((es) => es.map((e, idx) => (idx === i ? { ...e, value: value || undefined } : e)));
  const addEntry = () => setEntries((es) => [...es, { key: '', value: '' }]);
  const removeEntry = (i: number) => setEntries((es) => es.filter((_, idx) => idx !== i));

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit({ id: initial?.id, name, keyType, entries });
      }}
    >
      <div className="form-group">
        <label>Display Name</label>
        <input value={name} onChange={(e) => setName(e.target.value)} required />
        <p className={styles.hint}>
          {isEdit ? (
            <>Internal id: <code>{initial!.id}</code> (immutable)</>
          ) : (
            <>Internal id will be: <code>{slugify(name) || '—'}</code></>
          )}
        </p>
      </div>

      <div className="form-group">
        <label>Key Type {isEdit && '(immutable)'}</label>
        <select
          value={keyType}
          onChange={(e) => setKeyType(e.target.value as FieldType)}
          disabled={isEdit}
        >
          {KEY_TYPES.map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>
      </div>

      <div className="form-group">
        <label>Entries</label>
        {entries.length > 0 && (
          <div className={styles.entryHead}>
            <span className={styles.colLabel}>Key (matched)</span>
            <span className={styles.colLabel}>Value (description, optional)</span>
            <span />
          </div>
        )}
        {entries.map((e, i) => (
          <div key={i} className={styles.entryRow}>
            <input
              type={keyType === 'number' ? 'number' : 'text'}
              value={e.key === undefined || e.key === null ? '' : String(e.key)}
              onChange={(ev) => setKey(i, ev.target.value)}
              placeholder="key"
              required
            />
            <input
              value={e.value ?? ''}
              onChange={(ev) => setValue(i, ev.target.value)}
              placeholder="value (optional)"
            />
            <button type="button" className="btn-danger btn-sm" onClick={() => removeEntry(i)}>x</button>
          </div>
        ))}
        <button type="button" className="btn-ghost btn-sm" onClick={addEntry}>+ Add entry</button>
      </div>

      <div className="form-row" style={{ justifyContent: 'flex-end' }}>
        <button type="button" className="btn-ghost" onClick={onCancel}>Cancel</button>
        <button type="submit" className="btn-primary">{submitLabel}</button>
      </div>
    </form>
  );
}
