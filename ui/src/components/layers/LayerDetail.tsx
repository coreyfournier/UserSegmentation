import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import type { Layer } from '../../api/types';
import { useDeleteSegment } from '../../api/segments';
import ConfirmDialog from '../common/ConfirmDialog';
import styles from './LayerDetail.module.css';

interface Props {
  layer: Layer;
  onEdit: () => void;
  onDelete: () => void;
  onAddSegment: () => void;
}

/**
 * The right pane: one layer's segments and the actions that apply to it.
 *
 * Replaces the expand-in-place card. That card put every layer's segments on
 * the page at once and defaulted each to open, so the page's height was the
 * sum of the whole config and the actions sat at the far right of a
 * full-width row. Here only the selected layer is rendered, and its controls
 * sit beside the rail you just clicked.
 */
export default function LayerDetail({ layer, onEdit, onDelete, onAddSegment }: Props) {
  const navigate = useNavigate();
  const location = useLocation();
  const deleteSeg = useDeleteSegment();
  const [confirmSeg, setConfirmSeg] = useState<string | null>(null);

  // The current layers URL goes with it — selected layer and search included —
  // so the segment editor's Close returns to the list exactly as it was.
  const openSegment = (segId: string) =>
    navigate(
      `/layers/${encodeURIComponent(layer.key)}/segments/${encodeURIComponent(segId)}`,
      { state: { from: location.pathname + location.search } },
    );

  return (
    <div className={`card ${styles.pane}`}>
      <div className={styles.header}>
        <div className={styles.name}>
          <h3 className={styles.heading}>{layer.name || layer.key}</h3>
          {/* The key is always shown, even when it is also the heading: it is
              what a consumer reads out of the response and what dependsOn and
              layer: tokens name, so an author should never have to open the
              editor to find out what it is. */}
          <code className={styles.key} title="Stable key — the object name in the response">
            {layer.key}
          </code>
        </div>
        <div className={styles.actions}>
          <button type="button" className="btn-ghost btn-sm" onClick={onEdit}>edit</button>
          <button type="button" className="btn-danger btn-sm" onClick={onDelete}>x</button>
        </div>
      </div>

      <p className={styles.meta}>
        {layer.dependsOn && layer.dependsOn.length > 0
          ? `Runs after ${layer.dependsOn.join(', ')}`
          : 'No dependencies — runs whenever it is requested'}
      </p>

      <div className={styles.body}>
        {layer.segments.length === 0 && (
          <p className={styles.empty}>No segments in this layer yet.</p>
        )}
        {layer.segments.map((seg) => (
          <div key={seg.id} className={styles.segment}>
            <span className={styles.segLink} onClick={() => openSegment(seg.id)}>{seg.id}</span>
            <span className={styles.strategy}>{seg.strategy}</span>
            <div className={styles.segActions}>
              <button type="button" className="btn-ghost btn-sm" onClick={() => openSegment(seg.id)}>
                edit
              </button>
              <button type="button" className="btn-danger btn-sm" onClick={() => setConfirmSeg(seg.id)}>
                x
              </button>
            </div>
          </div>
        ))}
        <button type="button" className="btn-ghost btn-sm" onClick={onAddSegment}>
          + Add Segment
        </button>
      </div>

      <ConfirmDialog
        open={!!confirmSeg}
        title="Delete Segment"
        message={`Delete segment "${confirmSeg}"?`}
        onConfirm={() => {
          if (confirmSeg) {
            deleteSeg.mutate({ layerKey: layer.key, segId: confirmSeg });
          }
          setConfirmSeg(null);
        }}
        onCancel={() => setConfirmSeg(null)}
      />
    </div>
  );
}
