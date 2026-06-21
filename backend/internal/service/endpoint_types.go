package service

import (
	"context"
	"time"
)

// DBEndpoint 是 endpoints 表的 service 层 DTO（Phase 5 P5-1 引入）。
//
// stable_id 一旦写入不变更，历史 UsageLog 快照依赖此字段做 endpoint 归因。
// 详见 docs/glossary.md §4、docs/generic-channel-design.md §6。
type DBEndpoint struct {
	ID               int64     `json:"id"`
	AccountID        int64     `json:"account_id"`
	StableID         string    `json:"stable_id"`                  // 应用层稳定标识符
	OutboundProtocol string    `json:"outbound_protocol"`          // anthropic_messages | openai_chat | openai_responses | gemini_v1beta
	BaseURL          string    `json:"base_url"`                   // 上游根地址，不含尾部斜杠
	AuthHeader       string    `json:"auth_header"`                // 默认 Authorization
	AuthScheme       string    `json:"auth_scheme"`                // 默认 Bearer
	ModelsSource     string    `json:"models_source"`              // remote | manual | static_preset
	Priority         int       `json:"priority"`                   // 调度优先级，数值越小越优先
	Health           string    `json:"health"`                     // healthy | degraded | disabled
	Capabilities     []string  `json:"capabilities,omitempty"`     // 能力标签，对应 RequestFeatures
	SupportedModels  []string  `json:"supported_models,omitempty"` // 功能 25：该端点支持转发的模型 ID 列表
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// EndpointRepository 接口
//
// FindByStableID 是写 UsageLog 时的热路径：通过 (account_id, stable_id) 精确定位
// endpoint；未命中返回 (nil, nil)，不报错。
type EndpointRepository interface {
	// Create 创建 endpoint，成功后回写 ID、CreatedAt、UpdatedAt。
	Create(ctx context.Context, m *DBEndpoint) error

	// Update 更新 endpoint 所有可变字段，成功后回写 UpdatedAt。
	Update(ctx context.Context, m *DBEndpoint) error

	// Delete 按主键删除；记录不存在时返回 sentinel error。
	Delete(ctx context.Context, id int64) error

	// GetByID 按主键查询；未找到返回 sentinel error。
	GetByID(ctx context.Context, id int64) (*DBEndpoint, error)

	// ListByAccountID 返回指定账号下的全部 endpoint，按 priority ASC 排序。
	ListByAccountID(ctx context.Context, accountID int64) ([]*DBEndpoint, error)

	// FindByStableID 按 (account_id, stable_id) 精确查找；未命中返回 (nil, nil)。
	FindByStableID(ctx context.Context, accountID int64, stableID string) (*DBEndpoint, error)

	// UpdateSupportedModels 原子更新单条 endpoint 的 supported_models 列表。
	UpdateSupportedModels(ctx context.Context, id int64, models []string) error
}
