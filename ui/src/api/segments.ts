import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from './client';
import type { Segment, Snapshot } from './types';

export function useSegments(layerName: string) {
  return useQuery({
    queryKey: ['segments', layerName],
    queryFn: () =>
      apiFetch<Segment[]>(`/v1/admin/layers/${encodeURIComponent(layerName)}/segments`),
    enabled: !!layerName,
  });
}

export function useCreateSegment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ layerName, segment }: { layerName: string; segment: Segment }) =>
      apiFetch<Snapshot>(
        `/v1/admin/layers/${encodeURIComponent(layerName)}/segments`,
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
      layerName,
      segId,
      segment,
    }: {
      layerName: string;
      segId: string;
      segment: Segment;
    }) =>
      apiFetch<Snapshot>(
        `/v1/admin/layers/${encodeURIComponent(layerName)}/segments/${encodeURIComponent(segId)}`,
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
    mutationFn: ({ layerName, segId }: { layerName: string; segId: string }) =>
      apiFetch<Snapshot>(
        `/v1/admin/layers/${encodeURIComponent(layerName)}/segments/${encodeURIComponent(segId)}`,
        { method: 'DELETE' }
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['layers'] });
      qc.invalidateQueries({ queryKey: ['segments'] });
    },
  });
}
