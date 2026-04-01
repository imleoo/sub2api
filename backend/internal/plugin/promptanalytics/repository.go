package promptanalytics

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// KeywordRecord is a row to upsert into keyword_stats.
type KeywordRecord struct {
	UserID   int64
	APIKeyID int64
	GroupID  int64
	Keyword  string
	Period   string // e.g. "2024-01"
}

// Repository provides persistence operations for keyword statistics.
type Repository interface {
	// UpsertKeywords increments the count for each keyword record, inserting
	// a new row if the (user_id, keyword, period) combination does not yet exist.
	UpsertKeywords(ctx context.Context, records []KeywordRecord) error

	// GetTopKeywords returns the most frequent keywords for the given filters.
	GetTopKeywords(ctx context.Context, userID int64, period string, limit int) ([]KeywordCount, error)

	// GetGlobalTopKeywords returns global top keywords across all users.
	GetGlobalTopKeywords(ctx context.Context, period string, limit int) ([]KeywordCount, error)

	// CleanupOldKeywords deletes keyword stats records older than the given cutoff.
	CleanupOldKeywords(ctx context.Context, before time.Time) (int64, error)
}

// KeywordCount holds a keyword and its aggregated count.
type KeywordCount struct {
	Keyword string `json:"keyword"`
	Count   int64  `json:"count"`
}

type repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository backed by the given SQL connection.
func NewRepository(db *sql.DB) Repository {
	return &repository{db: db}
}

func (r *repository) UpsertKeywords(ctx context.Context, records []KeywordRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("promptanalytics: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO keyword_stats (user_id, api_key_id, group_id, keyword, count, period, created_at)
		VALUES ($1, $2, $3, $4, 1, $5, NOW())
		ON CONFLICT (user_id, keyword, period)
		DO UPDATE SET count = keyword_stats.count + 1
	`)
	if err != nil {
		return fmt.Errorf("promptanalytics: prepare upsert: %w", err)
	}
	defer stmt.Close()

	for _, rec := range records {
		if _, err := stmt.ExecContext(ctx, rec.UserID, rec.APIKeyID, rec.GroupID, rec.Keyword, rec.Period); err != nil {
			return fmt.Errorf("promptanalytics: upsert keyword %q: %w", rec.Keyword, err)
		}
	}

	return tx.Commit()
}

func (r *repository) GetTopKeywords(ctx context.Context, userID int64, period string, limit int) ([]KeywordCount, error) {
	query := `
		SELECT keyword, SUM(count) as total
		FROM keyword_stats
		WHERE user_id = $1 AND period = $2
		GROUP BY keyword
		ORDER BY total DESC
		LIMIT $3
	`
	return r.queryKeywordCounts(ctx, query, userID, period, limit)
}

func (r *repository) GetGlobalTopKeywords(ctx context.Context, period string, limit int) ([]KeywordCount, error) {
	query := `
		SELECT keyword, SUM(count) as total
		FROM keyword_stats
		WHERE period = $1
		GROUP BY keyword
		ORDER BY total DESC
		LIMIT $2
	`
	return r.queryKeywordCountsGlobal(ctx, query, period, limit)
}

func (r *repository) CleanupOldKeywords(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		"DELETE FROM keyword_stats WHERE created_at < $1", before)
	if err != nil {
		return 0, fmt.Errorf("promptanalytics: cleanup: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func (r *repository) queryKeywordCounts(ctx context.Context, query string, args ...any) ([]KeywordCount, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanKeywordCounts(rows)
}

func (r *repository) queryKeywordCountsGlobal(ctx context.Context, query string, args ...any) ([]KeywordCount, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanKeywordCounts(rows)
}

func scanKeywordCounts(rows *sql.Rows) ([]KeywordCount, error) {
	var results []KeywordCount
	for rows.Next() {
		var kc KeywordCount
		if err := rows.Scan(&kc.Keyword, &kc.Count); err != nil {
			return nil, err
		}
		results = append(results, kc)
	}
	return results, rows.Err()
}
