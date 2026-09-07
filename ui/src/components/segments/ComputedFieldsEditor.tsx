import type { ComputedField, FieldType } from '../../api/types';
import FormulaReference from './FormulaReference';
import ExpandableField from '../common/ExpandableField';
import { forwardReferences } from './computedFieldRules';
import { moveItem } from '../../utils/move';
import styles from './ComputedFieldsEditor.module.css';

interface Props {
  value: ComputedField[];
  onChange: (defs: ComputedField[]) => void;
  /** Called instead of applying a type change directly, so the owner can warn
   *  when rules compare this field with an operator the new type does not
   *  admit. The owner calls `apply` once the author confirms. */
  onChangeType?: (field: string, next: FieldType, apply: () => void) => void;
}

const FIELD_TYPES: FieldType[] = ['string', 'number', 'boolean', 'array'];

const empty = (): ComputedField => ({ name: '', type: 'number', formula: '' });

export default function ComputedFieldsEditor({ value, onChange, onChangeType }: Props) {
  const update = (idx: number, patch: Partial<ComputedField>) => {
    const next = value.map((d, i) => (i === idx ? { ...d, ...patch } : d));
    onChange(next);
  };

  // A retype can invalidate a rule that compares this field: an operator only
  // admits certain types, and the engine refuses the whole snapshot when one
  // does not. Retyping is legitimate and often the fix, so it is allowed — the
  // owner just gets the chance to say what it will cost first.
  const changeType = (idx: number, next: FieldType) => {
    const def = value[idx];
    if (!def || def.type === next) return;
    if (onChangeType) {
      onChangeType(def.name, next, () => update(idx, { type: next }));
      return;
    }
    update(idx, { type: next });
  };

  const remove = (idx: number) => onChange(value.filter((_, i) => i !== idx));

  const add = () => onChange([...value, empty()]);

  const move = (idx: number, delta: number) =>
    onChange(moveItem(value, idx, idx + delta));

  // Which formulas read a field declared below them — the case reordering
  // exists to fix, called out so it does not have to be worked out by hand.
  const forward = forwardReferences(value);

  return (
    <div className={styles.root}>
      {value.length > 0 && (
        <table className={styles.table}>
          <thead>
            <tr>
              <th></th>
              <th>Name</th>
              <th>Type</th>
              <th>Formula</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {value.map((def, i) => {
              const reads = forward.get(i);
              return (
                <tr key={i}>
                  {/* Order is evaluation order: a formula sees the fields above
                      it and nothing below. So these are not cosmetic. */}
                  <td className={styles.reorder}>
                    <button
                      type="button"
                      className="btn-ghost btn-sm"
                      onClick={() => move(i, -1)}
                      disabled={i === 0}
                      title="Move up — evaluated earlier"
                      aria-label={`Move ${def.name || 'field'} up`}
                    >
                      ↑
                    </button>
                    <button
                      type="button"
                      className="btn-ghost btn-sm"
                      onClick={() => move(i, 1)}
                      disabled={i === value.length - 1}
                      title="Move down — evaluated later"
                      aria-label={`Move ${def.name || 'field'} down`}
                    >
                      ↓
                    </button>
                  </td>
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
                      onChange={(e) => changeType(i, e.target.value as FieldType)}
                      aria-label={`${def.name || 'field'} type`}
                    >
                      {FIELD_TYPES.map((t) => (
                        <option key={t} value={t}>{t}</option>
                      ))}
                    </select>
                  </td>
                  <td>
                    <ExpandableField
                      value={def.formula}
                      onChange={(e) => update(i, { formula: e.target.value })}
                      placeholder='e.g. abs(Rating) * -1 + Bonus'
                      className={styles.expr}
                    />
                    {reads && (
                      <div className={styles.forward}>
                        reads <code>{reads.join(', ')}</code>, declared below — move this
                        row down, or those up, or it evaluates against nothing.
                      </div>
                    )}
                  </td>
                  <td>
                    <button type="button" className="btn-danger btn-sm" onClick={() => remove(i)}>x</button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
      {value.length > 1 && (
        <p className={styles.orderNote}>
          Fields evaluate top to bottom. A formula can read the fields above it, not the
          ones below.
        </p>
      )}
      <button type="button" className="btn-ghost btn-sm" style={{ marginTop: 8 }} onClick={add}>
        + Add Computed Field
      </button>
      <FormulaReference />
    </div>
  );
}
