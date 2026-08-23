import { useState, useEffect, useRef } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useLayers } from '../../api/layers';
import { useUpdateSegment } from '../../api/segments';
import type { Segment, StrategyType, InputSchema } from '../../api/types';
import StrategyPicker from './StrategyPicker';
import StaticConfig from './StaticConfig';
import PercentageConfig from './PercentageConfig';
import ExpressionConfig from './ExpressionConfig';
import RuleConfig from './RuleConfig';
import RuleTreeBuilder from '../rules/RuleTreeBuilder';
import MessagesEditor from '../rules/MessagesEditor';
import PredicateEditor from '../rules/PredicateEditor';
import PromotionEditor from '../promotion/PromotionEditor';
import InputSchemaEditor from '../schema/InputSchemaEditor';
import ErrorBanner from '../common/ErrorBanner';
import styles from './SegmentEditor.module.css';

export default function SegmentEditor() {
  const { name: layerName, id: segId } = useParams<{ name: string; id: string }>();
  const navigate = useNavigate();
  const { data: layers } = useLayers();
  const updateSegment = useUpdateSegment();

  const layer = layers?.find((l) => l.name === layerName);
  const original = layer?.segments.find((s) => s.id === segId);
  // A rule may only reference layers this one declares a dependency on, so the
  // picker offers exactly those — the UI cannot build a config validation rejects.
  const layerNames = layer?.dependsOn ?? [];

  const [seg, setSeg] = useState<Segment | null>(null);
  const segRef = useRef(seg);
  segRef.current = seg;

  useEffect(() => {
    if (original && !segRef.current) setSeg(structuredClone(original));
  }, [original]);

  if (!seg) return <p>Loading segment...</p>;

  const update = (partial: Partial<Segment>) =>
    setSeg((prev) => (prev ? { ...prev, ...partial } : prev));

  const switchStrategy = (strategy: StrategyType) => {
    setSeg((prev) => {
      if (!prev) return prev;
      const next: Partial<Segment> = { strategy };
      if (strategy === 'static') next.static = prev.static ?? { mappings: {}, default: '' };
      if (strategy === 'percentage') next.percentage = prev.percentage ?? { salt: '', buckets: [] };
      if (strategy === 'rule') {
        next.rules = prev.rules ?? [];
        next.default = prev.default ?? '';
      }
      if (strategy === 'expression') {
        next.expressions = prev.expressions ?? [];
        next.rules = prev.rules ?? [];
        next.default = prev.default ?? '';
      }
      if (strategy === 'assert') {
        // No default: every rule is an assertion that must hold, so there is no
        // "nothing matched" outcome to fall back to.
        next.expressions = prev.expressions ?? [];
        next.rules = prev.rules ?? [];
      }
      return { ...prev, ...next };
    });
  };

  // Expression and assert both compute fields before rules run, so merge them
  // into the schema used for the rule field autocomplete.
  const effectiveSchema = (s: Segment): InputSchema | undefined => {
    const computes = s.strategy === 'expression' || s.strategy === 'assert';
    if (!computes || !s.expressions?.length) return s.inputSchema;
    const merged: InputSchema = { ...s.inputSchema };
    for (const def of s.expressions) {
      if (def.name) merged[def.name] = { type: def.type, required: false };
    }
    return merged;
  };

  const handleSave = () => {
    if (!layerName || !segId || !segRef.current) return;
    updateSegment.mutate(
      { layerName, segId, segment: segRef.current },
      { onSuccess: () => navigate('/layers') }
    );
  };

  return (
    <div className={styles.editor}>
      <div className={styles.toolbar}>
        <h2>
          <span className={styles.breadcrumb} onClick={() => navigate('/layers')}>Layers</span>
          {' / '}
          <span className={styles.breadcrumb}>{layerName}</span>
          {' / '}
          {seg.id}
        </h2>
      </div>

      {updateSegment.error && <ErrorBanner message={(updateSegment.error as Error).message} />}

      {/* Strategy */}
      <section className={`card ${styles.section}`}>
        <h3>Strategy</h3>
        <StrategyPicker value={seg.strategy as StrategyType} onChange={switchStrategy} />
      </section>

      {/* Promotion */}
      <section className={`card ${styles.section}`}>
        <h3>Promotion Window</h3>
        <PromotionEditor value={seg.promotion} onChange={(p) => update({ promotion: p })} />
      </section>

      {/* Input Schema */}
      <section className={`card ${styles.section}`}>
        <h3>Input Schema</h3>
        <InputSchemaEditor
          value={seg.inputSchema}
          onChange={(s) => update({ inputSchema: s })}
        />
      </section>

      {/* Applicability — after the schema, because the condition picks its
          fields from it and would otherwise offer nothing to choose. */}
      <section className={`card ${styles.section}`}>
        <h3>Applies When</h3>
        <PredicateEditor
          value={seg.when}
          onChange={(when) => update({ when })}
          schema={seg.inputSchema}
          layerNames={layerNames}
          hint={
            'Dispatch condition for the whole segment, tested against the fields declared ' +
            'above. When it does not hold the segment is passed over entirely and the next ' +
            'one in the layer is tried — this is how one layer holds a variant per entity type.'
          }
        />
      </section>

      {/* Strategy Config */}
      <section className={`card ${styles.section}`}>
        <h3>Configuration</h3>
        {seg.strategy === 'static' && seg.static && (
          <StaticConfig value={seg.static} onChange={(v) => update({ static: v })} />
        )}
        {seg.strategy === 'percentage' && seg.percentage && (
          <PercentageConfig value={seg.percentage} onChange={(v) => update({ percentage: v })} />
        )}
        {seg.strategy === 'rule' && (
          <RuleConfig
            rules={seg.rules ?? []}
            overrides={seg.overrides ?? []}
            onRulesChange={(r) => update({ rules: r })}
            onOverridesChange={(r) => update({ overrides: r })}
            defaultValue={seg.default ?? ''}
            onDefaultChange={(v) => update({ default: v })}
            defaultMessages={seg.defaultMessages}
            onDefaultMessagesChange={(m) => update({ defaultMessages: m })}
            ruleSchema={seg.inputSchema}
            overrideSchema={seg.inputSchema}
            layerNames={layerNames}
          />
        )}
        {seg.strategy === 'expression' && (
          <RuleConfig
            rules={seg.rules ?? []}
            overrides={seg.overrides ?? []}
            onRulesChange={(r) => update({ rules: r })}
            onOverridesChange={(r) => update({ overrides: r })}
            defaultValue={seg.default ?? ''}
            onDefaultChange={(v) => update({ default: v })}
            defaultMessages={seg.defaultMessages}
            onDefaultMessagesChange={(m) => update({ defaultMessages: m })}
            ruleSchema={effectiveSchema(seg)}
            overrideSchema={seg.inputSchema}
            layerNames={layerNames}
            expressionsSlot={
              <div className="form-group">
                <label>Expressions</label>
                <ExpressionConfig
                  value={seg.expressions ?? []}
                  onChange={(e) => update({ expressions: e })}
                />
              </div>
            }
          />
        )}
        {seg.strategy === 'assert' && (
          <div>
            <p style={{ fontSize: 12, color: 'var(--text-muted)', margin: '0 0 16px' }}>
              Every assertion below must hold. The whole tree is evaluated — each one
              that fails is itemised, so the person fixing them sees all the problems at
              once. A failing <code>Or</code> reports itself rather than each branch.
              There is no default and no overrides: an assert segment has no
              &ldquo;nothing matched&rdquo; outcome.
            </p>

            <div className="form-group">
              <label>Expressions</label>
              <ExpressionConfig
                value={seg.expressions ?? []}
                onChange={(e) => update({ expressions: e })}
              />
              <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
                Computed fields are available to the assertions and to their messages.
                If one fails at runtime the whole gate reports <code>unevaluable</code>
                {' '}rather than reporting its dependent assertions as violations.
              </p>
            </div>

            <div style={{ marginTop: 24 }}>
              <RuleTreeBuilder
                rules={seg.rules ?? []}
                onChange={(r) => update({ rules: r })}
                schema={effectiveSchema(seg)}
                layerNames={layerNames}
                label="Assertions"
                perRuleMessages
                hint={
                  'Every assertion must hold. Drag the handle to move one into a group, out ' +
                  'of one, or across to another. Give a group an "Only when" condition to ' +
                  'gate a whole block of checks on one test.'
                }
              />
              <p style={{ fontSize: 11, color: 'var(--text-muted)', fontStyle: 'italic', margin: '4px 0 0' }}>
                Each rule name is the stable identifier reported with its failure, so make
                it descriptive and avoid renaming it once anything depends on it.
              </p>
            </div>
          </div>
        )}
      </section>

      {/* Overrides for strategies whose config section does not already include them */}
      {seg.strategy !== 'rule' && seg.strategy !== 'expression' && seg.strategy !== 'assert' && (
        <section className={`card ${styles.section}`}>
          <h3>Overrides</h3>
          <RuleTreeBuilder
            rules={seg.overrides ?? []}
            onChange={(r) => update({ overrides: r })}
            schema={seg.inputSchema}
            layerNames={layerNames}
            label="Override Rules"
          />
          <p style={{ fontSize: 11, color: 'var(--text-muted)', fontStyle: 'italic', margin: '4px 0 0' }}>
            Evaluated before the strategy result. Only raw input fields are available.
          </p>
          <div style={{ marginTop: 16 }}>
            <label>Default Value</label>
            <input
              value={seg.default ?? ''}
              onChange={(e) => update({ default: e.target.value || undefined })}
            />
            <MessagesEditor
              value={seg.defaultMessages}
              onChange={(m) => update({ defaultMessages: m })}
            />
          </div>
        </section>
      )}

      {/* Footer */}
      <div className={styles.footer}>
        <button className="btn-ghost" onClick={() => navigate('/layers')}>Cancel</button>
        <button className="btn-primary" onClick={handleSave} disabled={updateSegment.isPending}>
          {updateSegment.isPending ? 'Saving...' : 'Save'}
        </button>
      </div>
    </div>
  );
}
