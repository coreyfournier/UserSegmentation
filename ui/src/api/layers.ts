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
    mutationFn: ({ key, layer }: { key: string; layer: Partial<Layer> }) =>
      apiFetch<Snapshot>(`/v1/admin/layers/${encodeURIComponent(key)}`, {
        method: 'PUT',
        body: JSON.stringify(layer),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['layers'] }),
  });
}

export function useDeleteLayer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (key: string) =>
      apiFetch<Snapshot>(`/v1/admin/layers/${encodeURIComponent(key)}`, {
        method: 'DELETE',
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['layers'] }),
  });
}

/** One reference a key change would rewrite. Mirrors application.RekeyRef. */
export interface RekeyRef {
  layer: string;
  segment?: string;
  rule?: string;
  where: 'dependsOn' | 'condition' | 'message' | 'errorMessage' | 'defaultMessage' | 'output';
}

/**
 * What changing a layer's key would rewrite.
 *
 * Asked of the server rather than computed here, because the same code that
 * answers this performs the rewrite — so the preview cannot claim one thing and
 * the save do another. Disabled until the key actually differs and is valid.
 */
export function useRekeyPreview(from: string, to: string, enabled: boolean) {
  return useQuery({
    queryKey: ['rekey-preview', from, to],
    queryFn: () =>
      apiFetch<{ from: string; to: string; references: RekeyRef[] }>(
        `/v1/admin/layers/${encodeURIComponent(from)}/rekey-preview?to=${encodeURIComponent(to)}`
      ),
    enabled: enabled && !!from && !!to && from !== to,
  });
}
