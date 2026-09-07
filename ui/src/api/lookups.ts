import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from './client';
import type { LookupTable, Snapshot } from './types';

export function useLookups() {
  return useQuery({
    queryKey: ['lookups'],
    queryFn: () => apiFetch<LookupTable[]>('/v1/admin/lookups'),
    // Overrides the client-wide default (off). A schema's lookup link opens the
    // table in a new tab precisely so entries can be added there; without this
    // the author returns to a form still showing the table as it was, and a key
    // they just created is missing from the value picker.
    refetchOnWindowFocus: true,
  });
}

export function useCreateLookup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (table: Omit<LookupTable, 'id'>) =>
      apiFetch<Snapshot>('/v1/admin/lookups', {
        method: 'POST',
        body: JSON.stringify(table),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['lookups'] }),
  });
}

export function useUpdateLookup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, table }: { id: string; table: LookupTable }) =>
      apiFetch<Snapshot>(`/v1/admin/lookups/${encodeURIComponent(id)}`, {
        method: 'PUT',
        body: JSON.stringify(table),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['lookups'] }),
  });
}

export function useDeleteLookup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<Snapshot>(`/v1/admin/lookups/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['lookups'] }),
  });
}
