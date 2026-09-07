import Modal from './Modal';
import styles from './ConfirmDialog.module.css';

interface Props {
  open: boolean;
  title: string;
  message: string;
  onConfirm: () => void;
  onCancel: () => void;
  /** The confirm button's text. Defaults to "Delete" because every use was a
   *  removal when this was written, but the dialog is equally the right shape
   *  for any consequential change — and a button reading "Delete" under a
   *  message about changing a field's type is actively misleading. */
  confirmLabel?: string;
}

export default function ConfirmDialog({
  open,
  title,
  message,
  onConfirm,
  onCancel,
  confirmLabel = 'Delete',
}: Props) {
  return (
    <Modal open={open} onClose={onCancel} title={title}>
      <p className={styles.message}>{message}</p>
      <div className={styles.actions}>
        <button type="button" className="btn-ghost" onClick={onCancel}>Cancel</button>
        <button type="button" className="btn-danger" onClick={onConfirm}>{confirmLabel}</button>
      </div>
    </Modal>
  );
}
