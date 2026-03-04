import type { Rule, InputSchema } from '../../api/types';
import RuleNode from './RuleNode';
import styles from './RuleTreeBuilder.module.css';

interface Props {
  rules: Rule[];
  onChange: (rules: Rule[]) => void;
  schema?: InputSchema;
  layerNames?: string[];
  label?: string;
}

export default function RuleTreeBuilder({ rules, onChange, schema, layerNames, label = 'Rules' }: Props) {
  const updateRule = (idx: number, rule: Rule) => {
    const next = [...rules];
    next[idx] = rule;
    onChange(next);
  };

  const deleteRule = (idx: number) => {
    const next = [...rules];
    next.splice(idx, 1);
    onChange(next);
  };

  const addTopLevel = () => {
    onChange([
      ...rules,
      { ruleName: '', operator: 'And', successEvent: '', rules: [] },
    ]);
  };

  const addLeaf = () => {
    onChange([
      ...rules,
      { ruleName: '', expression: { field: '', operator: 'eq', value: '' } },
    ]);
  };

  return (
    <div className={styles.builder}>
      <label>{label}</label>
      {rules.map((rule, i) => (
        <RuleNode
          key={i}
          rule={rule}
          onChange={(r) => updateRule(i, r)}
          onDelete={() => deleteRule(i)}
          schema={schema}
          layerNames={layerNames}
        />
      ))}
      <div className={styles.addButtons}>
        <button className="btn-ghost btn-sm" onClick={addTopLevel}>+ Add Group</button>
        <button className="btn-ghost btn-sm" onClick={addLeaf}>+ Add Expression</button>
      </div>
    </div>
  );
}
