import type { Operator, FieldType } from '../../api/types';
import { operatorOptions } from './operatorRules';

interface Props {
  value: Operator;
  onChange: (op: Operator) => void;
  fieldType?: FieldType;
}

export default function OperatorSelect({ value, onChange, fieldType }: Props) {
  // The list always contains the current value, even when the field type does
  // not admit it — see operatorOptions. Filtering it out looked tidy and made
  // the stored value both invisible and uncorrectable: the select fell back to
  // displaying its first option, so a stranded gte on a boolean field read as
  // "eq", and choosing eq changed nothing the browser could see, so no event
  // fired and the gte stayed. The only sign was the engine refusing the save.
  const options = operatorOptions(value, fieldType);
  const incompatible = options.some((o) => o.op === value && !o.compatible);

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
      {options.map(({ op, compatible }) => (
        <option key={op} value={op}>
          {compatible ? op : `${op} — not valid for ${fieldType}`}
        </option>
      ))}
    </select>
  );
}
