import { useState } from 'react';
import type { PercentageConfig as PC } from '../../api/types';
import styles from './PercentageConfig.module.css';

interface Props {
  value: PC;
  onChange: (v: PC) => void;
}

export default function PercentageConfig({ value, onChange }: Props) {
  const [newSeg, setNewSeg] = useState('');
  const [newWeight, setNewWeight] = useState(0);

  const total = value.buckets.reduce((s, b) => s + b.weight, 0);

  const removeBucket = (idx: number) => {
    const b = [...value.buckets];
    b.splice(idx, 1);
    onChange({ ...value, buckets: b });
  };

  const addBucket = () => {
    if (!newSeg) return;
    onChange({ ...value, buckets: [...value.buckets, { segment: newSeg, weight: newWeight }] });
    setNewSeg('');
    setNewWeight(0);
  };

  const updateWeight = (idx: number, w: number) => {
    const b = [...value.buckets];
    b[idx] = { ...b[idx], weight: w };
    onChange({ ...value, buckets: b });
  };

  return (
    <div>
      <div className="form-group">
        <label>Salt</label>
        <input
          value={value.salt}
          onChange={(e) => onChange({ ...value, salt: e.target.value })}
        />
      </div>
      <label>
        Buckets{' '}
        <span className={total === 100 ? styles.valid : styles.invalid}>
          (total: {total}/100)
        </span>
      </label>
      <table className={styles.table}>
        <thead>
          <tr><th>Segment</th><th>Weight</th><th></th></tr>
        </thead>
        <tbody>
          {value.buckets.map((b, i) => (
            <tr key={i}>
              <td>{b.segment}</td>
              <td>
                <input
                  type="number"
                  value={b.weight}
                  onChange={(e) => updateWeight(i, Number(e.target.value))}
                  min={0} max={100}
                  style={{ width: '80px' }}
                />
              </td>
              <td><button className="btn-danger btn-sm" onClick={() => removeBucket(i)}>x</button></td>
            </tr>
          ))}
          <tr>
            <td><input value={newSeg} onChange={(e) => setNewSeg(e.target.value)} placeholder="segment" /></td>
            <td>
              <input
                type="number"
                value={newWeight}
                onChange={(e) => setNewWeight(Number(e.target.value))}
                min={0} max={100}
                style={{ width: '80px' }}
              />
            </td>
            <td><button className="btn-primary btn-sm" onClick={addBucket}>+</button></td>
          </tr>
        </tbody>
      </table>
    </div>
  );
}
