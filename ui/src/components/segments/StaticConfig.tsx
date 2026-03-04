import { useState } from 'react';
import type { StaticConfig as SC } from '../../api/types';
import styles from './StaticConfig.module.css';

interface Props {
  value: SC;
  onChange: (v: SC) => void;
}

export default function StaticConfig({ value, onChange }: Props) {
  const [newKey, setNewKey] = useState('');
  const [newVal, setNewVal] = useState('');

  const entries = Object.entries(value.mappings);

  const removeKey = (key: string) => {
    const m = { ...value.mappings };
    delete m[key];
    onChange({ ...value, mappings: m });
  };

  const addMapping = () => {
    if (!newKey) return;
    onChange({ ...value, mappings: { ...value.mappings, [newKey]: newVal } });
    setNewKey('');
    setNewVal('');
  };

  return (
    <div>
      <div className="form-group">
        <label>Default Value</label>
        <input
          value={value.default}
          onChange={(e) => onChange({ ...value, default: e.target.value })}
        />
      </div>
      <label>Mappings (subject_key → segment)</label>
      <table className={styles.table}>
        <thead>
          <tr><th>Subject Key</th><th>Segment</th><th></th></tr>
        </thead>
        <tbody>
          {entries.map(([k, v]) => (
            <tr key={k}>
              <td>{k}</td>
              <td>{v}</td>
              <td>
                <button className="btn-danger btn-sm" onClick={() => removeKey(k)}>x</button>
              </td>
            </tr>
          ))}
          <tr>
            <td><input value={newKey} onChange={(e) => setNewKey(e.target.value)} placeholder="key" /></td>
            <td><input value={newVal} onChange={(e) => setNewVal(e.target.value)} placeholder="segment" /></td>
            <td><button className="btn-primary btn-sm" onClick={addMapping}>+</button></td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}
