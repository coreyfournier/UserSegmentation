import { useState } from 'react';
import type { InputSchema, FieldType } from '../../api/types';
import styles from './InputSchemaEditor.module.css';

interface Props {
  value?: InputSchema;
  onChange: (s?: InputSchema) => void;
}

const TYPES: FieldType[] = ['string', 'number', 'boolean', 'array'];

export default function InputSchemaEditor({ value, onChange }: Props) {
  const schema = value ?? {};
  const entries = Object.entries(schema);
  const [newField, setNewField] = useState('');
  const [newType, setNewType] = useState<FieldType>('string');
  const [newReq, setNewReq] = useState(false);

  const remove = (field: string) => {
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
              <td>{sf.type}</td>
              <td>
                <input
                  type="checkbox"
                  checked={sf.required}
                  onChange={() => toggleRequired(f)}
                  style={{ width: 'auto' }}
                />
              </td>
              <td><button className="btn-danger btn-sm" onClick={() => remove(f)}>x</button></td>
            </tr>
          ))}
          <tr>
            <td><input value={newField} onChange={(e) => setNewField(e.target.value)} placeholder="field name" /></td>
            <td>
              <select value={newType} onChange={(e) => setNewType(e.target.value as FieldType)}>
                {TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </td>
            <td>
              <input type="checkbox" checked={newReq} onChange={(e) => setNewReq(e.target.checked)} style={{ width: 'auto' }} />
            </td>
            <td><button className="btn-primary btn-sm" onClick={add}>+</button></td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}
