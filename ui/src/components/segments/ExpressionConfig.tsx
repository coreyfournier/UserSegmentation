import type { ExpressionDef, FieldType } from '../../api/types';
import ExpressionHelpPanel from './ExpressionHelpPanel';
import styles from './ExpressionConfig.module.css';

interface Props {
  value: ExpressionDef[];
  onChange: (defs: ExpressionDef[]) => void;
}

const FIELD_TYPES: FieldType[] = ['string', 'number', 'boolean', 'array'];

const empty = (): ExpressionDef => ({ name: '', type: 'number', expression: '' });

export default function ExpressionConfig({ value, onChange }: Props) {
  const update = (idx: number, patch: Partial<ExpressionDef>) => {
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
              <th>Expression</th>
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
                    value={def.expression}
                    onChange={(e) => update(i, { expression: e.target.value })}
                    placeholder='e.g. abs(Rating) * -1 + Bonus'
                    className={styles.expr}
                  />
                </td>
                <td>
                  <button className="btn-danger btn-sm" onClick={() => remove(i)}>x</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <button className="btn-ghost btn-sm" style={{ marginTop: 8 }} onClick={add}>
        + Add Expression
      </button>
      <ExpressionHelpPanel />
    </div>
  );
}
