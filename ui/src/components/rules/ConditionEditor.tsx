import type { Condition, InputSchema } from '../../api/types';
import { LOOKUP_OPERATORS, UNARY_OPERATORS } from '../../api/types';
import { useLookups } from '../../api/lookups';
import { parseNumericInput } from '../../utils/parse';
import OperatorSelect from './OperatorSelect';
import ExpandableField from '../common/ExpandableField';
import styles from './ConditionEditor.module.css';

interface Props {
  value: Condition;
  onChange: (c: Condition) => void;
  schema?: InputSchema;
  layerNames?: string[];
}

export default function ConditionEditor({ value, onChange, schema, layerNames }: Props) {
  const { data: lookups } = useLookups();
  const suggestions: string[] = [
    ...Object.keys(schema ?? {}),
    ...(layerNames ?? []).map((n) => `layer:${n}`),
  ];

  const fieldType = schema?.[value.field]?.type;
  const isLookupOp = LOOKUP_OPERATORS.includes(value.operator);
  const isUnaryOp = UNARY_OPERATORS.includes(value.operator);
  // Offer only tables whose key type matches the field's type (all if type unknown).
  const lookupOptions = (lookups ?? []).filter((t) => !fieldType || t.keyType === fieldType);

  // A field bound to a lookup in the input schema has a known domain, so the
  // value is picked from that table's keys rather than typed.
  const boundTable = (lookups ?? []).find((t) => t.id === schema?.[value.field]?.lookup);
  const takesList = value.operator === 'in' || value.operator === 'not_in';
  const takesOneValue = !isUnaryOp && !isLookupOp && !takesList;
  const keyOptions = boundTable && takesOneValue ? boundTable.entries : undefined;
  // A list operator gets an adder instead of a replacing select: one select
  // cannot express a list, but it can append to one — which is the whole
  // difficulty with typing these by hand, since every key has to be recalled
  // and spelled exactly.
  const listKeyOptions = boundTable && !isUnaryOp && !isLookupOp && takesList
    ? boundTable.entries
    : undefined;
  const currentList = Array.isArray(value.value) ? value.value.map(String) : [];
  const appendKey = (key: string) => {
    if (!key || currentList.includes(key)) return;
    onChange({ ...value, value: [...currentList, key] });
  };
  const currentKey = String(value.value ?? '');
  // Keep a value the table does not (or no longer) list, so opening a rule
  // written before the binding — or after an entry was removed — does not
  // silently blank it on the next save.
  const strayKey =
    keyOptions && currentKey !== '' && !keyOptions.some((e) => String(e.key) === currentKey)
      ? currentKey
      : undefined;

  const formatValue = (v: unknown): string => {
    if (Array.isArray(v)) return v.join(', ');
    return String(v ?? '');
  };

  const parseValue = (raw: string, op: string): unknown => {
    if (op === 'in' || op === 'not_in') {
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
        onChange={(op) =>
          // Drop any value when switching to an operator that takes none, so a
          // stale one is not left behind in the saved config.
          onChange(
            UNARY_OPERATORS.includes(op)
              ? { field: value.field, operator: op }
              : { ...value, operator: op }
          )
        }
        fieldType={fieldType}
      />
      <div className={styles.value}>
        {isUnaryOp ? (
          <span className={styles.noValue}>no value needed</span>
        ) : isLookupOp ? (
          <select
            value={typeof value.value === 'string' ? value.value : ''}
            onChange={(e) => onChange({ ...value, value: e.target.value })}
          >
            <option value="">— select lookup —</option>
            {lookupOptions.map((t) => (
              <option key={t.id} value={t.id}>{t.name} ({t.keyType})</option>
            ))}
          </select>
        ) : keyOptions ? (
          <select
            value={currentKey}
            onChange={(e) => onChange({ ...value, value: parseValue(e.target.value, value.operator as string) })}
            aria-label={`${value.field} value`}
            title={`Values come from the lookup table "${boundTable!.name}"`}
          >
            <option value="">— select value —</option>
            {keyOptions.map((entry) => (
              <option key={String(entry.key)} value={String(entry.key)}>
                {entry.value ? `${String(entry.key)} — ${entry.value}` : String(entry.key)}
              </option>
            ))}
            {strayKey !== undefined && (
              <option value={strayKey}>{strayKey} (not in table)</option>
            )}
          </select>
        ) : (
          <>
            <ExpandableField
              value={formatValue(value.value)}
              onChange={(e) =>
                onChange({ ...value, value: parseValue(e.target.value, value.operator as string) })
              }
              placeholder={takesList ? 'val1, val2, ...' : 'value'}
            />
            {/* A list on a bound field gets its keys offered as an adder. The
                select appends and resets, rather than replacing the list, so
                the typed field stays authoritative and a value the table does
                not list is never taken away. */}
            {listKeyOptions && listKeyOptions.length > 0 && (
              <select
                className={styles.keyAdd}
                value=""
                onChange={(e) => appendKey(e.target.value)}
                aria-label={`add a ${boundTable!.name} key`}
                title={`Keys from the lookup table "${boundTable!.name}"`}
              >
                <option value="">add a {boundTable!.name} key…</option>
                {listKeyOptions
                  .filter((entry) => !currentList.includes(String(entry.key)))
                  .map((entry) => (
                    <option key={String(entry.key)} value={String(entry.key)}>
                      {entry.value ? `${String(entry.key)} — ${entry.value}` : String(entry.key)}
                    </option>
                  ))}
              </select>
            )}
          </>
        )}
      </div>
    </div>
  );
}
