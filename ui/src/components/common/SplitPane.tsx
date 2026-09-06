import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import styles from './SplitPane.module.css';

interface Props {
  /** The primary column. Its width is what the divider changes. */
  children: ReactNode;
  /** The companion column, which takes all remaining width. */
  side: ReactNode;
  /** Below this viewport width the two stack and the divider is not rendered. */
  minViewport?: number;
  /** Starting width of the primary column. */
  defaultWidth?: number;
  /** Floor for the primary column. */
  minWidth?: number;
  /** Width always left to the side column, whatever the primary is dragged to. */
  minSideWidth?: number;
  /** Width of the stacked layout, where one column has to stay readable alone. */
  stackedMaxWidth?: number;
  /** localStorage key for the remembered width. Omit to not remember it. */
  storageKey?: string;
}

/**
 * Two columns with a divider you can drag.
 *
 * The *primary* column carries the width and the side column takes whatever
 * is left. That is the way round it has to be for a wide monitor to be worth
 * anything: a form column has a width past which longer lines stop helping,
 * while a panel showing results has no such ceiling. Sizing the panel instead
 * and letting the form stretch would have left the extra pixels doing nothing
 * — which is exactly what a fixed panel inside a capped page was already
 * doing.
 *
 * Width is remembered in localStorage: a per-viewer convenience, worth nothing
 * to anyone else and not worth a round trip. Every access is guarded, since a
 * private window or blocked site data makes the accessor itself throw.
 */
export default function SplitPane({
  children,
  side,
  minViewport = 1200,
  defaultWidth = 900,
  minWidth = 480,
  minSideWidth = 320,
  stackedMaxWidth = 900,
  storageKey,
}: Props) {
  const rootRef = useRef<HTMLDivElement>(null);

  // The ceiling depends on how much room there is, not on a constant: on a
  // 2560px monitor the form may reasonably be 1400px wide, and on a 1280px one
  // it must not be, or the panel is squeezed to nothing.
  const clamp = useCallback(
    (w: number) => {
      const available = rootRef.current?.getBoundingClientRect().width ?? Infinity;
      const max = Math.max(minWidth, available - minSideWidth);
      return Math.min(max, Math.max(minWidth, w));
    },
    [minWidth, minSideWidth],
  );

  const [width, setWidth] = useState(() => {
    if (!storageKey) return defaultWidth;
    try {
      const saved = window.localStorage.getItem(storageKey);
      const n = saved ? Number(saved) : NaN;
      return Number.isFinite(n) ? n : defaultWidth;
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

  // A remembered width, or one from a wider window, can exceed what is
  // available now. Re-clamped whenever the room changes, so narrowing the
  // window never leaves the panel with nothing.
  useEffect(() => {
    if (!wide) return;
    const el = rootRef.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setWidth((w) => clamp(w)));
    ro.observe(el);
    return () => ro.disconnect();
  }, [wide, clamp]);

  const [dragging, setDragging] = useState(false);

  useEffect(() => {
    if (!dragging) return;

    const onMove = (e: PointerEvent) => {
      const rect = rootRef.current?.getBoundingClientRect();
      if (!rect) return;
      setWidth(clamp(e.clientX - rect.left));
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
      <div className={styles.stacked} style={{ maxWidth: stackedMaxWidth }}>
        {children}
        {side}
      </div>
    );
  }

  return (
    <div
      ref={rootRef}
      className={styles.split}
      style={{ gridTemplateColumns: `${width}px auto minmax(0, 1fr)` }}
    >
      <div className={styles.main}>{children}</div>
      {/* A separator rather than a button: it has a value and an orientation,
          and arrow keys move it, which is the whole keyboard story here. */}
      <div
        className={`${styles.divider} ${dragging ? styles.dragging : ''}`}
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize the columns"
        aria-valuenow={width}
        aria-valuemin={minWidth}
        tabIndex={0}
        onPointerDown={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDoubleClick={() => setWidth(clamp(defaultWidth))}
        onKeyDown={(e) => {
          if (e.key === 'ArrowLeft') setWidth((w) => clamp(w - 24));
          else if (e.key === 'ArrowRight') setWidth((w) => clamp(w + 24));
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
