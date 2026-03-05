# Outcome Attribution

Track which segment assignments led to specific user outcomes (conversions, purchases, engagement). This is essential for measuring the effectiveness of A/B experiments and targeted promotions.

## Problem

The evaluate endpoint returns segment assignments, but without attribution logging there's no way to answer questions like:

- Did the `new-checkout` experiment increase conversions?
- Which tier drives the most revenue?
- Did the summer promotion actually engage the intended audience?

## Current State

The `/v1/evaluate` response already contains all the data needed for attribution:

```json
{
  "subject_key": "user-pro-001",
  "layers": {
    "base-tier": { "segment": "pro", "strategy": "static", "reason": "static:mapping" },
    "experiments": { "segment": "new-checkout", "strategy": "percentage", "reason": "percentage:checkout-v2" }
  },
  "evaluated_at": "2026-03-04T21:30:45Z",
  "duration_us": 38
}
```

No changes are needed to the evaluation response — the client just needs to carry this context forward.

## Approaches

### 1. Client-Side Event Logging (Start Here)

The client attaches segment assignments to every analytics event it emits. No changes to this service required.

```js
const evalResponse = await fetch("/v1/evaluate", { ... }).then(r => r.json());

// Attach segments to all downstream analytics events
analytics.track("purchase", {
  user: "user-pro-001",
  amount: 49.99,
  segments: {
    "base-tier": "pro",
    "experiments": "new-checkout"
  },
  evaluated_at: evalResponse.evaluated_at
});
```

**Pros:** Simple, no backend changes, works with any analytics provider (Segment, Mixpanel, Amplitude, BigQuery).

**Cons:** Trusts the client to report accurately. No server-side record of who was exposed.

### 2. Exposure Logging Endpoint (Add for A/B Testing)

Add a `POST /v1/expose` endpoint that records when a user is actually exposed to a segment assignment. This is critical for valid A/B test analysis — you need to distinguish between users who were *assigned* a variant and users who actually *saw* it.

```json
POST /v1/expose
{
  "subject_key": "user-pro-001",
  "layer": "experiments",
  "segment": "new-checkout",
  "reason": "percentage:checkout-v2",
  "context": { "page": "/checkout" }
}
```

The endpoint writes to a durable log (Kafka topic, append-only file, or database table). The data pipeline joins exposure events with outcome events for analysis.

#### Suggested Schema (exposure_log table)

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Unique exposure event ID |
| subject_key | string | User identifier |
| layer | string | Layer name |
| segment | string | Assigned segment |
| strategy | string | How the segment was determined |
| reason | string | Specific rule/mapping that matched |
| config_version | int | Snapshot version at time of evaluation |
| exposed_at | timestamp | When the user was exposed |

#### Deduplication

The same user hitting the same page repeatedly shouldn't create duplicate exposures. Options:
- Deduplicate at write time (check if subject+layer+segment already logged within a window)
- Deduplicate at query time in the data warehouse (simpler, preferred)

### 3. Assignment Token (Add for Zero-Trust Attribution)

Return a signed token encoding the full assignment. The client passes this opaque token with every API call, and the backend can verify and log it server-side without trusting the client.

```json
{
  "subject_key": "user-pro-001",
  "layers": { ... },
  "assignment_token": "eyJhbGciOiJIUzI1NiJ9..."
}
```

**Pros:** Server-side verification, tamper-proof, single token replaces per-event segment attachment.

**Cons:** More complex, requires token signing/verification, token size grows with number of layers.

## Recommended Rollout Order

1. **Phase 1 — Client-side logging.** Zero backend work. Have the client attach `layers` from the evaluate response to all analytics events. Start building attribution dashboards.

2. **Phase 2 — Exposure logging endpoint.** Implement `POST /v1/expose` with a simple append-only store (file or database). Required before drawing statistical conclusions from A/B tests.

3. **Phase 3 — Assignment tokens.** Only if operating in a zero-trust environment or need server-side attribution without relying on client reporting.

## Implementation Notes

- Exposure logging should be **fire-and-forget** from the client's perspective (non-blocking, best-effort). Don't let logging failures break the user experience.
- Consider batching exposure events if volume is high.
- The `config_version` field (from `Snapshot.Version`) is important for attribution — it tells you which rules were in effect when the assignment was made.
- For A/B test analysis, the standard approach is: `exposure_log JOIN outcome_events ON subject_key WHERE layer = 'experiments'`, then compare conversion rates across segments.
