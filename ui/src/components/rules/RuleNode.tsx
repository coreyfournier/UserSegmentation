import { useRef } from 'react';
import type { Rule, InputSchema, CompositeOperator, OutputField, OutputSchema } from '../../api/types';
import ConditionEditor from './ConditionEditor';
import MessagesEditor from './MessagesEditor';
import OutputValuesEditor from './OutputValuesEditor';
import RuleList from './RuleList';
import { useRuleDrag } from './RuleDragContext';
import { describeRule, samePath, toGroup, toLeaf, type RulePath } from './ruleTree';
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
  /** The segment's output schema. Present only when the segment declares one. */
  outputSchema?: OutputSchema;
  onDeclareOutput?: (name: string, field: OutputField) => void;
  /**
   * This tree is an applicability predicate (a segment's `when`) rather than a
   * list of reporting rules.
   *
   * A predicate is only ever asked "does this hold?", so the fields a rule
   * carries for reporting — successEvent, errorMessage, messages — are read by
   * nothing when they sit on a `when`. Rendering editors for them would invite
   * writing config the engine never looks at.
   */
  predicate?: boolean;
}

export default function RuleNode({ rule, path, onChange, onDelete, index, total, onMove, depth = 0, schema, layerNames, perRuleMessages = false, outputSchema, onDeclareOutput, predicate = false }: Props) {
  const color = DEPTH_COLORS[depth % DEPTH_COLORS.length];
  const isLeaf = !!rule.condition;

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
          condition: { field: '', operator: 'eq', value: '' },
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
            <button type="button"
              className="btn-ghost btn-sm"
              onClick={() => onMove(-1)}
              disabled={index === 0}
              title="Move up"
              aria-label="Move up"
            >
              ▲
            </button>
            <button type="button"
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
        {/* What this node is, and the one control that changes it.

            A leaf used to be an inert LEAF badge, so a check could never become
            a group: you added a group beside it and dragged it in. That works
            in a rules list, but a predicate is capped at one root — there is
            nowhere to add the group — so a single condition could never grow
            into an And/Or without being deleted and retyped.

            Converting a leaf keeps its condition as the new group's first
            child, so nothing written is lost. Converting back is offered only
            while the group has no children, because absorbing several
            conditions into one is not a thing this control could do honestly. */}
        <select
          className={styles.opSelect}
          value={isLeaf ? 'Leaf' : rule.operator ?? 'And'}
          aria-label="node type"
          title={
            isLeaf
              ? 'A single condition. Switch to AND/OR to group it with others.'
              : 'A group of conditions.'
          }
          onChange={(e) => {
            const next = e.target.value;
            onChange(
              next === 'Leaf' ? toLeaf(rule) : toGroup(rule, next as CompositeOperator),
            );
          }}
          style={{ borderColor: color, color }}
        >
          <option value="Leaf" disabled={!isLeaf && childCount > 0}>
            CHECK
          </option>
          <option value="And">AND</option>
          <option value="Or">OR</option>
        </select>
        <input
          className={styles.ruleName}
          value={rule.ruleName}
          onChange={(e) => onChange({ ...rule, ruleName: e.target.value })}
          placeholder="rule name"
        />
        {/* A checklist resolves no segment value, so successEvent is dead
            config there. Neither does a predicate: it is asked whether it
            holds, and nothing reads what it would have reported. */}
        {!isLeaf && !perRuleMessages && !predicate && (
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
        {(perRuleMessages || !isLeaf) && !predicate && (
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
        <button type="button" className="btn-danger btn-sm" onClick={onDelete}>x</button>
      </div>

      {/* The logic first, then what it reports — the same order whether this
          node's logic is one condition or a group of nested rules. Children
          used to be rendered last, so a leaf read condition-then-outputs while
          a group read outputs-then-children, and the same two things swapped
          places depending on the node you were looking at. Overrides render
          through this component too, so they follow suit. */}
      {isLeaf && rule.condition && (
        <div className={styles.exprWrap}>
          <ConditionEditor
            value={rule.condition}
            onChange={(cond) => onChange({ ...rule, condition: cond })}
            schema={schema}
            layerNames={layerNames}
          />
        </div>
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
            outputSchema={outputSchema}
            onDeclareOutput={onDeclareOutput}
            predicate={predicate}
          />
          <div className={styles.addButtons}>
            <button type="button" className="btn-ghost btn-sm" onClick={addLeaf}>+ Add Check</button>
            <button type="button" className="btn-ghost btn-sm" onClick={addGroup}>+ Add Group</button>
          </div>
        </div>
      )}

      {/* Only a reporting rule emits a record, so only a reporting rule gets
          output values — and "reporting" means top-level, in both strategies.
          A checklist reports every top-level rule, leaf or And/Or group alike;
          a rule segment reports whichever top-level rule wins. Nothing nested
          reports in either: an And/Or group reports once, under its own name,
          and its branches only contribute to that one condition. Depth is the
          test, not leafness — gating on (perRuleMessages || !isLeaf) would let
          a leaf inside a checklist's And/Or group author values the engine
          never reads, which is dead config nothing would flag. */}
      {onDeclareOutput && depth === 0 && (
        // Framed and labelled as this rule's own. Rendered bare, a rule
        // setting six values was a wall of controls with nothing tying them to
        // the rule above — and nothing saying they belong to this rule rather
        // than to the segment.
        <div className={styles.outputs}>
          <div className={styles.outputsHead}>
            <span className={styles.outputsTitle}>
              Output values for <code>{rule.ruleName || '(unnamed)'}</code>
            </span>
            {Object.keys(rule.outputs ?? {}).length > 0 && (
              <span className={styles.outputsCount}>
                {Object.keys(rule.outputs ?? {}).length} set
              </span>
            )}
          </div>
          <OutputValuesEditor
            outputs={rule.outputs}
            schema={outputSchema ?? {}}
            onChange={(o) => onChange({ ...rule, outputs: o })}
            onDeclare={onDeclareOutput}
          />
        </div>
      )}

      {/* Under first-match strategies only the winning top-level rule's message
          is ever rendered, so nested editors would be dead config. A checklist
          is the opposite: every check that fires carries its own message. A
          predicate reports nothing at all, so it has neither. */}
      {(depth === 0 || perRuleMessages) && !predicate && (
        <MessagesEditor
          value={rule.messages}
          onChange={(m) => onChange({ ...rule, messages: m })}
          hint={
            perRuleMessages
              ? "Localized text reported when this check fires. Use ${field} for variables and formulas."
              : "Rendered when this rule wins. Use ${field} for variables and formulas."
          }
        />
      )}
    </div>
  );
}
