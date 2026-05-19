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
	ID               int64
	AccountID        int64
	StableID         string   // 应用层稳定标识符，格式建议：<provider>-<protocol>
	OutboundProtocol string   // anthropic_messages | openai_chat | openai_responses | gemini_v1beta
	BaseURL          string   // 上游根地址，不含尾部斜杠
	AuthHeader       string   // 默认 Authorization
	AuthScheme       string   // 默认 Bearer
	ModelsSource     string   // remote | manual | static_preset
	Priority         int      // 调度优先级，数值越小越优先
	Health           string   // healthy | degraded | disabled
	Capabilities     []string // 能力标签，对应 RequestFeatures
	CreatedAt        time.Time
	UpdatedAt        time.Time
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
}
