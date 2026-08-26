# FleetForge

FleetForge 是企业设备团队使用的固件分批发布编排服务。发布经理创建批次，指定设备、批次大小、暂停窗口和回滚阈值；边缘代理注册设备、领取租约任务并上报 preflight/install/confirmation 结果。

## 角色与动作

- 发布经理：创建、暂停、恢复、回滚和查询发布批次。
- 边缘代理：注册能力和当前固件，领取/拒绝/完成任务。
- 调度器：按并发限制和批次策略推进 queued→preflight→installing→awaiting-confirmation→completed，失败时进入 rollback-pending/rolled-back 或 failed。

## 持久化与恢复

事件以追加 JSONL 写入 `data/events.log`，快照写入 `data/snapshot.json`。每次命令先应用领域状态再持久化事件和快照；恢复器读取快照后重放事件，忽略损坏的尾记录并执行一致性检查。租约、重试次数和未完成动作都在快照中保存，重启不会重复派发导致状态倒退。

## HTTP 与 CLI

`go run ./cmd/fleetforge -mode serve -data ./data -addr :8080` 启动 HTTP 服务。数据目录会创建 `events.jsonl` 和原子替换的 `snapshot.json`；生产环境应将该目录置于持久卷。核心入口：

- `POST /v1/releases`, `GET /v1/releases/{id}`, `POST /v1/releases/{id}/pause|resume|rollback`
- `POST /v1/devices`, `POST /v1/agents/{device}/claim`, `POST /v1/tasks/{task}/ack|reject|complete`
- `GET /healthz`, `GET /v1/audit`

CLI：`go run ./cmd/fleetforge create-release --version 2.0.0 --devices dev-a,dev-b --batch-size 1`。可用 `go run ./cmd/fleetforge smoke --data ./data` 执行一次端到端 smoke；命令会使用 DurableStore 恢复并输出完成计数，不会创建仓库内临时目录。旧版 `-mode smoke -data ./data` 形式仍兼容。

## 运行与验证

```text
go test ./...
go vet ./...
go build ./...
go run ./cmd/fleetforge smoke --data ./data
```

预期 smoke 输出包含 `completed=1` 和恢复后的批次状态。健康检查包括 `/healthz`、`/readyz`、`/v1/metrics` 和 `/v1/storage/check`；审计摘要位于 `/v1/audit`。调度器使用可替换时钟、ID 生成器和 Store，便于测试租约回收、退避和恢复路径。
