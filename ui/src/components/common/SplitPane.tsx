import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import styles from './SplitPane.module.css';

interface Props {
  /** The flexible column. */
  children: ReactNode;
  /** The fixed-width column, whose width the divider changes. */
  side: ReactNode;
  /** Below this viewport width the two stack and the divider is not rendered. */
  minViewport?: number;
  defaultWidth?: number;
  minWidth?: number;
  maxWidth?: number;
  /** localStorage key for the remembered width. Omit to not remember it. */
  storageKey?: string;
}

/**
 * Two columns with a divider you can drag.
 *
 * The side column carries the width because it is the one with a natural size
 * — a panel is "about this wide" in a way that a document column is not — and
 * because pinning it keeps the flexible column responsive to the window.
 *
 * Width is remembered in localStorage: it is a per-viewer convenience, worth
 * nothing to anyone else and not worth a round trip. Every access is guarded,
 * since a private window or blocked site data makes the accessor itself throw.
 */
export default function SplitPane({
  children,
  side,
  minViewport = 1200,
  defaultWidth = 380,
  minWidth = 280,
  maxWidth = 720,
  storageKey,
}: Props) {
  const clamp = useCallback(
    (w: number) => Math.min(maxWidth, Math.max(minWidth, w)),
    [minWidth, maxWidth],
  );

  const [width, setWidth] = useState(() => {
    if (!storageKey) return defaultWidth;
    try {
      const saved = window.localStorage.getItem(storageKey);
      const n = saved ? Number(saved) : NaN;
      return Number.isFinite(n) ? Math.min(maxWidth, Math.max(minWidth, n)) : defaultWidth;
    } catch {
      return defaultWidth;
    }
  });

  // Whether there is room for two columns at all. Watched rather than read
  // once, so the divider appears and disappears as the window is resized
  // instead of only matching the width the page was opened at.
  const [wide, setWide] = useState(
    () => typeof window !== 'undefined' && window.innerWidth >= minViewport,
  );
  useEffect(() => {
    const mq = window.matchMedia(`(min-width: ${minViewport}px)`);
    const sync = () => setWide(mq.matches);
    sync();
    mq.addEventListener('change', sync);
    return () => mq.removeEventListener('change', sync);
  }, [minViewport]);

  const rootRef = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState(false);

  useEffect(() => {
    if (!dragging) return;

    const onMove = (e: PointerEvent) => {
      const rect = rootRef.current?.getBoundingClientRect();
      if (!rect) return;
      // The side column runs from the pointer to the right edge, so its width
      // is what remains — this is what makes the divider track the cursor
      // rather than drift by however wide the divider itself is.
      setWidth(clamp(rect.right - e.clientX));
    };
    const onUp = () => setDragging(false);

    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
    // Suppress selection while dragging, or the whole page highlights as the
    // pointer sweeps across it.
    const previous = document.body.style.userSelect;
    document.body.style.userSelect = 'none';
    return () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      document.body.style.userSelect = previous;
    };
  }, [dragging, clamp]);

  useEffect(() => {
    if (!storageKey) return;
    try {
      window.localStorage.setItem(storageKey, String(width));
    } catch {
      // A remembered width is a nicety; losing it changes nothing.
    }
  }, [storageKey, width]);

  if (!wide) {
    return (
      <div className={styles.stacked}>
        {children}
        {side}
      </div>
    );
  }

  return (
    <div
      ref={rootRef}
      className={styles.split}
      style={{ gridTemplateColumns: `minmax(0, 1fr) auto ${width}px` }}
    >
      <div className={styles.main}>{children}</div>
      {/* A separator rather than a button: it has a value and an orientation,
          and arrow keys move it, which is the whole keyboard story here. */}
      <div
        className={`${styles.divider} ${dragging ? styles.dragging : ''}`}
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize the tests panel"
        aria-valuenow={width}
        aria-valuemin={minWidth}
        aria-valuemax={maxWidth}
        tabIndex={0}
        onPointerDown={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDoubleClick={() => setWidth(defaultWidth)}
        onKeyDown={(e) => {
          if (e.key === 'ArrowLeft') setWidth((w) => clamp(w + 24));
          else if (e.key === 'ArrowRight') setWidth((w) => clamp(w - 24));
          else return;
          e.preventDefault();
        }}
        title="Drag to resize — double-click to reset"
      >
        <span className={styles.grip} />
      </div>
      <aside className={styles.side}>{side}</aside>
    </div>
  );
}
