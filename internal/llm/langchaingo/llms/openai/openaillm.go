package openai

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/callbacks"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms/openai/internal/openaiclient"
)

type ChatMessage = openaiclient.ChatMessage

type LLM struct {
	CallbacksHandler callbacks.Handler
	client           *openaiclient.Client
}

const (
	RoleSystem    = "system"
	RoleAssistant = "assistant"
	RoleUser      = "user"
	RoleFunction  = "function"
	RoleTool      = "tool"
)

var _ llms.Model = (*LLM)(nil)

// New returns a new OpenAI LLM.
func New(opts ...Option) (*LLM, error) {
	opt, c, err := newClient(opts...)
	if err != nil {
		return nil, err
	}
	return &LLM{
		client:           c,
		CallbacksHandler: opt.callbackHandler,
	}, err
}

// Call requests a completion for the given prompt.
func (o *LLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return llms.GenerateFromSinglePrompt(ctx, o, prompt, options...)
}

// GenerateContent implements the Model interface.
func (o *LLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) { //nolint: lll, cyclop, goerr113, funlen
	if o.CallbacksHandler != nil {
		o.CallbacksHandler.HandleLLMGenerateContentStart(ctx, messages)
	}

	opts := llms.CallOptions{}
	for _, opt := range options {
		opt(&opts)
	}

	if toolChoice, ok := opts.ToolChoice.(string); ok {
		if toolChoice == "any" {
			opts.ToolChoice = "required"
		}
	}

	chatMsgs := make([]*ChatMessage, 0, len(messages))
	for _, mc := range messages {
		msg := &ChatMessage{MultiContent: mc.Parts}
		switch mc.Role {
		case llms.ChatMessageTypeSystem:
			msg.Role = RoleSystem
		case llms.ChatMessageTypeAI:
			msg.Role = RoleAssistant
		case llms.ChatMessageTypeHuman:
			msg.Role = RoleUser
			if len(msg.MultiContent) == 1 {
				// skip context option
				_, ok := msg.MultiContent[0].(llms.ContextOption)
				if ok {
					continue
				}
			}
		case llms.ChatMessageTypeGeneric:
			msg.Role = RoleUser
		case llms.ChatMessageTypeFunction:
			msg.Role = RoleFunction
		case llms.ChatMessageTypeTool:
			msg.Role = RoleTool
			// Here we extract tool calls from the message and populate the ToolCalls field.
			if len(mc.Parts) > 1 {
				msg.MultiContent = nil
				for _, part := range mc.Parts {
					switch p := part.(type) {
					case llms.ToolCallResponse:
						msg.ToolCallID = p.ToolCallID
						msg.MultiContent = append(msg.MultiContent, llms.TextPart(p.Content))
					case llms.ContextOption:
						if p.CacheConfig != nil && o.client.GetApiType() == openaiclient.APITypeAzureDataBricks {
							msg.CacheControl = &openaiclient.CacheControl{
								Type: "ephemeral",
							}
						}
					default:
						msg.MultiContent = append(msg.MultiContent, part)
					}
				}
			} else {
				// parse mc.Parts (which should have one entry of type ToolCallResponse) and populate msg.Content and msg.ToolCallID
				if len(mc.Parts) != 1 {
					return nil, fmt.Errorf("expected exactly one part for role %v, got %v", mc.Role, len(mc.Parts))
				}
				switch p := mc.Parts[0].(type) {
				case llms.ToolCallResponse:
					msg.ToolCallID = p.ToolCallID
					msg.Content = p.Content
				default:
					return nil, fmt.Errorf("expected part of type ToolCallResponse for role %v, got %T", mc.Role, mc.Parts[0])
				}
			}

		default:
			return nil, fmt.Errorf("role %v not supported", mc.Role)
		}
		// Here we extract tool calls from the message and populate the ToolCalls field.
		newParts, toolCalls := o.ExtractToolParts(msg)
		msg.MultiContent = newParts
		msg.ToolCalls = toolCallsFromToolCalls(toolCalls)

		chatMsgs = append(chatMsgs, msg)
	}
	req := &openaiclient.ChatRequest{
		Model:               opts.Model,
		StopWords:           opts.StopWords,
		Messages:            chatMsgs,
		StreamingFunc:       opts.StreamingFunc,
		Temperature:         opts.Temperature,
		MaxTokens:           opts.MaxTokens,
		MaxCompletionTokens: opts.MaxCompletionTokens,
		N:                   opts.N,
		FrequencyPenalty:    opts.FrequencyPenalty,
		PresencePenalty:     opts.PresencePenalty,

		ToolChoice:           opts.ToolChoice,
		FunctionCallBehavior: openaiclient.FunctionCallBehavior(opts.FunctionCallBehavior),
		Seed:                 opts.Seed,
		Metadata:             opts.Metadata,
		PromptCacheKey:       opts.PromptCacheKey,
		PromptCacheRetention: opts.PromptCacheRetention,
		ExtraHeaders:         opts.ExtraHeaders,
		ExtraBody:            opts.ExtraBody,
		ReasoningEffort:      getReasoningEffort(&opts),
		Prediction:           opts.Prediction,
		Verbosity:            opts.Verbosity,
		ParallelToolCalls:    opts.ParallelToolCalls,
	}
	if opts.JSONMode {
		if opts.JSONSchema == nil {
			req.ResponseFormat = ResponseFormatJSON
		} else {
			req.ResponseFormat = &ResponseFormat{Type: "json_schema", JSONSchema: opts.JSONSchema}
		}
	}
	if opts.StreamOptions != nil {
		req.StreamOptions = &openaiclient.StreamOptions{
			IncludeUsage: opts.StreamOptions.IncludeUsage,
		}
	}

	// since req.Functions is deprecated, we need to use the new Tools API.
	for _, fn := range opts.Functions {
		req.Tools = append(req.Tools, openaiclient.Tool{
			Type: "function",
			Function: openaiclient.FunctionDefinition{
				Name:        fn.Name,
				Description: fn.Description,
				Parameters:  fn.Parameters,
			},
		})
	}
	// if opts.Tools is not empty, append them to req.Tools
	for _, tool := range opts.Tools {
		t, err := toolFromTool(tool)
		if err != nil {
			return nil, fmt.Errorf("failed to convert llms tool to openai tool: %w", err)
		}
		req.Tools = append(req.Tools, t)
	}

	result, err := o.client.CreateChat(ctx, req)
	if err != nil {
		if result != nil {
			resp := &llms.ContentResponse{}
			if len(result.ExtInfo) > 0 {
				resp.ExtInfo = result.ExtInfo
			}
			resp.SetHeader(result.Header())
			return resp, err
		}
		return nil, err
	}
	if len(result.Choices) == 0 {
		return nil, ErrEmptyResponse
	}

	choices := make([]*llms.ContentChoice, len(result.Choices))
	for i, c := range result.Choices {
		generationInfo := map[string]any{
			"CompletionTokens":            result.Usage.CompletionTokens,
			"PromptTokens":                result.Usage.PromptTokens,
			"TotalTokens":                 result.Usage.TotalTokens,
			"CachedTokens":                result.Usage.PromptTokensDetails.CachedTokens,
			"accepted_prediction_tokens":  result.Usage.CompletionTokensDetails.AcceptedPredictionTokens,
			"rejected_prediction_tokens":  result.Usage.CompletionTokensDetails.RejectedPredictionTokens,
			"cache_read_input_tokens":     result.Usage.CacheReadInputTokens,
			"cache_creation_input_tokens": result.Usage.CacheCreationInputTokens,
		}

		if len(result.PromptFilterResults) > 0 {
			promptFilterResultMap := make(map[string]any)
			for _, promptFilterResult := range result.PromptFilterResults {
				if promptFilterResult.ContentFilterResults.Jailbreak.Detected {
					promptFilterResultMap["jailbreak"] = true
				}
			}
			// 临时变量，后面可能会修改
			generationInfo["FilterResult"] = promptFilterResultMap
		}
		if result.Usage.CompletionTokensDetails.ReasoningTokens > 0 {
			generationInfo["CompletionTokensDetails"] = map[string]int{
				"ReasoningTokens": result.Usage.CompletionTokensDetails.ReasoningTokens,
			}
		}
		if result.Usage.PromptTokensDetails.CachedTokens > 0 {
			generationInfo["PromptTokensDetails"] = map[string]int{
				"CachedTokens": result.Usage.PromptTokensDetails.CachedTokens,
			}
		}
		choices[i] = &llms.ContentChoice{
			Content:          c.Message.Content,
			Refusal:          c.Message.Refusal,
			ReasoningContent: c.Message.ReasoningContent,
			StopReason:       fmt.Sprint(c.FinishReason),
			GenerationInfo:   generationInfo,
		}

		// Legacy function call handling
		if c.FinishReason == "function_call" {
			choices[i].FuncCall = &llms.FunctionCall{
				Name:      c.Message.FunctionCall.Name,
				Arguments: c.Message.FunctionCall.Arguments,
			}
		}
		for _, tool := range c.Message.ToolCalls {
			choices[i].ToolCalls = append(choices[i].ToolCalls, llms.ToolCall{
				ID:   tool.ID,
				Type: string(tool.Type),
				FunctionCall: &llms.FunctionCall{
					Name:      tool.Function.Name,
					Arguments: tool.Function.Arguments,
				},
			})
		}
		// populate legacy single-function call field for backwards compatibility
		if len(choices[i].ToolCalls) > 0 {
			choices[i].FuncCall = choices[i].ToolCalls[0].FunctionCall
		}
	}
	response := &llms.ContentResponse{Choices: choices}
	if len(result.ExtInfo) > 0 {
		response.ExtInfo = result.ExtInfo
	}
	if len(result.Citations) > 0 {
		response.Citations = result.Citations
	}
	response.SetHeader(result.Header())
	if o.CallbacksHandler != nil {
		o.CallbacksHandler.HandleLLMGenerateContentEnd(ctx, response)
	}
	return response, nil
}

