import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from './client';
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
    mutationFn: ({ layerKey, segment }: { layerKey: string; segment: Segment }) =>
      apiFetch<Snapshot>(
        `/v1/admin/layers/${encodeURIComponent(layerKey)}/segments`,
        { method: 'POST', body: JSON.stringify(segment) }
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
    }: {
      layerKey: string;
      segId: string;
      segment: Segment;
    }) =>
      apiFetch<Snapshot>(
        `/v1/admin/layers/${encodeURIComponent(layerKey)}/segments/${encodeURIComponent(segId)}`,
        { method: 'PUT', body: JSON.stringify(segment) }
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
