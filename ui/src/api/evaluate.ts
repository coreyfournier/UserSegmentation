import { useMutation } from '@tanstack/react-query';
import { apiFetch } from './client';
import type { EvaluateRequest, EvaluateResponse } from './types';

export function useEvaluate() {
  return useMutation({
    mutationFn: (req: EvaluateRequest) =>
      apiFetch<EvaluateResponse>('/v1/evaluate', {
        method: 'POST',
        body: JSON.stringify(req),
      }),
  });
}
