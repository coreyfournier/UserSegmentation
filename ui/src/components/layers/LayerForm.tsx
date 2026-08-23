import { useState } from 'react';
import type { Layer } from '../../api/types';

interface Props {
  initial?: Partial<Layer>;
  /** Every layer in the config, used to offer dependency choices. */
  allLayers?: Layer[];
  onSubmit: (layer: Partial<Layer>) => void;
  onCancel: () => void;
  submitLabel?: string;
}

export default function LayerForm({
  initial,
  allLayers = [],
  onSubmit,
  onCancel,
  submitLabel = 'Create',
}: Props) {
  const [name, setName] = useState(initial?.name ?? '');
  const [dependsOn, setDependsOn] = useState<string[]>(initial?.dependsOn ?? []);
  const [defaultLanguage, setDefaultLanguage] = useState(initial?.defaultLanguage ?? 'en');

  // A layer cannot depend on itself; everything else is a candidate.
  const candidates = allLayers.map((l) => l.name).filter((n) => n !== initial?.name);

  const toggleDependency = (layerName: string) => {
    setDependsOn((current) =>
      current.includes(layerName)
        ? current.filter((n) => n !== layerName)
        : [...current, layerName]
    );
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit({
          name,
          dependsOn: dependsOn.length ? dependsOn : undefined,
          defaultLanguage: defaultLanguage.trim() || undefined,
        });
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
