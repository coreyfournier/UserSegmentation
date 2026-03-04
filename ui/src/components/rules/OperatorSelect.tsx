import type { Operator, FieldType } from '../../api/types';
import { OPERATOR_TYPES } from '../../api/types';

interface Props {
  value: Operator;
  onChange: (op: Operator) => void;
  fieldType?: FieldType;
}

const ALL_OPS: Operator[] = ['eq', 'neq', 'gt', 'gte', 'lt', 'lte', 'in', 'contains'];

export default function OperatorSelect({ value, onChange, fieldType }: Props) {
  const ops = fieldType
    ? ALL_OPS.filter((op) => OPERATOR_TYPES[op].includes(fieldType))
    : ALL_OPS;

  return (
    <select value={value} onChange={(e) => onChange(e.target.value as Operator)} style={{ width: '110px' }}>
      {ops.map((op) => (
        <option key={op} value={op}>{op}</option>
      ))}
    </select>
  );
}
