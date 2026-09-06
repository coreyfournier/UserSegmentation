import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useLayers, useCreateLayer, useUpdateLayer, useDeleteLayer } from '../../api/layers';
import { useCreateSegment, useUpdateSegment } from '../../api/segments';
import { useSearch } from '../../api/search';
import { useDebounced } from '../../utils/useDebounced';
import type { Layer, Segment, StrategyType } from '../../api/types';
import { SUBJECT_KEY_FIELD } from '../../api/types';
import { STRATEGY_OPTIONS } from '../segments/StrategyPicker';
import LayerRail from './LayerRail';
import LayerDetail from './LayerDetail';
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
      (l.dependsOn ?? []).every((d) => placed.has(d) || !layers.some((x) => x.key === d))
    );
    if (index === -1) {
      out.push(...remaining); // cycle: show them anyway
      break;
    }
    const [next] = remaining.splice(index, 1);
    placed.add(next.key);
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
  const editingFromQuery = editParam ? layers?.find((l) => l.key === editParam) ?? null : null;
  const activeEditing = editing ?? editingFromQuery;
  const closeEditModal = () => {
    setEditing(null);
    if (searchParams.has('edit')) {
      const next = new URLSearchParams(searchParams);
      next.delete('edit');
      setSearchParams(next, { replace: true });
    }
  };

  // Search runs on the server, not over the cached layer list: filtering here
  // would work only for as long as the whole config fits in one response.
  //
  // The term lives in the URL, not in state, so it survives leaving the page:
  // editing a segment used to mean coming back to an unfiltered list and
  // retyping it, because this component had unmounted and taken the term with
  // it. Replaced rather than pushed, so a search does not fill the back stack
  // one keystroke at a time.
  const query = searchParams.get('q') ?? '';
  const setQuery = (q: string) => {
    const next = new URLSearchParams(searchParams);
    if (q) next.set('q', q);
    else next.delete('q');
    setSearchParams(next, { replace: true });
  };
  const debouncedQuery = useDebounced(query);
  const { data: searchResult } = useSearch(debouncedQuery);

  if (isLoading) return <p>Loading...</p>;
  if (error) return <ErrorBanner message={(error as Error).message} />;

  // Display layers in dependency order so gates read top-to-bottom the way they
  // execute. Purely presentational — the engine does its own topological sort.
  const sorted = sortByDependency(layers ?? []);

  // Fold the server's flat hit list into what the rail draws: which layers to
  // show, and which of their segments matched. A layer appears because it
  // matched itself or because one of its segments did; only the segment hits
  // are listed under it, since a layer-name match has nothing to point at.
  //
  // Held against the debounced query rather than what is typed, so the list
  // does not narrow to nothing between a keystroke and its response.
  const searching = debouncedQuery.trim().length > 0;
  const matches = new Map<string, string[]>();
  const matchedLayers = new Set<string>();
  for (const hit of searchResult?.hits ?? []) {
    matchedLayers.add(hit.layer);
    if (hit.kind === 'segment' && hit.segment) {
      matches.set(hit.layer, [...(matches.get(hit.layer) ?? []), hit.segment]);
    }
  }
  const visible = searching ? sorted.filter((l) => matchedLayers.has(l.key)) : sorted;

  // Which layer the detail pane shows, in the URL so a reload, a back button
  // and a pasted link all land on the same one. Falls back to ?edit= — arriving
  // from a segment's "Edit on the layer" should show that layer behind the
  // modal, not an unrelated one — and then to the first layer, so the pane is
  // never empty while layers exist. A name that no longer resolves (deleted, or
  // renamed by the edit modal) falls through the same way.
  // An explicit selection survives a search that filters it out of the rail —
  // it was chosen deliberately, and clearing the box brings its row back. Only
  // the fallback prefers a match, so searching with nothing selected lands on
  // something the query found rather than on the first layer overall.
  const selectParam = searchParams.get('layer');
  const selected =
    sorted.find((l) => l.key === selectParam) ??
    sorted.find((l) => l.key === editParam) ??
    visible[0] ??
    sorted[0] ??
    null;

  const select = (key: string) => {
    const next = new URLSearchParams(searchParams);
    next.set('layer', key);
    setSearchParams(next, { replace: true });
  };

  // Selects a layer and closes the edit modal in one write. Both params live in
  // the same URL, and each helper builds from this render's `searchParams` — so
  // calling select() then closeEditModal() would rebuild from the pre-select
  // snapshot and throw the selection away. Used after a save, which is also
  // where the name may have just changed.
  const selectAndCloseEdit = (key: string) => {
    setEditing(null);
    const next = new URLSearchParams(searchParams);
    next.set('layer', key);
    next.delete('edit');
    setSearchParams(next, { replace: true });
  };

  return (
    <div>
      <div className={styles.toolbar}>
        <h2>Layers</h2>
        <button type="button" className="btn-primary" onClick={() => setShowCreate(true)}>
          + Add Layer
        </button>
      </div>

      {createLayer.error && <ErrorBanner message={(createLayer.error as Error).message} />}
      {updateLayer.error && <ErrorBanner message={(updateLayer.error as Error).message} />}
      {updateSegment.error && <ErrorBanner message={(updateSegment.error as Error).message} />}

      {sorted.length === 0 ? (
        <p style={{ color: 'var(--text-muted)' }}>No layers yet.</p>
      ) : (
        <div className={styles.split}>
          <LayerRail
            layers={visible}
            selected={selected?.key ?? null}
            onSelect={select}
            query={query}
            onQueryChange={setQuery}
            matches={matches}
            searching={searching}
            truncated={!!searchResult?.truncated}
          />
          {selected && (
            <LayerDetail
              // Keyed by name so switching layers remounts the pane. Without
              // it, a pending "delete segment" confirmation would carry over
              // to whichever layer was selected next.
              key={selected.key}
              layer={selected}
              onEdit={() => setEditing(selected)}
              onDelete={() => setDeleting(selected.key)}
              onAddSegment={() => {
                setAddSegTo(selected.key);
                setNewSegId('');
                setNewSegStrategy('static');
              }}
            />
          )}
        </div>
      )}

      {/* Create Layer Modal */}
      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Add Layer">
        <LayerForm
          allLayers={layers ?? []}
          onSubmit={(l) => {
            createLayer.mutate(l, {
              onSuccess: () => {
                setShowCreate(false);
                // Show what was just created rather than leaving the pane on
                // whatever was selected before.
                if (l.key) select(l.key);
              },
            });
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
              const layerKey = activeEditing.key;
              try {
                if (changedSegments?.length) {
                  for (const seg of changedSegments) {
                    await updateSegment.mutateAsync({ layerKey, segId: seg.id, segment: seg });
                  }
                }
                await updateLayer.mutateAsync({ key: layerKey, layer: l });
                // Follow a rename: the selection is held by name, so keeping
                // the old one would silently bounce the pane to the first layer.
                selectAndCloseEdit(l.key ?? layerKey);
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
          onSubmit={async (e) => {
            e.preventDefault();
            if (!addSegTo) return;
            // Creating a static or percentage segment in a layer that does not
            // declare subjectKey would be refused by validation, so the field
            // is declared first and awaited — the segment POST validates the
            // whole snapshot, and would fail if the two raced.
            const target = sorted.find((l) => l.key === addSegTo);
            const needsKey = newSegStrategy === 'static' || newSegStrategy === 'percentage';
            if (target && needsKey && !target.inputSchema?.[SUBJECT_KEY_FIELD]) {
              try {
                await updateLayer.mutateAsync({
                  key: target.key,
                  layer: {
                    key: target.key,
                    name: target.name,
                    dependsOn: target.dependsOn,
                    defaultLanguage: target.defaultLanguage,
                    inputSchema: {
                      ...(target.inputSchema ?? {}),
                      [SUBJECT_KEY_FIELD]: { type: 'string', required: true },
                    },
                    outputSchema: target.outputSchema,
                  },
                });
              } catch {
                return; // updateLayer.error is rendered above.
              }
            }
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
              { layerKey: addSegTo, segment: seg },
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
