package promptanalytics

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// opsRequestBodyKey is the gin.Context key used by OpsErrorLoggerMiddleware
// to store the buffered request body. Defined here to avoid a dependency on
// the handler package (which would create an import cycle).
const opsRequestBodyKey = "ops_request_body"

// getOpsRequestBody retrieves the pre-buffered request body stored by OpsErrorLoggerMiddleware.
func getOpsRequestBody(c *gin.Context) ([]byte, bool) {
	v, ok := c.Get(opsRequestBodyKey)
	if !ok {
		return nil, false
	}
	b, ok := v.([]byte)
	return b, ok
}

// task is a unit of work queued for async keyword extraction.
type task struct {
	body     []byte
	userID   int64
	apiKeyID int64
	groupID  int64
}

// Plugin holds all runtime state for the prompt analytics plugin.
type Plugin struct {
	cfg       Config
	extractor *KeywordExtractor
	repo      Repository
	queue     chan task
}

// New creates and starts a Plugin. Call Shutdown to stop background goroutines.
func New(cfg Config, repo Repository) *Plugin {
	if cfg.MaxQueueSize <= 0 {
		cfg.MaxQueueSize = 1000
	}
	if cfg.SamplingRate <= 0 {
		cfg.SamplingRate = 0.05
	}
	if cfg.BatchWindow <= 0 {
		cfg.BatchWindow = 200 * time.Millisecond
	}
	if cfg.MaxKeywordsPerRequest <= 0 {
		cfg.MaxKeywordsPerRequest = 20
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 90
	}

	p := &Plugin{
		cfg:       cfg,
		extractor: NewKeywordExtractor(cfg.MaxKeywordsPerRequest),
		repo:      repo,
		queue:     make(chan task, cfg.MaxQueueSize),
	}

	go p.worker()
	go p.cleanupWorker()

	return p
}

// Middleware returns a Gin HandlerFunc that asynchronously samples and processes
// prompts. It must be registered AFTER OpsErrorLoggerMiddleware so that
// handler.GetOpsRequestBody returns a non-nil value.
func (p *Plugin) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Execute the downstream handler first.
		c.Next()

		// Only sample a fraction of requests.
		if rand.Float64() > p.cfg.SamplingRate {
			return
		}

		// Retrieve the pre-buffered body from context (set by OpsErrorLoggerMiddleware).
		body, ok := getOpsRequestBody(c)
		if !ok || len(body) == 0 {
			return
		}

		// Extract IDs from context (set during API key auth).
		apiKey, ok2 := middleware.GetAPIKeyFromContext(c)
		if !ok2 || apiKey == nil {
			return
		}
		userID := apiKey.UserID
		if userID == 0 {
			return
		}
		apiKeyID := apiKey.ID
		var groupID int64
		if apiKey.GroupID != nil {
			groupID = *apiKey.GroupID
		}

		// Non-blocking enqueue – drop silently if queue is full.
		select {
		case p.queue <- task{
			body:     body,
			userID:   userID,
			apiKeyID: apiKeyID,
			groupID:  groupID,
		}:
		default:
			// Queue full: skip this sample to avoid blocking the request.
		}
	}
}

// worker drains the queue and flushes batches to the repository.
func (p *Plugin) worker() {
	ticker := time.NewTicker(p.cfg.BatchWindow)
	defer ticker.Stop()

	batch := make([]KeywordRecord, 0, 64)
	period := currentPeriod()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.repo.UpsertKeywords(ctx, batch); err != nil {
			slog.Warn("promptanalytics: failed to upsert keywords", "error", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case t, ok := <-p.queue:
			if !ok {
				flush()
				return
			}
			// Refresh period at process time (not enqueue time).
			period = currentPeriod()
			keywords := p.extractor.ExtractKeywords(t.body)
			for _, kw := range keywords {
				batch = append(batch, KeywordRecord{
					UserID:   t.userID,
					APIKeyID: t.apiKeyID,
					GroupID:  t.groupID,
					Keyword:  kw,
					Period:   period,
				})
			}

		case <-ticker.C:
			flush()
		}
	}
}

// cleanupWorker runs a daily TTL cleanup of old keyword_stats rows.
func (p *Plugin) cleanupWorker() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().AddDate(0, 0, -p.cfg.RetentionDays)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		n, err := p.repo.CleanupOldKeywords(ctx, cutoff)
		cancel()
		if err != nil {
			slog.Warn("promptanalytics: cleanup failed", "error", err)
		} else if n > 0 {
			slog.Info("promptanalytics: cleaned up old keyword stats", "deleted", n)
		}
	}
}

// currentPeriod returns the current year-month string, e.g. "2024-01".
func currentPeriod() string {
	return time.Now().Format("2006-01")
}
