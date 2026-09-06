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
 * here because the row, the outcome column and the styling are all shaped
 * around it — when expectations arrive they set this, and nothing else about
 * the display has to move.
 */
export interface TestRunResult {
  layerResult?: LayerResult;
  response?: EvaluateResponse;
  error?: string;
  verdict?: 'pass' | 'fail';
}

/** Key for the unsaved draft's own result. Not a valid test id — ids are slugs. */
const DRAFT_KEY = 'draft:unsaved';

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
  // One name field serves both: the selected test's name while editing it, and
  // the new test's name otherwise. Two separate name inputs shown at once was
  // what made creating a test unclear.
  const [name, setName] = useState('');
  const [results, setResults] = useState<Record<string, TestRunResult>>({});
  const [running, setRunning] = useState<string[]>([]);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<SavedTest | null>(null);

  const tests = (allTests ?? []).filter((t) => scope === 'all' || t.layer === layerKey);
  const selected = tests.find((t) => t.id === selectedId) ?? null;

  // Grouped for the all-layers view so a row's layer is never in doubt, with
  // the layer being edited first — it is the one being worked on.
  const byLayer = new Map<string, SavedTest[]>();
  for (const t of tests) byLayer.set(t.layer, [...(byLayer.get(t.layer) ?? []), t]);
  const groups = [...byLayer.entries()].sort(([a], [b]) =>
    a === layerKey ? -1 : b === layerKey ? 1 : a.localeCompare(b),
  );

  const select = (test: SavedTest) => {
    setSelectedId(test.id);
    setName(test.name);
    setDraft(test.context ?? {});
  };

  const startNew = () => {
    setSelectedId(null);
    setName('');
    setDraft({});
  };

  /**
   * Evaluates one context against one layer and files the result under `key`.
   *
   * Scoped to that layer rather than the whole config because the test was
   * saved to ask about that layer; the engine still evaluates everything it
   * depends on, so a cross-layer gate is exercised either way.
   *
   * Uses apiFetch directly rather than a mutation, so a run-all can await each
   * one and keep the results apart — a shared mutation has a single `data` and
   * only the last response would survive.
   */
  const run = async (
    key: string,
    layer: string,
    context: Record<string, unknown>,
    languages?: string[],
    renderAll?: boolean,
  ) => {
    setRunning((r) => [...r, key]);
    try {
      const response = await apiFetch<EvaluateResponse>('/v1/evaluate', {
        method: 'POST',
        body: JSON.stringify({
          context,
          layers: [layer],
          languages: languages?.length ? languages : undefined,
          render_all: renderAll || undefined,
        }),
      });
      setResults((r) => ({ ...r, [key]: { response, layerResult: response.layers?.[layer] } }));
    } catch (e) {
      setResults((r) => ({ ...r, [key]: { error: (e as Error).message } }));
    } finally {
      setRunning((r) => r.filter((k) => k !== key));
    }
  };

  // A single run opens its own result. The outcome was reachable only by
  // clicking the status, which is not discoverable and is one click too many
  // when confirming what a change did — the reason to run it at all.
  const runOne = async (t: SavedTest, context?: Record<string, unknown>) => {
    setExpanded(t.id);
    await run(t.id, t.layer, context ?? t.context ?? {}, t.languages, t.renderAll);
  };

  const runDraft = async () => {
    setExpanded(DRAFT_KEY);
    await run(DRAFT_KEY, layerKey, draft);
  };

  // Sequential rather than parallel: these all write to the same result map,
  // and a handful of local evaluations is fast enough that predictable
  // ordering is worth more than the concurrency. Nothing is auto-expanded —
  // the outcome column is the point when running a set.
  const runAll = async () => {
    for (const t of tests) await runOne(t);
    setExpanded(null);
  };

  const save = () => {
    if (!selected) return;
    const trimmed = name.trim();
    if (!trimmed) return;
    updateTest.mutate({ id: selected.id, test: { ...selected, name: trimmed, context: draft } });
  };

  const saveAsNew = () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    createTest.mutate(
      { layer: layerKey, name: trimmed, context: draft },
      {
        onSuccess: (snap) => {
          // Select what was just created, so the panel is now editing it
          // rather than still offering to create the same thing again.
          const created = (snap.tests ?? []).find(
            (t) => t.layer === layerKey && t.name === trimmed,
          );
          if (created) setSelectedId(created.id);
        },
      }
    );
  };

  const ranCount = tests.filter((t) => results[t.id]).length;

  /** The outcome of a run, shown in full: what resolved, and what it emitted. */
  const renderResult = (r: TestRunResult) => (
    <div className={styles.detail}>
      {r.error && <div className={styles.fail}>{r.error}</div>}
      {r.layerResult && (
        <>
          <div className={styles.resultLine}>
            <span className={`${styles.status} ${styles[r.layerResult.status] ?? ''}`}>
              {r.layerResult.status}
            </span>
            {r.layerResult.segment && <code>{r.layerResult.segment}</code>}
            {r.layerResult.reason && (
              <span className={styles.reason}>{r.layerResult.reason}</span>
            )}
          </div>

          {/* The emitted record, laid out rather than buried in JSON — it is
              what an author is checking when they run a test at all. */}
          {r.layerResult.outputs && Object.keys(r.layerResult.outputs).length > 0 && (
            <table className={styles.outputs}>
              <tbody>
                {Object.entries(r.layerResult.outputs).map(([k, v]) => (
                  <tr key={k}>
                    <td className={styles.outKey}>{k}</td>
                    <td className={styles.outVal}>
                      {typeof v === 'object' ? JSON.stringify(v) : String(v)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {r.layerResult.failures && r.layerResult.failures.length > 0 && (
            <ul className={styles.failures}>
              {r.layerResult.failures.map((f) => (
                <li key={f.rule}>
                  <code>{f.rule}</code> {f.message}
                </li>
              ))}
            </ul>
          )}

          {!r.layerResult.outputs && !r.layerResult.failures?.length && (
            <div className={styles.none}>No outputs emitted.</div>
          )}
        </>
      )}

      {r.response?.warnings?.map((w, i) => (
        <div key={i} className={styles.warning}>
          <code>{w.field}</code> {w.message}
        </div>
      ))}

      <details>
        <summary className={styles.raw}>Raw response — drag its lower edge to see more</summary>
        <pre className={styles.pre}>{JSON.stringify(r.response ?? r, null, 2)}</pre>
      </details>
    </div>
  );

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
            : 'No saved tests for this layer yet — name a context below to keep one.'}
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
                        title={r ? 'Show or hide this result' : undefined}
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
                    {expanded === t.id && r && renderResult(r)}
                  </div>
                );
              })}
            </div>
          ))}
        </div>
      )}

      {/* Authoring half. The name leads: naming a context is what turns it into
          a test, and it used to sit below the context editor beside a Save
          button, where it read as an afterthought rather than the way you
          create one. */}
      <div className={styles.editor}>
        <div className={styles.editorHead}>
          <strong className={styles.editorTitle}>{selected ? 'Editing test' : 'New test'}</strong>
          {selected && (
            <button type="button" className={styles.linkish} onClick={startNew}>
              + new test
            </button>
          )}
        </div>

        <label className={styles.label} htmlFor="test-name">Name</label>
        <input
          id="test-name"
          className={styles.name}
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. high earner, low hours"
        />

        <label className={styles.label}>Context</label>
        <ContextEditor value={draft} onChange={setDraft} schemas={schema ? [schema] : []} />

        <div className={styles.actions}>
          <button
            type="button"
            className="btn-ghost btn-sm"
            onClick={() => (selected ? runOne(selected, draft) : runDraft())}
            disabled={running.length > 0}
          >
            Run without saving
          </button>
          <span className={styles.spacer} />
          <button
            type="button"
            className="btn-primary btn-sm"
            onClick={selected ? save : saveAsNew}
            disabled={!name.trim() || createTest.isPending || updateTest.isPending}
          >
            {selected ? 'Save changes' : 'Save test'}
          </button>
        </div>

        {/* The draft's own result, shown here rather than in the list, because
            an unsaved context has no row to hang it off. */}
        {!selected && results[DRAFT_KEY] && renderResult(results[DRAFT_KEY])}
      </div>

      <ConfirmDialog
        open={!!deleting}
        title="Delete Test"
        message={`Delete the saved test "${deleting?.name}"?`}
        onConfirm={() => {
          if (deleting) {
            deleteTest.mutate(deleting.id);
            if (deleting.id === selectedId) startNew();
          }
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
    </div>
  );
}
