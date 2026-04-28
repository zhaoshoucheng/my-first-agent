package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"log/slog"
	"strings"

	jsonrepair "github.com/RealAlexandreAI/json-repair"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/mitchellh/mapstructure"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/streaming"
)

// getSystemContentBlock 提取 system 信息
func getSystemContentBlock(parts []llms.ContentPart) ([]anthropic.TextBlockParam, error) {
	systemContent := make([]anthropic.TextBlockParam, 0)
	ephemeralCache := anthropic.NewCacheControlEphemeralParam()
	for _, part := range parts {
		switch p := part.(type) {
		case llms.TextContent:
			systemContent = append(systemContent, *anthropic.NewTextBlock(p.Text).OfText)
		case llms.ContextOption:
			if p.CacheConfig != nil && len(systemContent) > 0 {
				systemContent[len(systemContent)-1].CacheControl = ephemeralCache
			}
		default:
			return nil, errors.New("system content part must be text")
		}
	}
	return systemContent, nil
}

// Role attribute for the anthropic message.
const (
	AnthropicSystem        = "system"
	AnthropicRoleUser      = "user"
	AnthropicRoleAssistant = "assistant"
)

func getConverseAPIRole(role llms.ChatMessageType) (string, error) {
	switch role {
	case llms.ChatMessageTypeSystem:
		return AnthropicSystem, nil
	case llms.ChatMessageTypeAI:
		return AnthropicRoleAssistant, nil
	case llms.ChatMessageTypeGeneric:
		return "", errors.New("generic role not supported")
	case llms.ChatMessageTypeHuman:
		return AnthropicRoleUser, nil
	case llms.ChatMessageTypeFunction, llms.ChatMessageTypeTool:
		return AnthropicRoleUser, nil
	default:
		return "", errors.New("role not supported")
	}
}

// UpdateContentBlockCacheControl updates the CacheControl field of any ContentBlockParamUnion type
// to make it ephemeral. Returns the updated ContentBlockParamUnion.
func UpdateContentBlockCacheControl(block anthropic.ContentBlockParamUnion) anthropic.ContentBlockParamUnion {
	ephemeralCache := anthropic.NewCacheControlEphemeralParam()
	if !param.IsOmitted(block.OfText) {
		block.OfText.CacheControl = ephemeralCache
	} else if !param.IsOmitted(block.OfImage) {
		block.OfImage.CacheControl = ephemeralCache
	} else if !param.IsOmitted(block.OfToolUse) {
		block.OfToolUse.CacheControl = ephemeralCache
	} else if !param.IsOmitted(block.OfToolResult) {
		block.OfToolResult.CacheControl = ephemeralCache
	} else if !param.IsOmitted(block.OfDocument) {
		block.OfDocument.CacheControl = ephemeralCache
	}
	return block
}

