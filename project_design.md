# FleetForge Design

FleetForge is a durable rollout orchestration system that coordinates release managers, edge agents, task leases, retry policy, and restart recovery.

## Components

- `internal/httpapi` maps release-manager and edge-agent requests to the service layer.
- `internal/service` owns command validation, event emission, projections, leases, retries, and release progress.
- `internal/scheduler` evaluates capacity and chooses runnable work from consistent task snapshots.
- `internal/store` appends events, atomically replaces snapshots, and exposes durable restart recovery.
- `internal/recovery` validates and replays persisted state after restart.
- `internal/audit` and `internal/health` expose downstream operational evidence.

## State Flow

A release creates one task per target device. Tasks progress from queued through leased, preflight, installing, awaiting confirmation, and completed. Rejections and transient failures enter retry backoff; policy failures can trigger rollback. Every accepted transition is represented by an event and reflected in the snapshot used by queries, scheduling, audit, and recovery.

## Consistency Boundaries

Task claiming performs capacity selection, eligibility validation, lease creation, event persistence, and projection update as one serialized service operation. Snapshot readers receive copied state so scheduler calculations do not race with event application. Durable recovery reconstructs releases, tasks, leases, retries, and audit records before new commands are accepted.

## Validation

Host and Linux container checks run `go test ./...`, `go vet ./...`, and `go build ./...`. The smoke command verifies a complete production workflow and restart recovery using a fresh data directory.
