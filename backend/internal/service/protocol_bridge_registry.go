package service

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// ProtocolBridge 是协议桥的统一执行接口（Phase 3 P3-4 引入）。
//
// 桥数 ≥3 时按 plan 抽 interface（P3-3 完成后桥数=3 触发本步）。
// 接口的设计意图（docs/relay-architecture-design.md §4.2 BridgeInput / BridgePlan 简化版）：
//
//	桥 = "请求转换 + 上游调用 + 响应转换" 的单元，所有桥共享 ID/Inbound/Outbound 元数据。
//	实际转发逻辑（Forward 方法）由各实现负责；当前 fork 现有桥的 ForwardAsAnthropic /
//	ForwardAsResponses 暂未迁移到此接口（避免动 fork 已稳定的代码），但 Registry 元数据
//	可统一通过 Metadata() 暴露给监控 / 调度 / BridgeCapabilities（P3-6 落地）。
//
// 实现路线（按 sprint-plan P3-4 + P3-5）：
//   - P3-4：定义 interface，已有 stub 桥（apicompat.ForwardAnthropicAsChatCompletions）改为
//     interface 实现的方式注册
//   - P3-5：新桥（chatcompletions→anthropic / responses→chatcompletions）按 interface 落地
//   - P3-6+：fork 现有 ForwardAsAnthropic / ForwardAsResponses 渐进迁移到 interface
type ProtocolBridge interface {
	// Metadata 返回桥的稳定元数据（用于 Registry 注册 / 监控 / 调度）。
	Metadata() BridgeMetadata
	// Forward 执行请求转发：转换请求 → 调用上游 → 转换响应。
	// 当前阶段桥实现可返回 ErrBridgeNotImplemented 表示未就绪；调度层应配合
	// BridgeCapabilities 标记 FitUnknown 避免选中未实现的桥。
	Forward(ctx context.Context, payload *BridgePayload) (*BridgeResult, error)
}

// BridgePayload 是 Forward 的输入：原始请求体 + 上下文（账号 / endpoint / stream 偏好等）。
//
// 字段最小化（P3-4 范围）；P3-5 会按需扩展（RequestFeatures 嗅探结果、能力探测 hint 等）。
type BridgePayload struct {
	// Body 客户端原始请求体（按 InboundProtocol 解析）
	Body []byte
	// Model 客户端请求的模型名（未经映射）
	Model string
	// Stream 客户端是否要求流式响应
	Stream bool
}

// BridgeResult 是 Forward 的输出：转换后的响应 + 元信息。
type BridgeResult struct {
	// Body 转换后写回客户端的响应体（按 InboundProtocol 编码）
	Body []byte
	// ContentType 响应 Content-Type，便于 handler 透传
	ContentType string
	// UsageTokenInput / UsageTokenOutput 桥层观察到的 token 用量（可能与上游账单一致也可能近似）
	UsageTokenInput  int
	UsageTokenOutput int
}

