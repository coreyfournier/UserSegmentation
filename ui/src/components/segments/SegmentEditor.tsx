import { useState, useEffect, useRef } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useLayers, useUpdateLayer } from '../../api/layers';
import { useLookups } from '../../api/lookups';
import { useUpdateSegment } from '../../api/segments';
import type { Segment, StrategyType, InputSchema, OutputField } from '../../api/types';
import StrategyPicker from './StrategyPicker';
import StaticConfig from './StaticConfig';
import PercentageConfig from './PercentageConfig';
import ComputedFieldsEditor from './ComputedFieldsEditor';
import RuleConfig from './RuleConfig';
import RuleTreeBuilder from '../rules/RuleTreeBuilder';
import MessagesEditor from '../rules/MessagesEditor';
import PredicateEditor from '../rules/PredicateEditor';
import PromotionEditor from '../promotion/PromotionEditor';
import EmittedFieldsReference from '../schema/EmittedFieldsReference';
import LookupLink from '../lookups/LookupLink';
import OutputValuesEditor from '../rules/OutputValuesEditor';
import { fieldCoverage, supportsOutputSchema } from '../schema/outputSchemaRules';
import ErrorBanner from '../common/ErrorBanner';
import styles from './SegmentEditor.module.css';

export default function SegmentEditor() {
  const { name: layerName, id: segId } = useParams<{ name: string; id: string }>();
  const navigate = useNavigate();
  const { data: layers } = useLayers();
  const { data: lookups } = useLookups();
  const updateSegment = useUpdateSegment();
  // Resolves a field's lookup id to the table, for the read-only schema tables.
  const lookupById = (id?: string) => (id ? (lookups ?? []).find((t) => t.id === id) : undefined);
  const updateLayer = useUpdateLayer();

  const layer = layers?.find((l) => l.name === layerName);
  const original = layer?.segments.find((s) => s.id === segId);
  // A rule may only reference layers this one declares a dependency on, so the
  // picker offers exactly those — the UI cannot build a config validation rejects.
  const layerNames = layer?.dependsOn ?? [];
  // "Edit on the layer" must open this segment's own layer, not just the list —
  // LayerList reads this query param on mount and opens that layer's edit modal.
  const editLayerHref = layerName ? `/layers?edit=${encodeURIComponent(layerName)}` : '/layers';

  const [seg, setSeg] = useState<Segment | null>(null);
  const segRef = useRef(seg);
  segRef.current = seg;

  useEffect(() => {
    if (original && !segRef.current) setSeg(structuredClone(original));
  }, [original]);

  if (!seg) return <p>Loading segment...</p>;

  const update = (partial: Partial<Segment>) =>
    setSeg((prev) => (prev ? { ...prev, ...partial } : prev));

  // Output schema is declared on the layer, not the segment, so "declare a new
  // field inline while authoring a check's value" (from OutputValuesEditor,
  // via onDeclareOutput below) patches the layer instead of local segment
  // state — the same convenience, aimed at where the declaration now lives.
  const declareOutput = (name: string, field: OutputField) => {
    if (!layer) return;
    updateLayer.mutate({
      name: layer.name,
      layer: {
        name: layer.name,
        dependsOn: layer.dependsOn,
        defaultLanguage: layer.defaultLanguage,
        inputSchema: layer.inputSchema,
        outputSchema: { ...(layer.outputSchema ?? {}), [name]: field },
      },
    });
  };

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
      if (strategy === 'checklist') {
        // No default: every rule is a check that fires or does not, so there is no
        // "nothing matched" outcome to fall back to.
        next.computed = prev.computed ?? [];
        next.rules = prev.rules ?? [];
      }
      return { ...prev, ...next };
    });
  };

  // Computed and checklist both compute fields before rules run, so merge them
  // into the schema used for the rule field autocomplete. The input schema
  // itself comes from the layer — a segment declares none of its own, exactly
  // what the engine reads.
  const effectiveSchema = (s: Segment): InputSchema | undefined => {
    const base = layer?.inputSchema;
    if (!s.computed?.length) return base;
    const merged: InputSchema = { ...base };
    for (const def of s.computed) {
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
      {updateLayer.error && <ErrorBanner message={(updateLayer.error as Error).message} />}

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

      {/* Input Schema — declared on the layer, not here. Shown read-only so an
          author does not conclude this segment's rules are unchecked. */}
      <section id="input-schema" className={`card ${styles.section}`}>
        <h3>Input Schema</h3>
        <p className={styles.layerNote}>
          Declared on layer <strong>{layerName}</strong> — every segment in it shares this
          schema.{' '}
          <button type="button" className="btn-ghost btn-sm" onClick={() => navigate(editLayerHref)}>
            Edit on the layer
          </button>
        </p>
        {layer?.inputSchema && Object.keys(layer.inputSchema).length > 0 ? (
          <table className={styles.readonlyTable}>
            <thead>
              <tr><th>Field</th><th>Type</th><th>Lookup</th><th>Required</th></tr>
            </thead>
            <tbody>
              {Object.entries(layer.inputSchema).map(([f, sf]) => (
                <tr key={f}>
                  <td>{f}</td>
                  <td>{sf.type}</td>
                  <td><LookupLink table={lookupById(sf.lookup)} /></td>
                  <td>{sf.required ? 'yes' : '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <p style={{ fontSize: 12, color: 'var(--text-muted)', margin: 0 }}>
            No input schema declared on this layer yet.
          </p>
        )}
      </section>

      {/* Output Schema — after the input schema, because an output value
          interpolates the fields declared there. Also declared on the layer;
          shown read-only for the same reason as the input schema above. */}
      <section className={`card ${styles.section}`}>
        <h3>Output Schema</h3>
        {supportsOutputSchema(seg.strategy) ? (
          <>
            <p className={styles.layerNote}>
              Declared on layer <strong>{layerName}</strong> — every segment in it shares this
              schema.{' '}
              <button type="button" className="btn-ghost btn-sm" onClick={() => navigate(editLayerHref)}>
                Edit on the layer
              </button>
            </p>
            {layer?.outputSchema && Object.keys(layer.outputSchema).length > 0 ? (
              <table className={styles.readonlyTable}>
                <thead>
                  <tr><th>Field</th><th>Type</th><th>Lookup</th><th>Required</th></tr>
                </thead>
                <tbody>
                  {Object.entries(layer.outputSchema).map(([name, f]) => (
                    <tr key={name}>
                      <td>{name}</td>
                      <td>{f.type}</td>
                      <td><LookupLink table={lookupById(f.lookup)} /></td>
                      <td>{f.required ? 'yes' : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <p style={{ fontSize: 12, color: 'var(--text-muted)', margin: 0 }}>
                No output schema declared on this layer yet — values authored below have
                nothing to attach to until one is.
              </p>
            )}
            {layer?.outputSchema && Object.keys(layer.outputSchema).length > 0 && (() => {
              const layerOutputSchema = layer.outputSchema!;
              return (
                <div style={{ marginTop: 16 }}>
                  <label style={{ fontSize: 12, fontWeight: 600 }}>Segment Values</label>
                  <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 8px' }}>
                    Set a value once here to satisfy a field for every reporting rule at once —
                    the only way to satisfy a required field when this segment declares a{' '}
                    <code>default</code>, since the default path reads no rule values at all.
                  </p>
                  <OutputValuesEditor
                    outputs={seg.outputs}
                    schema={layerOutputSchema}
                    onChange={(o) => update({ outputs: o })}
                    onDeclare={declareOutput}
                    coverage={(name) => fieldCoverage(seg, layerOutputSchema, name)}
                  />
                </div>
              );
            })()}
            <div style={{ marginTop: 12 }}>
              <EmittedFieldsReference />
            </div>
            <details style={{ marginTop: 8 }}>
              <summary style={{ cursor: 'pointer', fontSize: 12, color: 'var(--text-muted)' }}>
                Fields available to output values
              </summary>
              <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 8 }}>
                {Object.keys(effectiveSchema(seg) ?? {}).length === 0 ? (
                  <p style={{ margin: 0 }}>
                    None declared yet — <a href="#input-schema">see the input schema above</a>.
                  </p>
                ) : (
                  <>
                    <p style={{ margin: '0 0 6px' }}>
                      From the layer's <a href="#input-schema">input schema</a> and this
                      segment's computed fields:
                    </p>
                    <ul style={{ margin: 0, paddingLeft: 18 }}>
                      {Object.entries(effectiveSchema(seg) ?? {}).map(([f, sf]) => (
                        <li key={f}><code>{f}</code> — {sf.type}</li>
                      ))}
                    </ul>
                  </>
                )}
              </div>
            </details>
          </>
        ) : (
          <p style={{ fontSize: 12, color: 'var(--text-muted)', margin: 0 }}>
            A <code>{seg.strategy}</code> segment resolves a segment value rather than reporting
            an item, so it emits no record and an output schema would do nothing. Output schemas
            apply to <code>checklist</code> and <code>rule</code> segments.
          </p>
        )}
      </section>

      {/* Applicability — after the schema, because the condition picks its
          fields from it and would otherwise offer nothing to choose. */}
      <section className={`card ${styles.section}`}>
        <h3>Applies When</h3>
        <PredicateEditor
          value={seg.when}
          onChange={(when) => update({ when })}
          schema={layer?.inputSchema}
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
            ruleSchema={effectiveSchema(seg)}
            overrideSchema={layer?.inputSchema}
            layerNames={layerNames}
            outputSchema={layer?.outputSchema}
            onDeclareOutput={declareOutput}
            computedSlot={
              <div className="form-group">
                <label>Computed Fields</label>
                <ComputedFieldsEditor
                  value={seg.computed ?? []}
                  onChange={(c) => update({ computed: c })}
                />
                <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
                  Optional. Values derived before the rules run, available to rule
                  conditions as ordinary fields and returned with the result.
                </p>
              </div>
            }
          />
        )}
        {seg.strategy === 'checklist' && (
          <div>
            <p style={{ fontSize: 12, color: 'var(--text-muted)', margin: '0 0 16px' }}>
              Every check runs and each one that fires is reported, so the person fixing
              them sees all the problems at once. A check states the condition for a
              problem — it fires when that condition holds.
              There is no default and no overrides: a checklist has no
              &ldquo;nothing matched&rdquo; outcome.
            </p>

            <div className="form-group">
              <label>Computed Fields</label>
              <ComputedFieldsEditor
                value={seg.computed ?? []}
                onChange={(c) => update({ computed: c })}
              />
              <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
                Computed fields are available to the checks and to their messages.
                If one fails at runtime the whole list reports <code>unevaluable</code>
                {' '}rather than reporting the checks that consumed it as real problems.
              </p>
            </div>

            <div style={{ marginTop: 24 }}>
              <RuleTreeBuilder
                rules={seg.rules ?? []}
                onChange={(r) => update({ rules: r })}
                schema={effectiveSchema(seg)}
                layerNames={layerNames}
                label="Checks"
                perRuleMessages
                outputSchema={layer?.outputSchema}
                onDeclareOutput={declareOutput}
                hint={
                  'Each check states a condition that describes a problem; when it holds, its ' +
                  'message is reported. Drag the handle to reorder or regroup. And/Or build ' +
                  'one check’s condition — a group reports once, with its own message.'
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
      {seg.strategy !== 'rule' && seg.strategy !== 'checklist' && (
        <section className={`card ${styles.section}`}>
          <h3>Overrides</h3>
          <RuleTreeBuilder
            rules={seg.overrides ?? []}
            onChange={(r) => update({ overrides: r })}
            schema={layer?.inputSchema}
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
        <button type="button" className="btn-ghost" onClick={() => navigate('/layers')}>Cancel</button>
        <button type="button" className="btn-primary" onClick={handleSave} disabled={updateSegment.isPending}>
          {updateSegment.isPending ? 'Saving...' : 'Save'}
        </button>
      </div>
    </div>
  );
}
