import { useState, useRef } from 'react';
import type { InputSchema, FieldType } from '../../api/types';
import styles from './InputSchemaEditor.module.css';

interface Props {
  value?: InputSchema;
  onChange: (s?: InputSchema) => void;
  /** Called instead of the local delete, so the owner can warn when removing
   *  the layer's last field would turn off rule-field validation entirely. */
  onRemoveField?: (field: string) => void;
  /** Called instead of applying a type change directly, so the owner can warn
   *  when rules use the field with an operator the new type does not support.
   *  The owner applies the change itself once the author confirms. */
  onChangeFieldType?: (field: string, next: FieldType) => void;
}

const TYPES: FieldType[] = ['string', 'number', 'boolean', 'array'];

export default function InputSchemaEditor({ value, onChange, onRemoveField, onChangeFieldType }: Props) {
  const schema = value ?? {};
  const entries = Object.entries(schema);
  const [newField, setNewField] = useState('');
  const [newType, setNewType] = useState<FieldType>('string');
  const [newReq, setNewReq] = useState(false);
  const addRowRef = useRef<HTMLTableRowElement>(null);

  const changeType = (field: string, next: FieldType) => {
    if (next === schema[field]?.type) return;
    if (onChangeFieldType) {
      onChangeFieldType(field, next);
      return;
    }
    onChange({ ...schema, [field]: { ...schema[field], type: next } });
  };

  const remove = (field: string) => {
    if (onRemoveField) {
      onRemoveField(field);
      return;
    }
    const s = { ...schema };
    delete s[field];
    onChange(Object.keys(s).length ? s : undefined);
  };

  const add = () => {
    if (!newField) return;
    onChange({ ...schema, [newField]: { type: newType, required: newReq } });
    setNewField('');
    setNewType('string');
    setNewReq(false);
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

  const toggleRequired = (field: string) => {
    onChange({
      ...schema,
      [field]: { ...schema[field], required: !schema[field].required },
    });
  };

  return (
    <div>
      <table className={styles.table}>
        <thead>
          <tr><th>Field</th><th>Type</th><th>Required</th><th></th></tr>
        </thead>
        <tbody>
          {entries.map(([f, sf]) => (
            <tr key={f}>
              <td>{f}</td>
              <td>
                {/* Editable after the fact. A type chosen while adding a field
                    is a guess as often as not, and the only way to correct it
                    used to be deleting the field and re-adding it — which loses
                    the required flag and, when it is the layer's last field,
                    silently turns rule-field validation off for every segment.
                    Changing it can invalidate rules that use an operator the
                    new type does not support, so the owner is given the chance
                    to warn first. */}
                <select
                  value={sf.type}
                  aria-label={`${f} type`}
                  onChange={(e) => changeType(f, e.target.value as FieldType)}
                >
                  {TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
                </select>
              </td>
              <td>
                <input
                  type="checkbox"
                  checked={sf.required}
                  onChange={() => toggleRequired(f)}
                  style={{ width: 'auto' }}
                />
              </td>
              <td><button type="button" className="btn-danger btn-sm" onClick={() => remove(f)}>x</button></td>
            </tr>
          ))}
          <tr ref={addRowRef} onBlur={handleRowBlur}>
            <td><input value={newField} onChange={(e) => setNewField(e.target.value)} onKeyDown={handleKeyDown} placeholder="field name" /></td>
            <td>
              <select value={newType} onChange={(e) => setNewType(e.target.value as FieldType)}>
                {TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </td>
            <td>
              <input type="checkbox" checked={newReq} onChange={(e) => setNewReq(e.target.checked)} style={{ width: 'auto' }} />
            </td>
            <td><button type="button" className="btn-primary btn-sm" onClick={add}>+</button></td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}
