package service

import (
	"context"
	"time"
)

// LingjingTask 视频生成任务领域对象（service 层，非 Ent 对象）。
type LingjingTask struct {
	ID           int64
	GenTaskID    string
	TaskType     string // "text2video" / "image2video"
	Status       string
	ErrorMessage string
	ResultURL    string
	RequestBody  map[string]any
	UserID       int64
	APIKeyID     int64
	AccountID    int64
	GroupID      *int64
	Model        string
	Duration     string
	Mode         string
	Cost         float64
	Billed       bool
	PollAttempts int
	StartedAt    *time.Time
	FinishedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// 任务状态常量
const (
	LingjingTaskStatusPending    = "pending"
	LingjingTaskStatusProcessing = "processing"
	LingjingTaskStatusSucceeded  = "succeeded"
	LingjingTaskStatusFailed     = "failed"
)

// 任务类型常量
const (
	LingjingTaskTypeText2Video  = "text2video"
	LingjingTaskTypeImage2Video = "image2video"
)

// LingjingTaskRepository 视频任务存储接口。
type LingjingTaskRepository interface {
	Create(ctx context.Context, task *LingjingTask) error
	GetByID(ctx context.Context, id int64) (*LingjingTask, error)
	GetByGenTaskID(ctx context.Context, genTaskID string) (*LingjingTask, error)
	// ListPendingTasks 查询所有待轮询任务（pending/processing，未超限），供后台 runner 使用
	ListPendingTasks(ctx context.Context, maxPollAttempts, limit int) ([]*LingjingTask, error)
	UpdateStatus(ctx context.Context, id int64, status, resultURL, errMsg string, finishedAt *time.Time) error
	IncrementPollAttempts(ctx context.Context, id int64) error
	MarkBilled(ctx context.Context, id int64, cost float64) error
}
