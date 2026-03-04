import type { Promotion } from '../../api/types';

interface Props {
  value?: Promotion;
  onChange: (p?: Promotion) => void;
}

export default function PromotionEditor({ value, onChange }: Props) {
  const promo = value ?? {};

  const toLocal = (iso?: string) => {
    if (!iso) return '';
    return iso.slice(0, 16); // "YYYY-MM-DDTHH:mm"
  };

  const update = (field: 'effective_from' | 'effective_until', val: string) => {
    const next = { ...promo, [field]: val ? new Date(val).toISOString() : undefined };
    if (!next.effective_from && !next.effective_until) {
      onChange(undefined);
    } else {
      onChange(next);
    }
  };

  return (
    <div>
      <div className="form-row">
        <div className="form-group" style={{ flex: 1 }}>
          <label>Effective From</label>
          <input
            type="datetime-local"
            value={toLocal(promo.effective_from)}
            onChange={(e) => update('effective_from', e.target.value)}
          />
        </div>
        <div className="form-group" style={{ flex: 1 }}>
          <label>Effective Until</label>
          <input
            type="datetime-local"
            value={toLocal(promo.effective_until)}
            onChange={(e) => update('effective_until', e.target.value)}
          />
        </div>
      </div>
    </div>
  );
}
