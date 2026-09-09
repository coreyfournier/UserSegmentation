import { Fragment } from 'react';
import type { Rule, InputSchema, OutputField, OutputSchema } from '../../api/types';
import RuleNode from './RuleNode';
import RuleDropZone from './RuleDropZone';
import type { RulePath } from './ruleTree';

interface Props {
  rules: Rule[];
  onChange: (rules: Rule[]) => void;
  /** Path of the parent whose children these are; `[]` at the root. */
  parentPath: RulePath;
  depth: number;
  schema?: InputSchema;
  layerNames?: string[];
  /** True when every rule reports its own message (checklist), not just the winner. */
  perRuleMessages?: boolean;
  /** The segment's output schema. Present only when the segment declares one. */
  outputSchema?: OutputSchema;
  onDeclareOutput?: (name: string, field: OutputField) => void;
  /** This tree is a segment's `when` predicate — see RuleNode. */
  predicate?: boolean;
}

/**
 * One level of the rule tree: each rule interleaved with the drop zones that
 * surround it. Used for both the root list and every group's children, so the
 * two behave identically.
 */
export default function RuleList({
  rules,
  onChange,
  parentPath,
  depth,
  schema,
  layerNames,
  perRuleMessages = false,
  outputSchema,
  onDeclareOutput,
  predicate = false,
}: Props) {
  const update = (index: number, rule: Rule) => {
    const next = [...rules];
    next[index] = rule;
    onChange(next);
  };

  const remove = (index: number) => {
    const next = [...rules];
    next.splice(index, 1);
    onChange(next);
  };

  // Siblings evaluate in array order, so position is precedence. The arrow
  // buttons stay alongside dragging — they are the keyboard-reachable path.
  const swap = (index: number, dir: -1 | 1) => {
    const target = index + dir;
    if (target < 0 || target >= rules.length) return;
    const next = [...rules];
    [next[index], next[target]] = [next[target], next[index]];
    onChange(next);
  };

  return (
    <>
      {rules.map((rule, i) => (
        <Fragment key={i}>
          <RuleDropZone path={[...parentPath, i]} />
          <RuleNode
            rule={rule}
            path={[...parentPath, i]}
            onChange={(r) => update(i, r)}
            onDelete={() => remove(i)}
            index={i}
            total={rules.length}
            onMove={(dir) => swap(i, dir)}
            depth={depth}
            schema={schema}
            layerNames={layerNames}
            perRuleMessages={perRuleMessages}
            outputSchema={outputSchema}
            onDeclareOutput={onDeclareOutput}
            predicate={predicate}
          />
        </Fragment>
      ))}
      <RuleDropZone path={[...parentPath, rules.length]} />
    </>
  );
}
