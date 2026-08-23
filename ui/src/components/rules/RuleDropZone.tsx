import { useState } from 'react';
import { useRuleDrag } from './RuleDragContext';
import { canDrop, type RulePath } from './ruleTree';
import styles from './RuleDropZone.module.css';

interface Props {
  /** Insertion point: the parent path plus the index the node would take. */
  path: RulePath;
}

/**
 * A landing strip between two sibling rules. One is rendered before every rule
 * and after the last, at every level, so a node can be dropped anywhere the
 * tree allows — including as the first child of an empty group.
 */
export default function RuleDropZone({ path }: Props) {
  const { dragPath, rootRules, dropAt } = useRuleDrag();
  const [over, setOver] = useState(false);

  // Drives the highlight only. Drop handling never consults this: a drag can
  // start and finish before React re-renders, so legality is decided when the
  // drop actually happens (moveRule rejects anything invalid).
  const active = dragPath !== null && canDrop(rootRules, dragPath, path);

  const className = [styles.zone, active && styles.active, active && over && styles.over]
    .filter(Boolean)
    .join(' ');

  return (
    <div
      className={className}
      data-drop-path={path.join('.')}
      onDragOver={(e) => {
        // Must preventDefault for the browser to treat this as a drop target.
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        if (active && !over) setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => {
        e.preventDefault();
        e.stopPropagation();
        setOver(false);
        dropAt(path);
      }}
    />
  );
}
