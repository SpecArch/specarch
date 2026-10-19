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

// AssignRequest is what an assignment decodes; its optional fields are
// checked only when they are there.
type AssignRequest struct {
	TaskID int32  `json:"task_id"`
	Email  string `json:"email,omitempty"`
	DueAt  string `json:"due_at,omitempty"`
	Count  int32  `json:"count,omitempty"`
}

// RestoreRequest is what a restore decodes, by the names its registration
// declares.
type RestoreRequest struct {
	TaskID int32  `json:"task_id"`
	Reason string `json:"reason,omitempty"`
}

// LabelRequest is what a labelling decodes.
type LabelRequest struct {
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
}
