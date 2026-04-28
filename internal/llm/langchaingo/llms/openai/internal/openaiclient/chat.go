package openaiclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
)

const (
	defaultChatModel = "gpt-3.5-turbo"
)

var ErrContentExclusive = errors.New("only one of Content / MultiContent allowed in message")

type StreamOptions struct {
	// If set, an additional chunk will be streamed before the data: [DONE] message.
	// The usage field on this chunk shows the token usage statistics for the entire request,
	// and the choices field will always be an empty array.
	// All other chunks will also include a usage field, but with a null value.
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ChatRequest is a request to complete a chat completion..
type ChatRequest struct {
	Model       string         `json:"model"`
	Messages    []*ChatMessage `json:"messages"`
	Temperature float64        `json:"temperature,omitempty"`
	TopP        float64        `json:"top_p,omitempty"`
	// This value is now deprecated in favor of `max_completion_tokens`, and is not
	// compatible with
	// [o1 series models](https://platform.openai.com/docs/guides/reasoning).
	MaxTokens int `json:"max_tokens,omitempty"`
	// An upper bound for the number of tokens that can be generated for a completion,
	// including visible output tokens and
	// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
	MaxCompletionTokens int      `json:"max_completion_tokens,omitempty"`
	N                   int      `json:"n,omitempty"`
	StopWords           []string `json:"stop,omitempty"`
	Stream              bool     `json:"stream,omitempty"`
	FrequencyPenalty    float64  `json:"frequency_penalty,omitempty"`
	PresencePenalty     float64  `json:"presence_penalty,omitempty"`
	Seed                int      `json:"seed,omitempty"`

	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	// ResponseFormat is the format of the response.
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`

	// LogProbs indicates whether to return log probabilities of the output tokens or not.
	// If true, returns the log probabilities of each output token returned in the content of message.
	// This option is currently not available on the gpt-4-vision-preview model.
	LogProbs bool `json:"logprobs,omitempty"`
	// TopLogProbs is an integer between 0 and 5 specifying the number of most likely tokens to return at each
	// token position, each with an associated log probability.
	// logprobs must be set to true if this parameter is used.
	TopLogProbs int `json:"top_logprobs,omitempty"`

	Tools []Tool `json:"tools,omitempty"`
	// This can be either a string or a ToolChoice object.
	// If it is a string, it should be one of 'none', or 'auto', otherwise it should be a ToolChoice object specifying a specific tool to use.
	ToolChoice any `json:"tool_choice,omitempty"`

	// Options for streaming response. Only set this when you set stream: true.
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`

	// StreamingFunc is a function to be called for each chunk of a streaming response.
	// Return an error to stop streaming early.
	StreamingFunc func(ctx context.Context, chunk []byte) error `json:"-"`
	ExtraBody     map[string]any                                `json:"-"`

	// Deprecated: use Tools instead.
	Functions []FunctionDefinition `json:"functions,omitempty"`
	// Deprecated: use ToolChoice instead.
	FunctionCallBehavior FunctionCallBehavior `json:"function_call,omitempty"`

	// ExtraHeaders adds additional headers to the request.
	ExtraHeaders http.Header `json:"-"`

	// Metadata allows you to specify additional information that will be passed to the model.
	Metadata map[string]any `json:"metadata,omitempty"`
	// PromptCacheKey buckets matching prompts for prompt caching.
	PromptCacheKey string `json:"prompt_cache_key,omitempty"`
	// PromptCacheRetention requests an extended prompt cache retention policy.
	PromptCacheRetention string `json:"prompt_cache_retention,omitempty"`

	// Prediction is a configuration for a predicted output.
	Prediction *llms.Prediction `json:"prediction,omitempty"`

	// Verbosity controls the expansiveness of the model's replies.
	// Supported values are "low", "medium" (default), and "high".
	Verbosity string `json:"verbosity,omitempty"`

	// Disable the default behavior of parallel tool calls by setting it: false.
	ParallelToolCalls any `json:"parallel_tool_calls,omitempty"`
}

// ToolType is the type of a tool.
type ToolType string

const (
	ToolTypeFunction ToolType = "function"
)

// Tool is a tool to use in a chat request.
type Tool struct {
	Type     ToolType           `json:"type"`
	Function FunctionDefinition `json:"function,omitempty"`
}

// ToolChoice is a choice of a tool to use.
type ToolChoice struct {
	Type     ToolType     `json:"type"`
	Function ToolFunction `json:"function,omitempty"`
}

// ToolFunction is a function to be called in a tool choice.
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall is a call to a tool.
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     ToolType     `json:"type"`
	Function ToolFunction `json:"function,omitempty"`
}

