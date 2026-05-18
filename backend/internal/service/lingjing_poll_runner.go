package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
)

const (
	lingjingPollTickInterval = 5 * time.Second
	lingjingPollWorkers      = 10
	lingjingMaxPollAttempts  = 240 // 240 * 5s ≈ 20 min
	lingjingPollQueryLimit   = 50
)

// LingjingPollRunner 后台轮询已提交视频任务的状态，任务完成后触发计费。
type LingjingPollRunner struct {
	taskRepo    LingjingTaskRepository
	svc         *LingjingGatewayService
	accountRepo AccountRepository
	billingSvc  *BillingService
	billingCache *BillingCacheService

	pool   pond.Pool
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	started bool
}

// NewLingjingPollRunner 构造轮询器，Start 在 wire 中调用一次。
func NewLingjingPollRunner(
	taskRepo LingjingTaskRepository,
	svc *LingjingGatewayService,
	accountRepo AccountRepository,
	billingSvc *BillingService,
	billingCache *BillingCacheService,
) *LingjingPollRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &LingjingPollRunner{
		taskRepo:    taskRepo,
		svc:         svc,
		accountRepo: accountRepo,
		billingSvc:  billingSvc,
		billingCache: billingCache,
		pool:        pond.NewPool(lingjingPollWorkers),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动轮询 goroutine，幂等。
func (r *LingjingPollRunner) Start() {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.mu.Unlock()

	r.wg.Add(1)
	go r.loop()
}

// Stop 优雅停止，等待所有 worker 结束。
func (r *LingjingPollRunner) Stop() {
	r.cancel()
	r.wg.Wait()
	r.pool.StopAndWait()
}

func (r *LingjingPollRunner) loop() {
	defer r.wg.Done()
	ticker := time.NewTicker(lingjingPollTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.tick()
		}
	}
}

func (r *LingjingPollRunner) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tasks, err := r.taskRepo.ListPendingTasks(ctx, lingjingMaxPollAttempts, lingjingPollQueryLimit)
	if err != nil {
		slog.Error("lingjing: poll runner list tasks failed", "error", err)
		return
	}
	for _, task := range tasks {
		r.pool.TrySubmit(func() {
			r.pollTask(task)
		})
	}
}

func (r *LingjingPollRunner) pollTask(task *LingjingTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	account, err := r.accountRepo.GetByID(ctx, task.AccountID)
	if err != nil {
		slog.Error("lingjing: poll task account not found", "task_id", task.ID, "account_id", task.AccountID, "error", err)
		return
	}

	apiKey, err := r.svc.GetAPIKey(account)
	if err != nil {
		slog.Error("lingjing: poll task api key missing", "task_id", task.ID, "error", err)
		return
	}

	queryResp, err := r.svc.GetClient().QueryTaskResult(ctx, apiKey, task.GenTaskID)
	if err != nil {
		slog.Warn("lingjing: poll task query failed, incrementing attempts", "task_id", task.ID, "error", err)
		_ = r.taskRepo.IncrementPollAttempts(ctx, task.ID)
		return
	}

	now := time.Now()

	switch {
	case queryResp.IsSuccess():
		urls := queryResp.SuccessURLs()
		resultURL := ""
		if len(urls) > 0 {
			resultURL = urls[0]
		}
		if err := r.taskRepo.UpdateStatus(ctx, task.ID, LingjingTaskStatusSucceeded, resultURL, "", &now); err != nil {
			slog.Error("lingjing: poll task mark succeeded failed", "task_id", task.ID, "error", err)
			return
		}
		r.triggerBilling(ctx, task)

	case queryResp.IsFailed():
		errMsg := queryResp.Result.Result.Error
		if err := r.taskRepo.UpdateStatus(ctx, task.ID, LingjingTaskStatusFailed, "", errMsg, &now); err != nil {
			slog.Error("lingjing: poll task mark failed failed", "task_id", task.ID, "error", err)
		}

	default:
		// 仍在处理中
		_ = r.taskRepo.IncrementPollAttempts(ctx, task.ID)
		if task.Status == LingjingTaskStatusPending {
			_ = r.taskRepo.UpdateStatus(ctx, task.ID, LingjingTaskStatusProcessing, "", "", nil)
		}
	}
}

func (r *LingjingPollRunner) triggerBilling(ctx context.Context, task *LingjingTask) {
	if r.billingSvc == nil || r.billingCache == nil {
		return
	}
	dur := task.Duration
	if dur == "" {
		dur = "5"
	}
	model := fmt.Sprintf("doubao-seedance-1.5-pro-%ss", dur)

	cost, err := r.billingSvc.CalculateCost(model, UsageTokens{ImageOutputTokens: 1}, 1.0)
	if err != nil {
		slog.Warn("lingjing: billing price not found, skipping deduction",
			"task_id", task.ID, "model", model, "error", err)
		_ = r.taskRepo.MarkBilled(ctx, task.ID, 0)
		return
	}

	if cost.ActualCost > 0 {
		if err := r.billingCache.DeductBalanceCache(ctx, task.UserID, cost.ActualCost); err != nil {
			slog.Error("lingjing: deduct balance failed", "task_id", task.ID, "user_id", task.UserID, "error", err)
			return
		}
	}

	if err := r.taskRepo.MarkBilled(ctx, task.ID, cost.ActualCost); err != nil {
		slog.Error("lingjing: mark billed failed", "task_id", task.ID, "error", err)
	}
}
