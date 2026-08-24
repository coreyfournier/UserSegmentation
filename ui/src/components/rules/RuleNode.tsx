import { useRef } from 'react';
import type { Rule, InputSchema, CompositeOperator } from '../../api/types';
import ExpressionEditor from './ExpressionEditor';
import MessagesEditor from './MessagesEditor';
import RuleList from './RuleList';
import { useRuleDrag } from './RuleDragContext';
import { describeRule, samePath, type RulePath } from './ruleTree';
import styles from './RuleNode.module.css';

const DEPTH_COLORS = ['#3b82f6', '#22c55e', '#f97316', '#8b5cf6', '#ec4899'];

interface Props {
  rule: Rule;
  /** This node's address in the tree, used to move it anywhere. */
  path: RulePath;
  onChange: (r: Rule) => void;
  onDelete: () => void;
  index?: number;
  total?: number;
  onMove?: (dir: -1 | 1) => void;
  depth?: number;
  schema?: InputSchema;
  layerNames?: string[];
  /** True when every rule reports its own message (checklist), not just the winner. */
  perRuleMessages?: boolean;
}

export default function RuleNode({ rule, path, onChange, onDelete, index, total, onMove, depth = 0, schema, layerNames, perRuleMessages = false }: Props) {
  const color = DEPTH_COLORS[depth % DEPTH_COLORS.length];
  const isLeaf = !!rule.expression;

  const { dragPath, beginDrag, endDrag } = useRuleDrag();
  const nodeRef = useRef<HTMLDivElement>(null);
  const isDragging = dragPath !== null && samePath(dragPath, path);

  const addLeaf = () => {
    onChange({
      ...rule,
      rules: [
        ...(rule.rules ?? []),
        {
          ruleName: '',
          expression: { field: '', operator: 'eq', value: '' },
        },
      ],
    });
  };

  const addGroup = () => {
    onChange({
      ...rule,
      rules: [
        ...(rule.rules ?? []),
        { ruleName: '', operator: 'And', rules: [] },
      ],
    });
  };

  const enabled = rule.enabled !== false;
  const childCount = (rule.rules ?? []).length;
  const groupHint =
    (rule.operator ?? 'And') === 'Or'
      ? 'Any match wins — evaluated in order, stops at the first match.'
      : 'All must pass — evaluated in order, stops at the first failure.';

  return (
    <div
      ref={nodeRef}
      className={`${styles.node} ${isDragging ? styles.dragging : ''}`}
      style={{ borderLeftColor: color }}
    >
      <div className={styles.header}>
        {/* Only the grip is draggable, so the inputs stay selectable. The drag
            image is the whole node, so what you see moving is what will move. */}
        <span
          className={styles.grip}
          draggable
          role="button"
          tabIndex={-1}
          aria-label={`Drag ${describeRule(rule)} to move it into or out of a group`}
          title="Drag to move — into a group, out of one, or across to another"
          onDragStart={(e) => {
            e.dataTransfer.effectAllowed = 'move';
            // Firefox will not start a drag unless some data is set.
            e.dataTransfer.setData('text/plain', describeRule(rule));
            if (nodeRef.current) e.dataTransfer.setDragImage(nodeRef.current, 12, 12);
            beginDrag(path);
          }}
          onDragEnd={endDrag}
        >
          ⠿
        </span>
        {index !== undefined && (
          <span className={styles.position} title="Evaluation order">{index + 1}</span>
        )}
        {onMove && (
          <span className={styles.moveButtons}>
            <button
              className="btn-ghost btn-sm"
              onClick={() => onMove(-1)}
              disabled={index === 0}
              title="Move up"
              aria-label="Move up"
            >
              ▲
            </button>
            <button
              className="btn-ghost btn-sm"
              onClick={() => onMove(1)}
              disabled={total !== undefined && index !== undefined && index === total - 1}
              title="Move down"
              aria-label="Move down"
            >
              ▼
            </button>
          </span>
        )}
        {isLeaf ? (
          <span className={styles.badge} style={{ background: color }}>LEAF</span>
        ) : (
          <select
            className={styles.opSelect}
            value={rule.operator ?? 'And'}
            onChange={(e) => onChange({ ...rule, operator: e.target.value as CompositeOperator })}
            style={{ borderColor: color, color }}
          >
            <option value="And">AND</option>
            <option value="Or">OR</option>
          </select>
        )}
        <input
          className={styles.ruleName}
          value={rule.ruleName}
          onChange={(e) => onChange({ ...rule, ruleName: e.target.value })}
          placeholder="rule name"
        />
        {/* A checklist resolves no segment value, so successEvent is dead
            config there. */}
        {!isLeaf && !perRuleMessages && (
          <input
            className={styles.small}
            value={rule.successEvent ?? ''}
            onChange={(e) => onChange({ ...rule, successEvent: e.target.value || undefined })}
            placeholder="successEvent"
          />
        )}
        {/* errorMessage is the text reported when a check fires, so under a
            checklist every rule needs it — including leaves, which are the
            common case. Elsewhere it stays where it has always been. */}
        {(perRuleMessages || !isLeaf) && (
          <input
            className={perRuleMessages ? styles.message : styles.small}
            value={rule.errorMessage ?? ''}
            onChange={(e) => onChange({ ...rule, errorMessage: e.target.value || undefined })}
            placeholder={perRuleMessages ? 'failure message' : 'errorMessage'}
            title={
              perRuleMessages
                ? 'errorMessage — reported when this check fires. Supports ${field} interpolation.'
                : 'errorMessage'
            }
          />
        )}
        <label className={styles.toggle}>
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => onChange({ ...rule, enabled: e.target.checked })}
            style={{ width: 'auto' }}
          />
          <span>enabled</span>
        </label>
        <button className="btn-danger btn-sm" onClick={onDelete}>x</button>
      </div>

      {isLeaf && rule.expression && (
        <div className={styles.exprWrap}>
          <ExpressionEditor
            value={rule.expression}
            onChange={(expr) => onChange({ ...rule, expression: expr })}
            schema={schema}
            layerNames={layerNames}
          />
        </div>
      )}

      {/* Under first-match strategies only the winning top-level rule's message
          is ever rendered, so nested editors would be dead config. A checklist
          is the opposite: every check that fires carries its own message. */}
      {(depth === 0 || perRuleMessages) && (
        <MessagesEditor
          value={rule.messages}
          onChange={(m) => onChange({ ...rule, messages: m })}
          hint={
            perRuleMessages
              ? "Localized text reported when this check fires. Use ${field} for variables and expressions."
              : "Rendered when this rule wins. Use ${field} for variables and expressions."
          }
        />
      )}

      {!isLeaf && (
        <div className={styles.children}>
          {childCount > 1 && <p className={styles.orderHint}>{groupHint}</p>}
          <RuleList
            rules={rule.rules ?? []}
            onChange={(rules) => onChange({ ...rule, rules })}
            parentPath={path}
            depth={depth + 1}
            schema={schema}
            layerNames={layerNames}
            perRuleMessages={perRuleMessages}
          />
          <div className={styles.addButtons}>
            <button className="btn-ghost btn-sm" onClick={addLeaf}>+ Add Expression</button>
            <button className="btn-ghost btn-sm" onClick={addGroup}>+ Add Group</button>
          </div>
        </div>
      )}
    </div>
  );
}
