import type { Rule, InputSchema, CompositeOperator } from '../../api/types';
import ExpressionEditor from './ExpressionEditor';
import MessagesEditor from './MessagesEditor';
import styles from './RuleNode.module.css';

const DEPTH_COLORS = ['#3b82f6', '#22c55e', '#f97316', '#8b5cf6', '#ec4899'];

interface Props {
  rule: Rule;
  onChange: (r: Rule) => void;
  onDelete: () => void;
  index?: number;
  total?: number;
  onMove?: (dir: -1 | 1) => void;
  depth?: number;
  schema?: InputSchema;
  layerNames?: string[];
}

export default function RuleNode({ rule, onChange, onDelete, index, total, onMove, depth = 0, schema, layerNames }: Props) {
  const color = DEPTH_COLORS[depth % DEPTH_COLORS.length];
  const isLeaf = !!rule.expression;

  const updateChild = (idx: number, child: Rule) => {
    const rules = [...(rule.rules ?? [])];
    rules[idx] = child;
    onChange({ ...rule, rules });
  };

  const deleteChild = (idx: number) => {
    const rules = [...(rule.rules ?? [])];
    rules.splice(idx, 1);
    onChange({ ...rule, rules });
  };

  // Children evaluate in array order (short-circuit), so position matters.
  const moveChild = (idx: number, dir: -1 | 1) => {
    const rules = [...(rule.rules ?? [])];
    const target = idx + dir;
    if (target < 0 || target >= rules.length) return;
    [rules[idx], rules[target]] = [rules[target], rules[idx]];
    onChange({ ...rule, rules });
  };

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
    <div className={styles.node} style={{ borderLeftColor: color }}>
      <div className={styles.header}>
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
        {!isLeaf && (
          <>
            <input
              className={styles.small}
              value={rule.successEvent ?? ''}
              onChange={(e) => onChange({ ...rule, successEvent: e.target.value || undefined })}
              placeholder="successEvent"
            />
            <input
              className={styles.small}
              value={rule.errorMessage ?? ''}
              onChange={(e) => onChange({ ...rule, errorMessage: e.target.value || undefined })}
              placeholder="errorMessage"
            />
          </>
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

      {/* Messages are only rendered for top-level rules (the one whose successEvent
          wins). Nested child rules are boolean conditions — their messages are never
          read — so the editor is hidden there. */}
      {depth === 0 && (
        <MessagesEditor
          value={rule.messages}
          onChange={(m) => onChange({ ...rule, messages: m })}
          hint="Rendered when this rule wins. Use ${field} for variables and expressions."
        />
      )}

      {!isLeaf && (
        <div className={styles.children}>
          {childCount > 1 && <p className={styles.orderHint}>{groupHint}</p>}
          {(rule.rules ?? []).map((child, i) => (
            <RuleNode
              key={i}
              rule={child}
              onChange={(r) => updateChild(i, r)}
              onDelete={() => deleteChild(i)}
              index={i}
              total={childCount}
              onMove={(dir) => moveChild(i, dir)}
              depth={depth + 1}
              schema={schema}
              layerNames={layerNames}
            />
          ))}
          <div className={styles.addButtons}>
            <button className="btn-ghost btn-sm" onClick={addLeaf}>+ Add Expression</button>
            <button className="btn-ghost btn-sm" onClick={addGroup}>+ Add Group</button>
          </div>
        </div>
      )}
    </div>
  );
}