// ResponseFormat is the format of the response.
type ResponseFormat struct {
	Type       string         `json:"type"`
	JSONSchema map[string]any `json:"json_schema,omitempty"`
}

// CacheControl https://openrouter.ai/docs/features/prompt-caching#anthropic-claude
type CacheControl struct {
	// ephemeral
	Type string `json:"type"`
}

// ChatMessage is a message in a chat request.
type ChatMessage struct { //nolint:musttag
	// The role of the author of this message. One of system, user, assistant, function, or tool.
	Role string

	// The content of the message.
	// This field is mutually exclusive with MultiContent.
	Content string `json:"content,omitempty"`

	// Deepseek generated reasoning content.
	ReasoningContent string

	// The refusal message generated by the model.
	Refusal string

	// MultiContent is a list of content parts to use in the message.
	MultiContent []llms.ContentPart

	// The name of the author of this message. May contain a-z, A-Z, 0-9, and underscores,
	// with a maximum length of 64 characters.
	Name string

	// ToolCalls is a list of tools that were called in the message.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// FunctionCall represents a function call that was made in the message.
	// Deprecated: use ToolCalls instead.
	FunctionCall *FunctionCall

	// ToolCallID is the ID of the tool call this message is for.
	// Only present in tool messages.
	ToolCallID string `json:"tool_call_id,omitempty"`

	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

func (m ChatMessage) MarshalJSON() ([]byte, error) {
	if m.Content != "" && m.MultiContent != nil {
		return nil, ErrContentExclusive
	}
	if text, ok := isSingleTextContent(m.MultiContent); ok {
		m.Content = text
		m.MultiContent = nil
	}
	if len(m.MultiContent) > 0 {
		msg := struct {
			Role             string             `json:"role"`
			Content          string             `json:"-"`
			ReasoningContent string             `json:"reasoning_content,omitempty"`
			Refusal          string             `json:"refusal,omitempty"`
			MultiContent     []llms.ContentPart `json:"content,omitempty"`
			Name             string             `json:"name,omitempty"`
			ToolCalls        []ToolCall         `json:"tool_calls,omitempty"`

			// Deprecated: use ToolCalls instead.
			FunctionCall *FunctionCall `json:"function_call,omitempty"`

			// ToolCallID is the ID of the tool call this message is for.
			// Only present in tool messages.
			ToolCallID   string        `json:"tool_call_id,omitempty"`
			CacheControl *CacheControl `json:"cache_control,omitempty"`
		}(m)
		return json.Marshal(msg)
	}
	msg := struct {
		Role             string             `json:"role"`
		Content          string             `json:"content,omitempty"`
		ReasoningContent string             `json:"reasoning_content,omitempty"`
		Refusal          string             `json:"refusal,omitempty"`
		MultiContent     []llms.ContentPart `json:"-"`
		Name             string             `json:"name,omitempty"`
		ToolCalls        []ToolCall         `json:"tool_calls,omitempty"`
		// Deprecated: use ToolCalls instead.
		FunctionCall *FunctionCall `json:"function_call,omitempty"`

		// ToolCallID is the ID of the tool call this message is for.
		// Only present in tool messages.
		ToolCallID   string        `json:"tool_call_id,omitempty"`
		CacheControl *CacheControl `json:"cache_control,omitempty"`
	}(m)
	return json.Marshal(msg)
}

func isSingleTextContent(parts []llms.ContentPart) (string, bool) {
	if len(parts) != 1 {
		return "", false
	}
	tc, isText := parts[0].(llms.TextContent)
	return tc.Text, isText
}

func (m *ChatMessage) UnmarshalJSON(data []byte) error {
	msg := struct {
		Role             string             `json:"role"`
		Content          string             `json:"content"`
		ReasoningContent string             `json:"reasoning_content"`
		Refusal          string             `json:"refusal,omitempty"`
		MultiContent     []llms.ContentPart `json:"-"` // not expected in response
		Name             string             `json:"name,omitempty"`
		ToolCalls        []ToolCall         `json:"tool_calls,omitempty"`
		// Deprecated: use ToolCalls instead.
		FunctionCall *FunctionCall `json:"function_call,omitempty"`

		// ToolCallID is the ID of the tool call this message is for.
		// Only present in tool messages.
		ToolCallID   string        `json:"tool_call_id,omitempty"`
		CacheControl *CacheControl `json:"cache_control,omitempty"`
	}{}
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return err
	}
	*m = ChatMessage(msg)
	return nil
}

