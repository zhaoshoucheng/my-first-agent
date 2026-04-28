package llms

import (
	"encoding/base64"
	"fmt"
	"io"
)

// MessageContent is the content of a message sent to a LLM. It has a role and a
// sequence of parts. For example, it can represent one message in a chat
// session sent by the user, in which case Role will be
// ChatMessageTypeHuman and Parts will be the sequence of items sent in
// this specific message.
type MessageContent struct {
	Role  ChatMessageType
	Parts []ContentPart
}

// TextPart creates TextContent from a given string.
func TextPart(s string) TextContent {
	return TextContent{Text: s}
}

// BinaryPart creates a new BinaryContent from the given MIME type (e.g.
// "image/png" and binary data).
func BinaryPart(mime string, data []byte) BinaryContent {
	return BinaryContent{
		MIMEType: mime,
		Data:     data,
	}
}

// ImageURLPart creates a new ImageURLContent from the given URL.
func ImageURLPart(url string) ImageURLContent {
	return ImageURLContent{
		URL: url,
	}
}

// FileURLPart creates a new FileURLContent from the given URL.
func FileURLPart(mimeType, url string) FileURLContent {
	return FileURLContent{
		MimeType: mimeType,
		URL:      url,
	}
}

// FileURLContent is content with an URL pointing to an file.
type FileURLContent struct {
	MimeType string `json:"mime_type"`
	URL      string `json:"url"`
}

func (FileURLContent) isPart() {}

// ImageURLWithDetailPart creates a new ImageURLContent from the given URL and detail.
func ImageURLWithDetailPart(url string, detail string) ImageURLContent {
	return ImageURLContent{
		URL:    url,
		Detail: detail,
	}
}

// ContentPart is an interface all parts of content have to implement.
type ContentPart interface {
	isPart()
}

// TextContent is content with some text.
type TextContent struct {
	Text      string
	Signature string
}

func (tc TextContent) String() string {
	return tc.Text
}

func (TextContent) isPart() {}

type DatabricksImageContent struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

func (dic DatabricksImageContent) String() string {
	return dic.Data
}

func (DatabricksImageContent) isPart() {}

// ThinkingContent stores reasoning and provider signature so callers can
// reconstruct interleaved thinking where available (e.g. Anthropic, Gemini).
type ThinkingContent struct {
	Thinking  string
	Signature string
}

func (ThinkingContent) isPart() {}

// ImageURLContent is content with an URL pointing to an image.
type ImageURLContent struct {
	URL      string `json:"url"`
	Detail   string `json:"detail,omitempty"` // Detail is the detail of the image, e.g. "low", "high".
	MimeType string `json:"mime_type"`
}

func (iuc ImageURLContent) String() string {
	return iuc.URL
}

func (ImageURLContent) isPart() {}

// BinaryContent is content holding some binary data with a MIME type.
type BinaryContent struct {
	MIMEType string
	Data     []byte
}

func (bc BinaryContent) String() string {
	base64Encoded := base64.StdEncoding.EncodeToString(bc.Data)
	return "data:" + bc.MIMEType + ";base64," + base64Encoded
}

func (BinaryContent) isPart() {}

