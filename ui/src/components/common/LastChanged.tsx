import { ago } from '../../utils/time';
import styles from './LastChanged.module.css';

interface Props {
  /** ISO timestamp of the last write, or absent for a layer never written. */
  at?: string;
  /** The concurrency token, shown as the tooltip's precise answer. */
  revision?: number;
  /** Render as a single line of small print rather than an inline chip. */
  block?: boolean;
}

/**
 * When a layer was last changed.
 *
 * Paired with optimistic locking rather than decoration: the reason to show
 * this is that a save can now be refused for staleness, and an author who can
 * see "changed 2 min ago" understands the refusal before it happens. Absent
 * timestamps are stated, not hidden — "never changed here" is information, and
 * a blank space reads as a bug.
 */
export default function LastChanged({ at, revision, block }: Props) {
  const title = at
    ? `${new Date(at).toLocaleString()}${revision === undefined ? '' : ` · revision ${revision}`}`
    : 'No recorded change since this layer was written by hand';

  return (
    <span className={block ? styles.block : styles.chip} title={title}>
      {at ? `Changed ${ago(at)}` : 'No change recorded'}
    </span>
  );
}
