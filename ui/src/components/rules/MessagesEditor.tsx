import { useEffect, useState } from 'react';
import styles from './MessagesEditor.module.css';

interface Props {
  value?: Record<string, string>;
  onChange: (v: Record<string, string> | undefined) => void;
  /** Hint shown under the header describing evaluation/rendering context. */
  hint?: string;
}

type Entry = [string, string];

// Assemble entries into a record, dropping rows with an empty locale code.
// Later rows win on duplicate codes.
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

  // Re-sync when the underlying value changes externally (e.g. rule reordered).
  useEffect(() => {
    setEntries(Object.entries(value ?? {}));
  }, [JSON.stringify(value ?? {})]);

  const count = Object.keys(value ?? {}).length;

  const commit = (next: Entry[]) => {
    setEntries(next);
    onChange(assemble(next));
  };

  const setLang = (i: number, lang: string) =>
    commit(entries.map((e, idx) => (idx === i ? [lang, e[1]] : e)));
  const setText = (i: number, text: string) =>
    commit(entries.map((e, idx) => (idx === i ? [e[0], text] : e)));
  const remove = (i: number) => commit(entries.filter((_, idx) => idx !== i));
  const add = () => commit([...entries, ['', '']]);

  return (
    <div className={styles.root}>
      <button type="button" className={styles.toggle} onClick={() => setOpen((o) => !o)}>
        {open ? '▾' : '▸'} Messages{count > 0 ? ` (${count})` : ''}
      </button>
      {open && (
        <div className={styles.body}>
          {hint && <p className={styles.hint}>{hint}</p>}
          {entries.map(([lang, text], i) => (
            <div key={i} className={styles.row}>
              <input
                className={styles.lang}
                value={lang}
                onChange={(e) => setLang(i, e.target.value)}
                placeholder="en"
                aria-label="language code"
              />
              <input
                className={styles.text}
                value={text}
                onChange={(e) => setText(i, e.target.value)}
                placeholder="Message with ${variables} and ${expressions}"
                aria-label="message text"
              />
              <button className="btn-danger btn-sm" onClick={() => remove(i)}>x</button>
            </div>
          ))}
          <button className="btn-ghost btn-sm" onClick={add}>+ Add message</button>
        </div>
      )}
    </div>
  );
}
