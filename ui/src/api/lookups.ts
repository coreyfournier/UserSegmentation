import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from './client';
import type { LookupTable, Snapshot } from './types';

export function useLookups() {
  return useQuery({
    queryKey: ['lookups'],
    queryFn: () => apiFetch<LookupTable[]>('/v1/admin/lookups'),
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
