// Package promptanalytics provides a zero-intrusion plugin for extracting
// keyword statistics from user prompts. It does NOT store raw prompt content.
package promptanalytics

import "time"

// Config holds configuration for the prompt analytics plugin.
type Config struct {
	// SamplingRate controls the fraction of requests to sample (0.0 - 1.0).
	// Default: 0.05 (5%).
	SamplingRate float64

	// MaxQueueSize is the maximum number of pending analysis tasks buffered in memory.
	// Default: 1000.
	MaxQueueSize int

	// BatchWindow is the duration to wait before flushing a batch of keyword upserts.
	// Default: 200ms.
	BatchWindow time.Duration

	// RetentionDays is the number of days to retain keyword stats.
	// Records older than this are deleted during cleanup. Default: 90.
	RetentionDays int

	// MaxKeywordsPerRequest limits extracted keywords per request. Default: 20.
	MaxKeywordsPerRequest int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		SamplingRate:          0.05,
		MaxQueueSize:          1000,
		BatchWindow:           200 * Millisecond,
		RetentionDays:         90,
		MaxKeywordsPerRequest: 20,
	}
}

// Millisecond is a convenience alias so the config file doesn't need an extra import.
const Millisecond = time.Millisecond
