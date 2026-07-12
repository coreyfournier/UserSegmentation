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
  const layerNames = layers?.map((l) => l.name) ?? [];

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
      return { ...prev, ...next };
    });
  };

  // For expression strategy, merge inputSchema with expression-defined fields so rules
  // can reference computed fields in the field autocomplete.
  const effectiveSchema = (s: Segment): InputSchema | undefined => {
    if (s.strategy !== 'expression' || !s.expressions?.length) return s.inputSchema;
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
      </section>

      {/* Overrides for non-rule/non-expression strategies */}
      {seg.strategy !== 'rule' && seg.strategy !== 'expression' && (
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
