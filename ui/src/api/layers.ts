import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch, ifMatch } from './client';
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
    // `revision` is the layer the caller read. Omitting it saves unconditionally,
    // which is how an overwrite is expressed after a conflict is accepted.
    mutationFn: ({ key, layer, revision }: { key: string; layer: Partial<Layer>; revision?: number }) =>
      apiFetch<Snapshot>(`/v1/admin/layers/${encodeURIComponent(key)}`, {
        method: 'PUT',
        headers: ifMatch(revision),
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

/**
 * The revision a write left behind, read out of the snapshot the write
 * returned.
 *
 * Every admin write responds with the whole snapshot, so a chain of writes can
 * carry the guard forward without assuming how the store advances the number.
 * Relying on "it goes up by one" would tie this client to one store's
 * implementation of the token, which is the thing the port exists to hide.
 *
 * Returns undefined when the layer is not in the snapshot — a key change, or a
 * delete — which the caller reads as "no expectation to carry".
 */
export function revisionOf(snap: Snapshot, key: string): number | undefined {
  return snap.layers?.find((l) => l.key === key)?.revision;
}
