//go:build unit

package service

import "time"

// tempUnschedCall 记录 SetTempUnschedulable 调用，供多个 *_test.go 共享
// （如 openai_upstream_transport_error_handle_test.go）。
type tempUnschedCall struct {
	accountID int64
	until     time.Time
	reason    string
}
