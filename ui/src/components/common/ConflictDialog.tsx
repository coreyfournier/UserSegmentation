import type { ConflictDetail } from '../../api/types';
import Modal from './Modal';
import { ago } from '../../utils/time';
import styles from './ConflictDialog.module.css';

interface Props {
  conflict: ConflictDetail | null;
  /** Save again against the revision the store now holds, keeping this work. */
  onOverwrite: () => void;
  /** Throw this work away and reload what is stored. */
  onDiscard: () => void;
  onCancel: () => void;
  /** What the caller is about to lose or impose, e.g. "your changes to ct-fee". */
  subject?: string;
}

/**
 * The choice after an optimistic-concurrency conflict.
 *
 * Both outcomes lose something, so neither is presented as the safe default and
 * both say what goes: overwriting discards the other person's save, discarding
 * throws away this work. Naming the cost is the whole job here — a dialog that
 * said only "conflict, retry?" would leave the author guessing which.
 *
 * Cancel is a third option on purpose. It keeps the editor exactly as it is so
 * the author can copy what they wrote somewhere before choosing, which is the
 * one thing neither other button allows.
 */
export default function ConflictDialog({
  conflict,
  onOverwrite,
  onDiscard,
  onCancel,
  subject = 'your changes',
}: Props) {
  const when = conflict?.changedAt ? ago(conflict.changedAt) : null;

  return (
    <Modal open={!!conflict} onClose={onCancel} title="Changed by someone else">
      {conflict && (
        <div>
          <p className={styles.lead}>
            <strong>{conflict.kind === 'layer' ? `Layer ${conflict.key}` : conflict.key}</strong>{' '}
            was saved by someone else{when ? ` ${when}` : ''}, after you loaded it.
          </p>
          <p className={styles.detail}>
            You have revision {conflict.expected}; the stored one is {conflict.actual}. Saving
            now would replace their version with yours.
          </p>

          <div className={styles.choices}>
            <div className={styles.choice}>
              <button type="button" className="btn-primary" onClick={onOverwrite}>
                Overwrite with mine
              </button>
              <p className={styles.cost}>
                Keeps {subject}. Their save is lost, and nothing here can show you what it
                was.
              </p>
            </div>

            <div className={styles.choice}>
              <button type="button" className="btn-danger" onClick={onDiscard}>
                Discard mine and reload
              </button>
              <p className={styles.cost}>Keeps their version. {subject} is lost.</p>
            </div>
          </div>

          <div className={styles.actions}>
            <button type="button" className="btn-ghost btn-sm" onClick={onCancel}>
              Cancel — decide later
            </button>
            <span className={styles.hint}>
              Leaves the editor as it is, so you can copy your work out first.
            </span>
          </div>
        </div>
      )}
    </Modal>
  );
}