type TopLogProbs struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
	Bytes   []byte  `json:"bytes,omitempty"`
}

// LogProb represents the probability information for a token.
type LogProb struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
	Bytes   []byte  `json:"bytes,omitempty"` // Omitting the field if it is null
	// TopLogProbs is a list of the most likely tokens and their log probability, at this token position.
	// In rare cases, there may be fewer than the number of requested top_logprobs returned.
	TopLogProbs []TopLogProbs `json:"top_logprobs"`
}

// LogProbs is the top-level structure containing the log probability information.
type LogProbs struct {
	// Content is a list of message content tokens with log probability information.
	Content []LogProb `json:"content"`
}

type FinishReason string

const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonFunctionCall  FinishReason = "function_call"
	FinishReasonToolCalls     FinishReason = "tool_calls"
	FinishReasonContentFilter FinishReason = "content_filter"
	FinishReasonNull          FinishReason = "null"
)

func (r FinishReason) MarshalJSON() ([]byte, error) {
	if r == FinishReasonNull || r == "" {
		return []byte("null"), nil
	}
	return []byte(`"` + string(r) + `"`), nil // best effort to not break future API changes
}

// ChatCompletionChoice is a choice in a chat response.
type ChatCompletionChoice struct {
	Index        int          `json:"index"`
	Message      ChatMessage  `json:"message"`
	FinishReason FinishReason `json:"finish_reason"`
	LogProbs     *LogProbs    `json:"logprobs,omitempty"`
}

