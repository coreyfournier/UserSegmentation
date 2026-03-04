import type { Rule, InputSchema } from '../../api/types';
import RuleTreeBuilder from '../rules/RuleTreeBuilder';

interface Props {
  rules: Rule[];
  overrides: Rule[];
  onRulesChange: (rules: Rule[]) => void;
  onOverridesChange: (rules: Rule[]) => void;
  defaultValue: string;
  onDefaultChange: (v: string) => void;
  schema?: InputSchema;
  layerNames?: string[];
}

export default function RuleConfig({
  rules,
  overrides,
  onRulesChange,
  onOverridesChange,
  defaultValue,
  onDefaultChange,
  schema,
  layerNames,
}: Props) {
  return (
    <div>
      <RuleTreeBuilder
        rules={rules}
        onChange={onRulesChange}
        schema={schema}
        layerNames={layerNames}
        label="Rules"
      />
      <div style={{ marginTop: 24 }}>
        <RuleTreeBuilder
          rules={overrides}
          onChange={onOverridesChange}
          schema={schema}
          layerNames={layerNames}
          label="Overrides"
        />
      </div>
      <div className="form-group" style={{ marginTop: 16 }}>
        <label>Default Value</label>
        <input value={defaultValue} onChange={(e) => onDefaultChange(e.target.value)} />
      </div>
    </div>
  );
}
