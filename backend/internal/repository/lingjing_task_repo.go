package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/lingjingtask"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type lingjingTaskRepository struct {
	client *dbent.Client
}

// NewLingjingTaskRepository 创建仓储实例。
func NewLingjingTaskRepository(client *dbent.Client) service.LingjingTaskRepository {
	return &lingjingTaskRepository{client: client}
}

func (r *lingjingTaskRepository) Create(ctx context.Context, task *service.LingjingTask) error {
	client := clientFromContext(ctx, r.client)
	row, err := client.LingjingTask.Create().
		SetGenTaskID(task.GenTaskID).
		SetTaskType(task.TaskType).
		SetStatus(task.Status).
		SetRequestBody(task.RequestBody).
		SetUserID(task.UserID).
		SetAPIKeyID(task.APIKeyID).
		SetAccountID(task.AccountID).
		SetNillableGroupID(task.GroupID).
		SetModel(task.Model).
		SetDuration(task.Duration).
		SetMode(task.Mode).
		SetNillableStartedAt(task.StartedAt).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("lingjing_task_repo: create: %w", err)
	}
	task.ID = row.ID
	task.CreatedAt = row.CreatedAt
	task.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *lingjingTaskRepository) GetByID(ctx context.Context, id int64) (*service.LingjingTask, error) {
	row, err := r.client.LingjingTask.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("lingjing_task_repo: get by id %d: %w", id, err)
	}
	return entToServiceLingjingTask(row), nil
}

func (r *lingjingTaskRepository) GetByGenTaskID(ctx context.Context, genTaskID string) (*service.LingjingTask, error) {
	row, err := r.client.LingjingTask.Query().
		Where(lingjingtask.GenTaskID(genTaskID)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("lingjing_task_repo: get by gen_task_id %s: %w", genTaskID, err)
	}
	return entToServiceLingjingTask(row), nil
}

func (r *lingjingTaskRepository) ListPendingTasks(ctx context.Context, maxPollAttempts, limit int) ([]*service.LingjingTask, error) {
	rows, err := r.client.LingjingTask.Query().
		Where(
			lingjingtask.StatusIn(service.LingjingTaskStatusPending, service.LingjingTaskStatusProcessing),
			lingjingtask.PollAttemptsLT(maxPollAttempts),
		).
		Order(dbent.Asc(lingjingtask.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("lingjing_task_repo: list pending: %w", err)
	}
	tasks := make([]*service.LingjingTask, len(rows))
	for i, row := range rows {
		tasks[i] = entToServiceLingjingTask(row)
	}
	return tasks, nil
}

func (r *lingjingTaskRepository) UpdateStatus(ctx context.Context, id int64, status, resultURL, errMsg string, finishedAt *time.Time) error {
	client := clientFromContext(ctx, r.client)
	upd := client.LingjingTask.UpdateOneID(id).
		SetStatus(status)
	if resultURL != "" {
		upd = upd.SetResultURL(resultURL)
	}
	if errMsg != "" {
		upd = upd.SetErrorMessage(errMsg)
	}
	if finishedAt != nil {
		upd = upd.SetFinishedAt(*finishedAt)
	}
	if err := upd.Exec(ctx); err != nil {
		return fmt.Errorf("lingjing_task_repo: update status id=%d: %w", id, err)
	}
	return nil
}

func (r *lingjingTaskRepository) IncrementPollAttempts(ctx context.Context, id int64) error {
	if err := r.client.LingjingTask.UpdateOneID(id).
		AddPollAttempts(1).
		Exec(ctx); err != nil {
		return fmt.Errorf("lingjing_task_repo: increment poll attempts id=%d: %w", id, err)
	}
	return nil
}

func (r *lingjingTaskRepository) MarkBilled(ctx context.Context, id int64, cost float64) error {
	if err := r.client.LingjingTask.UpdateOneID(id).
		SetBilled(true).
		SetCost(cost).
		Exec(ctx); err != nil {
		return fmt.Errorf("lingjing_task_repo: mark billed id=%d: %w", id, err)
	}
	return nil
}

func entToServiceLingjingTask(row *dbent.LingjingTask) *service.LingjingTask {
	t := &service.LingjingTask{
		ID:           row.ID,
		GenTaskID:    row.GenTaskID,
		TaskType:     row.TaskType,
		Status:       row.Status,
		RequestBody:  row.RequestBody,
		UserID:       row.UserID,
		APIKeyID:     row.APIKeyID,
		AccountID:    row.AccountID,
		GroupID:      row.GroupID,
		Model:        row.Model,
		Duration:     row.Duration,
		Mode:         row.Mode,
		Cost:         row.Cost,
		Billed:       row.Billed,
		PollAttempts: row.PollAttempts,
		StartedAt:    row.StartedAt,
		FinishedAt:   row.FinishedAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
	if row.ErrorMessage != nil {
		t.ErrorMessage = *row.ErrorMessage
	}
	if row.ResultURL != nil {
		t.ResultURL = *row.ResultURL
	}
	return t
}
