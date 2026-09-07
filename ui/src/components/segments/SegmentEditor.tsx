import { useState, useEffect, useRef } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { useLayers, useUpdateLayer } from '../../api/layers';
import { useLookups } from '../../api/lookups';
import { useUpdateSegment } from '../../api/segments';
import type { FieldType, Layer, Segment, StrategyType, InputSchema, OutputField } from '../../api/types';
import { SUBJECT_KEY_FIELD } from '../../api/types';
import StrategyPicker from './StrategyPicker';
import StaticConfig from './StaticConfig';
import PercentageConfig from './PercentageConfig';
import ComputedFieldsEditor from './ComputedFieldsEditor';
import RuleConfig from './RuleConfig';
import RuleTreeBuilder from '../rules/RuleTreeBuilder';
import PredicateEditor from '../rules/PredicateEditor';
import PromotionEditor from '../promotion/PromotionEditor';
import EmittedFieldsReference from '../schema/EmittedFieldsReference';
import LookupLink from '../lookups/LookupLink';
import LayerTests from '../testing/LayerTests';
import SplitPane from '../common/SplitPane';
import OutputValuesEditor from '../rules/OutputValuesEditor';
import { fieldCoverage, supportsOutputSchema } from '../schema/outputSchemaRules';
import { describeBreak, segmentRetypeBreaks } from '../rules/operatorRules';
import ConfirmDialog from '../common/ConfirmDialog';
import Modal from '../common/Modal';
import LayerForm from '../layers/LayerForm';
import ErrorBanner from '../common/ErrorBanner';
import styles from './SegmentEditor.module.css';

