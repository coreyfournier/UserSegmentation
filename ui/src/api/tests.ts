import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiFetch } from './client';
import type { SavedTest, Snapshot } from './types';

/**
 * Saved evaluation inputs, filed per layer.
 *
 * They live in the config snapshot beside lookup tables, so every mutation
 * invalidates the layer list too — a save bumps the snapshot the layers page is
 * reading, and a stale list would show the version before this write.
 */
export function useTests(layerKey?: string) {
  return useQuery({
    queryKey: ['tests', layerKey ?? ''],
    queryFn: () =>
      apiFetch<SavedTest[]>(
        layerKey ? `/v1/admin/tests?layer=${encodeURIComponent(layerKey)}` : '/v1/admin/tests'
      ),
  });
}

function useTestMutation<TArgs>(fn: (args: TArgs) => Promise<Snapshot>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['tests'] });
      qc.invalidateQueries({ queryKey: ['layers'] });
    },
  });
}

export function useCreateTest() {
  return useTestMutation((test: Omit<SavedTest, 'id'>) =>
    apiFetch<Snapshot>('/v1/admin/tests', { method: 'POST', body: JSON.stringify(test) })
  );
}

export function useUpdateTest() {
  return useTestMutation(({ id, test }: { id: string; test: SavedTest }) =>
    apiFetch<Snapshot>(`/v1/admin/tests/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(test),
    })
  );
}

export function useDeleteTest() {
  return useTestMutation((id: string) =>
    apiFetch<Snapshot>(`/v1/admin/tests/${encodeURIComponent(id)}`, { method: 'DELETE' })
  );
}
