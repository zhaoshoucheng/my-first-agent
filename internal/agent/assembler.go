// Package agent Alembic 的 Agent 核心（D 层）。
// 目标架构见 docs/architecture/agent-core.md：事件驱动主循环、修复链、计划元工具
// 都将落在本包。当前只有上下文组装的空壳。
package agent

import (
	"context"
	"fmt"

	"github.com/shoucheng/my-first-agent/internal/event"
	llmstypes "github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
	"github.com/shoucheng/my-first-agent/internal/task"
)

// ContextAssembler 把 task 的事件历史组装成可直接喂给 LLM 的消息列表。
// 滑动窗口压缩将来加在实现里，接口不变。
type ContextAssembler interface {
	Assemble(ctx context.Context, taskID string) ([]llmstypes.MessageContent, error)
}

// NaiveAssembler 朴素组装器（空壳）：只映射文本类事件。
// 待补：action_start/observation ↔ tool_call 消息映射、plan 注入、滑动窗口。
type NaiveAssembler struct {
	Store task.Store
}

func (a *NaiveAssembler) Assemble(ctx context.Context, taskID string) ([]llmstypes.MessageContent, error) {
	t, err := a.Store.Get(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("assemble: %w", err)
	}
	var msgs []llmstypes.MessageContent
	for _, e := range t.Events {
		switch p := e.Payload.(type) {
		case event.UserMessagePayload:
			msgs = append(msgs, llmstypes.TextParts(llmstypes.ChatMessageTypeHuman, p.Text))
		case event.AssistantMessagePayload:
			msgs = append(msgs, llmstypes.TextParts(llmstypes.ChatMessageTypeAI, p.Text))
		default:
			// status 永不进上下文；其余类型的映射待实现
		}
	}
	return msgs, nil
}
