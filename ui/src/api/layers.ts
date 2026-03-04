import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from './client';
import type { Layer, Snapshot } from './types';

export function useLayers() {
  return useQuery({
    queryKey: ['layers'],
    queryFn: () => apiFetch<Layer[]>('/v1/admin/layers'),
  });
}

export function useCreateLayer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (layer: Partial<Layer>) =>
      apiFetch<Snapshot>('/v1/admin/layers', {
        method: 'POST',
        body: JSON.stringify(layer),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['layers'] }),
  });
}

export function useUpdateLayer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ name, layer }: { name: string; layer: Partial<Layer> }) =>
      apiFetch<Snapshot>(`/v1/admin/layers/${encodeURIComponent(name)}`, {
        method: 'PUT',
        body: JSON.stringify(layer),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['layers'] }),
  });
}

export function useDeleteLayer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      apiFetch<Snapshot>(`/v1/admin/layers/${encodeURIComponent(name)}`, {
        method: 'DELETE',
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['layers'] }),
  });
}
