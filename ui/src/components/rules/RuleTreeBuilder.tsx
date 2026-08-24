import { useMemo, useRef, useState } from 'react';
import type { Rule, InputSchema } from '../../api/types';
import RuleList from './RuleList';
import { RuleDragContext, type RuleDragValue } from './RuleDragContext';
import { moveRule, type RulePath } from './ruleTree';
import styles from './RuleTreeBuilder.module.css';

interface Props {
  rules: Rule[];
  onChange: (rules: Rule[]) => void;
  schema?: InputSchema;
  layerNames?: string[];
  label?: string;
  /** Replaces the default evaluation-order caption. */
  hint?: string;
  /** Hides the add buttons once this many root rules exist. */
  maxRules?: number;
  /** True when every rule reports its own message (checklist), not just the winner. */
  perRuleMessages?: boolean;
}

export default function RuleTreeBuilder({
  rules,
  onChange,
  schema,
  layerNames,
  label = 'Rules',
  hint,
  maxRules,
  perRuleMessages = false,
}: Props) {
  const [dragPath, setDragPath] = useState<RulePath | null>(null);
  // The source is also held in a ref because a drop can arrive before React
  // re-renders with the new state. State drives the highlight; the ref is what
  // the drop handler reads.
  const dragSource = useRef<RulePath | null>(null);

  // Each tree owns its own drag state, so a rule cannot be dragged from the
  // rules list into the overrides list — they are separate concerns.
  const drag = useMemo<RuleDragValue>(
    () => ({
      dragPath,
      rootRules: rules,
      beginDrag: (path) => {
        dragSource.current = path;
        setDragPath(path);
      },
      endDrag: () => {
        dragSource.current = null;
        setDragPath(null);
      },
      dropAt: (to) => {
        const from = dragSource.current;
        dragSource.current = null;
        setDragPath(null);
        if (from) onChange(moveRule(rules, from, to));
      },
    }),
    [dragPath, rules, onChange]
  );

  const addTopLevel = () => {
    onChange([...rules, { ruleName: '', operator: 'And', successEvent: '', rules: [] }]);
  };

  const addLeaf = () => {
    onChange([...rules, { ruleName: '', condition: { field: '', operator: 'eq', value: '' } }]);
  };

  const atCapacity = maxRules !== undefined && rules.length >= maxRules;

  return (
    <div className={styles.builder}>
      {label && <label>{label}</label>}
      <p className={styles.orderHint}>
        {hint ??
          'Evaluated top to bottom — the first matching rule wins. Drag the ⠿ handle to move a rule into a group, out of one, or across to another.'}
      </p>
      <RuleDragContext.Provider value={drag}>
        <RuleList
          rules={rules}
          onChange={onChange}
          parentPath={[]}
          depth={0}
          schema={schema}
          layerNames={layerNames}
          perRuleMessages={perRuleMessages}
        />
      </RuleDragContext.Provider>
      {!atCapacity && (
        <div className={styles.addButtons}>
          <button className="btn-ghost btn-sm" onClick={addTopLevel}>+ Add Group</button>
          <button className="btn-ghost btn-sm" onClick={addLeaf}>+ Add Check</button>
        </div>
      )}
    </div>
  );
}
