//go:build e2e

package integration

// 回归守护 claudedocs/待办任务列表.md「T2 · Gemini 生图计费：未产出图片场景仍计费 1 张图」。
//
// 根因（见 root-cause 调查）：GeminiMessagesCompatService 的 Forward()/ForwardNative()/
// forwardClaudeBodyAsChatCompletions() 三处路径修复前统计 imageCount 时只看请求模型名是否命中
// isImageGenerationModel()，从不解析上游实际响应里有没有产出图片（inlineData）。修复后三处均改为
// 依据上游响应实际统计（countGeminiGeneratedImages），安全策略拦截/模型被引导只回复文本时计 0。
// 单测（gemini_messages_compat_service_test.go 的 *_SafetyBlockedImageBillsZero /
// *_ActualImageBillsOne 用例）已用 mock 上游确定性覆盖三条代码路径；本测试用真实上游从
// 「余额扣减」角度做一次端到端交叉验证，防止该修复被后续改动回归。
//
// 验证策略：请求生图模型但在 prompt 里明确要求"只回复文本，不要生成图片"，多数模型会遵从；
// 若响应体里确实没有 inlineData（没有产出任何图片），断言余额下降幅度低于"按图计价"量级
// （只应产生纯文本 token 成本），证明没有被误计费。若模型仍然产出了图片（未遵从指令），
// 说明本次运行无法验证该场景，跳过——这是 live 上游行为，非本项目可控，不视为测试失败。
import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestE2EFull_GeminiImageBillingOnNoImageProduced(t *testing.T) {
	pc := requireProvision(t)
	gp := pc.requirePlatform(t, "gemini")

	const imageModel = "gemini-2.5-flash-image"

	before, err := profileBalance(pc.adminToken)
	if err != nil {
		t.Fatalf("读余额失败: %v", err)
	}

	st, body, err := gwGeminiNoImageIntent(gp.gatewayKey, imageModel)
	if err != nil {
		t.Fatalf("请求错误: %v", err)
	}
	if st != 200 {
		t.Skipf("生图模型请求未返回 200（st=%d），无法验证该场景（可能账号无该模型访问权限）：%s", st, truncate(body, 300))
	}
	if bodyContains(body, `"inlineData"`) {
		t.Skip("上游仍然产出了图片（未遵从纯文本指令），本次运行无法验证「零产出不应计费」场景，跳过")
	}

	// $0.01 远高于几十 token 的纯文本成本（约 $0.0001 量级），但明显低于任意已知 Gemini 生图模型
	// 的单张图片价格（见 backend/data/model_pricing.json，最低约 $0.039/张）。修复后零产出场景
	// 的余额下降应远低于这个阈值；若达到或超过，说明图片计费 bug 回归了。
	const imageBillingThreshold = 0.01

	after, dropped := waitBalanceDrop(pc.adminToken, before, 15*time.Second)
	if dropped {
		delta := before - after
		if delta >= imageBillingThreshold {
			t.Fatalf("❌ 图片计费 bug 回归：生图模型请求未产出任何图片（响应无 inlineData），但余额下降 %.8f（≥ %.8f 图片计价量级），说明又被错误计费 1 张图。见 claudedocs/待办任务列表.md「T2」", delta, imageBillingThreshold)
		}
		t.Logf("✅ 零产出未被计费图片：余额下降 %.8f（< %.8f 图片计价量级，属正常 token 成本）", delta, imageBillingThreshold)
		return
	}

	// 极短回复可能命中免费额度/四舍五入到 0，余额压根没变化也算通过（没有按图片计费）。
	t.Logf("✅ 零产出未被计费图片：余额未变化（before=%.8f after=%.8f）", before, after)
}

// gwGeminiNoImageIntent 打 POST /v1beta/models/{model}:generateContent，明确要求模型只回复
// 文本、不要生成图片——用于验证「零图片产出但仍被计费」的已知 bug。
func gwGeminiNoImageIntent(gwKey, model string) (int, []byte, error) {
	payload := map[string]any{
		"contents": []map[string]any{
			{"role": "user", "parts": []map[string]string{
				{"text": "Do not generate or return any image. Reply with plain text only: the single word OK."},
			}},
		},
		"generationConfig": map[string]any{
			"maxOutputTokens": 64,
			"thinkingConfig":  map[string]any{"thinkingBudget": 0},
		},
	}
	b, _ := json.Marshal(payload)
	path := fmt.Sprintf("/v1beta/models/%s:generateContent", model)
	return apiCall("POST", path, map[string]string{"Authorization": "Bearer " + gwKey}, b)
}
