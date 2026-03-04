import { useState } from 'react';
import type { Layer } from '../../api/types';

interface Props {
  initial?: Partial<Layer>;
  onSubmit: (layer: Partial<Layer>) => void;
  onCancel: () => void;
  submitLabel?: string;
}

export default function LayerForm({ initial, onSubmit, onCancel, submitLabel = 'Create' }: Props) {
  const [name, setName] = useState(initial?.name ?? '');
  const [order, setOrder] = useState(initial?.order ?? 1);

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit({ name, order });
      }}
    >
      <div className="form-group">
        <label>Layer Name</label>
        <input value={name} onChange={(e) => setName(e.target.value)} required />
      </div>
      <div className="form-group">
        <label>Order</label>
        <input
          type="number"
          value={order}
          onChange={(e) => setOrder(Number(e.target.value))}
          min={0}
          required
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
  );
}
