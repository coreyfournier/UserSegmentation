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
    <dialog ref={ref} className={styles.dialog} onClose={onClose}>
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
