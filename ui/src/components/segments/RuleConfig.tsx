import type { ReactNode } from 'react';
import type { Rule, InputSchema } from '../../api/types';
import RuleTreeBuilder from '../rules/RuleTreeBuilder';
import MessagesEditor from '../rules/MessagesEditor';

interface Props {
  rules: Rule[];
  overrides: Rule[];
  onRulesChange: (rules: Rule[]) => void;
  onOverridesChange: (rules: Rule[]) => void;
  defaultValue: string;
  onDefaultChange: (v: string) => void;
  defaultMessages?: Record<string, string>;
  onDefaultMessagesChange: (v: Record<string, string> | undefined) => void;
  /** Schema for rules — includes computed expression fields when applicable. */
  ruleSchema?: InputSchema;
  /** Schema for overrides — raw input fields only (no computed fields). */
  overrideSchema?: InputSchema;
  layerNames?: string[];
  /** Expressions editor, rendered between overrides and rules (expression strategy). */
  expressionsSlot?: ReactNode;
}

// Sections are laid out top-to-bottom in evaluation order:
// overrides → expressions → rules → default.
export default function RuleConfig({
  rules,
  overrides,
  onRulesChange,
  onOverridesChange,
  defaultValue,
  onDefaultChange,
  defaultMessages,
  onDefaultMessagesChange,
  ruleSchema,
  overrideSchema,
  layerNames,
  expressionsSlot,
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
          Evaluated first, before expressions and rules. Only raw input fields are
          available here — computed expression fields cannot be referenced.
        </p>
      </div>

      {expressionsSlot && <div style={{ marginTop: 24 }}>{expressionsSlot}</div>}

      <div style={{ marginTop: 24 }}>
        <RuleTreeBuilder
          rules={rules}
          onChange={onRulesChange}
          schema={ruleSchema}
          layerNames={layerNames}
          label="Rules"
        />
      </div>

      <div className="form-group" style={{ marginTop: 16 }}>
        <label>Default Value</label>
        <input value={defaultValue} onChange={(e) => onDefaultChange(e.target.value)} />
        <MessagesEditor value={defaultMessages} onChange={onDefaultMessagesChange} />
      </div>
    </div>
  );
}