export default function SegmentEditor() {
  const { key: layerKey, id: segId } = useParams<{ key: string; id: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  // When the last save happened, so the button can confirm it landed.
  const [savedAt, setSavedAt] = useState<number | null>(null);
  // A pending computed-field retype the author has been warned about but not
  // yet confirmed. Holds the apply callback so confirming performs the exact
  // change that was described, rather than one reconstructed from indices.
  const [pendingComputedRetype, setPendingComputedRetype] = useState<
    { field: string; next: FieldType; broken: string[]; apply: () => void } | null
  >(null);
  // The layer editor, opened over this page. "Edit on the layer" used to
  // navigate to /layers?edit=, which unmounted this editor and took every
  // unsaved change with it — for a schema tweak the author only wanted so they
  // could carry on here.
  const [editingLayer, setEditingLayer] = useState(false);
  const { data: layers } = useLayers();
  const { data: lookups } = useLookups();
  const updateSegment = useUpdateSegment();
  // Resolves a field's lookup id to the table, for the read-only schema tables.
  const lookupById = (id?: string) => (id ? (lookups ?? []).find((t) => t.id === id) : undefined);
  const updateLayer = useUpdateLayer();

  const layer = layers?.find((l) => l.key === layerKey);
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

  // Output schema is declared on the layer, not the segment, so "declare a new
  // field inline while authoring a check's value" (from OutputValuesEditor,
  // via onDeclareOutput below) patches the layer instead of local segment
  // state — the same convenience, aimed at where the declaration now lives.
  const declareOutput = (name: string, field: OutputField) => {
    if (!layer) return;
    updateLayer.mutate({
      key: layer.key,
      layer: {
        key: layer.key,
        name: layer.name,
        dependsOn: layer.dependsOn,
        defaultLanguage: layer.defaultLanguage,
        inputSchema: layer.inputSchema,
        outputSchema: { ...(layer.outputSchema ?? {}), [name]: field },
      },
    });
  };

  // Static and percentage read the subject key from context, and the engine
  // refuses a snapshot where the layer does not declare it. Declaring it here
  // is the same convenience as declareOutput above — the field lives on the
  // layer, so picking the strategy patches the layer rather than local state.
  //
  // Deliberately an immediate write, matching declareOutput: the alternative is
  // staging it until the segment is saved, and a save that fails validation
  // because of a field the author was never shown is worse than a layer write
  // they can see in the read-only schema table above.
  const ensureSubjectKey = (strategy: StrategyType) => {
    if (strategy !== 'static' && strategy !== 'percentage') return;
    if (!layer || layer.inputSchema?.[SUBJECT_KEY_FIELD]) return;
    updateLayer.mutate({
      key: layer.key,
      layer: {
        key: layer.key,
        name: layer.name,
        dependsOn: layer.dependsOn,
        defaultLanguage: layer.defaultLanguage,
        inputSchema: {
          ...(layer.inputSchema ?? {}),
          // Required so an absent value is reported by the existing
          // missing-input warning as well as by the strategy's own check.
          [SUBJECT_KEY_FIELD]: { type: 'string', required: true },
        },
        outputSchema: layer.outputSchema,
      },
    });
  };

  const switchStrategy = (strategy: StrategyType) => {
    ensureSubjectKey(strategy);
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

  // Retyping a computed field can strand a rule that compares it: gte is legal
  // on a number and not on a boolean, and the engine refuses the whole snapshot
  // for it. A formula like "10 >= 1" is exactly how that happens — it reads as a
  // comparison, so the field is easily left as the type dropdown's default and
  // compared with gte, then later declared the boolean it always was. Without
  // this the first sign is a save rejected over a rule the author never touched.
  const handleComputedRetype = (field: string, next: FieldType, apply: () => void) => {
    const broken = segmentRetypeBreaks(seg, field, next);
    if (broken.length === 0) {
      apply();
      return;
    }
    setPendingComputedRetype({ field, next, broken: broken.map(describeBreak), apply });
  };

  // Saves the layer from the modal, the same way the layers page does: any
  // segment the form pruned goes first, because a layer PUT validates the whole
  // snapshot and a segment still holding a value for the field being removed
  // would reject the very schema change that orphaned it.
  //
  // The form is handed this page's in-progress segment rather than the server's
  // copy (see initialLayer below), so if removing an output field prunes it,
  // what gets written is the author's live work and not a stale version of it.
  const saveLayerFromModal = async (l: Partial<Layer>, changedSegments?: Segment[]) => {
    if (!layerKey) return;
    try {
      for (const s of changedSegments ?? []) {
        await updateSegment.mutateAsync({ layerKey, segId: s.id, segment: s });
        // A pruned copy of the segment being edited is now what the server
        // holds, so the editor adopts it — otherwise local state would still
        // carry the value that was just removed and the next save would be
        // rejected for it.
        if (s.id === segId) setSeg(structuredClone(s));
      }
      await updateLayer.mutateAsync({ key: layerKey, layer: l });
      setEditingLayer(false);
      // A layer key change moves this page's address, like a segment rename.
      if (l.key && l.key !== layerKey) {
        navigate(
          `/layers/${encodeURIComponent(l.key)}/segments/${encodeURIComponent(segId ?? '')}`,
          { replace: true, state: location.state },
        );
      }
    } catch {
      // Left open; the error banners above render what failed.
    }
  };

  // What the modal edits. The segments are the server's, except for the one on
  // screen — the form prunes segments when an output field is removed, and it
  // should prune what the author can see rather than the version they have
  // been editing away from.
  const initialLayer: Layer | undefined = layer && {
    ...layer,
    segments: layer.segments.map((s) => (s.id === segId ? seg : s)),
  };

  const handleSave = () => {
    if (!layerKey || !segId || !segRef.current) return;
    const saving = segRef.current;
    updateSegment.mutate(
      // Addressed by the id in the URL, which is the one the server still
      // holds; the payload carries the new one when the author has renamed it.
      { layerKey, segId, segment: saving },
      {
        // Stays on the page. Saving used to navigate back to the layer list,
        // which threw away the editor you were working in — so testing a change
        // meant walking back in, and any search that got you here was gone.
        // Leaving is a separate decision, made with the Close button.
        onSuccess: () => {
          setSavedAt(Date.now());
          // A rename changes this page's own address. Replacing the URL keeps
          // the editor open on the same segment rather than leaving it pointed
          // at an id the server no longer has — the next reload, or any save
          // after it, would 404.
          if (saving.id !== segId) {
            navigate(
              `/layers/${encodeURIComponent(layerKey)}/segments/${encodeURIComponent(saving.id)}`,
              { replace: true, state: location.state },
            );
          }
        },
      }
    );
  };

  // Where Close returns to. LayerList hands over its own URL when it opens a
  // segment, so closing restores the list exactly as it was — same selected
  // layer, same search. Falls back for a segment reached by a pasted link.
  const backHref = (location.state as { from?: string } | null)?.from ?? '/layers';
  // Where the layer crumb goes when the editor was reached by a pasted link,
  // so it still lands on this layer rather than the top of the list.
  const layersHref = layerKey ? `/layers?layer=${encodeURIComponent(layerKey)}` : '/layers';

  return (
    <div className={styles.editor}>
      <div className={styles.toolbar}>
        <h2>
          <span className={styles.breadcrumb} onClick={() => navigate(backHref)}>Layers</span>
          {' / '}
          {/* The layer name carried the breadcrumb styling and no handler — it
              looked like a link and did nothing. It goes back to the list with
              this layer selected, which is what backHref already encodes when
              the editor was opened from there. */}
          <span
            className={styles.breadcrumb}
            onClick={() => navigate(backHref.includes('layer=') ? backHref : layersHref)}
            title={`Back to ${layer?.name || layerKey}`}
          >
            {layer?.name || layerKey}
          </span>
          {' / '}
          {seg.name || seg.id}
        </h2>
      </div>

      {updateSegment.error && <ErrorBanner message={(updateSegment.error as Error).message} />}
      {updateLayer.error && <ErrorBanner message={(updateLayer.error as Error).message} />}

      {/* Two columns where there is room: the segment on the left, its tests
          pinned on the right and resizable by the divider between them. Below
          that width they stack and the tests fall to the bottom, which is
          where they were before. */}
      <SplitPane
        storageKey="segment-editor.form-width"
        side={
          <section className={`card ${styles.testCard}`}>
            <h3>Tests</h3>
            {layerKey && <LayerTests layerKey={layerKey} schema={layer?.inputSchema} />}
          </section>
        }
      >

      {/* Identity — first, because the id was previously uneditable through the
          API at all and invisible here except as breadcrumb text. */}
      <section className={`card ${styles.section}`}>
        <h3>Identity</h3>
        <div className="form-group">
          <label>Name</label>
          <input
            value={seg.name ?? ''}
            onChange={(e) => update({ name: e.target.value || undefined })}
            placeholder="e.g. Employee readiness"
          />
          <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
            The friendly label, shown here and in the layer&rsquo;s segment list. Nothing
            references it, so it can be changed freely.
          </p>
        </div>
        <div className="form-group">
          <label>Segment ID</label>
          <input
            value={seg.id}
            onChange={(e) => update({ id: e.target.value })}
            aria-invalid={!seg.id.trim()}
          />
          <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
            The stable identity: unique within this layer, how the admin API addresses this
            segment, and what appears in a <code>reason</code> and in any warning it
            produces. Unlike a layer key it is not restricted to letters and digits — a
            segment is never an object name in the response.
          </p>
          {seg.id !== segId && (
            <p style={{ fontSize: 11, color: 'var(--danger)', margin: '4px 0 0' }}>
              Renaming from <code>{segId}</code> on save. Nothing inside the config refers
              to a segment by id, so there is nothing to update — but a saved test or a
              consumer reading <code>reason</code> may mention the old one.
            </p>
          )}
        </div>
      </section>

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
          Declared on layer <strong>{layerKey}</strong> — every segment in it shares this
          schema.{' '}
          <button type="button" className="btn-ghost btn-sm" onClick={() => setEditingLayer(true)}>
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
              Declared on layer <strong>{layerKey}</strong> — every segment in it shares this
              schema.{' '}
              <button type="button" className="btn-ghost btn-sm" onClick={() => setEditingLayer(true)}>
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
            defaultOutputs={seg.defaultOutputs}
            onDefaultOutputsChange={(o) => update({ defaultOutputs: o })}
            computed={seg.computed}
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
                  onChangeType={handleComputedRetype}
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
                onChangeType={handleComputedRetype}
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
          {/* Stated at more length here than for a rule segment, because this
              is where overrides matter most: static maps a key and percentage
              hashes one, and neither can express a condition at all. An
              override is the only way to attach one. */}
          <p style={{ fontSize: 12, lineHeight: 1.5, margin: '0 0 8px' }}>
            <strong>
              Use an override to force an outcome regardless of what the strategy would
              decide.
            </strong>{' '}
            This is the only place a condition can be attached to a <code>{seg.strategy}</code>{' '}
            segment — a rollout carve-out, a subject pinned for a support escalation, or a
            gate on what an earlier layer resolved (<code>layer:x</code>). Each override has
            an <code>enabled</code> flag, so an exception can be switched off without losing
            how it was written.
          </p>
          <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '0 0 12px', lineHeight: 1.5 }}>
            The first override that matches wins and the strategy never runs. Only raw input
            fields are available.
          </p>
          <RuleTreeBuilder
            rules={seg.overrides ?? []}
            onChange={(r) => update({ overrides: r })}
            schema={layer?.inputSchema}
            layerNames={layerNames}
            label="Override Rules"
          />
          {/* No default here. Segment.default is read by the rule strategy
              alone: static has its own default inside its mappings, and
              percentage has no such notion. The editor used to offer one for
              these strategies, writing a field nothing would ever read. */}
        </section>
      )}

      {/* Footer, inside the form column: these act on the segment, and a
          right-aligned footer spanning an uncapped page would put Save at the
          far edge of a wide monitor, nowhere near the form. */}
      <div className={styles.footer}>
        {/* "Close" rather than "Cancel": saving no longer leaves the page, so
            this is how you leave — and it discards nothing that was saved. */}
        <button type="button" className="btn-ghost" onClick={() => navigate(backHref)}>Close</button>
        {savedAt !== null && !updateSegment.isPending && (
          <span className={styles.saved} role="status">Saved</span>
        )}
        <button type="button" className="btn-primary" onClick={handleSave} disabled={updateSegment.isPending}>
          {updateSegment.isPending ? 'Saving...' : 'Save'}
        </button>
      </div>

      </SplitPane>

      {/* The layer's own editor, over this page rather than instead of it.
          Closing it leaves the segment exactly as it was; saving refreshes the
          read-only schema tables through the layers query, so the change is
          visible here without a navigation. */}
      <Modal open={editingLayer} onClose={() => setEditingLayer(false)} title="Edit Layer">
        {initialLayer && (
          <LayerForm
            initial={initialLayer}
            allLayers={layers ?? []}
            submitLabel="Save"
            onSubmit={saveLayerFromModal}
            onCancel={() => setEditingLayer(false)}
          />
        )}
      </Modal>

      <ConfirmDialog
        open={!!pendingComputedRetype}
        title="Change computed field type"
        confirmLabel="Change type"
        message={
          pendingComputedRetype
            ? `Changing "${pendingComputedRetype.field}" to ${pendingComputedRetype.next} leaves ` +
              `${pendingComputedRetype.broken.length} condition(s) using an operator that type does ` +
              `not allow, and the save will be refused until they are fixed — ` +
              `${pendingComputedRetype.broken.join('; ')}.`
            : ''
        }
        onConfirm={() => {
          pendingComputedRetype?.apply();
          setPendingComputedRetype(null);
        }}
        onCancel={() => setPendingComputedRetype(null)}
      />
    </div>
  );
}
