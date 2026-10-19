package api

import "time"

// Owner is the person a task is for.
type Owner struct {
	ID    int32  `json:"id"`
	Email string `json:"email,omitempty"`
}

// CreateTaskRequest is the body of a new task.
type CreateTaskRequest struct {
	Title         string     `json:"title"`
	Note          string     `json:"note,omitempty"`
	Priority      string     `json:"priority"`
	EstimateHours int32      `json:"estimate_hours,omitempty"`
	Labels        []string   `json:"labels,omitempty"`
	DueAt         *time.Time `json:"due_at,omitempty"`
	Owner         Owner      `json:"owner"`
	Draft         bool       `json:"-"`
}

// ArchiveRequest is what an archive decodes.
type ArchiveRequest struct {
	TaskID int32  `json:"task_id"`
	Reason string `json:"reason,omitempty"`
}
