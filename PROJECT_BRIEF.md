# FleetForge Project Brief

FleetForge is a firmware rollout service for managed edge devices. Release managers define target devices, rollout capacity, retry policy, pause windows, and rollback thresholds. Edge agents register device capabilities, claim leased work, and report preflight, installation, and confirmation outcomes.

The service persists domain events to append-only JSONL and maintains an atomic snapshot for restart recovery. The scheduling path combines release capacity, task state, active leases, retry backoff, and device ownership. HTTP and CLI entry points use the same service and durable store, so smoke and acceptance checks exercise the production state flow.

Primary runtime commands:

```text
go run ./cmd/fleetforge -mode serve -data ./data -addr :8080
go run ./cmd/fleetforge smoke --data ./data
```

The smoke workflow creates a release, registers a device, claims and completes its task, reopens the durable store, and verifies the recovered release is completed.
