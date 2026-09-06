import type { Layer } from '../../api/types';
import styles from './LayerRail.module.css';

interface Props {
  layers: Layer[];
  selected: string | null;
  onSelect: (name: string) => void;
}

/**
 * The left rail: every layer at once, in dependency order.
 *
 * The whole point is that this list does not grow the page — it stays put
 * while the detail pane changes, so picking a layer never costs a scroll. It
 * carries only what distinguishes one row from another: the name, how many
 * segments it holds, and a marker for a layer that waits on others. Everything
 * else belongs in the pane beside it.
 */
export default function LayerRail({ layers, selected, onSelect }: Props) {
  return (
    <nav className={styles.rail} aria-label="Layers">
      {layers.map((layer) => {
        const deps = layer.dependsOn ?? [];
        return (
          <button
            key={layer.name}
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
        );
      })}
    </nav>
  );
}