// CreateEmbedding creates embeddings for the given input texts.
func (o *LLM) CreateEmbedding(ctx context.Context, inputTexts []string) ([][]float32, error) {
	embeddings, err := o.client.CreateEmbedding(ctx, &openaiclient.EmbeddingRequest{
		Input: inputTexts,
		Model: o.client.EmbeddingModel,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create openai embeddings: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, ErrEmptyResponse
	}
	if len(inputTexts) != len(embeddings) {
		return embeddings, ErrUnexpectedResponseLength
	}
	return embeddings, nil
}

// data:[image/webp];base64,[content]
func parseData(data string) (string, string) {

	if !strings.HasPrefix(data, "data:") {
		// URI
		if strings.HasPrefix(data, "http://") || strings.HasPrefix(data, "https://") {
			return "", data
		}
		// origin data
		return "", base64.StdEncoding.EncodeToString([]byte(data))
	}

	// 去掉 "data:" 前缀
	data = strings.TrimPrefix(data, "data:")

	// 按第一个 ',' 分割，前半部分是头部（例如 image/jpeg;base64），后半部分是内容
	parts := strings.SplitN(data, ",", 2)
	if len(parts) != 2 {
		return "", data
	}

	header, content := parts[0], parts[1]

	// MIME 类型在 ';' 之前
	mimeParts := strings.SplitN(header, ";", 2)
	mimeType := mimeParts[0]

	// 检查是否包含 base64 标识
	if len(mimeParts) < 2 || mimeParts[1] != "base64" {
		return "", data
	}

	return mimeType, content
}

// ExtractToolParts extracts the tool parts from a message.
func (o *LLM) ExtractToolParts(msg *ChatMessage) ([]llms.ContentPart, []llms.ToolCall) {
	var content []llms.ContentPart
	var toolCalls []llms.ToolCall
	for _, part := range msg.MultiContent {
		switch p := part.(type) {
		case llms.TextContent:
			content = append(content, p)
		case llms.ImageURLContent:
			if o.client.GetApiType() == openaiclient.APITypeAzureDataBricks && msg.Role == RoleTool {
				mediaType, data := parseData(p.URL)
				if mediaType == "" {
					mediaType = p.MimeType
				}
				content = append(content, llms.DatabricksImageContent{
					MediaType: mediaType,
					Data:      data,
				})
			} else {
				content = append(content, p)
			}
		case llms.BinaryContent:
			content = append(content, p)
		case llms.ToolCall:
			toolCalls = append(toolCalls, p)
		}
	}
	return content, toolCalls
}

// toolFromTool converts an llms.Tool to a Tool.
func toolFromTool(t llms.Tool) (openaiclient.Tool, error) {
	tool := openaiclient.Tool{
		Type: openaiclient.ToolType(t.Type),
	}
	switch t.Type {
	case string(openaiclient.ToolTypeFunction):
		tool.Function = openaiclient.FunctionDefinition{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		}
	default:
		return openaiclient.Tool{}, fmt.Errorf("tool type %v not supported", t.Type)
	}
	return tool, nil
}

// toolCallsFromToolCalls converts a slice of llms.ToolCall to a slice of ToolCall.
func toolCallsFromToolCalls(tcs []llms.ToolCall) []openaiclient.ToolCall {
	toolCalls := make([]openaiclient.ToolCall, len(tcs))
	for i, tc := range tcs {
		toolCalls[i] = toolCallFromToolCall(tc)
	}
	return toolCalls
}

// toolCallFromToolCall converts an llms.ToolCall to a ToolCall.
func toolCallFromToolCall(tc llms.ToolCall) openaiclient.ToolCall {
	return openaiclient.ToolCall{
		ID:   tc.ID,
		Type: openaiclient.ToolType(tc.Type),
		Function: openaiclient.ToolFunction{
			Name:      tc.FunctionCall.Name,
			Arguments: tc.FunctionCall.Arguments,
		},
	}
}

func reasonEffortFromThinking(thinking *llms.Thinking) string {
	if thinking == nil {
		return ""
	}
	return thinking.ReasoningEffort
}

// getReasoningEffort returns the reasoning effort from either the direct field or the thinking field
// with the direct field taking precedence
func getReasoningEffort(opts *llms.CallOptions) string {
	if opts.ReasoningEffort != "" {
		return opts.ReasoningEffort
	}
	return reasonEffortFromThinking(opts.Thinking)
}