// ErrBridgeNotImplemented 桥实现尚未就绪的通用信号（Phase 3 P3-4）。
var ErrBridgeNotImplemented = fmt.Errorf("bridge implementation not yet available")

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

	// Bridge #3（Phase 3 P3-3 占位）: Anthropic Messages 入站 → OpenAI Chat Completions 上游
	// 实现状态：apicompat.ForwardAnthropicAsChatCompletions 为 stub，返回 ErrAnthropicToChatNotImplemented
	// 注册目的：让桥数=3 解锁 P3-4 interface 抽离；完整实现见独立 PR
	r.mustRegister(BridgeMetadata{
		ID:               BridgeID(domain.ProtocolAnthropicMessages, domain.ProtocolOpenAIChat),
		InboundProtocol:  domain.ProtocolAnthropicMessages,
		OutboundProtocol: domain.ProtocolOpenAIChat,
		Implementation:   "apicompat.ForwardAnthropicAsChatCompletions (stub, P3-3 follow-up)",
		Description:      "Claude Code 客户端通过 /v1/messages 访问 DeepSeek/Kimi/Qwen 等 OpenAI Chat Completions 上游（实现中）",
	})

	// Bridge #4（Phase 3 P3-5 占位）: OpenAI Chat Completions 入站 → Anthropic Messages 上游
	r.mustRegister(BridgeMetadata{
		ID:               BridgeID(domain.ProtocolOpenAIChat, domain.ProtocolAnthropicMessages),
		InboundProtocol:  domain.ProtocolOpenAIChat,
		OutboundProtocol: domain.ProtocolAnthropicMessages,
		Implementation:   "apicompat.ForwardChatAsAnthropic (stub, P3-5 follow-up)",
		Description:      "OpenAI Chat Completions 客户端访问 Anthropic 上游账号（实现中）",
	})

	// Bridge #5（Phase 3 P3-5 占位）: OpenAI Responses 入站 → OpenAI Chat Completions 上游（同源协议互转）
	r.mustRegister(BridgeMetadata{
		ID:               BridgeID(domain.ProtocolOpenAIResponses, domain.ProtocolOpenAIChat),
		InboundProtocol:  domain.ProtocolOpenAIResponses,
		OutboundProtocol: domain.ProtocolOpenAIChat,
		Implementation:   "apicompat.ForwardResponsesAsChat (stub, P3-5 follow-up)",
		Description:      "OpenAI Responses 客户端访问只支持 Chat Completions 的 OpenAI-compatible 上游（实现中）",
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

// --- Phase 3 P3-4 interface-based registration ---
//
// 注册表同时支持元数据注册（fork 已有桥，Implementation 字符串引用）和
// interface 实现注册（新桥，运行时可通过 Lookup + GetBridge 拿到 ProtocolBridge 调用 Forward）。

// implementations 保存按桥 ID 索引的 interface 实现（与 bridges 元数据 map 并存）。
// 拆开存的好处：fork 已有桥保留元数据但不需要 wrap interface（避免破坏稳定代码）。
var bridgeImplsMu sync.RWMutex
var bridgeImpls = map[string]ProtocolBridge{}

// RegisterBridge 在全局注册表中加入一个 interface 实现。
//
// 与 ProtocolBridgeRegistry 的 Register 互补：
//   - Register：注册元数据（fork 旧桥 / stub 桥用）
//   - RegisterBridge：注册可执行实现（新桥用）
//
// 注册时会校验 bridge.Metadata() 的 protocol 合法性（避免短名 / 空 ID）。
func RegisterBridge(b ProtocolBridge) error {
	if b == nil {
		return fmt.Errorf("nil ProtocolBridge")
	}
	meta := b.Metadata()
	if meta.ID == "" {
		return fmt.Errorf("bridge metadata ID is required")
	}
	if !domain.IsValidProtocol(meta.InboundProtocol) {
		return fmt.Errorf("invalid inbound protocol %q", meta.InboundProtocol)
	}
	if !domain.IsValidProtocol(meta.OutboundProtocol) {
		return fmt.Errorf("invalid outbound protocol %q", meta.OutboundProtocol)
	}
	bridgeImplsMu.Lock()
	defer bridgeImplsMu.Unlock()
	if _, exists := bridgeImpls[meta.ID]; exists {
		return fmt.Errorf("bridge %q implementation already registered", meta.ID)
	}
	bridgeImpls[meta.ID] = b
	return nil
}

// LookupBridge 按桥 ID 查找已注册的 interface 实现；未注册返回 (nil, false)。
//
// 与 ProtocolBridgeRegistry.Lookup 互补：调用方先用元数据 Lookup 判断桥是否存在，
// 再用 LookupBridge 拿到执行器调用 Forward。
func LookupBridge(id string) (ProtocolBridge, bool) {
	bridgeImplsMu.RLock()
	defer bridgeImplsMu.RUnlock()
	b, ok := bridgeImpls[id]
	return b, ok
}

// ResetBridgeImpls 仅供测试使用，清空全局实现表。
func ResetBridgeImpls() {
	bridgeImplsMu.Lock()
	defer bridgeImplsMu.Unlock()
	bridgeImpls = map[string]ProtocolBridge{}
}
