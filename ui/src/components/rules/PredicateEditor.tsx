import { useState } from 'react';
import type { Rule, InputSchema } from '../../api/types';
import RuleTreeBuilder from './RuleTreeBuilder';
import styles from './PredicateEditor.module.css';

interface Props {
  value?: Rule;
  onChange: (rule: Rule | undefined) => void;
  schema?: InputSchema;
  layerNames?: string[];
  /** Text describing what the condition governs. */
  hint: string;
}

/**
 * Edits an applicability condition — the `when` on a segment or a rule.
 *
 * The condition is a rule tree in its own right, so it reuses the same builder
 * and can itself be an And/Or of several tests. It is capped at one root rule
 * because a condition is a single question, not a first-match list.
 *
 * This lives on a segment only. Gating a block of checks is done structurally —
 * put them in their own segment or layer — rather than per rule.
 */
export default function PredicateEditor({ value, onChange, schema, layerNames, hint }: Props) {
  const [open, setOpen] = useState(!!value);

  return (
    <div className={styles.root}>
      <button type="button" className={styles.toggle} onClick={() => setOpen((o) => !o)}>
        {open ? '▾' : '▸'} Only when{value ? ' (set)' : ''}
      </button>

      {open && (
        <div className={styles.body}>
          <p className={styles.hint}>{hint}</p>
          {value ? (
            <RuleTreeBuilder
              rules={[value]}
              onChange={(rules) => onChange(rules[0])}
              schema={schema}
              layerNames={layerNames}
              label=""
              hint="Delete the condition to make this apply unconditionally."
              maxRules={1}
            />
          ) : (
            <button
              type="button"
              className="btn-ghost btn-sm"
              onClick={() => onChange({ ruleName: '', expression: { field: '', operator: 'eq', value: '' } })}
            >
              + Add condition
            </button>
          )}
        </div>
      )}
    </div>
  );
}
