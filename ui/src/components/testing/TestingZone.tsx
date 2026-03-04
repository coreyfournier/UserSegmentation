import { useState } from 'react';
import { useLayers } from '../../api/layers';
import { useEvaluate } from '../../api/evaluate';
import type { InputSchema, EvaluateResponse } from '../../api/types';
import ContextEditor from './ContextEditor';
import ResultDisplay from './ResultDisplay';
import ErrorBanner from '../common/ErrorBanner';
import styles from './TestingZone.module.css';

export default function TestingZone() {
  const { data: layers } = useLayers();
  const evaluate = useEvaluate();

  const [userKey, setUserKey] = useState('');
  const [selectedLayers, setSelectedLayers] = useState<string[]>([]);
  const [context, setContext] = useState<Record<string, unknown>>({});
  const [result, setResult] = useState<EvaluateResponse | null>(null);

  const allSchemas: InputSchema[] = [];
  for (const layer of layers ?? []) {
    for (const seg of layer.segments) {
      if (seg.inputSchema) allSchemas.push(seg.inputSchema);
    }
  }

  const toggleLayer = (name: string) => {
    setSelectedLayers((prev) =>
      prev.includes(name) ? prev.filter((n) => n !== name) : [...prev, name]
    );
  };

  const handleEvaluate = () => {
    evaluate.mutate(
      {
        user_key: userKey,
        context,
        layers: selectedLayers.length ? selectedLayers : undefined,
      },
      { onSuccess: (data) => setResult(data) }
    );
  };

  return (
    <div className={styles.zone}>
      <h2>Testing Zone</h2>
      <div className={styles.grid}>
        <div className={styles.input}>
          <div className="form-group">
            <label>User Key</label>
            <input
              value={userKey}
              onChange={(e) => setUserKey(e.target.value)}
              placeholder="user-123"
            />
          </div>

          <div className="form-group">
            <label>Layers (leave unchecked for all)</label>
            <div className={styles.checkboxes}>
              {(layers ?? []).map((l) => (
                <label key={l.name} className={styles.checkbox}>
                  <input
                    type="checkbox"
                    checked={selectedLayers.includes(l.name)}
                    onChange={() => toggleLayer(l.name)}
                    style={{ width: 'auto' }}
                  />
                  {l.name}
                </label>
              ))}
            </div>
          </div>

          <div className="form-group">
            <label>Context</label>
            <ContextEditor value={context} onChange={setContext} schemas={allSchemas} />
          </div>

          {evaluate.error && <ErrorBanner message={(evaluate.error as Error).message} />}

          <button
            className="btn-primary"
            onClick={handleEvaluate}
            disabled={!userKey || evaluate.isPending}
          >
            {evaluate.isPending ? 'Evaluating...' : 'Evaluate'}
          </button>
        </div>

        <div className={styles.output}>
          <h3>Results</h3>
          {result ? (
            <ResultDisplay result={result} />
          ) : (
            <p className={styles.placeholder}>Run an evaluation to see results</p>
          )}
        </div>
      </div>
    </div>
  );
}