type CompletionTokensDetails struct {
	ReasoningTokens          int `json:"reasoning_tokens"`
	AudioTokens              int `json:"audio_tokens"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
}

type PromptTokensDetails struct {
	AudioTokens  int `json:"audio_tokens"`
	CachedTokens int `json:"cached_tokens"`
}

// ChatUsage is the usage of a chat completion request.
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	// CompletionTokensDetails is the details of the completion tokens.
	CompletionTokensDetails CompletionTokensDetails `json:"completion_tokens_details,omitempty"`
	PromptTokensDetails     PromptTokensDetails     `json:"prompt_tokens_details,omitempty"`
	//databricks
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type PromptFilterResult struct {
	PromptIndex          int `json:"prompt_index"`
	ContentFilterResults struct {
		Jailbreak struct {
			Filtered bool   `json:"filtered"`
			Detected bool   `json:"detected"`
			Severity string `json:"severity,omitempty"`
		} `json:"jailbreak"`
	} `json:"content_filter_results"`
}

// ChatCompletionResponse is a response to a chat request.
type ChatCompletionResponse struct {
	header http.Header

	ExtInfo map[llms.ExtInfoKey]any `json:"-"`

	ID                  string                  `json:"id,omitempty"`
	Created             int64                   `json:"created,omitempty"`
	Choices             []*ChatCompletionChoice `json:"choices,omitempty"`
	Model               string                  `json:"model,omitempty"`
	Object              string                  `json:"object,omitempty"`
	PromptFilterResults []PromptFilterResult    `json:"prompt_filter_results,omitempty"`
	Usage               ChatUsage               `json:"usage,omitempty"`
	SystemFingerprint   string                  `json:"system_fingerprint"`

	// This field is not an official OpenAI field, only useful for perplexity sonar model.
	Citations []string `json:"citations,omitempty"`
}

func (r *ChatCompletionResponse) Header() http.Header {
	return r.header
}

// StreamedChatResponsePayload is a chunk from the stream.
type StreamedChatResponsePayload struct {
	ID      string  `json:"id,omitempty"`
	Created float64 `json:"created,omitempty"`
	Model   string  `json:"model,omitempty"`
	Object  string  `json:"object,omitempty"`
	Choices []struct {
		Index float64 `json:"index,omitempty"`
		Delta struct {
			Role             string        `json:"role,omitempty"`
			Content          string        `json:"content,omitempty"`
			ReasoningContent string        `json:"reasoning_content,omitempty"`
			FunctionCall     *FunctionCall `json:"function_call,omitempty"`
			// ToolCalls is a list of tools that were called in the message.
			ToolCalls []*ToolCall `json:"tool_calls,omitempty"`
			Refusal   string      `json:"refusal,omitempty"`
		} `json:"delta,omitempty"`
		FinishReason FinishReason `json:"finish_reason,omitempty"`
	} `json:"choices,omitempty"`
	SystemFingerprint   string               `json:"system_fingerprint"`
	PromptFilterResults []PromptFilterResult `json:"prompt_filter_results,omitempty"`

	// An optional field that will only be present when you set stream_options: {"include_usage": true} in your request.
	// When present, it contains a null value except for the last chunk which contains the token usage statistics
	// for the entire request.
	Usage *ChatUsage `json:"usage,omitempty"`
	Error error      `json:"-"` // use for error handling only

	// This field is not an official OpenAI field, only useful for perplexity sonar model.
	Citations []string `json:"citations,omitempty"`
}

func (p *StreamedChatResponsePayload) UnmarshalJSON(data []byte) error {
	type Alias StreamedChatResponsePayload
	aux := (*Alias)(p)

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	reasoningList := gjson.GetBytes(data, "choices.#.delta.reasoning").Array()
	// both length must equal
	if len(reasoningList) != len(aux.Choices) {
		return nil
	}
	for i := range reasoningList {
		reasoning := reasoningList[i].String()
		if reasoning != "" {
			p.Choices[i].Delta.ReasoningContent = reasoning
		}
	}

	return nil
}

// FunctionDefinition is a definition of a function that can be called by the model.
type FunctionDefinition struct {
	// Name is the name of the function.
	Name string `json:"name"`
	// Description is a description of the function.
	Description string `json:"description,omitempty"`
	// Parameters is a list of parameters for the function.
	Parameters any `json:"parameters"`
}

// FunctionCallBehavior is the behavior to use when calling functions.
type FunctionCallBehavior string

const (
	// FunctionCallBehaviorUnspecified is the empty string.
	FunctionCallBehaviorUnspecified FunctionCallBehavior = ""
	// FunctionCallBehaviorNone will not call any functions.
	FunctionCallBehaviorNone FunctionCallBehavior = "none"
	// FunctionCallBehaviorAuto will call functions automatically.
	FunctionCallBehaviorAuto FunctionCallBehavior = "auto"
)

// FunctionCall is a call to a function.
type FunctionCall struct {
	// Name is the name of the function to call.
	Name string `json:"name"`
	// Arguments is the set of arguments to pass to the function.
	Arguments string `json:"arguments"`
}

func setExtInfoFromHeader(resp *ChatCompletionResponse, header http.Header) {
	if resp == nil || header == nil {
		return
	}

	set := func(key llms.ExtInfoKey, value string) {
		if value == "" {
			return
		}
		if resp.ExtInfo == nil {
			resp.ExtInfo = make(map[llms.ExtInfoKey]any)
		}
		resp.ExtInfo[key] = value
	}

	set(llms.ExtInfoKeyAzureRequestID, header.Get("X-Request-Id"))
	set(llms.ExtInfoKeyAzureAPIMRequestID, header.Get("Apim-Request-Id"))
	set(llms.ExtInfoKeyAzureModelSession, header.Get("Azureml-Model-Session"))
	set(llms.ExtInfoKeyFireworksGenerationQueueDuration, header.Get("Fireworks-Generation-Queue-Duration"))
	set(llms.ExtInfoKeyFireworksPrefillDuration, header.Get("Fireworks-Prefill-Duration"))
	set(llms.ExtInfoKeyFireworksPrefillQueueDuration, header.Get("Fireworks-Prefill-Queue-Duration"))
	set(llms.ExtInfoKeyFireworksServerTimeToFirstToken, header.Get("Fireworks-Server-Time-To-First-Token"))
	set(llms.ExtInfoKeyFireworksTokenizerDuration, header.Get("Fireworks-Tokenizer-Duration"))
	set(llms.ExtInfoKeyFireworksTokenizerQueueDuration, header.Get("Fireworks-Tokenizer-Queue-Duration"))
}

func mergeExtraBodyFields(payloadBytes []byte, extraBody map[string]any) ([]byte, error) {
	if len(extraBody) == 0 {
		return payloadBytes, nil
	}

	payload := make(map[string]any)
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, err
	}
	for key, value := range extraBody {
		payload[key] = value
	}

	return json.Marshal(payload)
}

func (c *Client) createChat(ctx context.Context, payload *ChatRequest) (*ChatCompletionResponse, error) {
	if payload.StreamingFunc != nil {
		payload.Stream = true
		if payload.StreamOptions == nil && c.apiType != APITypeAzureDataBricks {
			// Azure un-support stream_options
			payload.StreamOptions = &StreamOptions{IncludeUsage: true}
		}
	}
	// Build request payload

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payloadBytes, err = mergeExtraBodyFields(payloadBytes, payload.ExtraBody)
	if err != nil {
		return nil, err
	}

	// Build request
	body := bytes.NewReader(payloadBytes)
	if c.baseURL == "" {
		c.baseURL = defaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.buildURL("/chat/completions", payload.Model), body)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)
	c.setExtraHeaders(req, payload.ExtraHeaders)
	if payload.Stream {
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
	}

	// Send request
	r, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = r.Body.Close()
	}()

	baseResponse := &ChatCompletionResponse{
		header: r.Header,
	}
	setExtInfoFromHeader(baseResponse, r.Header)

	if r.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("API returned unexpected status code: %d", r.StatusCode)

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return baseResponse, fmt.Errorf("%s: failed to read response body: %w", msg, err)
		}

		var errResp errorMessage
		if c.apiType == APITypeAzureDataBricks {
			err = json.Unmarshal(bodyBytes, &errResp.Error)
		} else {
			err = json.Unmarshal(bodyBytes, &errResp)
		}
		if err != nil {
			return baseResponse, fmt.Errorf("%s: %s", msg, string(bodyBytes)) // nolint:goerr113
		}

		return baseResponse, fmt.Errorf("%s: %s", msg, errResp.messageWithDetails()) // nolint:goerr113
	}
	if payload.StreamingFunc != nil {
		resp, err := parseStreamingChatResponse(ctx, r, payload)
		if resp == nil {
			resp = baseResponse
		}
		if resp.header == nil {
			resp.header = r.Header
		}
		setExtInfoFromHeader(resp, r.Header)
		return resp, err
	}
	// Parse response
	if err := json.NewDecoder(r.Body).Decode(baseResponse); err != nil {
		return baseResponse, err
	}
	return baseResponse, nil
}

func parseStreamingChatResponse(ctx context.Context, r *http.Response, payload *ChatRequest) (*ChatCompletionResponse,
	error,
) { //nolint:cyclop,lll
	scanner := bufio.NewScanner(r.Body)
	responseChan := make(chan StreamedChatResponsePayload)
	go func() {
		defer close(responseChan)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			// compatible comment line
			// https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#examples
			if strings.HasPrefix(line, ":") {
				continue
			}

			data := strings.TrimPrefix(line, "data:") // here use `data:` instead of `data: ` for compatibility
			data = strings.TrimSpace(data)
			if data == "[DONE]" {
				return
			}
			var streamPayload StreamedChatResponsePayload
			err := json.NewDecoder(bytes.NewReader([]byte(data))).Decode(&streamPayload)
			if err != nil {
				streamPayload.Error = fmt.Errorf("error decoding streaming response: %w", err)
				responseChan <- streamPayload
				return
			}
			responseChan <- streamPayload
		}
		if err := scanner.Err(); err != nil {
			// do not return error directly, because it will be handled by the caller
			if regexp.MustCompile(`stream ID \d+; CANCEL; received from peer`).MatchString(err.Error()) {
				slog.ErrorContext(ctx, "stream closed unexpectedly",
					slog.Any("error", err), slog.String("request_id", getRequestID(r)))
			} else {
				responseChan <- StreamedChatResponsePayload{Error: fmt.Errorf("error reading streaming response: %w", err)}
			}
			return
		}
	}()
	// Combine response
	resp, err := combineStreamingChatResponse(ctx, payload, responseChan)
	if resp != nil {
		resp.header = r.Header
	}
	return resp, err
}

func combineStreamingChatResponse(
	ctx context.Context,
	payload *ChatRequest,
	responseChan chan StreamedChatResponsePayload,
) (*ChatCompletionResponse, error) {
	response := ChatCompletionResponse{
		Choices: []*ChatCompletionChoice{
			{},
		},
	}

	for streamResponse := range responseChan {
		if streamResponse.Error != nil {
			return nil, streamResponse.Error
		}

		if streamResponse.Usage != nil {
			response.Usage.CompletionTokens = streamResponse.Usage.CompletionTokens
			response.Usage.PromptTokens = streamResponse.Usage.PromptTokens
			response.Usage.TotalTokens = streamResponse.Usage.TotalTokens
			response.Usage.PromptTokensDetails = streamResponse.Usage.PromptTokensDetails
			response.Usage.CompletionTokensDetails = streamResponse.Usage.CompletionTokensDetails
			response.Usage.CacheCreationInputTokens = streamResponse.Usage.CacheCreationInputTokens
			response.Usage.CacheReadInputTokens = streamResponse.Usage.CacheReadInputTokens
		}

		if len(streamResponse.PromptFilterResults) > 0 {
			response.PromptFilterResults = append(response.PromptFilterResults, streamResponse.PromptFilterResults...)
		}

		if len(streamResponse.Choices) == 0 {
			continue
		}
		choice := streamResponse.Choices[0]
		chunk := []byte(choice.Delta.Content)
		response.Choices[0].Message.Content += choice.Delta.Content
		response.Choices[0].FinishReason = choice.FinishReason
		if len(choice.Delta.Refusal) > 0 {
			response.Choices[0].Message.Refusal += choice.Delta.Refusal
			chunk = []byte(fmt.Sprintf(`{"refusal":"%s", "loc":"refusal"}`, choice.Delta.Refusal))
		}
		if len(choice.Delta.ReasoningContent) > 0 {
			chunk, _ = json.Marshal(llms.StreamResponseField{
				Key:   "reasoning_content",
				Value: choice.Delta.ReasoningContent,
			})
			response.Choices[0].Message.ReasoningContent += choice.Delta.ReasoningContent
		}

		if len(streamResponse.Citations) > 0 {
			// both content and citations are present probably
			chunk, _ = json.Marshal(struct {
				Text      string   `json:"text"`
				Citations []string `json:"citations"`
				Loc       string   `json:"loc"`
			}{
				Text:      choice.Delta.Content,
				Citations: streamResponse.Citations,
				Loc:       "citations",
			})
			response.Citations = streamResponse.Citations
		}

		if choice.Delta.FunctionCall != nil {
			chunk = updateFunctionCall(response.Choices[0].Message, choice.Delta.FunctionCall)
		}

		if len(choice.Delta.ToolCalls) > 0 {
			chunk, response.Choices[0].Message.ToolCalls = updateToolCalls(response.Choices[0].Message.ToolCalls,
				choice.Delta.ToolCalls)
		}

		if payload.StreamingFunc != nil {
			err := payload.StreamingFunc(ctx, chunk)
			if err != nil {
				return nil, fmt.Errorf("streaming func returned an error: %w", err)
			}
		}
	}
	return &response, nil
}

func updateFunctionCall(message ChatMessage, functionCall *FunctionCall) []byte {
	if message.FunctionCall == nil {
		message.FunctionCall = functionCall
	} else {
		message.FunctionCall.Arguments += functionCall.Arguments
	}
	chunk, _ := json.Marshal(message.FunctionCall) // nolint:errchkjson
	return chunk
}

func updateToolCalls(tools []ToolCall, delta []*ToolCall) ([]byte, []ToolCall) {
	if len(delta) == 0 {
		return []byte{}, tools
	}
	for _, t := range delta {
		// if we have arguments append to the last Tool call
		if t.ID == `` && t.Function.Arguments != `` {
			lindex := len(tools) - 1
			if lindex < 0 {
				continue
			}

			tools[lindex].Function.Arguments += t.Function.Arguments
			continue
		}

		// Otherwise, this is a new tool call, append that to the stack
		tools = append(tools, *t)
	}

	chunk, _ := json.Marshal(delta) // nolint:errchkjson

	return chunk, tools
}

// StreamingChatResponseTools is a helper function to append tool calls to the stack.
func StreamingChatResponseTools(tools []ToolCall, delta []*ToolCall) ([]byte, []ToolCall) {
	if len(delta) == 0 {
		return []byte{}, tools
	}
	for _, t := range delta {
		// if we have arguments append to the last Tool call
		if t.Type == `` && t.Function.Arguments != `` {
			lindex := len(tools) - 1
			if lindex < 0 {
				continue
			}

			tools[lindex].Function.Arguments += t.Function.Arguments
			continue
		}

		// Otherwise, this is a new tool call, append that to the stack
		tools = append(tools, *t)
	}

	chunk, _ := json.Marshal(delta) // nolint:errchkjson

	return chunk, tools
}
