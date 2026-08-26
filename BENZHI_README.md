# BENZHI Validation Entry

FleetForge 是一个用于边缘设备固件发布、分批调度与回滚的 Go 服务，服务于企业设备团队的固件交付业务。

FleetForge is a Go 1.23 service with no third-party module dependencies. The validation container builds the production CLI from `cmd/fleetforge`; runtime state is written beneath the directory supplied with `--data`.

Build the pinned Linux validation image with:

```text
sh ./build_benzhi_docker.sh
```

The resulting `fleetforge-benzhi:go1.23.12` image contains the compiled `fleetforge` binary. Repository validation remains `go test ./...`, `go vet ./...`, `go build ./...`, plus the durable smoke workflow documented in `README.md`.
