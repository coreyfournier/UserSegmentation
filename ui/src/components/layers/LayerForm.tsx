import { useState } from 'react';
import { useLookups } from '../../api/lookups';
import type { InputSchema, Layer, OutputSchema, Segment } from '../../api/types';
import InputSchemaEditor from '../schema/InputSchemaEditor';
import OutputSchemaEditor from '../schema/OutputSchemaEditor';
import ConfirmDialog from '../common/ConfirmDialog';

interface Props {
  initial?: Partial<Layer>;
  /** Every layer in the config, used to offer dependency choices. */
  allLayers?: Layer[];
  /**
   * `changedSegments` carries any segment this form pruned while removing an
   * output field the caller had authored a value against (see
   * handleRemoveOutputField below) — only the segments actually touched, so
   * the caller can persist them alongside the layer without also resending
   * every untouched segment.
   */
  onSubmit: (layer: Partial<Layer>, changedSegments?: Segment[]) => void;
  onCancel: () => void;
  submitLabel?: string;
}

/** Every place a segment can carry an authored value for an output field:
 *  the segment-level fallback, each top-level rule, each override. Mirrors
 *  the engine's own outputAuthoringMaps (internal/domain/validation). */
function outputFieldUsage(segments: Segment[], field: string): { valueCount: number; segmentIds: string[] } {
  let valueCount = 0;
  const segmentIds: string[] = [];
  for (const seg of segments) {
    let hit = false;
    if (seg.outputs?.[field]) {
      valueCount++;
      hit = true;
    }
    for (const r of seg.rules ?? []) {
      if (r.outputs?.[field]) {
        valueCount++;
        hit = true;
      }
    }
    for (const r of seg.overrides ?? []) {
      if (r.outputs?.[field]) {
        valueCount++;
        hit = true;
      }
    }
    if (hit) segmentIds.push(seg.id);
  }
  return { valueCount, segmentIds };
}

/**
 * Removes every authored value for `field` from one segment: its own
 * segment-level fallback, and each top-level rule's and override's own
 * value. Without this, deleting a field from the layer's output schema
 * leaves these values in place, and the engine rejects the very next save
 * with "output %q is not declared in outputSchema" — with no way to see or
 * clear the stale value, since OutputValuesEditor only ever iterates the
 * schema's own keys.
 *
 * Returns `seg` itself, unchanged, when nothing needed pruning — so a caller
 * can tell which segments were actually touched by reference.
 */
function pruneOutputField(seg: Segment, field: string): Segment {
  let changed = false;

  let outputs = seg.outputs;
  if (outputs && field in outputs) {
    const next = { ...outputs };
    delete next[field];
    outputs = Object.keys(next).length ? next : undefined;
    changed = true;
  }

  let rules = seg.rules;
  if (rules?.some((r) => r.outputs && field in r.outputs)) {
    rules = rules.map((r) => {
      if (!r.outputs || !(field in r.outputs)) return r;
      const next = { ...r.outputs };
      delete next[field];
      return { ...r, outputs: Object.keys(next).length ? next : undefined };
    });
    changed = true;
  }

  let overrides = seg.overrides;
  if (overrides?.some((r) => r.outputs && field in r.outputs)) {
    overrides = overrides.map((r) => {
      if (!r.outputs || !(field in r.outputs)) return r;
      const next = { ...r.outputs };
      delete next[field];
      return { ...r, outputs: Object.keys(next).length ? next : undefined };
    });
    changed = true;
  }

  return changed ? { ...seg, outputs, rules, overrides } : seg;
}

/** Segments whose rule fields would go unvalidated if this layer lost its
 *  last input field — mirrors the Go side's WarnMissingInputSchemas exactly:
 *  a segment with its own computed fields is still validated against those,
 *  so it does not count here. */
function segmentsNeedingInputSchema(segments: Segment[]): number {
  return segments.filter(
    (s) => !(s.computed?.length) && (s.rules?.length || s.overrides?.length || s.when)
  ).length;
}

