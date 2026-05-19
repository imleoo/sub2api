package service

import "sort"

const (
	EndpointHealthHealthy  = "healthy"
	EndpointHealthDegraded = "degraded"
	EndpointHealthDisabled = "disabled"
)

// SelectHealthyEndpoints 返回 endpoints 中 health=healthy 的条目，
// 按 priority ASC 稳定排序（priority 相同时按 ID 升序）。
// 当所有 endpoint 不可用时返回空切片，调用方据此判断账号暂不可调度。
// 供 generic 账号路由层使用（P5-5 chaos 验证通过后 P5-6 接入主路径）。
func SelectHealthyEndpoints(endpoints []*DBEndpoint) []*DBEndpoint {
	out := make([]*DBEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep != nil && ep.Health == EndpointHealthHealthy {
			out = append(out, ep)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}
