import type { Expression, InputSchema } from '../../api/types';
import { LOOKUP_OPERATORS } from '../../api/types';
import { useLookups } from '../../api/lookups';
import { parseNumericInput } from '../../utils/parse';
import OperatorSelect from './OperatorSelect';
import styles from './ExpressionEditor.module.css';

interface Props {
  value: Expression;
  onChange: (e: Expression) => void;
  schema?: InputSchema;
  layerNames?: string[];
}

export default function ExpressionEditor({ value, onChange, schema, layerNames }: Props) {
  const { data: lookups } = useLookups();
  const suggestions: string[] = [
    ...Object.keys(schema ?? {}),
    ...(layerNames ?? []).map((n) => `layer:${n}`),
  ];

  const fieldType = schema?.[value.field]?.type;
  const isLookupOp = LOOKUP_OPERATORS.includes(value.operator);
  // Offer only tables whose key type matches the field's type (all if type unknown).
  const lookupOptions = (lookups ?? []).filter((t) => !fieldType || t.keyType === fieldType);

  const formatValue = (v: unknown): string => {
    if (Array.isArray(v)) return v.join(', ');
    return String(v ?? '');
  };

  const parseValue = (raw: string, op: string): unknown => {
    if (op === 'in') {
      return raw.split(',').map((s) => s.trim()).filter(Boolean);
    }
    if (raw === 'true') return true;
    if (raw === 'false') return false;
    return parseNumericInput(raw);
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
        {isLookupOp ? (
          <select
            value={typeof value.value === 'string' ? value.value : ''}
            onChange={(e) => onChange({ ...value, value: e.target.value })}
          >
            <option value="">— select lookup —</option>
            {lookupOptions.map((t) => (
              <option key={t.id} value={t.id}>{t.name} ({t.keyType})</option>
            ))}
          </select>
        ) : (
          <input
            value={formatValue(value.value)}
            onChange={(e) =>
              onChange({ ...value, value: parseValue(e.target.value, value.operator as string) })
            }
            placeholder={value.operator === 'in' ? 'val1, val2, ...' : 'value'}
          />
        )}
      </div>
    </div>
  );
}
