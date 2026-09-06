import type { TextareaHTMLAttributes } from 'react';
import styles from './ExpandableField.module.css';

type Props = TextareaHTMLAttributes<HTMLTextAreaElement>;

/**
 * A single-line text field the author can drag taller.
 *
 * Used for the fields that hold expressions, templates and value lists — long
 * enough that one line truncates them, and awkward to read a character at a
 * time through a 30-column window. It resolves to a textarea, so the native
 * resize handle does the work; at rest it is the same height as the input it
 * replaces, so no layout changes until the author asks for it.
 *
 * The height is not remembered between visits. Persisting it would mean
 * per-field state keyed by a field identity these editors do not have — rows
 * are positional and get renamed and reordered.
 *
 * The class is applied before the caller's, so a call site can still restyle
 * the field. Font face is left to the global `textarea` rule (monospace),
 * which is what these four fields hold: expressions, templates and value
 * lists.
 *
 * Drop-in for an `<input>`: same value/onChange/placeholder/aria props. Only
 * use it where Enter does not commit — a textarea takes Enter as a newline. At
 * the four call sites it is used, Enter previously did nothing at all.
 */
export default function ExpandableField({ className, ...rest }: Props) {
  return (
    <textarea
      rows={1}
      className={className ? `${styles.field} ${className}` : styles.field}
      {...rest}
    />
  );
}
