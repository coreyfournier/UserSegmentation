import { useNavigate } from 'react-router-dom';
import type { Layer } from '../../api/types';
import styles from './LayerRail.module.css';

interface Props {
  layers: Layer[];
  selected: string | null;
  onSelect: (name: string) => void;
  query: string;
  onQueryChange: (q: string) => void;
  /** Matched segment ids per layer, keyed by layer name. Empty when not searching. */
  matches: Map<string, string[]>;
  searching: boolean;
  /** The server stopped at its limit; the list below is partial. */
  truncated: boolean;
}

/**
 * The left rail: every layer at once, in dependency order.
 *
 * The whole point is that this list does not grow the page — it stays put
 * while the detail pane changes, so picking a layer never costs a scroll. It
 * carries only what distinguishes one row from another: the name, how many
 * segments it holds, and a marker for a layer that waits on others.
 *
 * While a query is active the rail narrows to what matched, and a layer that
 * is only in the list because one of its segments matched shows those segments
 * beneath it — otherwise the row would be unexplained, and the author would
 * have to open the layer to find out why it is there.
 */
export default function LayerRail({
  layers,
  selected,
  onSelect,
  query,
  onQueryChange,
  matches,
  searching,
  truncated,
}: Props) {
  const navigate = useNavigate();

  return (
    <div className={styles.rail}>
      <div className={styles.searchRow}>
        <input
          className={styles.search}
          value={query}
          onChange={(e) => onQueryChange(e.target.value)}
          placeholder="Search layers and segments"
          aria-label="Search layers and segments"
          type="search"
        />
      </div>

      {searching && layers.length === 0 && (
        <p className={styles.note}>No layers or segments match.</p>
      )}
      {truncated && <p className={styles.note}>Showing the first matches only.</p>}

      <nav aria-label="Layers">
        {layers.map((layer) => {
          const deps = layer.dependsOn ?? [];
          const matched = matches.get(layer.name) ?? [];
          return (
            <div key={layer.name}>
              <button
                type="button"
                className={`${styles.row} ${layer.name === selected ? styles.active : ''}`}
                onClick={() => onSelect(layer.name)}
                aria-current={layer.name === selected ? 'true' : undefined}
                title={deps.length ? `Runs after ${deps.join(', ')}` : undefined}
              >
                <span className={styles.name}>{layer.name}</span>
                {deps.length > 0 && (
                  <span className={styles.dep} aria-label={`runs after ${deps.join(', ')}`}>↳</span>
                )}
                <span className={styles.count}>{layer.segments.length}</span>
              </button>
              {matched.map((segId) => (
                <button
                  key={segId}
                  type="button"
                  className={styles.segRow}
                  onClick={() =>
                    navigate(
                      `/layers/${encodeURIComponent(layer.name)}/segments/${encodeURIComponent(segId)}`
                    )
                  }
                  title={`Open segment ${segId}`}
                >
                  {segId}
                </button>
              ))}
            </div>
          );
        })}
      </nav>
    </div>
  );
}
