import type { Expression, InputSchema } from '../../api/types';
import OperatorSelect from './OperatorSelect';
import styles from './ExpressionEditor.module.css';

interface Props {
  value: Expression;
  onChange: (e: Expression) => void;
  schema?: InputSchema;
  layerNames?: string[];
}

export default function ExpressionEditor({ value, onChange, schema, layerNames }: Props) {
  const suggestions: string[] = [
    ...Object.keys(schema ?? {}),
    ...(layerNames ?? []).map((n) => `layer:${n}`),
  ];

  const fieldType = schema?.[value.field]?.type;

  const formatValue = (v: unknown): string => {
    if (Array.isArray(v)) return v.join(', ');
    return String(v ?? '');
  };

  const parseValue = (raw: string, op: string): unknown => {
    if (op === 'in') {
      return raw.split(',').map((s) => s.trim()).filter(Boolean);
    }
    const n = Number(raw);
    if (!isNaN(n) && raw.trim() !== '') return n;
    if (raw === 'true') return true;
    if (raw === 'false') return false;
    return raw;
  };

  return (
    <div className={styles.row}>
      <div className={styles.field}>
        <input
          value={value.field}
          onChange={(e) => onChange({ ...value, field: e.target.value })}
          placeholder="field"
          list="field-suggestions"
        />
        <datalist id="field-suggestions">
          {suggestions.map((s) => <option key={s} value={s} />)}
        </datalist>
      </div>
      <OperatorSelect
        value={value.operator}
        onChange={(op) => onChange({ ...value, operator: op })}
        fieldType={fieldType}
      />
      <div className={styles.value}>
        <input
          value={formatValue(value.value)}
          onChange={(e) =>
            onChange({ ...value, value: parseValue(e.target.value, value.operator as string) })
          }
          placeholder={value.operator === 'in' ? 'val1, val2, ...' : 'value'}
        />
      </div>
    </div>
  );
}
