package service

import (
	"fmt"
	"sort"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// Phase 3 P3-1 Bridge Registry（map，**不抽 interface**）。
//
// 设计意图（docs/relay-architecture-design.md §4.1 + sprint-plan P3-1）：
//
//	桥（Bridge）= "<入站协议>-><出站协议>" 的转换执行器；当前 fork 已有 2 条桥
//	  - anthropic_messages->openai_responses：OpenAIGatewayService.ForwardAsAnthropic
//	  - openai_responses->anthropic_messages：GatewayService.ForwardAsResponses
//
// 桥数=2 时按 YAGNI 不抽 interface（P3-4 桥数≥3 时再做）；本 PR 只提供：
//   - BridgeID 规范命名（与 docs/glossary.md §1.1 protocol 长名一致）
//   - 注册表元数据查询（Lookup / List）
//   - 与旧路径行为等价（实际转换仍在原 service method 内）
//
// 用途：
//   - 后续 P3-3 / P3-5 添加新桥时统一注册到这里
//   - 调度路径（sticky / 候选 / failover）通过 Lookup 验证桥存在性
//   - 监控 / 调试日志统一打 BridgeID 标签
type ProtocolBridgeRegistry struct {
	mu      sync.RWMutex
	bridges map[string]BridgeMetadata
}

// BridgeMetadata 单条桥的注册元数据（Phase 3 P3-1 不含转换函数本身）。
type BridgeMetadata struct {
	// ID 桥的稳定标识，命名规则：<inbound>-><outbound>，如 "anthropic_messages->openai_responses"
	ID string

	// InboundProtocol 入站协议（取值见 docs/glossary.md §1.1）
	InboundProtocol string

	// OutboundProtocol 出站协议
	OutboundProtocol string

	// Implementation 转换器在 fork 中的现状描述（fork-specific 锚点，便于审计）。
	// 不参与运行时调度——只是元数据。
	Implementation string

	// Description 中文描述，便于运维定位
	Description string
}

// NewProtocolBridgeRegistry 构造注册表并预注册 fork 已有的 2 条桥。
//
// 桥数=2 时 YAGNI 不抽 interface；P3-4 桥数≥3 时再迁移到 interface-based registry。
func NewProtocolBridgeRegistry() *ProtocolBridgeRegistry {
	r := &ProtocolBridgeRegistry{
		bridges: make(map[string]BridgeMetadata),
	}

	// Bridge #1（fork 已有）: Anthropic Messages 入站 → OpenAI Responses 上游
	r.mustRegister(BridgeMetadata{
		ID:               BridgeID(domain.ProtocolAnthropicMessages, domain.ProtocolOpenAIResponses),
		InboundProtocol:  domain.ProtocolAnthropicMessages,
		OutboundProtocol: domain.ProtocolOpenAIResponses,
		Implementation:   "OpenAIGatewayService.ForwardAsAnthropic (openai_gateway_messages.go:28)",
		Description:      "Claude Code 客户端通过 /v1/messages 访问 OpenAI 上游账号",
	})

	// Bridge #2（fork 已有）: OpenAI Responses 入站 → Anthropic Messages 上游
	r.mustRegister(BridgeMetadata{
		ID:               BridgeID(domain.ProtocolOpenAIResponses, domain.ProtocolAnthropicMessages),
		InboundProtocol:  domain.ProtocolOpenAIResponses,
		OutboundProtocol: domain.ProtocolAnthropicMessages,
		Implementation:   "GatewayService.ForwardAsResponses (gateway_forward_as_responses.go:31)",
		Description:      "OpenAI Responses 客户端通过 /v1/responses 访问 Anthropic 上游账号",
	})

	return r
}

// BridgeID 按 docs/glossary.md §1.1 决议 #7 规范构造桥 ID（统一用 protocol 长名）。
//
// 命名规则：<inbound>-><outbound>，禁止使用短名（如 "openai" / "anthropic" / "gemini"）。
func BridgeID(inbound, outbound string) string {
	return inbound + "->" + outbound
}

// Register 在注册表中加入一条桥（Phase 3 后续 PR 用）。
//
// 重复 ID 会返回错误，避免静默覆盖造成行为漂移。
func (r *ProtocolBridgeRegistry) Register(meta BridgeMetadata) error {
	if r == nil {
		return fmt.Errorf("nil registry")
	}
	if meta.ID == "" {
		return fmt.Errorf("bridge id is required")
	}
	if !domain.IsValidProtocol(meta.InboundProtocol) {
		return fmt.Errorf("invalid inbound protocol %q (see docs/glossary.md §1.1)", meta.InboundProtocol)
	}
	if !domain.IsValidProtocol(meta.OutboundProtocol) {
		return fmt.Errorf("invalid outbound protocol %q (see docs/glossary.md §1.1)", meta.OutboundProtocol)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.bridges[meta.ID]; exists {
		return fmt.Errorf("bridge %q already registered", meta.ID)
	}
	r.bridges[meta.ID] = meta
	return nil
}

// mustRegister 仅供构造时调用（panic on duplicate / invalid）。
func (r *ProtocolBridgeRegistry) mustRegister(meta BridgeMetadata) {
	if err := r.Register(meta); err != nil {
		panic(fmt.Sprintf("registry boot failure: %v", err))
	}
}

// Lookup 按 (inbound, outbound) 查找桥；未注册返回 (BridgeMetadata{}, false)。
//
// 调度路径用此方法判断是否存在跨协议桥；不存在则路由器应回退到同协议透传或拒绝。
func (r *ProtocolBridgeRegistry) Lookup(inbound, outbound string) (BridgeMetadata, bool) {
	if r == nil {
		return BridgeMetadata{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.bridges[BridgeID(inbound, outbound)]
	return meta, ok
}

// LookupByID 直接按 BridgeID 查找。
func (r *ProtocolBridgeRegistry) LookupByID(id string) (BridgeMetadata, bool) {
	if r == nil {
		return BridgeMetadata{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.bridges[id]
	return meta, ok
}

// List 返回已注册的所有桥（按 ID 字典序，便于稳定的监控输出）。
func (r *ProtocolBridgeRegistry) List() []BridgeMetadata {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.bridges))
	for id := range r.bridges {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]BridgeMetadata, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.bridges[id])
	}
	return out
}

// Count 返回当前注册的桥数（用于 P3-4 时机判断：桥数≥3 时抽 interface）。
func (r *ProtocolBridgeRegistry) Count() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.bridges)
}
