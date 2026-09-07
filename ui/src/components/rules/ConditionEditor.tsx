import { useId } from 'react';
import type { Condition, InputSchema } from '../../api/types';
import { LOOKUP_OPERATORS, UNARY_OPERATORS } from '../../api/types';
import { useLookups } from '../../api/lookups';
import { parseNumericInput } from '../../utils/parse';
import OperatorSelect from './OperatorSelect';
import { valueFieldOptions } from './operatorRules';
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
  const fieldListId = useId();
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
  // Comparing against another field rather than a literal. The condition
  // itself says which mode it is in — there is no separate UI state to drift
  // out of step with what would be saved.
  const comparesField = !!value.valueField;
  const refOptions = valueFieldOptions(schema, value.field, value.operator);
  const canCompareField = comparesField || refOptions.length > 0;
  // A boolean field gets a picker instead of a text box, for the same reason
  // the test panel's context editor does — but here the cost of typing it is
  // worse than an awkward field. The engine compares the authored value to the
  // context value as-is, so "True" is a string, a string never equals the
  // boolean true, and the check simply never fires. No error, no warning: the
  // rule is just quietly dead. Two options cannot be misspelled.
  const boolPicker = fieldType === 'boolean' && takesOneValue;
  // What is stored, when it is neither boolean. Kept and labelled rather than
  // coerced: silently rewriting "True" to true would repair the config without
  // the author ever learning the rule had not been firing.
  const strayBool =
    boolPicker && value.value !== undefined && typeof value.value !== 'boolean' && currentKey !== ''
      ? currentKey
      : undefined;
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
        {/* The datalist id must be unique per instance. A datalist is addressed
            document-wide, and many of these editors are on the page at once
            with deliberately different schemas — the segment's `when` gets the
            raw input schema, because it is evaluated before computed fields,
            while the rules get those fields merged in. With one shared id every
            input resolved to whichever datalist rendered first, so the whole
            page offered the `when` predicate's fields and a segment's computed
            fields were never listed anywhere. */}
        <input
          value={value.field}
          onChange={(e) => onChange({ ...value, field: e.target.value })}
          placeholder="field"
          list={fieldListId}
        />
        <datalist id={fieldListId}>
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
        {/* The control and its mode switch sit on one line, with the control
            taking the width. Each branch below can render more than one
            element — a warning under a select, a key adder under a textarea —
            so they get their own column here rather than being laid out
            alongside the switch. */}
        <div className={styles.control}>
          {comparesField ? (
            <select
              value={value.valueField ?? ''}
              onChange={(e) => onChange({ ...value, value: undefined, valueField: e.target.value })}
              aria-label={`field to compare ${value.field} against`}
            >
              <option value="" disabled>
                — select field —
              </option>
              {refOptions.map((n) => (
                <option key={n} value={n}>{n}</option>
              ))}
              {/* A reference the schema no longer offers — the field was renamed
                  or retyped out of eligibility. Kept so opening the rule does
                  not silently repoint it at the first option in the list. */}
              {value.valueField && !refOptions.includes(value.valueField) && (
                <option value={value.valueField}>{value.valueField} — no longer comparable</option>
              )}
            </select>
          ) : isUnaryOp ? (
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
          ) : boolPicker ? (
            <>
              <select
                value={typeof value.value === 'boolean' ? String(value.value) : strayBool ?? ''}
                onChange={(e) => {
                  const picked = e.target.value;
                  // Picking the stray option again is not a decision to convert
                  // it — `"True" === 'true'` is false, so a naive parse here
                  // would turn it into the boolean false and hide the mistake
                  // behind a new one.
                  if (picked === 'true' || picked === 'false') {
                    onChange({ ...value, value: picked === 'true' });
                  }
                }}
                aria-label={`${value.field} value`}
              >
                <option value="" disabled>
                  — select —
                </option>
                <option value="true">true</option>
                <option value="false">false</option>
                {strayBool !== undefined && (
                  <option value={strayBool}>{strayBool} — text, not a boolean</option>
                )}
              </select>
              {strayBool !== undefined && (
                <div className={styles.warn}>
                  <code>{strayBool}</code> is text. <code>{value.field}</code> is a boolean, and the
                  engine compares them as they are — so this check never matches. Pick true or false.
                </div>
              )}
            </>
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

        {/* The literal/field switch. Only rendered where a reference is legal
            and there is something to point at — offering it on an operator
            that cannot take one, or on a schema with no comparable field,
            would be a control whose only outcome is a rejected save.

            Switching clears the other side rather than keeping both: a
            condition compares against one or the other, and validation refuses
            a config that sets both. */}
        {canCompareField && (
          <button
            type="button"
            className={styles.modeToggle}
            onClick={() =>
              onChange(
                comparesField
                  ? { ...value, valueField: undefined, value: '' }
                  : { ...value, value: undefined, valueField: refOptions[0] },
              )
            }
            title={
              comparesField
                ? 'Compare against a fixed value instead'
                : 'Compare against another field instead of a fixed value'
            }
          >
            {comparesField ? 'use a value' : 'use a field'}
          </button>
        )}
      </div>
    </div>
  );
}
