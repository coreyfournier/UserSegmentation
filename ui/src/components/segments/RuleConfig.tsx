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
      <div>
        <RuleTreeBuilder
          rules={overrides}
          onChange={onOverridesChange}
          schema={overrideSchema}
          layerNames={layerNames}
          label="Overrides"
        />
        <p style={{ fontSize: 11, color: 'var(--text-muted)', fontStyle: 'italic', margin: '4px 0 0' }}>
          Evaluated first, before computed fields and rules. Only raw input fields are
          available here — computed fields cannot be referenced.
        </p>
      </div>

      {computedSlot && <div style={{ marginTop: 24 }}>{computedSlot}</div>}

      <div style={{ marginTop: 24 }}>
        <RuleTreeBuilder
          rules={rules}
          onChange={onRulesChange}
          schema={ruleSchema}
          layerNames={layerNames}
          label="Rules"
          outputSchema={outputSchema}
          onDeclareOutput={onDeclareOutput}
        />
      </div>

      {/* The default is an outcome, not a trailing field: it is what the
          segment resolves to whenever no rule matches, which for most subjects
          is most of the time. It used to be a bare input at the bottom, easy
          to skim past — hence the framing and the rule above it. */}
      <div className={styles.default}>
        <div className={styles.defaultHead}>
          <h4 className={styles.defaultTitle}>Default — when no rule matches</h4>
          {!defaultValue && <span className={styles.unset}>not set</span>}
        </div>
        <p className={styles.defaultNote}>
          With no default, a segment where nothing matches resolves to nothing at all and
          the layer reports <code>unresolved</code>.
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
      </div>
    </div>
  );
}
