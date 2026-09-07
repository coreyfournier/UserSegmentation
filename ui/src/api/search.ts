import { useQuery, keepPreviousData } from '@tanstack/react-query';
import { apiFetch } from './client';
import type { SearchResult } from './types';

/**
 * Runs a query against the server rather than filtering the layer list in the
 * browser. Filtering locally would work today — the whole config is already
 * cached — and would stop working the moment the config outgrows one response.
 * The endpoint is the contract; what is behind it (a snapshot scan now, a
 * database later) is not this component's concern.
 *
 * Disabled for a blank query, so clearing the box costs no request and the
 * caller falls back to the unfiltered list it already has.
 */
export function useSearch(query: string) {
  const trimmed = query.trim();
  return useQuery({
    queryKey: ['search', trimmed],
    queryFn: () => apiFetch<SearchResult>(`/v1/admin/search?q=${encodeURIComponent(trimmed)}`),
    enabled: trimmed.length > 0,
    // Keep showing the previous matches while the next query is in flight, so
    // the list does not blank out on every keystroke.
    placeholderData: keepPreviousData,
  });
}
