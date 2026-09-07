import { useEffect, useState } from 'react';

/**
 * The value, settling after `delay` ms of no further changes.
 *
 * Used to keep a per-keystroke input from becoming a per-keystroke request.
 * Typing "checklist" would otherwise be nine round trips whose first eight
 * answers are thrown away.
 */
export function useDebounced<T>(value: T, delay = 200): T {
  const [settled, setSettled] = useState(value);

  useEffect(() => {
    const id = setTimeout(() => setSettled(value), delay);
    return () => clearTimeout(id);
  }, [value, delay]);

  return settled;
}
