package domain

type ReleaseStatus string

const (
	StatusQueued          ReleaseStatus = "queued"
	StatusPreflight       ReleaseStatus = "preflight"
	StatusInstalling      ReleaseStatus = "installing"
	StatusAwaiting        ReleaseStatus = "awaiting-confirmation"
	StatusCompleted       ReleaseStatus = "completed"
	StatusPaused          ReleaseStatus = "paused"
	StatusRollbackPending ReleaseStatus = "rollback-pending"
	StatusRolledBack      ReleaseStatus = "rolled-back"
	StatusFailed          ReleaseStatus = "failed"
)

type TaskStatus string

const (
	TaskQueued         TaskStatus = "queued"
	TaskLeased         TaskStatus = "leased"
	TaskInstalling     TaskStatus = "installing"
	TaskAwaiting       TaskStatus = "awaiting-confirmation"
	TaskCompleted      TaskStatus = "completed"
	TaskRejected       TaskStatus = "rejected"
	TaskFailed         TaskStatus = "failed"
	TaskRollbackQueued TaskStatus = "rollback-queued"
	TaskRollingBack    TaskStatus = "rolling-back"
	TaskRolledBack     TaskStatus = "rolled-back"
)
