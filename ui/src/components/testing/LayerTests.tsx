import { useState } from 'react';
import { useTests, useCreateTest, useUpdateTest, useDeleteTest } from '../../api/tests';
import { apiFetch } from '../../api/client';
import { duration } from '../../utils/time';
import { formatOutput } from './outputFormat';
import type { EvaluateResponse, Failure, InputSchema, LayerResult, SavedTest, Segment } from '../../api/types';
import ContextEditor from './ContextEditor';
import ConfirmDialog from '../common/ConfirmDialog';
import ErrorBanner from '../common/ErrorBanner';
import styles from './LayerTests.module.css';

interface Props {
  /** The layer being edited. Every test shown is filed under it. */
  layerKey: string;
  /** The segment being edited — the default scope, and where a new test is filed. */
  segmentId: string;
  /** The layer's segments, so a test can say which one it exercises. */
  segments: Segment[];
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
  /**
   * Round trip measured here, in microseconds to match the engine's own unit.
   *
   * Kept alongside `response.duration_us` rather than instead of it: they
   * answer different questions. The engine's number is what the configuration
   * costs to evaluate — the one that means something about the rules being
   * authored. This one includes HTTP, JSON and the browser, and is the only
   * one available when the request fails.
   */
  elapsedUs?: number;
}

/** Key for the unsaved draft's own result. Not a valid test id — ids are slugs. */
const DRAFT_KEY = 'draft:unsaved';

/**
 * Groups findings by the segment that reported them, preserving the order the
 * engine returned.
 *
 * The key is "" when the response carries no segment, which is the case for a
 * layer that ran only one — there is then a single group and no heading, so a
 * single-segment run reads exactly as it always did.
 */
function groupFindings(failures: Failure[]): [string, Failure[]][] {
  const groups: [string, Failure[]][] = [];
  for (const f of failures) {
    const key = f.segment ?? '';
    const last = groups[groups.length - 1];
    if (last && last[0] === key) last[1].push(f);
    else groups.push([key, [f]]);
  }
  return groups;
}

