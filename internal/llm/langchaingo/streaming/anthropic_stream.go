package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
)

// StreamProcessor handles streaming events one by one (using anthropic.MessageStreamEvent)
type StreamProcessor struct {
	contentChoices []*llms.ContentChoice
	tools          []llms.ToolCall
	options        *llms.CallOptions
	responseID     string
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(options *llms.CallOptions) *StreamProcessor {
	return &StreamProcessor{
		contentChoices: []*llms.ContentChoice{{GenerationInfo: map[string]interface{}{}}},
		tools:          make([]llms.ToolCall, 0),
		options:        options,
	}
}

// ProcessEvent processes a single anthropic.MessageStreamEvent
func (sp *StreamProcessor) ProcessEvent(ctx context.Context, event anthropic.MessageStreamEventUnion) error {
	var chunk []byte
	var err error

	switch event.AsAny().(type) {
	case anthropic.ContentBlockStartEvent:
		block := event.AsContentBlockStart()
		switch block.ContentBlock.AsAny().(type) {
		case anthropic.TextBlock:
		case anthropic.ToolUseBlock:
			chunk, sp.tools, err = updateToolUse(sp.tools, nil, &block.ContentBlock)
			if err != nil {
				return err
			}
		}
	case anthropic.ContentBlockStopEvent:
		slog.DebugContext(ctx, "content block stop received")
	case anthropic.MessageStartEvent:
		msg := event.AsMessageStart()
		if sp.responseID == "" {
			sp.responseID = msg.Message.ID
		}
		sp.contentChoices[0].GenerationInfo["input_tokens"] = msg.Message.Usage.InputTokens
		sp.contentChoices[0].GenerationInfo["cache_read_input_tokens"] = msg.Message.Usage.CacheReadInputTokens
		sp.contentChoices[0].GenerationInfo["cache_creation_input_tokens"] = msg.Message.Usage.CacheCreationInputTokens
	case anthropic.MessageStopEvent:
		msg := event.AsMessageStop()
		raw := msg.RawJSON()
		if raw != "" && sp.options.MessageStopFunc != nil {
			if err = sp.options.MessageStopFunc(ctx, []byte(raw)); err != nil {
				return err
			}
		}
	case anthropic.ContentBlockDeltaEvent:
		block := event.AsContentBlockDelta()
		switch block.Delta.AsAny().(type) {
		case anthropic.InputJSONDelta:
			chunk, sp.tools, err = updateToolUse(sp.tools, &block.Delta, nil)
			if err != nil {
				return err
			}
		case anthropic.TextDelta:
			t := block.Delta.AsTextDelta()
			if err = sp.options.StreamingFunc(ctx, []byte(t.Text)); err != nil {
				return err
			}
			sp.contentChoices[0].Content += t.Text
		case anthropic.ThinkingDelta:
			t := block.Delta.AsThinkingDelta()
			chunk, _ = json.Marshal(llms.StreamResponseField{
				Value: t.Thinking,
				Key:   "reasoning_content",
			})
			sp.contentChoices[0].ReasoningContent += t.Thinking
		case anthropic.CitationsDelta:
		}
	case anthropic.MessageDeltaEvent:
		msg := event.AsMessageDelta()
		sp.contentChoices[0].StopReason = string(msg.Delta.StopReason)
		if event.Usage.OutputTokens > 0 {
			sp.contentChoices[0].GenerationInfo["output_tokens"] = event.Usage.OutputTokens
		}
	}

	if chunk != nil {
		if err = sp.options.StreamingFunc(ctx, chunk); err != nil {
			return err
		}
	}

	return nil
}

// 这里如果出现吐一半吞字可能需要修改为没有 output 才报错

// updateToolUse updates tool calls for anthropic events
func updateToolUse(
	tools []llms.ToolCall,
	delta *anthropic.RawContentBlockDeltaUnion,
	start *anthropic.ContentBlockStartEventContentBlockUnion,
) ([]byte, []llms.ToolCall, error) {
	var chunkToolCalls []*llms.ToolCall

	if start != nil {
		// if the tool is not the same as the last tool, add a new tool call
		if len(tools) == 0 || (tools[len(tools)-1].ID != "" && tools[len(tools)-1].ID != start.ID) {
			toolCall := llms.ToolCall{
				ID:           start.ID,
				Type:         "function",
				FunctionCall: &llms.FunctionCall{Name: start.Name},
			}
			tools = append(tools, toolCall)
			chunkToolCalls = append(chunkToolCalls, &toolCall)
		}
	}

	if delta != nil && len(delta.PartialJSON) > 0 {
		tools[len(tools)-1].FunctionCall.Arguments += delta.PartialJSON
		if len(chunkToolCalls) == 0 {
			chunkToolCalls = append(chunkToolCalls, &llms.ToolCall{
				FunctionCall: &llms.FunctionCall{
					Arguments: delta.PartialJSON,
				},
			})
		}
	}

	if len(chunkToolCalls) == 0 {
		return []byte(""), tools, nil
	}

	chunk, err := json.Marshal(chunkToolCalls)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal tool calls: %w", err)
	}
	return chunk, tools, nil
}

// GetResult returns the final content response
func (sp *StreamProcessor) GetResult() *llms.ContentResponse {
	sp.contentChoices[0].ToolCalls = sp.tools
	return &llms.ContentResponse{
		Choices: sp.contentChoices,
		ID:      sp.responseID,
	}
}
