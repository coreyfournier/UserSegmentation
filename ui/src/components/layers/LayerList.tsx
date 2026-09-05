import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useLayers, useCreateLayer, useUpdateLayer, useDeleteLayer } from '../../api/layers';
import { useCreateSegment, useUpdateSegment } from '../../api/segments';
import type { Layer, Segment, StrategyType } from '../../api/types';
import { STRATEGY_OPTIONS } from '../segments/StrategyPicker';
import LayerCard from './LayerCard';
import LayerForm from './LayerForm';
import Modal from '../common/Modal';
import ConfirmDialog from '../common/ConfirmDialog';
import ErrorBanner from '../common/ErrorBanner';
import styles from './LayerList.module.css';

/**
 * Orders layers so each one follows the layers it depends on. Layers whose
 * dependencies are missing or cyclic are appended rather than dropped, so a
 * broken config is still editable in the UI.
 */
function sortByDependency(layers: Layer[]): Layer[] {
  const remaining = [...layers];
  const placed = new Set<string>();
  const out: Layer[] = [];

  while (remaining.length > 0) {
    const index = remaining.findIndex((l) =>
      (l.dependsOn ?? []).every((d) => placed.has(d) || !layers.some((x) => x.name === d))
    );
    if (index === -1) {
      out.push(...remaining); // cycle: show them anyway
      break;
    }
    const [next] = remaining.splice(index, 1);
    placed.add(next.name);
    out.push(next);
  }
  return out;
}

export default function LayerList() {
  const { data: layers, isLoading, error } = useLayers();
  const createLayer = useCreateLayer();
  const updateLayer = useUpdateLayer();
  const updateSegment = useUpdateSegment();
  const deleteLayer = useDeleteLayer();
  const createSegment = useCreateSegment();

  const [showCreate, setShowCreate] = useState(false);
  const [editing, setEditing] = useState<Layer | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [addSegTo, setAddSegTo] = useState<string | null>(null);
  const [newSegId, setNewSegId] = useState('');
  const [newSegStrategy, setNewSegStrategy] = useState<StrategyType>('static');

  // "Edit on the layer" (SegmentEditor) links here with ?edit=<layer name> so
  // it opens that specific layer's editor rather than just the list. Derived
  // at render instead of synced into state via an effect — `editing` (an
  // explicit click on a card) always wins once set, and closing clears both.
  const [searchParams, setSearchParams] = useSearchParams();
  const editParam = searchParams.get('edit');
  const editingFromQuery = editParam ? layers?.find((l) => l.name === editParam) ?? null : null;
  const activeEditing = editing ?? editingFromQuery;
  const closeEditModal = () => {
    setEditing(null);
    if (searchParams.has('edit')) {
      const next = new URLSearchParams(searchParams);
      next.delete('edit');
      setSearchParams(next, { replace: true });
    }
  };

  if (isLoading) return <p>Loading...</p>;
  if (error) return <ErrorBanner message={(error as Error).message} />;

  // Display layers in dependency order so gates read top-to-bottom the way they
  // execute. Purely presentational — the engine does its own topological sort.
  const sorted = sortByDependency(layers ?? []);

  return (
    <div>
      <div className={styles.toolbar}>
        <h2>Layers</h2>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>
          + Add Layer
        </button>
      </div>

      {createLayer.error && <ErrorBanner message={(createLayer.error as Error).message} />}
      {updateLayer.error && <ErrorBanner message={(updateLayer.error as Error).message} />}
      {updateSegment.error && <ErrorBanner message={(updateSegment.error as Error).message} />}

      {sorted.map((layer) => (
        <LayerCard
          key={layer.name}
          layer={layer}
          onEdit={() => setEditing(layer)}
          onDelete={() => setDeleting(layer.name)}
          onAddSegment={() => {
            setAddSegTo(layer.name);
            setNewSegId('');
            setNewSegStrategy('static');
          }}
        />
      ))}

      {sorted.length === 0 && <p style={{ color: 'var(--text-muted)' }}>No layers yet.</p>}

      {/* Create Layer Modal */}
      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Add Layer">
        <LayerForm
          allLayers={layers ?? []}
          onSubmit={(l) => {
            createLayer.mutate(l, { onSuccess: () => setShowCreate(false) });
          }}
          onCancel={() => setShowCreate(false)}
        />
      </Modal>

      {/* Edit Layer Modal */}
      <Modal open={!!activeEditing} onClose={closeEditModal} title="Edit Layer">
        {activeEditing && (
          <LayerForm
            initial={activeEditing}
            allLayers={layers ?? []}
            submitLabel="Save"
            onSubmit={async (l, changedSegments) => {
              // Segments pruned of a just-removed output field's stale values
              // (LayerForm's onRemoveField) must be saved BEFORE the layer:
              // a layer PUT validates the whole snapshot as it stands, so a
              // segment still carrying a value for the field being removed
              // would reject the very schema change that orphaned it. They
              // are saved under the layer's current name — a rename, if any,
              // is part of the layer PUT that follows.
              const layerName = activeEditing.name;
              try {
                if (changedSegments?.length) {
                  for (const seg of changedSegments) {
                    await updateSegment.mutateAsync({ layerName, segId: seg.id, segment: seg });
                  }
                }
                await updateLayer.mutateAsync({ name: layerName, layer: l });
                closeEditModal();
              } catch {
                // Left open; updateLayer.error / updateSegment.error above
                // render what failed so the author can retry or adjust.
              }
            }}
            onCancel={closeEditModal}
          />
        )}
      </Modal>

      {/* Delete Layer Confirm */}
      <ConfirmDialog
        open={!!deleting}
        title="Delete Layer"
        message={`Delete layer "${deleting}" and all its segments?`}
        onConfirm={() => {
          if (deleting) deleteLayer.mutate(deleting);
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />

      {/* Add Segment Modal */}
      <Modal open={!!addSegTo} onClose={() => setAddSegTo(null)} title={`Add Segment to ${addSegTo}`}>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (!addSegTo) return;
            const seg: Segment = {
              id: newSegId,
              strategy: newSegStrategy,
              ...(newSegStrategy === 'static' && {
                static: { mappings: {}, default: '' },
              }),
              ...(newSegStrategy === 'percentage' && {
                percentage: { salt: '', buckets: [] },
              }),
              ...(newSegStrategy === 'rule' && {
                rules: [],
                default: '',
              }),
              // A checklist has no default: every rule is a check that fires or
              // does not, so there is no "nothing matched" outcome.
              ...(newSegStrategy === 'checklist' && {
                computed: [],
                rules: [],
              }),
            };
            createSegment.mutate(
              { layerName: addSegTo, segment: seg },
              { onSuccess: () => setAddSegTo(null) }
            );
          }}
        >
          <div className="form-group">
            <label>Segment ID</label>
            <input value={newSegId} onChange={(e) => setNewSegId(e.target.value)} required />
          </div>
          <div className="form-group">
            <label>Strategy</label>
            <select
              value={newSegStrategy}
              onChange={(e) => setNewSegStrategy(e.target.value as StrategyType)}
            >
              {STRATEGY_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </div>
          <div className="form-row" style={{ justifyContent: 'flex-end' }}>
            <button type="button" className="btn-ghost" onClick={() => setAddSegTo(null)}>
              Cancel
            </button>
            <button type="submit" className="btn-primary">Create</button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
