import { useState } from 'react';
import { useLayers, useCreateLayer, useUpdateLayer, useDeleteLayer } from '../../api/layers';
import { useCreateSegment } from '../../api/segments';
import type { Layer, Segment } from '../../api/types';
import LayerCard from './LayerCard';
import LayerForm from './LayerForm';
import Modal from '../common/Modal';
import ConfirmDialog from '../common/ConfirmDialog';
import ErrorBanner from '../common/ErrorBanner';
import styles from './LayerList.module.css';

export default function LayerList() {
  const { data: layers, isLoading, error } = useLayers();
  const createLayer = useCreateLayer();
  const updateLayer = useUpdateLayer();
  const deleteLayer = useDeleteLayer();
  const createSegment = useCreateSegment();

  const [showCreate, setShowCreate] = useState(false);
  const [editing, setEditing] = useState<Layer | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [addSegTo, setAddSegTo] = useState<string | null>(null);
  const [newSegId, setNewSegId] = useState('');
  const [newSegStrategy, setNewSegStrategy] = useState<'static' | 'rule' | 'percentage'>('static');

  if (isLoading) return <p>Loading...</p>;
  if (error) return <ErrorBanner message={(error as Error).message} />;

  const sorted = [...(layers ?? [])].sort((a, b) => a.order - b.order);

  return (
    <div>
      <div className={styles.toolbar}>
        <h2>Layers</h2>
        <button className="btn-primary" onClick={() => setShowCreate(true)}>
          + Add Layer
        </button>
      </div>

      {createLayer.error && <ErrorBanner message={(createLayer.error as Error).message} />}

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
          onSubmit={(l) => {
            createLayer.mutate(l, { onSuccess: () => setShowCreate(false) });
          }}
          onCancel={() => setShowCreate(false)}
        />
      </Modal>

      {/* Edit Layer Modal */}
      <Modal open={!!editing} onClose={() => setEditing(null)} title="Edit Layer">
        {editing && (
          <LayerForm
            initial={editing}
            submitLabel="Save"
            onSubmit={(l) => {
              updateLayer.mutate(
                { name: editing.name, layer: l },
                { onSuccess: () => setEditing(null) }
              );
            }}
            onCancel={() => setEditing(null)}
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
              onChange={(e) => setNewSegStrategy(e.target.value as 'static' | 'rule' | 'percentage')}
            >
              <option value="static">Static</option>
              <option value="rule">Rule</option>
              <option value="percentage">Percentage</option>
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