export default function LayerTests({ layerKey, segmentId, segments, schema }: Props) {
  const { data: allTests } = useTests(layerKey);
  const createTest = useCreateTest();
  const updateTest = useUpdateTest();
  const deleteTest = useDeleteTest();

  // Which of the layer's tests are listed. Was "this layer / all layers", and
  // the wide half was both too broad to be useful and the reason the narrow
  // half was wrong: a layer's tests all looked like they belonged to whichever
  // segment happened to be open. The choice that matters is within the layer.
  const [scope, setScope] = useState<'segment' | 'layer'>('segment');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  // The context being edited, held locally so typing does not write to the
  // server on every keystroke. Seeded when a test is selected.
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  // One name field serves both: the selected test's name while editing it, and
  // the new test's name otherwise. Two name inputs shown at once was what made
  // creating a test unclear in the first place — and what makes one field
  // workable is Duplicate: with a way to branch from a test, editing the name
  // can only mean renaming this one, and the hint below it says so.
  const [name, setName] = useState('');
  // Rendering options carried alongside the draft. The panel has no controls
  // for them, so without holding them a duplicate would quietly lose whatever
  // the original was saved with.
  const [draftMeta, setDraftMeta] = useState<{ languages?: string[]; renderAll?: boolean }>({});
  const [results, setResults] = useState<Record<string, TestRunResult>>({});
  const [running, setRunning] = useState<string[]>([]);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<SavedTest | null>(null);

  // A test with no segment is assigned to none, so it is listed under every
  // one — there is nothing saying it belongs anywhere else, and hiding it from
  // the segment view would make it look deleted. Tests written before they
  // were kept per segment are the ones like this.
  const tests = (allTests ?? []).filter(
    (t) => t.layer === layerKey && (scope === 'layer' || !t.segment || t.segment === segmentId),
  );
  const selected = tests.find((t) => t.id === selectedId) ?? null;

  /**
   * What a run is scoped to, taken from the tab above the list.
   *
   * The tab governs the run, not just which tests are listed. It was a list
   * filter at first and the run was scoped by the test's own `segment` — which
   * meant a tab labelled "This segment" ran the whole layer for any test filed
   * before segments existed, and nothing on screen explained why. One control,
   * one meaning: "This segment" runs this segment, "Whole layer" runs them all.
   *
   * A test's `segment` still decides where it is filed — which group it appears
   * under, and what its name has to be unique against — but no longer what a
   * run does, so a test written before per-segment filing needs no migration
   * to be run against one segment.
   */
  const runScope = () => (scope === 'segment' ? segmentId : undefined);

  const segmentLabel = (id: string) => {
    const s = segments.find((x) => x.id === id);
    return s ? s.name || s.id : id;
  };

  // Grouped by segment for the layer-wide view, with the segment being edited
  // first, then the rest in the order they are evaluated — which is the order
  // that decides which of them a real evaluation can reach.
  const bySegment = new Map<string, SavedTest[]>();
  for (const t of tests) {
    const key = t.segment ?? '';
    bySegment.set(key, [...(bySegment.get(key) ?? []), t]);
  }
  const order = new Map(segments.map((s, i) => [s.id, i]));
  const groups = [...bySegment.entries()].sort(([a], [b]) => {
    if (a === segmentId) return -1;
    if (b === segmentId) return 1;
    return (order.get(a) ?? 99) - (order.get(b) ?? 99);
  });

  const select = (test: SavedTest) => {
    setSelectedId(test.id);
    setName(test.name);
    setDraft(test.context ?? {});
    setDraftMeta({ languages: test.languages, renderAll: test.renderAll });
  };

  const startNew = () => {
    setSelectedId(null);
    setName('');
    setDraft({});
    setDraftMeta({});
  };

  /**
   * Starts a new test from the selected one.
   *
   * Deliberately leaves it unsaved rather than creating it outright: the point
   * of copying a scenario is to change something about it, and a copy that
   * already exists has to be found and edited afterwards. The languages and
   * render-all flags come along too — the panel has no controls for them, so
   * dropping them here would silently make the copy a different test.
   */
  const duplicate = () => {
    if (!selected) return;
    setSelectedId(null);
    setName(`${selected.name} copy`);
    setDraft({ ...(selected.context ?? {}) });
    setDraftMeta({ languages: selected.languages, renderAll: selected.renderAll });
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
    segment: string | undefined,
    context: Record<string, unknown>,
    languages?: string[],
    renderAll?: boolean,
  ) => {
    setRunning((r) => [...r, key]);
    const started = performance.now();
    const elapsedUs = () => Math.round((performance.now() - started) * 1000);
    try {
      const response = await apiFetch<EvaluateResponse>('/v1/evaluate', {
        method: 'POST',
        body: JSON.stringify({
          context,
          layers: [layer],
          // Scoped by the tab above the list (see runScope). Without a scope
          // the layer resolves to whichever segments apply, so a run aimed at
          // one of them reports on all of them — and before segment scoping
          // existed at all, a segment behind one that always answers could not
          // be exercised at any price.
          segments: segment ? [segment] : undefined,
          languages: languages?.length ? languages : undefined,
          render_all: renderAll || undefined,
        }),
      });
      setResults((r) => ({
        ...r,
        [key]: { response, layerResult: response.layers?.[layer], elapsedUs: elapsedUs() },
      }));
    } catch (e) {
      setResults((r) => ({
        ...r,
        [key]: { error: (e as Error).message, elapsedUs: elapsedUs() },
      }));
    } finally {
      setRunning((r) => r.filter((k) => k !== key));
    }
  };

  // A single run opens its own result. The outcome was reachable only by
  // clicking the status, which is not discoverable and is one click too many
  // when confirming what a change did — the reason to run it at all.
  const runOne = async (t: SavedTest, context?: Record<string, unknown>) => {
    setExpanded(t.id);
    await run(t.id, t.layer, runScope(), context ?? t.context ?? {}, t.languages, t.renderAll);
  };

  const runDraft = async () => {
    setExpanded(DRAFT_KEY);
    await run(DRAFT_KEY, layerKey, runScope(), draft);
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
    // draftMeta last: it holds the edited languages and render-all, and
    // spreading `selected` alone would save the stored values back over them.
    updateTest.mutate({
      id: selected.id,
      test: { ...selected, name: trimmed, context: draft, ...draftMeta },
    });
  };

  const saveAsNew = () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    createTest.mutate(
      { layer: layerKey, segment: segmentId, name: trimmed, context: draft, ...draftMeta },
      {
        onSuccess: (snap) => {
          // Select what was just created, so the panel is now editing it
          // rather than still offering to create the same thing again.
          const created = (snap.tests ?? []).find(
            (t) => t.layer === layerKey && t.segment === segmentId && t.name === trimmed,
          );
          if (created) setSelectedId(created.id);
        },
      }
    );
  };

  const ranCount = tests.filter((t) => results[t.id]).length;
  // A selected test whose name has been edited: the save will rename it.
  const renaming = !!selected && !!name.trim() && name.trim() !== selected.name;

  /**
   * How long a run took, shown with the outcome.
   *
   * The engine's own measurement is the headline where it exists — it is what
   * this configuration costs to evaluate, the number worth watching as rules
   * are added. The round trip is the tooltip: larger and mostly transport, so
   * leading with it would make every layer look slow.
   *
   * The exception is a reported zero, which is the common case and is not a
   * measurement. The engine times itself with Go's clock, and that clock
   * advances in ticks of about half a millisecond on Windows — an evaluation
   * of a handful of layers takes tens of microseconds, so both reads usually
   * land in the same tick and the subtraction is exactly 0. Printing "0 µs"
   * would state a precision that does not exist and make a fast run look
   * broken; the honest report is that it finished inside one tick, with the
   * round trip as the only number actually measured.
   */
  const renderTiming = (r: TestRunResult) => {
    const engine = r.response?.duration_us;
    const trip = r.elapsedUs;
    if (engine === undefined && trip === undefined) return null;

    // Below the server clock's resolution: no engine figure to report.
    if (engine === 0) {
      return (
        <span
          className={styles.timing}
          title={
            'The engine reported 0 µs: the evaluation finished within one tick of the ' +
            "server's clock (about 0.5 ms on Windows), so it is too fast for that clock " +
            'to measure. ' +
            (trip === undefined
              ? ''
              : `The ${duration(trip)} round trip measured here is mostly HTTP and JSON.`)
          }
        >
          &lt; 1 tick
        </span>
      );
    }

    return (
      <span
        className={styles.timing}
        title={
          engine === undefined
            ? 'Round trip from this browser; the run reported no engine time'
            : `Engine ${duration(engine)} · round trip ${duration(trip ?? NaN)}`
        }
      >
        {engine === undefined ? `${duration(trip ?? NaN)} round trip` : duration(engine)}
      </span>
    );
  };

  /** The outcome of a run, shown in full: what resolved, and what it emitted. */
  const renderResult = (r: TestRunResult) => (
    <div className={styles.detail}>
      {r.error && (
        <div className={styles.fail}>
          {r.error} {renderTiming(r)}
        </div>
      )}
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
            {renderTiming(r)}
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
                      {formatOutput(v)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {/* Findings, grouped under the segment that reported them whenever
              the layer ran more than one. Grouping rather than a tag per row:
              a merged list is one flat array in the response, but the thing an
              author is reading it for is "what did each stage say", and a
              heading answers that at a glance where a repeated inline label
              does not. A single-segment run has nothing to group, so it stays
              a plain list — the segment is already in the reason. */}
          {r.layerResult.failures && r.layerResult.failures.length > 0 && (
            <>
              {groupFindings(r.layerResult.failures).map(([segment, findings]) => (
                <div key={segment}>
                  {/* Shown for one segment as readily as for several: the
                      engine stamps every finding, and a heading that came and
                      went with the layer's segment count would make the reader
                      work out which case they were looking at. Friendly name,
                      with the id it resolves from on hover — the id is what
                      the response carries and what a saved test is filed
                      under. */}
                  {segment && (
                    <div className={styles.findingGroup} title={segment}>
                      {segmentLabel(segment)}
                    </div>
                  )}
                  <ul className={styles.failures}>
                    {findings.map((f) => (
                      <li key={`${segment}/${f.rule}`}>
                        <code>{f.rule}</code> {f.message}
                        {/* The localized set, present only when the run asked
                            for languages. Shown per language rather than
                            folded into the line above, because the point of
                            asking for them is to compare what each caller
                            gets — including which ones fell back. */}
                        {f.messages && Object.keys(f.messages).length > 0 && (
                          <ul className={styles.messages}>
                            {Object.entries(f.messages).map(([lang, text]) => (
                              <li key={lang}>
                                <span className={styles.langTag}>{lang}</span> {text}
                              </li>
                            ))}
                          </ul>
                        )}
                        {/* The record this finding emitted. A checklist puts
                            its outputs here, per finding, never on the layer —
                            so the table above is always empty for one, and
                            without this the emitted record was visible only in
                            the raw response. */}
                        {f.outputs && Object.keys(f.outputs).length > 0 && (
                          <table className={styles.outputs}>
                            <tbody>
                              {Object.entries(f.outputs).map(([k, v]) => (
                                <tr key={k}>
                                  <td className={styles.outKey}>{k}</td>
                                  <td className={styles.outVal}>{formatOutput(v)}</td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        )}
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </>
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
        <summary className={styles.raw}>Raw response</summary>
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
            className={`${styles.tab} ${scope === 'segment' ? styles.tabOn : ''}`}
            onClick={() => setScope('segment')}
            title={`Tests filed against ${segmentLabel(segmentId)}`}
          >
            This segment
          </button>
          <button
            type="button"
            className={`${styles.tab} ${scope === 'layer' ? styles.tabOn : ''}`}
            onClick={() => setScope('layer')}
            title="Every test in this layer, grouped by the segment it exercises"
          >
            Whole layer
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

      {/* What the tab above actually does to a run, stated rather than
          implied. Which segment answers a layer is otherwise invisible: a
          layer resolves to the first one that applies, so a run that was not
          scoped can report on a segment you are not looking at. */}
      <p className={styles.note}>
        {scope === 'segment' ? (
          <>
            Runs <strong>{segmentLabel(segmentId)}</strong> only — other segments of this
            layer are passed over, even ones that would normally answer first.
          </>
        ) : (
          <>
            Runs the <strong>whole layer</strong>: every segment that applies, findings
            merged.
          </>
        )}{' '}
        Against the configuration as saved — save your changes first, or the result
        reflects the version before them.
        {ranCount > 0 && ` ${ranCount} of ${tests.length} run.`}
      </p>

      {createTest.error && <ErrorBanner message={(createTest.error as Error).message} />}
      {updateTest.error && <ErrorBanner message={(updateTest.error as Error).message} />}
      {deleteTest.error && <ErrorBanner message={(deleteTest.error as Error).message} />}

      {tests.length === 0 ? (
        <p className={styles.empty}>
          {scope === 'layer'
            ? 'No saved tests in this layer yet.'
            : 'No saved tests for this segment yet — name a context below to keep one.'}
        </p>
      ) : (
        <div className={styles.list}>
          {groups.map(([segment, group]) => (
            <div key={segment}>
              {scope === 'layer' && (
                <div className={styles.group}>
                  {/* An unscoped test is not filed against any segment: it
                      predates per-segment filing and runs them all. */}
                  {segment === '' ? 'no segment' : segmentLabel(segment)}
                  {segment === segmentId && <span className={styles.here}>editing</span>}
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
                        title={
                          t.segment
                            ? `Filed under ${segmentLabel(t.segment)}`
                            : 'This test is not assigned to a segment, so it is listed under ' +
                              'every one. Tests written before they were kept per segment are ' +
                              'like this. It does not affect what a run covers — the tab above ' +
                              'decides that.'
                        }
                      >
                        {t.name}
                        {/* Where the test is filed, which is no longer what a
                            run covers — the tab decides that. Still worth
                            showing: it is why this row appears under every
                            segment rather than one. */}
                        {!t.segment && <span className={styles.wholeLayer}>no segment</span>}
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
                      {/* Its own fixed column rather than inside the outcome
                          button, so a run-all leaves a readable column of
                          times without knocking the statuses out of line. */}
                      <span className={styles.rowTiming}>{r && renderTiming(r)}</span>
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
          <strong className={styles.editorTitle}>
            {selected ? `Editing “${selected.name}”` : 'New test'}
          </strong>
          {selected && (
            <>
              {/* Duplicate is what makes the name field unambiguous: with a way
                  to branch from a test, editing the name can only mean
                  renaming this one. */}
              <button type="button" className={styles.linkish} onClick={duplicate}>
                duplicate
              </button>
              <button type="button" className={styles.linkish} onClick={startNew}>
                + new test
              </button>
            </>
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
        {/* The ambiguity this answers: with a test selected, typing in the name
            field looks like it might be starting a new one. It renames, and
            says so — with the other intent named and one click away. */}
        {renaming && (
          <p className={styles.renameHint}>
            Renames <code>{selected!.name}</code> when you save. To keep both, use{' '}
            <button type="button" className={styles.linkish} onClick={duplicate}>
              duplicate
            </button>{' '}
            instead.
          </p>
        )}

        {/* Message rendering. A saved test has carried these two fields since
            it existed, but nothing could set them — so a run never asked for a
            language, the engine never built the localized `messages` map, and
            the only way to see an authored `en` message was the flattened
            `message` the default language produces. That is the whole reason
            these fields are on the model. */}
        <label className={styles.label}>Messages</label>
        <div className={styles.langRow}>
          <input
            className={styles.lang}
            value={(draftMeta.languages ?? []).join(', ')}
            onChange={(e) =>
              setDraftMeta((m) => ({
                ...m,
                languages: e.target.value
                  .split(',')
                  .map((s) => s.trim())
                  .filter(Boolean),
              }))
            }
            placeholder="en, es — blank for none"
            aria-label="languages to render"
            title="Language codes to render messages in. The layer's defaultLanguage fills a gap."
          />
          <label className={styles.langAll}>
            <input
              type="checkbox"
              checked={!!draftMeta.renderAll}
              onChange={(e) => setDraftMeta((m) => ({ ...m, renderAll: e.target.checked || undefined }))}
              style={{ width: 'auto' }}
            />
            <span>every language</span>
          </label>
        </div>
        <p className={styles.langHint}>
          Leave both empty and a finding carries only its rendered <code>message</code>. Name a
          language, or tick every one, to see the localized set a caller would receive.
        </p>

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
            {selected ? (renaming ? 'Rename & save' : 'Save changes') : 'Save test'}
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
