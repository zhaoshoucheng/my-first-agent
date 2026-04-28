package thinking

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
)

const (
	AnthropicThinkingDisabled = "disabled"
	AnthropicThinkingEnabled  = "enabled"
)

func ToAnthropicThinking(thinking *llms.Thinking) *anthropic.ThinkingConfigParamUnion {
	if thinking == nil || thinking.BudgetTokens == nil {
		return nil
	}
	if thinking.EnabledType == AnthropicThinkingEnabled {
		enable := anthropic.ThinkingConfigParamOfEnabled(*thinking.BudgetTokens)
		return &enable
	}
	if thinking.EnabledType == AnthropicThinkingDisabled {
		ofDisabled := anthropic.NewThinkingConfigDisabledParam()
		return &anthropic.ThinkingConfigParamUnion{
			OfDisabled: &ofDisabled,
		}
	}
	return nil
}
