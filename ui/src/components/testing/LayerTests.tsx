import { useState } from 'react';
import { useTests, useCreateTest, useUpdateTest, useDeleteTest } from '../../api/tests';
import { useEvaluate } from '../../api/evaluate';
import type { EvaluateResponse, InputSchema, SavedTest } from '../../api/types';
import ContextEditor from './ContextEditor';
import ConfirmDialog from '../common/ConfirmDialog';
import ErrorBanner from '../common/ErrorBanner';
import styles from './LayerTests.module.css';

interface Props {
  layerKey: string;
  /** The layer's input schema, so the context editor can offer its fields. */
  schema?: InputSchema;
}

/**
 * A layer's saved tests, with the context of whichever is selected, and a Run
 * that evaluates only this layer.
 *
 * Sits inside the segment editor so the loop is edit, save, run without
 * leaving the page. Running always evaluates the config as saved — the engine
 * reads the stored snapshot, not the editor's unsaved state — so an unsaved
 * change is invisible here. The panel says so rather than pretending
 * otherwise.
 */
export default function LayerTests({ layerKey, schema }: Props) {
  const { data: tests } = useTests(layerKey);
  const createTest = useCreateTest();
  const updateTest = useUpdateTest();
  const deleteTest = useDeleteTest();
  const evaluate = useEvaluate();

  const [selectedId, setSelectedId] = useState<string | null>(null);
  // The context being edited, held locally so typing does not write to the
  // server on every keystroke. Seeded when a test is selected.
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [newName, setNewName] = useState('');
  const [result, setResult] = useState<EvaluateResponse | null>(null);
  const [deleting, setDeleting] = useState<SavedTest | null>(null);

  const list = tests ?? [];
  const selected = list.find((t) => t.id === selectedId) ?? null;

  const select = (test: SavedTest) => {
    setSelectedId(test.id);
    setDraft(test.context ?? {});
    setResult(null);
  };

  const run = (context: Record<string, unknown>) => {
    evaluate.mutate(
      { context, layers: [layerKey] },
      { onSuccess: (data) => setResult(data) }
    );
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
          // Select what was just created, so Run applies to it rather than to
          // whatever was selected before.
          const created = (snap.tests ?? []).find(
            (t) => t.layer === layerKey && t.name === name
          );
          if (created) setSelectedId(created.id);
        },
      }
    );
  };

  const layerResult = result?.layers?.[layerKey];

  return (
    <div>
      {createTest.error && <ErrorBanner message={(createTest.error as Error).message} />}
      {updateTest.error && <ErrorBanner message={(updateTest.error as Error).message} />}
      {deleteTest.error && <ErrorBanner message={(deleteTest.error as Error).message} />}
      {evaluate.error && <ErrorBanner message={(evaluate.error as Error).message} />}

      <p className={styles.note}>
        Runs against the configuration as saved. Save your changes above first, or the
        result reflects the version before them.
      </p>

      {list.length === 0 ? (
        <p className={styles.empty}>
          No saved tests for this layer yet. Fill in a context below and name it to keep it.
        </p>
      ) : (
        <div className={styles.list}>
          {list.map((t) => (
            <div key={t.id} className={styles.row}>
              <button
                type="button"
                className={`${styles.pick} ${t.id === selectedId ? styles.active : ''}`}
                onClick={() => select(t)}
              >
                {t.name}
              </button>
              <button
                type="button"
                className="btn-ghost btn-sm"
                onClick={() => {
                  select(t);
                  run(t.context ?? {});
                }}
                disabled={evaluate.isPending}
                title="Evaluate this layer with this test's context"
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
        <button
          type="button"
          className="btn-primary btn-sm"
          onClick={() => run(draft)}
          disabled={evaluate.isPending}
        >
          {evaluate.isPending ? 'Running...' : 'Run'}
        </button>
        {selected && (
          <button
            type="button"
            className="btn-ghost btn-sm"
            onClick={save}
            disabled={updateTest.isPending}
          >
            Save to &ldquo;{selected.name}&rdquo;
          </button>
        )}
        <span className={styles.spacer} />
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

      {layerResult && (
        <div className={styles.result}>
          <div className={styles.resultHead}>
            <span className={`${styles.status} ${styles[layerResult.status] ?? ''}`}>
              {layerResult.status}
            </span>
            {layerResult.segment && <code>{layerResult.segment}</code>}
            {layerResult.reason && <span className={styles.reason}>{layerResult.reason}</span>}
          </div>
          {result?.warnings && result.warnings.length > 0 && (
            <ul className={styles.warnings}>
              {result.warnings.map((w, i) => (
                <li key={i}>
                  <code>{w.field}</code> {w.message}
                </li>
              ))}
            </ul>
          )}
          <details>
            <summary className={styles.raw}>Full result</summary>
            <pre className={styles.pre}>{JSON.stringify(layerResult, null, 2)}</pre>
          </details>
        </div>
      )}

      <ConfirmDialog
        open={!!deleting}
        title="Delete Test"
        message={`Delete the saved test "${deleting?.name}"?`}
        onConfirm={() => {
          if (deleting) {
            deleteTest.mutate(deleting.id);
            if (deleting.id === selectedId) {
              setSelectedId(null);
              setResult(null);
            }
          }
          setDeleting(null);
        }}
        onCancel={() => setDeleting(null)}
      />
    </div>
  );
}
