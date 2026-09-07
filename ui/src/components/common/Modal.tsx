import { useEffect, useRef, type ReactNode } from 'react';
import styles from './Modal.module.css';

interface Props {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
}

export default function Modal({ open, onClose, title, children }: Props) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    else if (!open && el.open) el.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className={styles.dialog}
      // Only close for this dialog's own close event. A ConfirmDialog rendered
      // inside another modal is a React child of it, and React propagates the
      // synthetic close event up the component tree — so dismissing the inner
      // confirm was also firing the outer onClose and tearing down the whole
      // form behind it. Measured: two open dialogs went to zero on one Cancel.
      onClose={(e) => {
        if (e.target === ref.current) onClose();
      }}
    >
      <div className={styles.header}>
        <h3>{title}</h3>
        <button type="button" className="btn-ghost btn-sm" onClick={onClose}>X</button>
      </div>
      {/* Children are mounted only while open. A native <dialog> keeps its
          subtree in the DOM when closed, so a form left mounted holds the
          state it was last given — reopening "Add" would show whatever was
          typed the time before. Unmounting resets it, and does so for every
          modal rather than needing each call site to remember a guard. */}
      <div className={styles.body}>{open && children}</div>
    </dialog>
  );
}