// partToContentBlock part 转成 content block
func partToContentBlock(parts []llms.ContentPart) ([]anthropic.ContentBlockParamUnion, error) {
	convertedParts := make([]anthropic.ContentBlockParamUnion, 0, len(parts))
	var toolResultBlock *anthropic.ToolResultBlockParam
	for _, part := range parts {
		inToolResult := toolResultBlock != nil && toolResultBlock.ToolUseID != ""
		var out *anthropic.ContentBlockParamUnion
		switch p := part.(type) {
		case llms.TextContent:
			if inToolResult {
				toolResultBlock.Content = append(toolResultBlock.Content,
					anthropic.ToolResultBlockParamContentUnion{OfText: &anthropic.TextBlockParam{Text: p.Text}})
			} else {
				testBlock := anthropic.NewTextBlock(p.Text)
				out = &testBlock
			}
		case llms.BinaryContent:
			return nil, errors.New("binary content not supported")
		case llms.FileURLContent:
			return nil, errors.New("file url content not supported")
		case llms.ImageURLContent:
			if strings.HasPrefix(p.URL, "http") {
				block := &anthropic.ImageBlockParam{
					Source: anthropic.ImageBlockParamSourceUnion{OfURL: &anthropic.URLImageSourceParam{
						Type: constant.URL("").Default(),
						URL:  p.URL,
					}},
					Type: constant.Image("").Default(),
				}
				if inToolResult {
					toolResultBlock.Content = append(toolResultBlock.Content, anthropic.ToolResultBlockParamContentUnion{
						OfImage: block,
					})
				} else {
					out = &anthropic.ContentBlockParamUnion{
						OfImage: block,
					}
				}
			} else if strings.HasPrefix(p.URL, "data:") {
				// 处理 data URL 格式的图片
				parts := strings.SplitN(p.URL, ",", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("invalid base64 url format, example: data:image/jpeg;base64,{base64_image}")
				}

				metaParts := strings.SplitN(parts[0], ";", 2)
				if len(metaParts) != 2 || metaParts[1] != "base64" {
					return nil, fmt.Errorf("only base64 encoded data URLs are supported")
				}

				mimeType := strings.TrimPrefix(metaParts[0], "data:")
				var t anthropic.Base64ImageSourceMediaType
				switch {
				case strings.Contains(mimeType, "png"):
					t = anthropic.Base64ImageSourceMediaTypeImagePNG
				case strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg"):
					t = anthropic.Base64ImageSourceMediaTypeImageJPEG
				case strings.Contains(mimeType, "gif"):
					t = anthropic.Base64ImageSourceMediaTypeImageGIF
				case strings.Contains(mimeType, "webp"):
					t = anthropic.Base64ImageSourceMediaTypeImageWebP
				default:
					return nil, fmt.Errorf("unsupported image MIME type: %s", mimeType)
				}
				block := anthropic.NewImageBlockBase64(string(t), parts[1])
				if inToolResult {
					toolResultBlock.Content = append(toolResultBlock.Content,
						anthropic.ToolResultBlockParamContentUnion{OfImage: block.OfImage})
				} else {
					out = &block
				}
			} else {
				block := anthropic.NewImageBlockBase64(string(anthropic.Base64ImageSourceMediaTypeImageJPEG), p.URL)
				if inToolResult {
					toolResultBlock.Content = append(toolResultBlock.Content,
						anthropic.ToolResultBlockParamContentUnion{OfImage: block.OfImage})
				} else {
					out = &block
				}
			}
		case llms.ToolCall:
			input := make(map[string]any)
			err := json.Unmarshal([]byte(p.FunctionCall.Arguments), &input)
			if err != nil {
				dst, err := jsonrepair.RepairJSON(p.FunctionCall.Arguments)
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal tool call input: %w", err)
				}
				err = json.Unmarshal([]byte(dst), &input)
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal tool call input: %w", err)
				}
			}
			toolUseBlock := anthropic.NewToolUseBlock(p.ID, input, p.FunctionCall.Name)
			out = &toolUseBlock
		case llms.ToolCallResponse:
			isErr := false
			if p.IsError != nil {
				isErr = *p.IsError
			}
			toolResultBlock = &anthropic.ToolResultBlockParam{
				ToolUseID: p.ToolCallID,
				Type:      constant.ToolResult("").Default(),
				IsError:   param.NewOpt(isErr),
				Content:   []anthropic.ToolResultBlockParamContentUnion{{OfText: anthropic.NewTextBlock(p.Content).OfText}},
			}
			contentBlockParamUnion := anthropic.ContentBlockParamUnion{OfToolResult: toolResultBlock}
			out = &contentBlockParamUnion
		case llms.ContextOption:
			if p.CacheConfig != nil && len(convertedParts) > 0 {
				convertedParts[len(convertedParts)-1] = UpdateContentBlockCacheControl(convertedParts[len(convertedParts)-1])
			}
			continue
		default:
			return nil, errors.New("unsupported content part")
		}
		if out != nil {
			convertedParts = append(convertedParts, *out)
		}
	}
	return convertedParts, nil
}

func mergeSameRoleMessages(messages []llms.MessageContent) ([][]llms.MessageContent, error) {
	chunkedMessages := make([][]llms.MessageContent, 0, len(messages))
	currentChunk := make([]llms.MessageContent, 0, len(messages))
	var lastRole string
	for _, message := range messages {
		role, err := getConverseAPIRole(message.Role)
		if err != nil {
			return nil, err
		}
		if role != lastRole {
			if len(currentChunk) > 0 {
				chunkedMessages = append(chunkedMessages, currentChunk)
			}
			currentChunk = make([]llms.MessageContent, 0)
		}
		currentChunk = append(currentChunk, message)
		lastRole = role
	}

	if len(currentChunk) > 0 {
		chunkedMessages = append(chunkedMessages, currentChunk)
	}
	return chunkedMessages, nil
}

// process the input messages to anthropic supported input
// returns the input content and system prompt.
func processInputMessages(
	messages []llms.MessageContent,
) ([]anthropic.TextBlockParam, []anthropic.MessageParam, error) {
	mergedMessages, err := mergeSameRoleMessages(messages)
	if err != nil {
		return nil, nil, err
	}
	inputContents := make([]anthropic.MessageParam, 0, len(messages))
	systemContents := make([]anthropic.TextBlockParam, 0)
	for _, chunk := range mergedMessages {
		role, err := getConverseAPIRole(chunk[0].Role)
		if err != nil {
			return nil, nil, err
		}
		if role == AnthropicSystem {
			for _, message := range chunk {
				content, err := getSystemContentBlock(message.Parts)
				if err != nil {
					return nil, nil, err
				}
				systemContents = append(systemContents, content...)
			}
			continue
		}

		contentBlocks := make([]anthropic.ContentBlockParamUnion, 0, len(chunk))
		for _, message := range chunk {
			blocks, err := partToContentBlock(message.Parts)
			if err != nil {
				return nil, nil, err
			}
			contentBlocks = append(contentBlocks, blocks...)
		}
		switch role {
		case AnthropicRoleAssistant:
			inputContents = append(inputContents, anthropic.NewAssistantMessage(contentBlocks...))
		case AnthropicRoleUser:
			inputContents = append(inputContents, anthropic.NewUserMessage(contentBlocks...))
		default:
			return nil, nil, errors.New("role not supported")
		}
	}
	return systemContents, inputContents, nil
}