// FileContent is content with some file.
type FileContent struct {
	// file url or file base64 data
	FileData string `json:"file_data,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	Filename string `json:"filename,omitempty"`
}

func (FileContent) isPart() {}

// FunctionCall is the name and arguments of a function call.
type FunctionCall struct {
	// The name of the function to call.
	Name string `json:"name"`
	// The arguments to pass to the function, as a JSON string.
	Arguments string `json:"arguments"`
}

// ToolCall is a call to a tool (as requested by the model) that should be executed.
type ToolCall struct {
	// ID is the unique identifier of the tool call.
	ID string `json:"id"`
	// Type is the type of the tool call. Typically, this would be "function".
	Type string `json:"type"`
	// FunctionCall is the function call to be executed.
	FunctionCall *FunctionCall `json:"function,omitempty"`
	Signature    string        `json:"signature,omitempty"`
}

func (ToolCall) isPart() {}

// ToolCallResponse is the response returned by a tool call.
type ToolCallResponse struct {
	// ToolCallID is the ID of the tool call this response is for.
	ToolCallID string `json:"tool_call_id"`
	// Name is the name of the tool that was called.
	Name string `json:"name"`
	// Content is the textual content of the response.
	Content string `json:"content"`
	// IsError Identify whether the current response is incorrect
	IsError   *bool  `json:"is_error"`
	Signature string `json:"signature,omitempty"`
}

func (ToolCallResponse) isPart() {}

// ExtInfoKey is the key of the extend data
// Unified management of extension data keys for easier maintenance
type ExtInfoKey string

const (
	// ExtInfoKeyBedrockRequestID only exists in bedrock response
	ExtInfoKeyBedrockRequestID ExtInfoKey = "bedrock_request_id"
	// ExtInfoKeyBedrockFirstByteLatency only exists in bedrock response
	ExtInfoKeyBedrockFirstByteLatency ExtInfoKey = "bedrock_first_byte_latency_ms"
	// ExtInfoKeyBedrockInputTokenCount only exists in bedrock response
	ExtInfoKeyBedrockInputTokenCount ExtInfoKey = "bedrock_input_token_count"
	// ExtInfoKeyBedrockOutputTokenCount only exists in bedrock response
	ExtInfoKeyBedrockOutputTokenCount ExtInfoKey = "bedrock_output_token_count"
	// ExtInfoKeyBedrockInvocationLatency only exists in bedrock response
	ExtInfoKeyBedrockInvocationLatency ExtInfoKey = "bedrock_invocation_latency_ms"
	// ExtInfoKeyAzureRequestID stores the X-Request-Id from Azure OpenAI responses
	ExtInfoKeyAzureRequestID ExtInfoKey = "gpt_x_request_id"
	// ExtInfoKeyAzureAPIMRequestID stores the Apim-Request-Id from Azure OpenAI responses
	ExtInfoKeyAzureAPIMRequestID ExtInfoKey = "gpt_apim_request_id"
	// ExtInfoKeyAzureModelSession stores the Azureml-Model-Session from Azure OpenAI responses
	ExtInfoKeyAzureModelSession ExtInfoKey = "azureml_model_session"
	// ExtInfoKeyFireworksGenerationQueueDuration stores the Fireworks-Generation-Queue-Duration response header
	ExtInfoKeyFireworksGenerationQueueDuration ExtInfoKey = "fireworks_generation_queue_duration"
	// ExtInfoKeyFireworksPrefillDuration stores the Fireworks-Prefill-Duration response header
	ExtInfoKeyFireworksPrefillDuration ExtInfoKey = "fireworks_prefill_duration"
	// ExtInfoKeyFireworksPrefillQueueDuration stores the Fireworks-Prefill-Queue-Duration response header
	ExtInfoKeyFireworksPrefillQueueDuration ExtInfoKey = "fireworks_prefill_queue_duration"
	// ExtInfoKeyFireworksServerTimeToFirstToken stores the Fireworks-Server-Time-To-First-Token response header
	ExtInfoKeyFireworksServerTimeToFirstToken ExtInfoKey = "fireworks_server_time_to_first_token"
	// ExtInfoKeyFireworksTokenizerDuration stores the Fireworks-Tokenizer-Duration response header
	ExtInfoKeyFireworksTokenizerDuration ExtInfoKey = "fireworks_tokenizer_duration"
	// ExtInfoKeyFireworksTokenizerQueueDuration stores the Fireworks-Tokenizer-Queue-Duration response header
	ExtInfoKeyFireworksTokenizerQueueDuration ExtInfoKey = "fireworks_tokenizer_queue_duration"
)

// ContentResponse is the response returned by a GenerateContent call.
// It can potentially return multiple content choices.
type ContentResponse struct {
	httpHeader

	ExtInfo map[ExtInfoKey]any `json:"ext_info,omitempty"` // extend information, added by langchaingo, not real data from model

	ID string

	// This field is not an official OpenAI field, only useful for perplexity sonar model.
	Citations []string
	Choices   []*ContentChoice
}

type ChatCompletionMessageMetadata struct {
	// WebSearchQueries web search tool 搜索的关键词列表
	WebSearchQueries []string `json:"web_search_queries,omitempty"`

	// Annotations for the message, when applicable, as when using the
	// [web search tool](https://platform.openai.com/docs/guides/tools-web-search?api-mode=chat).
	// 后面也会支持其他家的工具
	Annotations []ChatCompletionMessageAnnotation `json:"annotations,omitempty"`

	// SourceCitations 包含响应中引用的文本段落信息
	// 用于标识响应中引用的内容及其在文本中的位置
	SourceCitations []SourceCitation `json:"source_citations,omitempty"`
}

type ChatCompletionMessageAnnotationURLCitation struct {
	// Domain of the (original) URI.
	Domain string `json:"domain,omitempty"`
	// The title of the web resource.
	Title string `json:"title,required"`
	// The URL of the web resource.
	URL string `json:"url,required"`
}

type SourceCitation struct {
	// StartIndex 引用文本在消息中的起始字符索引
	StartIndex int64 `json:"start_index"`
	// EndIndex 引用文本在消息中的结束字符索引
	EndIndex int64 `json:"end_index"`
	// SegmentText 被引用的文本片段内容
	SegmentText string `json:"segment_text"`
	// AnnotationIndices 关联的注释索引，指向Annotations数组中的元素,一段文本可能引用多个 web
	AnnotationIndices []int32 `json:"annotation_indices"`
}

type AnnotationType string

const (
	AnnotationTypeURLCitation AnnotationType = "url_citation"
)

// ChatCompletionMessageAnnotation A URL citation when using web search.
type ChatCompletionMessageAnnotation struct {
	// The type of the URL citation. Always `url_citation`.
	Type AnnotationType `json:"type,required"`
	// A URL citation when using web search.
	URLCitation ChatCompletionMessageAnnotationURLCitation `json:"url_citation,required"`
}

type ContentChoiceFileData struct {
	FileUrl   string `json:"file_url,omitempty"`
	Data      []byte `json:"data,omitempty"`
	MimeType  string `json:"mime_type,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// ContentChoice is one of the response choices returned by GenerateContent
// calls.
type ContentChoice struct {
	// Content is the textual content of a response
	Content string

	// ReasoningContent Deepseek generated reasoning content.
	ReasoningContent string

	// ContentParts preserves the provider ordered content blocks (text, tool
	// calls, interleaved thinking, etc.).
	ContentParts []ContentPart

	// Metadata is arbitrary information the model adds to the response.
	// 自定义字段，openai 不支持这个字段
	// 暂时不支持增量 delta stream
	Metadata *ChatCompletionMessageMetadata `json:"metadata"`

	FileData []*ContentChoiceFileData

	// Refusal is the textual content when json schema refusal
	Refusal string

	// StopReason is the reason the model stopped generating output.
	StopReason string

	// GenerationInfo is arbitrary information the model adds to the response.
	GenerationInfo map[string]any

	// FuncCall is non-nil when the model asks to invoke a function/tool.
	// If a model invokes more than one function/tool, this field will only
	// contain the first one.
	FuncCall *FunctionCall

	// ToolCalls is a list of tool calls the model asks to invoke.
	ToolCalls []ToolCall
}

// TextParts is a helper function to create a MessageContent with a role and a
// list of text parts.
func TextParts(role ChatMessageType, parts ...string) MessageContent {
	result := MessageContent{
		Role:  role,
		Parts: []ContentPart{},
	}
	for _, part := range parts {
		result.Parts = append(result.Parts, TextPart(part))
	}
	return result
}

// StreamResponseField represents additional fields in the streaming response besides the main content.
// It is used to handle extra information such as reasoning content, reference sources, or any
// other supplementary data that comes with the model's response.
type StreamResponseField struct {
	// Value contains the actual content of the extra field
	Value string `json:"value"`
	// Key identifies the type of extra field, such as "reasoning_content" or "reference_source"
	Key string `json:"field"`
}

// ShowMessageContents is a debugging helper for MessageContent.
func ShowMessageContents(w io.Writer, msgs []MessageContent) {
	fmt.Fprintf(w, "MessageContent (len=%v)\n", len(msgs))
	for i, mc := range msgs {
		fmt.Fprintf(w, "[%d]: Role=%s\n", i, mc.Role)
		for j, p := range mc.Parts {
			fmt.Fprintf(w, "  Parts[%v]: ", j)
			switch pp := p.(type) {
			case TextContent:
				fmt.Fprintf(w, "TextContent %q signature=%q\n", pp.Text, pp.Signature)
			case ThinkingContent:
				fmt.Fprintf(w, "ThinkingContent signature=%q text=%q\n", pp.Signature, pp.Thinking)
			case ImageURLContent:
				fmt.Fprintf(w, "ImageURLPart %q\n", pp.URL)
			case BinaryContent:
				fmt.Fprintf(w, "BinaryContent MIME=%q, size=%d\n", pp.MIMEType, len(pp.Data))
			case ToolCall:
				fmt.Fprintf(w, "ToolCall ID=%v, Type=%v, Func=%v(%v) signature=%q\n", pp.ID, pp.Type, pp.FunctionCall.Name, pp.FunctionCall.Arguments, pp.Signature)
			case ToolCallResponse:
				fmt.Fprintf(w, "ToolCallResponse ID=%v, Name=%v, Content=%v signature=%q\n", pp.ToolCallID, pp.Name, pp.Content, pp.Signature)
			default:
				fmt.Fprintf(w, "unknown type %T\n", pp)
			}
		}
	}
}
