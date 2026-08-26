package domain

func ValidTransition(from TaskStatus, to TaskStatus) bool {
	switch from {
	case TaskQueued:
		return to == TaskLeased
	case TaskLeased:
		return to == TaskInstalling || to == TaskRejected
	case TaskInstalling:
		return to == TaskAwaiting || to == TaskCompleted || to == TaskFailed
	case TaskAwaiting:
		return to == TaskCompleted || to == TaskFailed
	case TaskRejected:
		return to == TaskQueued || to == TaskFailed
	case TaskCompleted, TaskFailed:
		return to == TaskRollbackQueued
	case TaskRollbackQueued:
		return to == TaskRollingBack
	case TaskRollingBack:
		return to == TaskRolledBack || to == TaskFailed
	}
	return false
}
