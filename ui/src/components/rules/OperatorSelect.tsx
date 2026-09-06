import type { Operator, FieldType } from '../../api/types';
import { OPERATOR_TYPES } from '../../api/types';
import { operatorSupports } from './operatorRules';

interface Props {
  value: Operator;
  onChange: (op: Operator) => void;
  fieldType?: FieldType;
}

const ALL_OPS: Operator[] = [
  'eq', 'neq', 'gt', 'gte', 'lt', 'lte', 'in', 'not_in', 'contains', 'in_lookup', 'not_in_lookup',
  'is_null', 'is_null_or_empty',
];

export default function OperatorSelect({ value, onChange, fieldType }: Props) {
  const ops = fieldType
    ? ALL_OPS.filter((op) => OPERATOR_TYPES[op].includes(fieldType))
    : ALL_OPS;

  // An operator the field type no longer admits is kept in the list, marked.
  //
  // Filtering it out looks tidy and is a dead end: a select whose value is not
  // among its options renders blank, so the condition shows nothing while the
  // config still holds the operator — and the first sign of trouble is the
  // engine refusing the save, naming a rule the author may not have touched.
  // A field retyped after its rules were written is exactly how that happens:
  // a formula like "10 >= 1" sits happily in a number field compared with gte
  // until someone declares it the boolean it always was.
  const incompatible = !operatorSupports(value, fieldType);

  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value as Operator)}
      style={{
        width: '110px',
        ...(incompatible ? { borderColor: 'var(--danger)', color: 'var(--danger)' } : {}),
      }}
      aria-invalid={incompatible}
      title={
        incompatible
          ? `"${value}" cannot be used on a ${fieldType} field — the engine will refuse this on save. Pick another.`
          : undefined
      }
    >
      {incompatible && (
        <option value={value}>{value} — not valid for {fieldType}</option>
      )}
      {ops.map((op) => (
        <option key={op} value={op}>{op}</option>
      ))}
    </select>
  );
}
