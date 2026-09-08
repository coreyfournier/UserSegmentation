import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch, ifMatch } from './client';
import type { Segment, Snapshot } from './types';

export function useSegments(layerKey: string) {
  return useQuery({
    queryKey: ['segments', layerKey],
    queryFn: () =>
      apiFetch<Segment[]>(`/v1/admin/layers/${encodeURIComponent(layerKey)}/segments`),
    enabled: !!layerKey,
  });
}

export function useCreateSegment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ layerKey, segment, revision }: { layerKey: string; segment: Segment; revision?: number }) =>
      apiFetch<Snapshot>(
        `/v1/admin/layers/${encodeURIComponent(layerKey)}/segments`,
        { method: 'POST', headers: ifMatch(revision), body: JSON.stringify(segment) }
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['layers'] });
      qc.invalidateQueries({ queryKey: ['segments'] });
    },
  });
}

export function useUpdateSegment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      layerKey,
      segId,
      segment,
      revision,
    }: {
      layerKey: string;
      segId: string;
      segment: Segment;
      // The owning layer's revision: a segment write advances it, so it is
      // what guards the write. Omitted to save unconditionally.
      revision?: number;
    }) =>
      apiFetch<Snapshot>(
        `/v1/admin/layers/${encodeURIComponent(layerKey)}/segments/${encodeURIComponent(segId)}`,
        { method: 'PUT', headers: ifMatch(revision), body: JSON.stringify(segment) }
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['layers'] });
      qc.invalidateQueries({ queryKey: ['segments'] });
    },
  });
}

export function useDeleteSegment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ layerKey, segId }: { layerKey: string; segId: string }) =>
      apiFetch<Snapshot>(
        `/v1/admin/layers/${encodeURIComponent(layerKey)}/segments/${encodeURIComponent(segId)}`,
        { method: 'DELETE' }
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['layers'] });
      qc.invalidateQueries({ queryKey: ['segments'] });
    },
  });
}
