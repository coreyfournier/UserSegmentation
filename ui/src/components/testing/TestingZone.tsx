import { useState } from 'react';
import { useLayers } from '../../api/layers';
import { useTests } from '../../api/tests';
import { useEvaluate } from '../../api/evaluate';
import type { InputSchema, EvaluateResponse } from '../../api/types';
import ContextEditor from './ContextEditor';
import ResultDisplay from './ResultDisplay';
import ErrorBanner from '../common/ErrorBanner';
import styles from './TestingZone.module.css';

export default function TestingZone() {
  const { data: layers } = useLayers();
  const { data: savedTests } = useTests();
  const evaluate = useEvaluate();

  const [selectedLayers, setSelectedLayers] = useState<string[]>([]);
  const [context, setContext] = useState<Record<string, unknown>>({});
  const [languages, setLanguages] = useState('');
  const [renderAll, setRenderAll] = useState(false);
  const [result, setResult] = useState<EvaluateResponse | null>(null);
  const [loadedTestId, setLoadedTestId] = useState('');

  const allSchemas: InputSchema[] = [];
  for (const layer of layers ?? []) {
    // When layers are selected, only show schemas for those; otherwise show all.
    if (selectedLayers.length && !selectedLayers.includes(layer.key)) continue;
    if (layer.inputSchema) allSchemas.push(layer.inputSchema);
  }

  // Loading a saved test fills the context and narrows the run to the layer it
  // is filed under — running it against every layer would be answering a
  // different question than the one the test was saved to ask.
  const loadTest = (id: string) => {
    setLoadedTestId(id);
    const t = (savedTests ?? []).find((x) => x.id === id);
    if (!t) return;
    setContext(t.context ?? {});
    setSelectedLayers([t.layer]);
    setLanguages((t.languages ?? []).join(', '));
    setRenderAll(!!t.renderAll);
  };

  const toggleLayer = (name: string) => {
    setSelectedLayers((prev) =>
      prev.includes(name) ? prev.filter((n) => n !== name) : [...prev, name]
    );
  };

  const handleEvaluate = () => {
    const langs = languages
      .split(',')
      .map((l) => l.trim())
      .filter(Boolean);
    evaluate.mutate(
      {
        context,
        layers: selectedLayers.length ? selectedLayers : undefined,
        languages: langs.length ? langs : undefined,
        render_all: renderAll || undefined,
      },
      { onSuccess: (data) => setResult(data) }
    );
  };

  return (
    <div className={styles.zone}>
      <h2>Testing Zone</h2>
      <div className={styles.grid}>
        <div className={styles.input}>
          {/* No Subject Key box. The subject key is an ordinary context field
              named subjectKey now, so it appears in the context editor below
              for any selected layer that declares it — and does not appear at
              all for layers that never read one. */}
          {(savedTests ?? []).length > 0 && (
            <div className="form-group">
              <label>Saved test</label>
              <select
                value={loadedTestId}
                onChange={(e) => loadTest(e.target.value)}
                aria-label="load a saved test"
              >
                <option value="">— none —</option>
                {(savedTests ?? []).map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.layer} / {t.name}
                  </option>
                ))}
              </select>
              <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
                Loads that test&rsquo;s context and narrows the run to its layer. Saved tests
                are managed in the segment editor.
              </p>
            </div>
          )}

          <div className="form-group">
            <label>Layers (leave unchecked for all)</label>
            <div className={styles.checkboxes}>
              {(layers ?? []).map((l) => (
                <label key={l.key} className={styles.checkbox}>
                  <input
                    type="checkbox"
                    checked={selectedLayers.includes(l.key)}
                    onChange={() => toggleLayer(l.key)}
                    style={{ width: 'auto' }}
                  />
                  {l.name || l.key}
                </label>
              ))}
            </div>
          </div>

          <div className="form-group">
            <label>Languages (comma-separated, blank for none)</label>
            <input
              value={languages}
              onChange={(e) => setLanguages(e.target.value)}
              placeholder="en, es"
              disabled={renderAll}
            />
            <label className={styles.checkbox} style={{ marginTop: 6 }}>
              <input
                type="checkbox"
                checked={renderAll}
                onChange={(e) => setRenderAll(e.target.checked)}
                style={{ width: 'auto' }}
              />
              Render all languages (testing)
            </label>
          </div>

          <div className="form-group">
            <label>Context</label>
            <ContextEditor value={context} onChange={setContext} schemas={allSchemas} />
          </div>

          {evaluate.error && <ErrorBanner message={(evaluate.error as Error).message} />}

          <button type="button"
            className="btn-primary"
            onClick={handleEvaluate}
            disabled={evaluate.isPending}
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
