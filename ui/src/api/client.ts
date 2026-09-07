import type { ConflictDetail } from './types';

const BASE = '';

export class ApiError extends Error {
  status: number;
  /**
   * The parsed body of a 409, when the server sent one.
   *
   * Kept rather than flattened to a message because a conflict is the one
   * failure the UI has to *act* on: offering "overwrite" needs the revision to
   * retry with, and saying how stale the copy is needs the timestamp. A string
   * would make the dialog parse prose back into numbers.
   */
  conflict?: ConflictDetail;

  constructor(status: number, message: string, conflict?: ConflictDetail) {
    super(message);
    this.status = status;
    this.conflict = conflict;
  }
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  });
  const body = await res.json();
  if (!res.ok) {
    throw new ApiError(
      res.status,
      body.error || res.statusText,
      body?.conflict ? (body as ConflictDetail) : undefined,
    );
  }
  return body as T;
}

/**
 * `If-Match` for an optimistic write, or nothing when the caller has no
 * expectation — which the server reads as "do not check".
 *
 * Quoted, because that is what an ETag is. The server tolerates the bare form
 * for hand-written requests, but there is no reason for this client to rely on
 * that leniency.
 */
export function ifMatch(revision?: number): Record<string, string> {
  return revision === undefined ? {} : { 'If-Match': `"${revision}"` };
}
