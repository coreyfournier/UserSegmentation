import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import type { Layer } from '../../api/types';
import { useDeleteSegment } from '../../api/segments';
import ConfirmDialog from '../common/ConfirmDialog';
import styles from './LayerCard.module.css';

interface Props {
  layer: Layer;
  onEdit: () => void;
  onDelete: () => void;
  onAddSegment: () => void;
}

export default function LayerCard({ layer, onEdit, onDelete, onAddSegment }: Props) {
  const navigate = useNavigate();
  const deleteSeg = useDeleteSegment();
  const [confirmSeg, setConfirmSeg] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(true);

  return (
    <div className={`card ${styles.card}`}>
      <div className={styles.header} onClick={() => setExpanded(!expanded)}>
        <span className={styles.order}>[{layer.order}]</span>
        <span className={styles.name}>{layer.name}</span>
        <span className={styles.count}>{layer.segments.length} segment(s)</span>
        <div className={styles.actions} onClick={(e) => e.stopPropagation()}>
          <button className="btn-ghost btn-sm" onClick={onEdit}>edit</button>
          <button className="btn-danger btn-sm" onClick={onDelete}>x</button>
        </div>
      </div>
      {expanded && (
        <div className={styles.body}>
          {layer.segments.map((seg) => (
            <div key={seg.id} className={styles.segment}>
              <span
                className={styles.segLink}
                onClick={() =>
                  navigate(`/layers/${encodeURIComponent(layer.name)}/segments/${encodeURIComponent(seg.id)}`)
                }
              >
                {seg.id}
              </span>
              <span className={styles.strategy}>{seg.strategy}</span>
              <div className={styles.segActions}>
                <button
                  className="btn-ghost btn-sm"
                  onClick={() =>
                    navigate(`/layers/${encodeURIComponent(layer.name)}/segments/${encodeURIComponent(seg.id)}`)
                  }
                >
                  edit
                </button>
                <button
                  className="btn-danger btn-sm"
                  onClick={() => setConfirmSeg(seg.id)}
                >
                  x
                </button>
              </div>
            </div>
          ))}
          <button className="btn-ghost btn-sm" onClick={onAddSegment}>
            + Add Segment
          </button>
        </div>
      )}
      <ConfirmDialog
        open={!!confirmSeg}
        title="Delete Segment"
        message={`Delete segment "${confirmSeg}"?`}
        onConfirm={() => {
          if (confirmSeg) {
            deleteSeg.mutate({ layerName: layer.name, segId: confirmSeg });
          }
          setConfirmSeg(null);
        }}
        onCancel={() => setConfirmSeg(null)}
      />
    </div>
  );
}
