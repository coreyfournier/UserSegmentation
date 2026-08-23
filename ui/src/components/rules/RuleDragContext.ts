import { createContext, useContext } from 'react';
import type { Rule } from '../../api/types';
import type { RulePath } from './ruleTree';

export interface RuleDragValue {
  /** Path of the node being dragged, or null when no drag is in progress. */
  dragPath: RulePath | null;
  /** The whole tree — a drop target needs it to check the move is legal. */
  rootRules: Rule[];
  beginDrag: (path: RulePath) => void;
  endDrag: () => void;
  /** Completes the drag by inserting the dragged node at `to`. */
  dropAt: (to: RulePath) => void;
}

export const RuleDragContext = createContext<RuleDragValue | null>(null);

/**
 * Drag state is shared through context rather than passed down, because the
 * rule tree is recursive and a drop target may sit at any depth.
 */
export function useRuleDrag(): RuleDragValue {
  const value = useContext(RuleDragContext);
  if (!value) {
    throw new Error('Rule drag state is only available inside a RuleTreeBuilder');
  }
  return value;
}
