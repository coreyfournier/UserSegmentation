import { useEffect, useRef, useState } from 'react';
import styles from './MessagesEditor.module.css';

interface Props {
  value?: Record<string, string>;
  onChange: (v: Record<string, string> | undefined) => void;
  /** Hint shown under the header describing evaluation/rendering context. */
  hint?: string;
}

type Entry = [string, string];

/**
 * Messages are keyed by locale, so a row with no code cannot be stored. Rows in
 * that state are surfaced in the UI rather than dropped quietly — losing typed
 * text with no indication is worse than refusing to save it.
 */
const isUnsaveable = ([lang, text]: Entry) => !lang.trim() && text.trim() !== '';

// Assemble entries into a record. Later rows win on duplicate codes.
function assemble(entries: Entry[]): Record<string, string> | undefined {
  const out: Record<string, string> = {};
  for (const [lang, text] of entries) {
    if (lang.trim()) out[lang.trim()] = text;
  }
  return Object.keys(out).length ? out : undefined;
}

export default function MessagesEditor({ value, onChange, hint }: Props) {
  const [open, setOpen] = useState(false);
  // Local draft so an in-progress empty locale row survives re-renders.
  const [entries, setEntries] = useState<Entry[]>(() => Object.entries(value ?? {}));

  // What this editor last sent upward. A row being edited often assembles to
  // the same value twice — clearing a locale drops the row from the assembled
  // record — and re-syncing on that echo would delete the row the user is
  // still typing in. Only a change that did not originate here re-seeds.
  const lastEmitted = useRef<string | null>(null);
  const incoming = JSON.stringify(value ?? {});

  useEffect(() => {
    if (incoming === lastEmitted.current) return;
    setEntries(Object.entries(value ?? {}));
    // `value` is reconstructed from `incoming`, which is the real dependency.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [incoming]);

  const count = Object.keys(value ?? {}).length;

  const commit = (next: Entry[]) => {
    setEntries(next);
    const assembled = assemble(next);
    lastEmitted.current = JSON.stringify(assembled ?? {});
    onChange(assembled);
  };

  const setLang = (i: number, lang: string) =>
    commit(entries.map((e, idx) => (idx === i ? [lang, e[1]] : e)));
  const setText = (i: number, text: string) =>
    commit(entries.map((e, idx) => (idx === i ? [e[0], text] : e)));
  const remove = (i: number) => commit(entries.filter((_, idx) => idx !== i));
  // Seed the locale so the common case needs no thought; it stays editable.
  const add = () => commit([...entries, [entries.length ? '' : 'en', '']]);

  const unsaveable = entries.filter(isUnsaveable).length;

  return (
    <div className={styles.root}>
      <button type="button" className={styles.toggle} onClick={() => setOpen((o) => !o)}>
        {open ? '▾' : '▸'} Messages{count > 0 ? ` (${count})` : ''}
        {unsaveable > 0 && (
          <span className={styles.badge} title="This text will not be saved without a language code">
            {unsaveable} won&rsquo;t save
          </span>
        )}
      </button>
      {open && (
        <div className={styles.body}>
          {hint && <p className={styles.hint}>{hint}</p>}
          {entries.map((entry, i) => {
            const [lang, text] = entry;
            const blocked = isUnsaveable(entry);
            return (
              <div key={i} className={styles.row}>
                <input
                  className={`${styles.lang} ${blocked ? styles.invalid : ''}`}
                  value={lang}
                  onChange={(e) => setLang(i, e.target.value)}
                  placeholder="en"
                  aria-label="language code"
                  aria-invalid={blocked}
                />
                <input
                  className={styles.text}
                  value={text}
                  onChange={(e) => setText(i, e.target.value)}
                  placeholder="Message with ${variables} and ${formulas}"
                  aria-label="message text"
                />
                <button className="btn-danger btn-sm" onClick={() => remove(i)}>x</button>
              </div>
            );
          })}
          {unsaveable > 0 && (
            <p className={styles.warning}>
              A message is stored under its language code. Enter one (for example{' '}
              <code>en</code>) or this text is discarded on save.
            </p>
          )}
          <button className="btn-ghost btn-sm" onClick={add}>+ Add message</button>
        </div>
      )}
    </div>
  );
}
