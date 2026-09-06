import type { LookupTable } from '../../api/types';

interface Props {
  /** The table this field is bound to, or undefined when it is not bound. */
  table?: LookupTable;
  /** Shown when the field has no binding. */
  emptyLabel?: string;
}

/**
 * A field's lookup binding, wherever a schema is shown.
 *
 * Bound or not is always stated — an unbound field reads as a dash rather than
 * as blank space, so an author can tell "no table" from "this view does not
 * mention tables".
 *
 * The link opens the table's editor in a new tab rather than navigating. Every
 * place this appears sits inside unsaved work — a layer form in a modal, a
 * segment editor mid-edit — and consulting a table is a side errand, not a
 * departure. `useLookups` refetches on window focus so entries added over there
 * are present when the author comes back.
 */
export default function LookupLink({ table, emptyLabel = '—' }: Props) {
  if (!table) {
    return <span style={{ color: 'var(--text-muted)' }}>{emptyLabel}</span>;
  }
  return (
    <a
      href={`/lookups?edit=${encodeURIComponent(table.id)}`}
      target="_blank"
      rel="noopener noreferrer"
      title={`Open the "${table.name}" lookup table in a new tab (${table.entries?.length ?? 0} entries)`}
    >
      {table.name} ↗
    </a>
  );
}
