package llms

import (
	"context"
	"net/http"
)

const (
	ExtraHeaderSessionID                     = "session_id"
	ExtraHeaderXVertexAILLMRequestType       = "X-Vertex-AI-LLM-Request-Type"
	ExtraHeaderXVertexAILLMSharedRequestType = "X-Vertex-AI-LLM-Shared-Request-Type"
)

// CallOption is a function that configures a CallOptions.
type CallOption func(*CallOptions)

// CallOptions is a set of options for calling models. Not all models support
// all options.
type CallOptions struct {
	// Model is the model to use.
	Model string `json:"model"`
	// CandidateCount is the number of response candidates to generate.
	CandidateCount int `json:"candidate_count"`
	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int `json:"max_tokens"`
	// An upper bound for the number of tokens that can be generated for a completion,
	// including visible output tokens and
	// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
	MaxCompletionTokens int `json:"max_completion_tokens,omitempty"`

	Temperature float64 `json:"temperature"`
	// StopWords is a list of words to stop on.
	StopWords []string `json:"stop_words"`
	// StreamingFunc is a function to be called for each chunk of a streaming response.
	// Return an error to stop streaming early.
	StreamingFunc func(ctx context.Context, chunk []byte) error `json:"-"`
	// MessageStopFunc is an optional callback invoked when the provider emits a
	// message stop event during streaming. The payload contains the raw stop
	// event data (for example, Bedrock invocation metrics). Return an error to
	// stop further processing.
	MessageStopFunc func(ctx context.Context, payload []byte) error `json:"-"`
	// TopK is the number of tokens to consider for top-k sampling.
	TopK int `json:"top_k"`
	// TopP is the cumulative probability for top-p sampling.
	TopP float64 `json:"top_p"`
	// Seed is a seed for deterministic sampling.
	Seed int `json:"seed"`
	// MinLength is the minimum length of the generated text.
	MinLength int `json:"min_length"`
	// MaxLength is the maximum length of the generated text.
	MaxLength int `json:"max_length"`
	// N is how many chat completion choices to generate for each input message.
	N int `json:"n"`
	// RepetitionPenalty is the repetition penalty for sampling.
	RepetitionPenalty float64 `json:"repetition_penalty"`
	// FrequencyPenalty is the frequency penalty for sampling.
	FrequencyPenalty float64 `json:"frequency_penalty"`
	// PresencePenalty is the presence penalty for sampling.
	PresencePenalty float64 `json:"presence_penalty"`

	// JSONMode is a flag to enable JSON mode.
	JSONMode   bool           `json:"json"`
	JSONSchema map[string]any `json:"json_schema"`

	// DisabledFunctionNames is a list of function names to disable.
	DisabledFunctionNames []string `json:"disabled_function_names,omitempty"`

	// Thinking 控制
	Thinking *Thinking `json:"thinking,omitempty"`
	// SignatureScope scopes provider-native replay signatures to the selected backend.
	SignatureScope *SignatureScope `json:"signature_scope,omitempty"`

	// Tools is a list of tools to use. Each tool can be a specific tool or a function.
	Tools []Tool `json:"tools,omitempty"`
	// ToolChoice is the choice of tool to use, it can either be "none", "auto" (the default behavior), or a specific tool as described in the ToolChoice type.
	ToolChoice any `json:"tool_choice"`

	// Options for streaming response. Only set this when you set stream: true.
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`

	// Function defitions to include in the request.
	// Deprecated: Use Tools instead.
	Functions []FunctionDefinition `json:"functions,omitempty"`
	// FunctionCallBehavior is the behavior to use when calling functions.
	//
	// If a specific function should be invoked, use the format:
	// `{"name": "my_function"}`
	// Deprecated: Use ToolChoice instead.
	FunctionCallBehavior FunctionCallBehavior `json:"function_call,omitempty"`

	// Metadata is a map of metadata to include in the request.
	// The meaning of this field is specific to the backend in use.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// PromptCacheKey is a stable cache bucket identifier for OpenAI Responses API.
	PromptCacheKey string `json:"prompt_cache_key,omitempty"`
	// PromptCacheRetention controls how long the prompt cache should be retained.
	PromptCacheRetention string `json:"prompt_cache_retention,omitempty"`
	// ExtraHeaders adds additional headers to the request.
	ExtraHeaders http.Header            `json:"-,omitempty"`
	ExtraBody    map[string]interface{} `json:"-,omitempty"`
	// Disable the default behavior of parallel tool calls by setting it: false.
	ParallelToolCalls any `json:"parallel_tool_calls,omitempty"`
	// ParallelFunctionCallConfig is a list of parallel function calls.
	ParallelFunctionCallConfig *ParallelFunctionCallConfig `json:"parallel_function_call_config,omitempty"`
	// Prediction is a configuration for a predicted output.
	Prediction *Prediction `json:"prediction,omitempty"`
	// Verbosity controls the expansiveness of the model's replies.
	// Supported values are "low", "medium" (default), and "high".
	// Only supported by OpenAI GPT models currently.
	Verbosity string `json:"verbosity,omitempty"`
	// ReasoningEffort openai o1 and o3-mini models only
	// Constrains effort on reasoning for reasoning models.
	// Currently supported values are "low", "medium", and "high".
	// Reducing reasoning effort can result in faster responses and fewer tokens used on reasoning in a response.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	// Optional. The requested modalities of the response. Represents the set of
	// modalities that the model can return.
	ResponseModalities []string `json:"response_modalities,omitempty"`
}

type StreamOptions struct {
	// If set, an additional chunk will be streamed before the data: [DONE] message.
	// The usage field on this chunk shows the token usage statistics for the entire request,
	// and the choices field will always be an empty array.
	// All other chunks will also include a usage field, but with a null value.
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type Thinking struct {
	// ReasoningEffort openai o1 and o3-mini models only
	// Constrains effort on reasoning for reasoning models.
	//Currently supported values are low, medium, and high.
	//Reducing reasoning effort can result in faster responses and fewer tokens used on reasoning in a response.
	ReasoningEffort string

	// Anthropic  reasoning models only, if this is not set, it will be set max_token * 0.75
	BudgetTokens *int64
	// EnabledType anthropic  reasoning models only, enums: "disabled", "enabled"
	EnabledType string
	// EnableSignature enable signature for reasoning models
	EnableSignature bool
}

// Tool is a tool that can be used by the model.
type Tool struct {
	// Type is the type of the tool.
	Type string `json:"type"`
	// Function is the function to call.
	Function *FunctionDefinition `json:"function,omitempty"`
}

// FunctionDefinition is a definition of a function that can be called by the model.
type FunctionDefinition struct {
	// Name is the name of the function.
	Name string `json:"name"`
	// Description is a description of the function.
	Description string `json:"description"`
	// Parameters is a list of parameters for the function.
	Parameters any `json:"parameters,omitempty"`
}

// ToolChoice is a specific tool to use.
type ToolChoice struct {
	// Type is the type of the tool.
	Type string `json:"type"`
	// Function is the function to call (if the tool is a function).
	Function *FunctionReference `json:"function,omitempty"`
}

// FunctionReference is a reference to a function.
type FunctionReference struct {
	// Name is the name of the function.
	Name string `json:"name"`
}

// FunctionCallBehavior is the behavior to use when calling functions.
type FunctionCallBehavior string

const (
	// FunctionCallBehaviorNone will not call any functions.
	FunctionCallBehaviorNone FunctionCallBehavior = "none"
	// FunctionCallBehaviorAuto will call functions automatically.
	FunctionCallBehaviorAuto FunctionCallBehavior = "auto"
)

// ParallelFunctionCallConfig is a list of parallel function calls.
type ParallelFunctionCallConfig []ParallelFunctionCall

// ParallelFunctionCall is a list of function names.
type ParallelFunctionCall struct {
	Names []string `json:"names"`
}

// Prediction is a configuration for a predicted output.
type Prediction struct {
	Content string `json:"content"`
	Type    string `json:"type"`
}

func WithStreamOptions(includeUsage bool) CallOption {
	return func(o *CallOptions) {
		o.StreamOptions = &StreamOptions{includeUsage}
	}
}

// WithModel specifies which model name to use.
func WithModel(model string) CallOption {
	return func(o *CallOptions) {
		o.Model = model
	}
}

// WithMaxTokens specifies the max number of tokens to generate.
func WithMaxTokens(maxTokens int) CallOption {
	return func(o *CallOptions) {
		o.MaxTokens = maxTokens
	}
}

// WithMaxCompletionTokens specifies the max number of tokens to generate for a completion.
// This only used for openai client.
func WithMaxCompletionTokens(maxCompletionTokens int) CallOption {
	return func(o *CallOptions) {
		o.MaxCompletionTokens = maxCompletionTokens
	}
}

// WithCandidateCount specifies the number of response candidates to generate.
func WithCandidateCount(c int) CallOption {
	return func(o *CallOptions) {
		o.CandidateCount = c
	}
}

// WithTemperature specifies the model temperature, a hyperparameter that
// regulates the randomness, or creativity, of the AI's responses.
func WithTemperature(temperature float64) CallOption {
	return func(o *CallOptions) {
		o.Temperature = temperature
	}
}

// WithStopWords specifies a list of words to stop generation on.
func WithStopWords(stopWords []string) CallOption {
	return func(o *CallOptions) {
		o.StopWords = stopWords
	}
}

// WithOptions specifies options.
func WithOptions(options CallOptions) CallOption {
	return func(o *CallOptions) {
		(*o) = options
	}
}

// WithStreamingFunc specifies the streaming function to use.
func WithStreamingFunc(streamingFunc func(ctx context.Context, chunk []byte) error) CallOption {
	return func(o *CallOptions) {
		o.StreamingFunc = streamingFunc
	}
}

// WithMessageStopFunc specifies the callback to use for streaming message stop events.
func WithMessageStopFunc(messageStopFunc func(ctx context.Context, payload []byte) error) CallOption {
	return func(o *CallOptions) {
		o.MessageStopFunc = messageStopFunc
	}
}

// WithTopK will add an option to use top-k sampling.
func WithTopK(topK int) CallOption {
	return func(o *CallOptions) {
		o.TopK = topK
	}
}

// WithTopP	will add an option to use top-p sampling.
func WithTopP(topP float64) CallOption {
	return func(o *CallOptions) {
		o.TopP = topP
	}
}

// WithSeed will add an option to use deterministic sampling.
func WithSeed(seed int) CallOption {
	return func(o *CallOptions) {
		o.Seed = seed
	}
}

// WithMinLength will add an option to set the minimum length of the generated text.
func WithMinLength(minLength int) CallOption {
	return func(o *CallOptions) {
		o.MinLength = minLength
	}
}

// WithMaxLength will add an option to set the maximum length of the generated text.
func WithMaxLength(maxLength int) CallOption {
	return func(o *CallOptions) {
		o.MaxLength = maxLength
	}
}

// WithN will add an option to set how many chat completion choices to generate for each input message.
func WithN(n int) CallOption {
	return func(o *CallOptions) {
		o.N = n
	}
}

// WithRepetitionPenalty will add an option to set the repetition penalty for sampling.
func WithRepetitionPenalty(repetitionPenalty float64) CallOption {
	return func(o *CallOptions) {
		o.RepetitionPenalty = repetitionPenalty
	}
}

// WithFrequencyPenalty will add an option to set the frequency penalty for sampling.
func WithFrequencyPenalty(frequencyPenalty float64) CallOption {
	return func(o *CallOptions) {
		o.FrequencyPenalty = frequencyPenalty
	}
}

// WithPresencePenalty will add an option to set the presence penalty for sampling.
func WithPresencePenalty(presencePenalty float64) CallOption {
	return func(o *CallOptions) {
		o.PresencePenalty = presencePenalty
	}
}

// WithFunctionCallBehavior will add an option to set the behavior to use when calling functions.
// Deprecated: Use WithToolChoice instead.
func WithFunctionCallBehavior(behavior FunctionCallBehavior) CallOption {
	return func(o *CallOptions) {
		o.FunctionCallBehavior = behavior
	}
}

// WithFunctions will add an option to set the functions to include in the request.
// Deprecated: Use WithTools instead.
func WithFunctions(functions []FunctionDefinition) CallOption {
	return func(o *CallOptions) {
		o.Functions = functions
	}
}

// WithToolChoice will add an option to set the choice of tool to use.
// It can either be "none", "auto" (the default behavior), or a specific tool as described in the ToolChoice type.
func WithToolChoice(choice any) CallOption {
	// TODO: Add type validation for choice.
	return func(o *CallOptions) {
		o.ToolChoice = choice
	}
}

// WithTools will add an option to set the tools to use.
func WithTools(tools []Tool) CallOption {
	return func(o *CallOptions) {
		o.Tools = tools
	}
}

// WithThinking will add an option to set the thinking behavior.
func WithThinking(thinking Thinking) CallOption {
	return func(o *CallOptions) {
		o.Thinking = &thinking
	}
}

// WithSignatureScope scopes provider-native replay signatures to the selected backend.
func WithSignatureScope(scope SignatureScope) CallOption {
	return func(o *CallOptions) {
		if !scope.Valid() {
			o.SignatureScope = nil
			return
		}
		scopeCopy := scope
		o.SignatureScope = &scopeCopy
	}
}

// WithJSONMode will add an option to set the response format to JSON.
// This is useful for models that return structured data.
func WithJSONMode() CallOption {
	return func(o *CallOptions) {
		o.JSONMode = true
	}
}

// WithJSONSchema will add an option to set the JSON schema for the response.
// This only used for openai client.
func WithJSONSchema(schema map[string]any) CallOption {
	return func(o *CallOptions) {
		o.JSONSchema = schema
		o.JSONMode = true
	}
}

// WithDisabledFunctionNames will add an option to disable specific functions.
func WithDisabledFunctionNames(disabledFunctionNames []string) CallOption {
	return func(o *CallOptions) {
		o.DisabledFunctionNames = disabledFunctionNames
	}
}

// WithMetadata will add an option to set metadata to include in the request.
// The meaning of this field is specific to the backend in use.
func WithMetadata(metadata map[string]interface{}) CallOption {
	return func(o *CallOptions) {
		o.Metadata = metadata
	}
}

// WithPromptCacheKey will add an option to set the prompt cache key.
func WithPromptCacheKey(promptCacheKey string) CallOption {
	return func(o *CallOptions) {
		o.PromptCacheKey = promptCacheKey
	}
}

// WithPromptCacheRetention will add an option to set the prompt cache retention.
func WithPromptCacheRetention(promptCacheRetention string) CallOption {
	return func(o *CallOptions) {
		o.PromptCacheRetention = promptCacheRetention
	}
}

// WithExtraHeaders will add extra headers to the request.
func WithExtraHeaders(headers map[string][]string) CallOption {
	return func(o *CallOptions) {
		if headers == nil {
			return
		}
		if o.ExtraHeaders == nil {
			o.ExtraHeaders = make(http.Header)
		}
		for k, values := range headers {
			all := append(o.ExtraHeaders.Values(k), values...)
			dedup := make(map[string]struct{}, len(all))
			o.ExtraHeaders.Del(k)
			for _, v := range all {
				if _, ok := dedup[v]; ok {
					continue
				}
				dedup[v] = struct{}{}
				o.ExtraHeaders.Add(k, v)
			}
		}
	}
}

func WithExtraBody(extraBody map[string]any) CallOption {
	return func(o *CallOptions) {
		if extraBody == nil {
			return
		}
		if o.ExtraBody == nil {
			o.ExtraBody = make(map[string]any)
		}
		for k, v := range extraBody {
			o.ExtraBody[k] = v
		}
	}
}

func WithResponseModalities(responseModalities []string) CallOption {
	return func(o *CallOptions) {
		o.ResponseModalities = responseModalities
	}
}

func WithParallelToolCalls(parallelToolCalls any) CallOption {
	return func(o *CallOptions) {
		o.ParallelToolCalls = parallelToolCalls
	}
}

func WithParallelFunctionCallConfig(config *ParallelFunctionCallConfig) CallOption {
	return func(o *CallOptions) {
		o.ParallelFunctionCallConfig = config
	}
}

func WithPrediction(prediction *Prediction) CallOption {
	return func(o *CallOptions) {
		o.Prediction = prediction
	}
}

// WithVerbosity will add an option to set the verbosity level for the model's replies.
// Supported values are "low", "medium" (default), and "high".
// Only supported by OpenAI GPT models currently.
func WithVerbosity(verbosity string) CallOption {
	return func(o *CallOptions) {
		o.Verbosity = verbosity
	}
}

// WithReasoningEffort will add an option to set the reasoning effort for reasoning models.
// Supported values are "low", "medium", and "high".
// Only supported by OpenAI o1 and o3-mini models currently.
func WithReasoningEffort(reasoningEffort string) CallOption {
	return func(o *CallOptions) {
		o.ReasoningEffort = reasoningEffort
	}
}
