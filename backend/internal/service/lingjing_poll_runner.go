package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
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
	taskRepo     LingjingTaskRepository
	svc          *LingjingGatewayService
	accountRepo  AccountRepository
	billingSvc   *BillingService
	billingCache *BillingCacheService

	// Phase 0 P0-7：lingjing 异步计费接入上游成本快照
	// upstreamCostResolver 可空（测试场景）；非空时 triggerBilling 在 MarkBilled 之后写一行 UsageLog
	// 异步路径**不读** USAGE_UPSTREAM_COST_ENABLED flag——异步任务计费状态由 lingjing_task 表
	// 主导，避免 poll 期间 flag 切换导致同一任务两次轮询出现 NULL/非 NULL 摇摆。
	// 详见 docs/upstream-cost-snapshot.md §4.1
	upstreamCostResolver *UpstreamCostResolver
	usageLogRepo         UsageLogRepository

	pool    pond.Pool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	started bool
}

// NewLingjingPollRunner 构造轮询器，Start 在 wire 中调用一次。
func NewLingjingPollRunner(
	taskRepo LingjingTaskRepository,
	svc *LingjingGatewayService,
	accountRepo AccountRepository,
	billingSvc *BillingService,
	billingCache *BillingCacheService,
	upstreamCostResolver *UpstreamCostResolver,
	usageLogRepo UsageLogRepository,
) *LingjingPollRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &LingjingPollRunner{
		taskRepo:             taskRepo,
		svc:                  svc,
		accountRepo:          accountRepo,
		billingSvc:           billingSvc,
		billingCache:         billingCache,
		upstreamCostResolver: upstreamCostResolver,
		usageLogRepo:         usageLogRepo,
		pool:                 pond.NewPool(lingjingPollWorkers),
		ctx:                  ctx,
		cancel:               cancel,
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
	// 按任务实际模型计费（不再硬编码 1.5-pro），使 cinema-generate-2.0 等各按自己的 lingjing.json 档计价。
	model := strings.TrimSpace(task.Model)
	if model == "" {
		model = "doubao-seedance-1.5-pro"
	}

	// task.Duration 是字符串秒数（lingjing API 输入参数原样存储），fallback 5 秒兜底。
	seconds, _ := strconv.ParseFloat(strings.TrimSpace(task.Duration), 64)
	if seconds <= 0 {
		seconds = 5
	}

	// 灵境视频按火山官方 token 公式计费（tokens=分辨率档×帧率×秒数 × lingjing.json 选档单价），
	// mode 取请求分辨率档，generateAudio 取请求音频开关。
	mode := strings.TrimSpace(task.Mode)
	generateAudio := false
	if task.RequestBody != nil {
		if v, ok := task.RequestBody["generate_audio"].(bool); ok {
			generateAudio = v
		}
	}
	cost := r.billingSvc.CalculateSeedanceVideoCost(model, mode, seconds, generateAudio, 1.0)
	if cost == nil {
		cost = &CostBreakdown{}
	}

	if cost.ActualCost > 0 {
		if err := r.billingCache.DeductBalanceCache(ctx, task.UserID, cost.ActualCost); err != nil {
			slog.Error("lingjing: deduct balance failed", "task_id", task.ID, "user_id", task.UserID, "error", err)
			return
		}
	}

	if err := r.taskRepo.MarkBilled(ctx, task.ID, cost.ActualCost); err != nil {
		slog.Error("lingjing: mark billed failed", "task_id", task.ID, "error", err)
		return
	}

	// Phase 0 P0-7：UsageLog 第 4 装配点（异步路径，feature flag 强制 ON）
	// 失败/超时任务不进入此分支（triggerBilling 仅在 success 后调用），符合"超时归待结算超时"语义
	r.writeAsyncUsageLog(ctx, task, model, seconds, cost)
}

// writeAsyncUsageLog 为 lingjing 异步任务写入一行 UsageLog（Phase 0 P0-7）。
//
// 与同步路径的 3 处装配点（gateway/openai/usage）等价，但：
//   - 不读 UsageLog 配置 flag（异步路径强制 ON，详见 §4.1）
//   - AsyncTaskID = task.GenTaskID（用于 lingjing_task 表 join 对账）
//   - CostFinalizedAt = poll_runner 触发计费时刻（不是 created_at）
//
// 失败场景：写入失败仅 log warn，不阻塞 MarkBilled 已完成的计费状态。
func (r *LingjingPollRunner) writeAsyncUsageLog(ctx context.Context, task *LingjingTask, model string, seconds float64, cost *CostBreakdown) {
	if r.usageLogRepo == nil {
		return
	}

	now := time.Now()
	usageLog := &UsageLog{
		UserID:            task.UserID,
		APIKeyID:          task.APIKeyID,
		AccountID:         task.AccountID,
		RequestID:         task.GenTaskID, // 用 gen_task_id 作为 request_id，保证唯一
		Model:             model,
		GroupID:           task.GroupID,
		VideoSeconds:      seconds,
		InputCost:         cost.InputCost,
		OutputCost:        cost.OutputCost,
		ImageOutputCost:   cost.ImageOutputCost,
		CacheCreationCost: cost.CacheCreationCost,
		CacheReadCost:     cost.CacheReadCost,
		TotalCost:         cost.TotalCost,
		ActualCost:        cost.ActualCost,
		BillingType:       BillingTypeBalance,
		Stream:            false,
		CreatedAt:         now,
	}

	billingMode := cost.BillingMode
	if billingMode == "" {
		billingMode = string(BillingModeVideo)
	}
	usageLog.BillingMode = &billingMode

	// 上游成本快照（强制 ON，不读 USAGE_UPSTREAM_COST_ENABLED）
	ApplyUpstreamCostSnapshot(
		ctx,
		usageLog,
		r.upstreamCostResolver,
		"lingjing", // provider 固定为 lingjing（fork 12 platform 常量，规范化后仍为 lingjing）
		model,
		UsageTokens{ImageOutputTokens: 1},
		now,
		true, // flag 强制 ON
	)

	// AsyncTaskID 在 helper 之外单独写入（helper 不区分同步/异步）
	taskID := task.GenTaskID
	usageLog.AsyncTaskID = &taskID

	if _, err := r.usageLogRepo.Create(ctx, usageLog); err != nil {
		slog.Warn("lingjing: write async UsageLog failed",
			"task_id", task.ID, "gen_task_id", task.GenTaskID, "error", err)
	}
}