export default function LayerForm({
  initial,
  allLayers = [],
  onSubmit,
  onCancel,
  submitLabel = 'Create',
}: Props) {
  const { data: lookups } = useLookups();
  const [name, setName] = useState(initial?.name ?? '');
  const [dependsOn, setDependsOn] = useState<string[]>(initial?.dependsOn ?? []);
  const [defaultLanguage, setDefaultLanguage] = useState(initial?.defaultLanguage ?? 'en');
  const [inputSchema, setInputSchema] = useState<InputSchema | undefined>(initial?.inputSchema);
  const [outputSchema, setOutputSchema] = useState<OutputSchema | undefined>(initial?.outputSchema);
  // Working copies of this layer's segments, pruned as output fields are
  // removed above. Untouched segments stay the exact same reference, which is
  // how changedSegments (below) tells which ones actually need to be saved.
  const [segments, setSegments] = useState<Segment[]>(initial?.segments ?? []);
  const [changedIds, setChangedIds] = useState<Set<string>>(new Set());

  const [pendingOutputRemoval, setPendingOutputRemoval] = useState<
    { field: string; valueCount: number; segmentCount: number } | null
  >(null);
  const [pendingInputRemoval, setPendingInputRemoval] = useState<
    { field: string; affectedSegments: number } | null
  >(null);

  // A layer cannot depend on itself; everything else is a candidate.
  const candidates = allLayers.map((l) => l.name).filter((n) => n !== initial?.name);

  const toggleDependency = (layerName: string) => {
    setDependsOn((current) =>
      current.includes(layerName)
        ? current.filter((n) => n !== layerName)
        : [...current, layerName]
    );
  };

  const applyRemoveOutputField = (field: string) => {
    setOutputSchema((prev) => {
      if (!prev) return prev;
      const next = { ...prev };
      delete next[field];
      return Object.keys(next).length ? next : undefined;
    });
    setSegments((prev) => {
      const next = prev.map((s) => pruneOutputField(s, field));
      const touched = prev.filter((s, i) => s !== next[i]).map((s) => s.id);
      if (touched.length) {
        setChangedIds((ids) => new Set([...ids, ...touched]));
      }
      return next;
    });
  };

  const handleRemoveOutputField = (field: string) => {
    const { valueCount, segmentIds } = outputFieldUsage(segments, field);
    if (valueCount > 0) {
      setPendingOutputRemoval({ field, valueCount, segmentCount: segmentIds.length });
      return;
    }
    applyRemoveOutputField(field);
  };

  const handleRemoveInputField = (field: string) => {
    const next = { ...(inputSchema ?? {}) };
    delete next[field];
    const becomesEmpty = Object.keys(next).length === 0;
    const affected = segmentsNeedingInputSchema(segments);
    if (becomesEmpty && affected > 0) {
      setPendingInputRemoval({ field, affectedSegments: affected });
      return;
    }
    setInputSchema(Object.keys(next).length ? next : undefined);
  };

  return (
    <>
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(
          {
            name,
            dependsOn: dependsOn.length ? dependsOn : undefined,
            defaultLanguage: defaultLanguage.trim() || undefined,
            inputSchema,
            outputSchema,
          },
          segments.filter((s) => changedIds.has(s.id))
        );
      }}
    >
      <div className="form-group">
        <label>Layer Name</label>
        <input value={name} onChange={(e) => setName(e.target.value)} required />
      </div>

      <div className="form-group">
        <label>Depends On</label>
        {candidates.length === 0 ? (
          <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
            No other layers to depend on yet.
          </p>
        ) : (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 4 }}>
            {candidates.map((layerName) => (
              <label
                key={layerName}
                style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12 }}
              >
                <input
                  type="checkbox"
                  checked={dependsOn.includes(layerName)}
                  onChange={() => toggleDependency(layerName)}
                />
                {layerName}
              </label>
            ))}
          </div>
        )}
        <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
          Sets execution order. A rule referencing <code>layer:x</code> must declare x here.
          If a dependency does not resolve, this layer is skipped rather than evaluated
          against missing values.
        </p>
      </div>

      <div className="form-group">
        <label>Default Language</label>
        <input
          value={defaultLanguage}
          onChange={(e) => setDefaultLanguage(e.target.value)}
          placeholder="en"
        />
        <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
          Fallback locale used when a requested message language is missing.
        </p>
      </div>

      <div className="form-group">
        <label>Input Schema</label>
        <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
          Shared by every segment in this layer — a segment's rule field picker offers
          exactly these fields, plus its own computed ones.
        </p>
        <InputSchemaEditor
          value={inputSchema}
          onChange={setInputSchema}
          onRemoveField={handleRemoveInputField}
        />
      </div>

      <div className="form-group">
        <label>Output Schema</label>
        <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
          Declares the record every <code>rule</code> or <code>checklist</code> segment in
          this layer emits with each reported item. Values are still authored per check on
          each segment.
        </p>
        <OutputSchemaEditor
          value={outputSchema}
          onChange={setOutputSchema}
          lookups={lookups ?? []}
          onRemoveField={handleRemoveOutputField}
        />
      </div>

      <div className="form-row" style={{ justifyContent: 'flex-end' }}>
        <button type="button" className="btn-ghost" onClick={onCancel}>
          Cancel
        </button>
        <button type="submit" className="btn-primary">
          {submitLabel}
        </button>
      </div>
    </form>

      <ConfirmDialog
        open={!!pendingOutputRemoval}
        title="Remove Output Field"
        message={
          pendingOutputRemoval
            ? `Removing "${pendingOutputRemoval.field}" will clear ${pendingOutputRemoval.valueCount} ` +
              `authored value(s) across ${pendingOutputRemoval.segmentCount} segment(s). Continue?`
            : ''
        }
        onConfirm={() => {
          if (pendingOutputRemoval) applyRemoveOutputField(pendingOutputRemoval.field);
          setPendingOutputRemoval(null);
        }}
        onCancel={() => setPendingOutputRemoval(null)}
      />

      <ConfirmDialog
        open={!!pendingInputRemoval}
        title="Remove Input Field"
        message={
          pendingInputRemoval
            ? `"${pendingInputRemoval.field}" is the last input field on this layer. Removing it ` +
              `turns off rule-field checking for ${pendingInputRemoval.affectedSegments} segment(s) ` +
              `in it. Continue?`
            : ''
        }
        onConfirm={() => {
          if (pendingInputRemoval) setInputSchema(undefined);
          setPendingInputRemoval(null);
        }}
        onCancel={() => setPendingInputRemoval(null)}
      />
    </>
  );
}
