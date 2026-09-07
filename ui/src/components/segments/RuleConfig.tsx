import type { ReactNode } from 'react';
import type { ComputedField, Rule, InputSchema, OutputField, OutputSchema } from '../../api/types';
import RuleTreeBuilder from '../rules/RuleTreeBuilder';
import MessagesEditor from '../rules/MessagesEditor';
import OutputValuesEditor from '../rules/OutputValuesEditor';
import styles from './RuleConfig.module.css';

interface Props {
  rules: Rule[];
  overrides: Rule[];
  onRulesChange: (rules: Rule[]) => void;
  onOverridesChange: (rules: Rule[]) => void;
  defaultValue: string;
  onDefaultChange: (v: string) => void;
  defaultMessages?: Record<string, string>;
  onDefaultMessagesChange: (v: Record<string, string> | undefined) => void;
  /** The default path's own output values, authored like a rule's. */
  defaultOutputs?: Record<string, string>;
  onDefaultOutputsChange: (o?: Record<string, string>) => void;
  /** The segment's computed fields, so an output a computed one could supply
   *  can offer it. */
  computed?: ComputedField[];
  /** Schema for rules — includes computed fields when applicable. */
  ruleSchema?: InputSchema;
  /** Schema for overrides — raw input fields only (no computed fields). */
  overrideSchema?: InputSchema;
  layerNames?: string[];
  /** Computed-fields editor, rendered between overrides and rules. */
  computedSlot?: ReactNode;
  /** The segment's output schema. Present only when the segment declares one. Applies to rules only — not overrides. */
  outputSchema?: OutputSchema;
  onDeclareOutput?: (name: string, field: OutputField) => void;
}

// Sections are laid out top-to-bottom in evaluation order:
// overrides → computed fields → rules → default.
export default function RuleConfig({
  rules,
  overrides,
  onRulesChange,
  onOverridesChange,
  defaultValue,
  onDefaultChange,
  defaultMessages,
  onDefaultMessagesChange,
  defaultOutputs,
  onDefaultOutputsChange,
  computed,
  ruleSchema,
  overrideSchema,
  layerNames,
  computedSlot,
  outputSchema,
  onDeclareOutput,
}: Props) {
  return (
    <div>
      {/* Each stage is framed and numbered in the order it runs. They used to
          be four unlabelled divs separated by margin, so Overrides read as a
          preamble and Rules and Default ran together as one thing — the
          numbering is what says these are stages of one pass, not a list of
          settings. */}
      <Stage step={1} title="Overrides" accent="override">
        <p className={styles.why}>
          <strong>Use an override to force an outcome regardless of what the strategy would
          decide.</strong>{' '}
          They are the only way to attach a condition to a <code>static</code> or{' '}
          <code>percentage</code> segment, which are otherwise conditionless — an
          enterprise account skipping an experiment, a specific subject pinned for a
          support escalation, a carve-out gated on what an earlier layer resolved
          (<code>layer:x</code>). Each has an <code>enabled</code> flag, so an exception
          can be switched off without losing how it was written.
        </p>
        <p className={styles.note}>
          The first override that matches wins and the strategy never runs. Evaluated
          before computed fields, so only raw input fields are available here.
        </p>
        <RuleTreeBuilder
          rules={overrides}
          onChange={onOverridesChange}
          schema={overrideSchema}
          layerNames={layerNames}
          label="Override Rules"
        />
      </Stage>

      {computedSlot && (
        <Stage step={2} title="Computed fields" accent="computed">
          <p className={styles.note}>
            Derived before the rules run and available to them as ordinary fields.
          </p>
          {computedSlot}
        </Stage>
      )}

      <Stage step={computedSlot ? 3 : 2} title="Rules" accent="rule">
        <p className={styles.note}>
          Evaluated in order; the first match wins and decides the segment. Reached only
          when no override matched.
        </p>
        <RuleTreeBuilder
          rules={rules}
          onChange={onRulesChange}
          schema={ruleSchema}
          layerNames={layerNames}
          label="Rules"
          outputSchema={outputSchema}
          onDeclareOutput={onDeclareOutput}
        />
      </Stage>

      {/* The default is an outcome, not a trailing field: it is what the
          segment resolves to whenever no rule matches, which for most subjects
          is most of the time. */}
      <Stage
        step={computedSlot ? 4 : 3}
        title="Default"
        accent="default"
        badge={!defaultValue ? 'not set' : undefined}
      >
        <p className={styles.note}>
          What the segment resolves to when no rule matched. With none set, it resolves to
          nothing and the layer reports <code>unresolved</code>.
        </p>

        <div className="form-group">
          <label>Resolved value</label>
          <input
            value={defaultValue}
            onChange={(e) => onDefaultChange(e.target.value)}
            placeholder="e.g. standard"
          />
        </div>

        <MessagesEditor value={defaultMessages} onChange={onDefaultMessagesChange} />

        {/* The default authors its own output values, the same way a rule does.
            Without this a field whose value depends on the outcome had to be
            set once for the whole segment — and that one value was then wrong
            for every rule that did match. */}
        {defaultValue && outputSchema && Object.keys(outputSchema).length > 0 && (
          <div style={{ marginTop: 12 }}>
            <label>Output values</label>
            <OutputValuesEditor
              outputs={defaultOutputs}
              schema={outputSchema}
              onChange={onDefaultOutputsChange}
              onDeclare={onDeclareOutput ?? (() => {})}
              computed={computed}
            />
          </div>
        )}
      </Stage>
    </div>
  );
}

interface StageProps {
  step: number;
  title: string;
  /** Selects the accent colour, so a stage is recognisable before it is read. */
  accent: 'override' | 'computed' | 'rule' | 'default';
  /** Short state worth seeing without reading the body — "not set", so far. */
  badge?: string;
  children: ReactNode;
}

/**
 * One stage of a segment's evaluation, framed and numbered.
 *
 * The number is the point: these are steps of a single pass, in the order the
 * engine takes them, not four independent panels. Unnumbered and unframed, the
 * boundary between Rules and Default was invisible and Overrides read as
 * preamble to the section below it rather than a stage of its own.
 */
function Stage({ step, title, accent, badge, children }: StageProps) {
  return (
    <section className={`${styles.stage} ${styles[accent]}`}>
      <div className={styles.stageHead}>
        <span className={styles.step} aria-hidden="true">{step}</span>
        <h4 className={styles.stageTitle}>{title}</h4>
        {badge && <span className={styles.badge}>{badge}</span>}
      </div>
      {children}
    </section>
  );
}
