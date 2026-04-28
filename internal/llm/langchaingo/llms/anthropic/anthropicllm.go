package anthropic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/thinking"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/callbacks"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
)

var (
	ErrEmptyResponse            = errors.New("no response")
	ErrMissingToken             = errors.New("missing the Anthropic API key, set it in the ANTHROPIC_API_KEY environment variable")
	ErrUnexpectedResponseLength = errors.New("unexpected length of response")
	ErrInvalidContentType       = errors.New("invalid content type")
	ErrUnsupportedMessageType   = errors.New("unsupported message type")
	ErrUnsupportedContentType   = errors.New("unsupported content type")
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

type LLM struct {
	CallbacksHandler callbacks.Handler
	client           *anthropic.Client
	options          options
}

var _ llms.Model = (*LLM)(nil)

// New returns a new Anthropic LLM.
func New(opts ...Option) (*LLM, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	c := anthropic.NewClient(o.initOptions...)
	return &LLM{client: &c, options: *o}, nil
}

// Call requests a completion for the given prompt.
func (o *LLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return llms.GenerateFromSinglePrompt(ctx, o, prompt, options...)
}

// GenerateContent implements the Model interface.
func (o *LLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if o.CallbacksHandler != nil {
		o.CallbacksHandler.HandleLLMGenerateContentStart(ctx, messages)
	}

	opts := &llms.CallOptions{}
	for _, opt := range options {
		opt(opts)
	}
	if opts.Model == "" {
		opts.Model = o.options.model
	}
	if opts.MaxTokens == 0 {
		opts.MaxTokens = 4096
	}
	return generateMessagesContent(ctx, o, messages, opts)
}

func generateMessagesContent(ctx context.Context, o *LLM, messages []llms.MessageContent, opts *llms.CallOptions) (*llms.ContentResponse, error) {
	systemPrompt, chatMessages, err := processInputMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("anthropic: failed to process messages: %w", err)
	}

	params := anthropic.MessageNewParams{
		Messages: chatMessages,
		Model:    anthropic.Model(opts.Model),
	}
	apcThink := thinking.ToAnthropicThinking(opts.Thinking)
	if apcThink != nil {
		params.Thinking = *apcThink
	}
	if opts.Temperature > 0 {
		params.Temperature = anthropic.Float(opts.Temperature)
	}
	if opts.TopK > 0 {
		params.TopK = anthropic.Int(int64(opts.TopK))
	}
	if opts.TopP > 0 {
		params.TopP = anthropic.Float(opts.TopP)
	}
	if len(systemPrompt) > 0 {
		params.System = systemPrompt
	}
	if len(opts.StopWords) > 0 {
		params.StopSequences = opts.StopWords
	}
	if opts.MaxTokens > 0 {
		params.MaxTokens = int64(opts.MaxTokens)
	}
	if len(opts.Tools) > 0 {
		params.Tools, err = toolsToTools(opts.Tools)
		if err != nil {
			return nil, fmt.Errorf("anthropic: failed to convert tools: %w", err)
		}
		choice, e := convertToolChoice(opts.ToolChoice)
		if e != nil {
			return nil, e
		}
		if parallelToolCalls, ok := opts.ParallelToolCalls.(bool); ok && !parallelToolCalls {
			if choice.OfAny != nil {
				choice.OfAny.DisableParallelToolUse = anthropic.Bool(true)
			}
			if choice.OfTool != nil {
				choice.OfTool.DisableParallelToolUse = anthropic.Bool(true)
			}
			if choice.OfAuto != nil {
				choice.OfAuto.DisableParallelToolUse = anthropic.Bool(true)
			}
		}
		params.ToolChoice = choice
	}

	// Add metadata support
	if opts.Metadata != nil {
		if userID, exists := opts.Metadata["user_id"]; exists {
			if userIDStr, ok := userID.(string); ok && userIDStr != "" {
				params.Metadata = anthropic.MetadataParam{
					UserID: anthropic.String(userIDStr),
				}
			}
		}
	}
	if len(opts.ExtraBody) > 0 {
		params.SetExtraFields(opts.ExtraBody)
	}

	sdkOpts := []option.RequestOption{}
	if opts.ExtraHeaders != nil {
		for k, v := range opts.ExtraHeaders {
			sdkOpts = append(sdkOpts, option.WithHeader(k, strings.Join(v, ",")))
		}
	}

	if opts.StreamingFunc != nil {
		stream := o.client.Messages.NewStreaming(ctx, params, sdkOpts...)
		return handleStreamEvents(ctx, stream, opts)
	}

	result, err := o.client.Messages.New(ctx, params, option.WithRequestTimeout(time.Second*60*5))
	if err != nil {
		if o.CallbacksHandler != nil {
			o.CallbacksHandler.HandleLLMError(ctx, err)
		}
		return nil, fmt.Errorf("anthropic: failed to create message: %w", err)
	}
	return responseToContentResponse(result)
}