// handleStreamEvents handles the stream events and returns the content response.
func handleStreamEvents(
	ctx context.Context,
	streamOutput *ssestream.Stream[anthropic.MessageStreamEventUnion],
	options *llms.CallOptions,
) (*llms.ContentResponse, error) {
	if e := streamOutput.Err(); e != nil {
		return nil, e
	}
	defer func() {
		if err := streamOutput.Close(); err != nil && !errors.Is(err, context.Canceled) {
			slog.ErrorContext(ctx, "failed to close stream", "err", err)
		}
	}()

	processor := streaming.NewStreamProcessor(options)
	for streamOutput.Next() {
		event := streamOutput.Current()
		if err := processor.ProcessEvent(ctx, event); err != nil {
			return nil, err
		}
	}
	if e := streamOutput.Err(); e != nil {
		return nil, e
	}
	return processor.GetResult(), nil
}

func responseToContentResponse(resp *anthropic.Message) (*llms.ContentResponse, error) {
	if len(resp.Content) == 0 {
		return nil, ErrEmptyResponse
	}
	choices := []*llms.ContentChoice{{}}
	for _, content := range resp.Content {
		switch content.AsAny().(type) {
		case anthropic.TextBlock:
			choices[0] = &llms.ContentChoice{Content: content.Text}
		case anthropic.ToolUseBlock:
			choices[0].ToolCalls = []llms.ToolCall{{
				ID:   content.ID,
				Type: "function",
				FunctionCall: &llms.FunctionCall{
					Name:      content.Name,
					Arguments: string(content.Input),
				},
			}}
		case anthropic.ThinkingBlock:
			choices[0].ReasoningContent = content.Text
		case anthropic.RedactedThinkingBlock:
			// 暂时不处理
		default:
			return nil, fmt.Errorf("anthropic: %w: %v", ErrUnsupportedContentType, content.Type)
		}
	}

	choices[0].StopReason = string(resp.StopReason)
	choices[0].GenerationInfo = map[string]any{
		"InputTokens":                 resp.Usage.InputTokens,
		"OutputTokens":                resp.Usage.OutputTokens,
		"TotalTokens":                 resp.Usage.InputTokens + resp.Usage.OutputTokens,
		"cache_read_input_tokens":     resp.Usage.CacheReadInputTokens,
		"cache_creation_input_tokens": resp.Usage.CacheCreationInputTokens,
	}

	r := &llms.ContentResponse{
		Choices: choices,
		ID:      resp.ID,
	}
	return r, nil
}

func convertToolChoice(toolChoice any) (anthropic.ToolChoiceUnionParam, error) {
	if toolChoice == nil {
		return anthropic.ToolChoiceUnionParam{OfAuto: &anthropic.ToolChoiceAutoParam{Type: constant.Auto("").Default()}}, nil
	}
	if toolChoice, ok := toolChoice.(string); ok {
		switch toolChoice {
		case "none":
			return anthropic.ToolChoiceUnionParam{}, errors.New("tool choice none not supported")
		case "any", "required":
			return anthropic.ToolChoiceUnionParam{OfAny: &anthropic.ToolChoiceAnyParam{Type: constant.Any("").Default()}}, nil
		case "auto":
			return anthropic.ToolChoiceUnionParam{OfAuto: &anthropic.ToolChoiceAutoParam{Type: constant.Auto("").Default()}}, nil
		default:
			return anthropic.ToolChoiceUnionParam{}, errors.New("unsupported tool choice")
		}
	}
	if toolChoice, ok := toolChoice.(llms.ToolChoice); ok {
		if toolChoice.Function == nil {
			return anthropic.ToolChoiceUnionParam{OfAuto: &anthropic.ToolChoiceAutoParam{Type: constant.Auto("").Default()}}, nil
		}
		// TODO 这里可以控制并发调用
		return anthropic.ToolChoiceUnionParam{OfTool: &anthropic.ToolChoiceToolParam{Type: constant.Tool("").Default(),
			Name: toolChoice.Function.Name}}, nil
	}

	var llmsToolChoice llms.ToolChoice
	if err := mapstructure.Decode(toolChoice, &toolChoice); err != nil {
		return anthropic.ToolChoiceUnionParam{}, fmt.Errorf("failed to decode tool choice: %w", err)
	}
	return anthropic.ToolChoiceUnionParam{OfTool: &anthropic.ToolChoiceToolParam{Type: constant.Tool("").Default(),
		Name: llmsToolChoice.Function.Name}}, nil
}

func toolsToTools(tools []llms.Tool) ([]anthropic.ToolUnionParam, error) {
	toolReq := make([]anthropic.ToolUnionParam, len(tools))
	for i, tool := range tools {
		inputBytes, err := json.Marshal(tool.Function.Parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool parameters: %w, name %s", err, tool.Function.Name)
		}
		var InputSchema anthropic.ToolInputSchemaParam
		err = InputSchema.UnmarshalJSON(inputBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool parameters: %w, name %s", err, tool.Function.Name)
		}
		toolReq[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Function.Name,
				Description: anthropic.String(tool.Function.Description),
				InputSchema: InputSchema,
			},
		}
	}
	return toolReq, nil
}
