import type { ComputedField, FieldType } from '../../api/types';
import FormulaReference from './FormulaReference';
import styles from './ComputedFieldsEditor.module.css';

interface Props {
  value: ComputedField[];
  onChange: (defs: ComputedField[]) => void;
}

const FIELD_TYPES: FieldType[] = ['string', 'number', 'boolean', 'array'];

const empty = (): ComputedField => ({ name: '', type: 'number', formula: '' });

export default function ComputedFieldsEditor({ value, onChange }: Props) {
  const update = (idx: number, patch: Partial<ComputedField>) => {
    const next = value.map((d, i) => (i === idx ? { ...d, ...patch } : d));
    onChange(next);
  };

  const remove = (idx: number) => onChange(value.filter((_, i) => i !== idx));

  const add = () => onChange([...value, empty()]);

  return (
    <div className={styles.root}>
      {value.length > 0 && (
        <table className={styles.table}>
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Formula</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {value.map((def, i) => (
              <tr key={i}>
                <td>
                  <input
                    value={def.name}
                    onChange={(e) => update(i, { name: e.target.value })}
                    placeholder="fieldName"
                  />
                </td>
                <td>
                  <select
                    value={def.type}
                    onChange={(e) => update(i, { type: e.target.value as FieldType })}
                  >
                    {FIELD_TYPES.map((t) => (
                      <option key={t} value={t}>{t}</option>
                    ))}
                  </select>
                </td>
                <td>
                  <input
                    value={def.formula}
                    onChange={(e) => update(i, { formula: e.target.value })}
                    placeholder='e.g. abs(Rating) * -1 + Bonus'
                    className={styles.expr}
                  />
                </td>
                <td>
                  <button type="button" className="btn-danger btn-sm" onClick={() => remove(i)}>x</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <button type="button" className="btn-ghost btn-sm" style={{ marginTop: 8 }} onClick={add}>
        + Add Computed Field
      </button>
      <FormulaReference />
    </div>
  );
}
