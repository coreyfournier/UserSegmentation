import { useState } from 'react';
import { useTests, useCreateTest, useUpdateTest, useDeleteTest } from '../../api/tests';
import { apiFetch } from '../../api/client';
import type { EvaluateResponse, InputSchema, LayerResult, SavedTest } from '../../api/types';
import ContextEditor from './ContextEditor';
import ConfirmDialog from '../common/ConfirmDialog';
import ErrorBanner from '../common/ErrorBanner';
import styles from './LayerTests.module.css';

interface Props {
  /** The layer being edited — the default scope, and where a new test is filed. */
  layerKey: string;
  /** That layer's input schema, so the context editor can offer its fields. */
  schema?: InputSchema;
}

/**
 * One test's outcome from the last run.
 *
 * `verdict` is unset today: a saved test records inputs and no expectation, so
 * a run produces a result to read rather than a pass or a fail. The field is
 * here because the row, the roll-up and the styling are all shaped around it —
 * when expectations arrive, they set this and nothing else about the display
 * has to move.
 */
export interface TestRunResult {
  layerResult?: LayerResult;
  response?: EvaluateResponse;
  error?: string;
  verdict?: 'pass' | 'fail';
}

export default function LayerTests({ layerKey, schema }: Props) {
  const { data: allTests } = useTests();
  const createTest = useCreateTest();
  const updateTest = useUpdateTest();
  const deleteTest = useDeleteTest();

  // Which tests are listed. A change to one layer can break another's rules —
  // layers gate and override each other — so the tests worth running after an
  // edit are not only the ones filed under the layer being edited.
  const [scope, setScope] = useState<'layer' | 'all'>('layer');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  // The context being edited, held locally so typing does not write to the
  // server on every keystroke. Seeded when a test is selected.
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [newName, setNewName] = useState('');
  const [results, setResults] = useState<Record<string, TestRunResult>>({});
  const [running, setRunning] = useState<string[]>([]);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<SavedTest | null>(null);

  const tests = (allTests ?? []).filter((t) => scope === 'all' || t.layer === layerKey);
  const selected = tests.find((t) => t.id === selectedId) ?? null;

  // Grouped for the "all" view so a row's layer is never in doubt. Insertion
  // order follows the list, with the layer being edited pulled to the front —
  // it is the one the author is working on.
  const byLayer = new Map<string, SavedTest[]>();
  for (const t of tests) byLayer.set(t.layer, [...(byLayer.get(t.layer) ?? []), t]);
  const groups = [...byLayer.entries()].sort(([a], [b]) =>
    a === layerKey ? -1 : b === layerKey ? 1 : a.localeCompare(b),
  );

  const select = (test: SavedTest) => {
    setSelectedId(test.id);
    setDraft(test.context ?? {});
  };

  /**
   * Runs one test against the layer it is filed under.
   *
   * Scoped to that layer rather than the whole config because the test was
   * saved to ask about that layer; the engine still evaluates everything it
   * depends on, so a cross-layer gate is exercised either way.
   *
   * Uses apiFetch directly rather than the mutation, so a run-all can await
   * each one and keep per-test results apart — a shared mutation has one
   * `data` and the last response would win.
   */
  const runOne = async (test: SavedTest, context?: Record<string, unknown>) => {
    setRunning((r) => [...r, test.id]);
    try {
      const response = await apiFetch<EvaluateResponse>('/v1/evaluate', {
        method: 'POST',
        body: JSON.stringify({
          context: context ?? test.context ?? {},
          layers: [test.layer],
          languages: test.languages?.length ? test.languages : undefined,
          render_all: test.renderAll || undefined,
        }),
      });
      setResults((r) => ({
        ...r,
        [test.id]: { response, layerResult: response.layers?.[test.layer] },
      }));
    } catch (e) {
      setResults((r) => ({ ...r, [test.id]: { error: (e as Error).message } }));
    } finally {
      setRunning((r) => r.filter((id) => id !== test.id));
    }
  };

  // Sequential rather than parallel: these all write to the same result map,
  // and a handful of local evaluations is fast enough that the ordering is
  // worth more than the concurrency.
  const runAll = async () => {
    for (const t of tests) await runOne(t);
  };

  const save = () => {
    if (!selected) return;
    updateTest.mutate({ id: selected.id, test: { ...selected, context: draft } });
  };

  const saveAsNew = () => {
    const name = newName.trim();
    if (!name) return;
    createTest.mutate(
      { layer: layerKey, name, context: draft },
      {
        onSuccess: (snap) => {
          setNewName('');
          const created = (snap.tests ?? []).find((t) => t.layer === layerKey && t.name === name);
          if (created) setSelectedId(created.id);
        },
      }
    );
  };

  const ranCount = tests.filter((t) => results[t.id]).length;

  return (
    <div className={styles.panel}>
      <div className={styles.head}>
        <div className={styles.scope} role="group" aria-label="Which tests to show">
          <button
            type="button"
            className={`${styles.tab} ${scope === 'layer' ? styles.tabOn : ''}`}
            onClick={() => setScope('layer')}
          >
            This layer
          </button>
          <button
            type="button"
            className={`${styles.tab} ${scope === 'all' ? styles.tabOn : ''}`}
            onClick={() => setScope('all')}
            title="A change here can break another layer's rules — they gate and override each other"
          >
            All layers
          </button>
        </div>
        <button
          type="button"
          className="btn-primary btn-sm"
          onClick={runAll}
          disabled={tests.length === 0 || running.length > 0}
        >
          {running.length > 0 ? 'Running...' : `Run all (${tests.length})`}
        </button>
      </div>

      <p className={styles.note}>
        Runs the configuration as saved — save your changes first, or the result reflects
        the version before them.
        {ranCount > 0 && ` ${ranCount} of ${tests.length} run.`}
      </p>

      {createTest.error && <ErrorBanner message={(createTest.error as Error).message} />}
      {updateTest.error && <ErrorBanner message={(updateTest.error as Error).message} />}
      {deleteTest.error && <ErrorBanner message={(deleteTest.error as Error).message} />}

      {tests.length === 0 ? (
        <p className={styles.empty}>
          {scope === 'all'
            ? 'No saved tests anywhere yet.'
            : 'No saved tests for this layer yet. Fill in a context below and name it to keep it.'}
        </p>
      ) : (
        <div className={styles.list}>
          {groups.map(([layer, group]) => (
            <div key={layer}>
              {scope === 'all' && (
                <div className={styles.group}>
                  {layer}
                  {layer === layerKey && <span className={styles.here}>editing</span>}
                </div>
              )}
              {group.map((t) => {
                const r = results[t.id];
                const isRunning = running.includes(t.id);
                return (
                  <div key={t.id}>
                    <div className={styles.row}>
                      <button
                        type="button"
                        className={`${styles.pick} ${t.id === selectedId ? styles.active : ''}`}
                        onClick={() => select(t)}
                        title={t.layer === layerKey ? undefined : `Filed under ${t.layer}`}
                      >
                        {t.name}
                      </button>
                      {/* The outcome sits between the name and the controls, so
                          a column of them reads down the panel at a glance —
                          the same slot a pass/fail verdict will occupy. */}
                      <button
                        type="button"
                        className={styles.outcome}
                        onClick={() => setExpanded(expanded === t.id ? null : t.id)}
                        disabled={!r}
                        aria-label={r ? `Show the result for ${t.name}` : undefined}
                      >
                        {isRunning && <span className={styles.pending}>…</span>}
                        {!isRunning && r?.error && <span className={styles.fail}>error</span>}
                        {!isRunning && r?.layerResult && (
                          <span className={`${styles.status} ${styles[r.layerResult.status] ?? ''}`}>
                            {r.layerResult.status}
                          </span>
                        )}
                      </button>
                      <button
                        type="button"
                        className="btn-ghost btn-sm"
                        onClick={() => runOne(t)}
                        disabled={isRunning}
                      >
                        run
                      </button>
                      <button
                        type="button"
                        className="btn-danger btn-sm"
                        onClick={() => setDeleting(t)}
                      >
                        x
                      </button>
                    </div>
                    {expanded === t.id && r && (
                      <div className={styles.detail}>
                        {r.error && <div className={styles.fail}>{r.error}</div>}
                        {r.response?.warnings?.map((w, i) => (
                          <div key={i} className={styles.warning}>
                            <code>{w.field}</code> {w.message}
                          </div>
                        ))}
                        <pre className={styles.pre}>
                          {JSON.stringify(r.layerResult ?? r.response, null, 2)}
                        </pre>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          ))}
        </div>
      )}

      <div className={styles.context}>
        <label className={styles.label}>
          Context{selected ? ` — ${selected.name}` : ' — unsaved'}
        </label>
        <ContextEditor value={draft} onChange={setDraft} schemas={schema ? [schema] : []} />
      </div>

      <div className={styles.actions}>
        {selected && (
          <>
            <button
              type="button"
              className="btn-ghost btn-sm"
              onClick={() => runOne(selected, draft)}
              disabled={running.includes(selected.id)}
            >
              Run edited
            </button>
            <button
              type="button"
              className="btn-ghost btn-sm"
              onClick={save}
              disabled={updateTest.isPending}
            >
              Save to &ldquo;{selected.name}&rdquo;
            </button>
          </>
        )}
        <input
          className={styles.name}
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          placeholder="name this context"
          aria-label="new test name"
        />
        <button
          type="button"
          className="btn-ghost btn-sm"
          onClick={saveAsNew}
          disabled={!newName.trim() || createTest.isPending}
        >
          Save as test
        </button>
      </div>

      <ConfirmDialog
        open={!!deleting}
        title="Delete Test"
        message={`Delete the saved test "${deleting?.name}"?`}
        onConfirm={() => {
          if (deleting) {
            deleteTest.mutate(deleting.id);
            if (deleting.id === selectedId) setSelectedId(null);
          }
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
    </div>
  );
}
