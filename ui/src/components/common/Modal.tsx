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
        <button className="btn-ghost btn-sm" onClick={onClose}>X</button>
      </div>
      <div className={styles.body}>{children}</div>
    </dialog>
  );
}
