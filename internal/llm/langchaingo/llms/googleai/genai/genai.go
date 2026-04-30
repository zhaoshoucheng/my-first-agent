//nolint:all
package genai

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	"google.golang.org/genai"

	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/internal/util"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
)

var (
	ErrNoContentInResponse   = errors.New("no content in generation response")
	ErrUnknownPartInResponse = errors.New("unknown part type in generation response")
	ErrInvalidMimeType       = errors.New("invalid mime type on content")
)

const (
	CITATIONS  = "citations"
	SAFETY     = "safety"
	RoleSystem = "system"
	RoleModel  = "model"
	RoleUser   = "user"
	RoleTool   = "tool"

	gemini31ProDummyThoughtSignatureBase64 = "c2lnbmF0dXJlLTQ3NzQ4MTdiLTFhNGItNDU5MC04MWRmLWY4ZjZkOWY0NzM3YQ=="
)

var gemini31ProDummyThoughtSignature = mustDecodeThoughtSignatureBase64(gemini31ProDummyThoughtSignatureBase64)

// Call implements the [llms.Model] interface.
func (g *Vertex) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return llms.GenerateFromSinglePrompt(ctx, g, prompt, options...)
}

// GenerateContent implements the [llms.Model] interface.
func (g *Vertex) GenerateContent(
	ctx context.Context,
	messages []llms.MessageContent,
	options ...llms.CallOption,
) (*llms.ContentResponse, error) {
	if g.CallbacksHandler != nil {
		g.CallbacksHandler.HandleLLMGenerateContentStart(ctx, messages)
	}

	opts := llms.CallOptions{
		Model:          g.opts.DefaultModel,
		CandidateCount: g.opts.DefaultCandidateCount,
		MaxTokens:      g.opts.DefaultMaxTokens,
		Temperature:    g.opts.DefaultTemperature,
		TopP:           g.opts.DefaultTopP,
		TopK:           g.opts.DefaultTopK,
	}
	for _, opt := range options {
		opt(&opts)
	}

	cfg := &genai.GenerateContentConfig{
		CandidateCount:  int32(opts.CandidateCount),
		MaxOutputTokens: int32(opts.MaxTokens),
		StopSequences:   opts.StopWords,
	}
	if opts.Temperature > 0 {
		t := float32(opts.Temperature)
		cfg.Temperature = &t
	}
	if opts.TopP > 0 {
		t := float32(opts.TopP)
		cfg.TopP = &t
	}
	if opts.TopK > 0 {
		k := float32(opts.TopK)
		cfg.TopK = &k
	}

	var err error
	if cfg.Tools, err = convertTools(opts.Tools); err != nil {
		return nil, err
	}
	if cfg.ToolConfig, err = convertToolChoice(opts.ToolChoice); err != nil {
		return nil, err
	}
	if len(opts.DisabledFunctionNames) > 0 {
		// 这是为了保护 Gemini 无限复读而引入的解套方案，将无限重复的函数调用从 AllowedFunctionNames 中移除
		// Create a map for O(1) lookup of disabled function names
		disabledMap := make(map[string]bool)
		for _, name := range opts.DisabledFunctionNames {
			disabledMap[name] = true
		}

		// get allowed function names by removing disabled function names
		allowedFunctionNames := make([]string, 0, len(cfg.Tools))
		for _, tool := range cfg.Tools {
			for _, functionDeclaration := range tool.FunctionDeclarations {
				// Only add function if it's not in the disabled map
				if !disabledMap[functionDeclaration.Name] {
					allowedFunctionNames = append(allowedFunctionNames, functionDeclaration.Name)
				}
			}
		}
		cfg.ToolConfig.FunctionCallingConfig.AllowedFunctionNames = allowedFunctionNames
	}

	if opts.JSONMode {
		if opts.JSONSchema != nil {
			cfg.ResponseMIMEType = "application/json"
			schema, err := convertJSONSchemaToGenaiSchema(opts.JSONSchema)
			if err != nil {
				return nil, fmt.Errorf("failed to convert JSON schema: %w", err)
			}
			cfg.ResponseSchema = schema
		} else {
			cfg.ResponseMIMEType = "application/json"
		}
	}
	if opts.Thinking != nil && opts.Thinking.BudgetTokens != nil {
		cfg.ThinkingConfig = &genai.ThinkingConfig{
			IncludeThoughts: *opts.Thinking.BudgetTokens != 0,
			ThinkingBudget:  util.ToPtr(int32(*opts.Thinking.BudgetTokens)),
		}
	}
	if opts.ReasoningEffort != "" {
		if cfg.ThinkingConfig == nil {
			cfg.ThinkingConfig = &genai.ThinkingConfig{}
		}
		// reasoning effort 和 budget tokens 不能同时设置
		// 优先使用 reasoning effort
		cfg.ThinkingConfig.ThinkingBudget = nil
		cfg.ThinkingConfig.ThinkingLevel = convertThinkingLevel(opts.ReasoningEffort)
		cfg.ThinkingConfig.IncludeThoughts = true
	}
	if opts.ExtraHeaders != nil {
		convertExtraHeader(cfg, opts.ExtraHeaders)
	}
	applyExtraBody(cfg, opts.ExtraBody)

	// 目的是使用 model service, 暂时只有生图模型(gemini-2.5-flash-image-preview)使用
	// 理论上文本生成也应该使用 model service
	if len(opts.ResponseModalities) > 0 {
		cfg.ResponseModalities = opts.ResponseModalities
		response, err := generateContent(ctx, g.client, cfg, messages, &opts)
		if err != nil {
			return nil, err
		}
		if g.CallbacksHandler != nil {
			g.CallbacksHandler.HandleLLMGenerateContentEnd(ctx, response)
		}
		return response, nil
	}

	response, err := generateFromMessages(ctx, g.client, cfg, messages, &opts)
	if err != nil {
		return nil, err
	}
	if g.CallbacksHandler != nil {
		g.CallbacksHandler.HandleLLMGenerateContentEnd(ctx, response)
	}

	return response, nil
}

func convertAnnotations(
	metadata *genai.GroundingMetadata,
) ([]llms.ChatCompletionMessageAnnotation, []llms.SourceCitation) {
	if metadata == nil {
		return nil, nil
	}
	var annotations []llms.ChatCompletionMessageAnnotation
	var sourceCitations []llms.SourceCitation
	for _, chunk := range metadata.GroundingChunks {
		if chunk.Web != nil {
			citation := llms.ChatCompletionMessageAnnotationURLCitation{
				URL:    chunk.Web.URI,
				Title:  chunk.Web.Title,
				Domain: chunk.Web.Domain,
			}

			annotations = append(
				annotations, llms.ChatCompletionMessageAnnotation{
					Type:        llms.AnnotationTypeURLCitation,
					URLCitation: citation,
				},
			)
		}
	}

	for _, support := range metadata.GroundingSupports {
		sourceCitations = append(
			sourceCitations,
			llms.SourceCitation{
				StartIndex:        int64(support.Segment.StartIndex),
				EndIndex:          int64(support.Segment.EndIndex),
				SegmentText:       support.Segment.Text,
				AnnotationIndices: support.GroundingChunkIndices,
			},
		)
	}

	return annotations, sourceCitations
}

// convertResponse converts a sequence of genai.Candidate to a response.
func convertResponse(response *genai.GenerateContentResponse, opts *llms.CallOptions) (*llms.ContentResponse, error) {
	var contentResponse llms.ContentResponse

	contentResponse.ID = response.ResponseID

	if err := promptFeedbackError(response); err != nil {
		return nil, err
	}

	captureReasoning := opts.Thinking != nil
	includeContentParts := opts.Thinking != nil && opts.Thinking.EnableSignature

	for _, candidate := range response.Candidates {
		buf := strings.Builder{}
		reasoningBuf := strings.Builder{}
		var fileData []*llms.ContentChoiceFileData
		toolCalls := make([]llms.ToolCall, 0)
		var contentParts []llms.ContentPart
		if includeContentParts {
			contentParts = make([]llms.ContentPart, 0)
		}
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				signature := encodeThoughtSignature(part.ThoughtSignature)
				switch {
				case part.Text != "":
					var err error
					if part.Thought && captureReasoning {
						_, err = reasoningBuf.WriteString(part.Text)
					} else {
						_, err = buf.WriteString(part.Text)
					}
					if err != nil {
						return nil, err
					}
					if includeContentParts {
						if part.Thought {
							contentParts = append(contentParts, llms.ThinkingContent{
								Thinking:  part.Text,
								Signature: signature,
							})
						} else {
							contentParts = append(contentParts, llms.TextContent{
								Text:      part.Text,
								Signature: signature,
							})
						}
					}
				case part.FunctionCall != nil:
					b, err := json.Marshal(part.FunctionCall.Args)
					if err != nil {
						return nil, err
					}
					toolCall := llms.ToolCall{
						ID:   part.FunctionCall.ID,
						Type: "function",
						FunctionCall: &llms.FunctionCall{
							Name:      part.FunctionCall.Name,
							Arguments: string(b),
						},
					}
					if includeContentParts {
						toolCall.Signature = signature
					}
					toolCalls = append(toolCalls, toolCall)
				case part.FunctionResponse != nil:
					// signature stored internally; no content part
				case part.InlineData != nil:
					fileData = append(fileData, &llms.ContentChoiceFileData{
						Signature: signature,
						Data:      part.InlineData.Data,
						MimeType:  part.InlineData.MIMEType,
					})
				case part.FileData != nil:
					fileData = append(fileData, &llms.ContentChoiceFileData{
						FileUrl:   part.FileData.FileURI,
						MimeType:  part.FileData.MIMEType,
						Signature: signature,
					})
				default:
					// TODO 支持其他类型
				}

			}
		}

		generateInfo := make(map[string]any)
		generateInfo[CITATIONS] = candidate.CitationMetadata
		generateInfo[SAFETY] = candidate.SafetyRatings
		if response.UsageMetadata != nil {
			generateInfo["input_tokens"] = response.UsageMetadata.PromptTokenCount
			generateInfo["output_tokens"] = response.UsageMetadata.CandidatesTokenCount
			generateInfo["total_tokens"] = response.UsageMetadata.TotalTokenCount
			generateInfo["cached_tokens"] = response.UsageMetadata.CachedContentTokenCount
			generateInfo["thoughts_tokens"] = response.UsageMetadata.ThoughtsTokenCount
			generateInfo["traffic_type"] = string(response.UsageMetadata.TrafficType)
			generateInfo["input_tokens_detail"] = aggregateTokenDetails(response.UsageMetadata.PromptTokensDetails)
			generateInfo["output_tokens_detail"] = aggregateTokenDetails(response.UsageMetadata.CandidatesTokensDetails)

		}
		var metadata *llms.ChatCompletionMessageMetadata
		if candidate.GroundingMetadata != nil {
			metadata = &llms.ChatCompletionMessageMetadata{
				WebSearchQueries: candidate.GroundingMetadata.WebSearchQueries,
			}
			metadata.Annotations, metadata.SourceCitations = convertAnnotations(candidate.GroundingMetadata)
		}

		refusal := ""
		if candidate.FinishReason == genai.FinishReasonMalformedFunctionCall {
			refusal = candidate.FinishMessage
		}

		contentResponse.Choices = append(
			contentResponse.Choices,
			&llms.ContentChoice{
				Content:          buf.String(),
				ReasoningContent: reasoningBuf.String(),
				ContentParts:     contentParts,
				StopReason:       string(candidate.FinishReason),
				Refusal:          refusal,
				GenerationInfo:   generateInfo,
				ToolCalls:        toolCalls,
				Metadata:         metadata,
				FileData:         fileData,
			},
		)
	}

	return &contentResponse, nil
}

func encodeThoughtSignature(sig []byte) string {
	if len(sig) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func isLegacyGeminiSignature(sig string, scope *llms.SignatureScope) bool {
	if sig == "" || scope == nil || !scope.Valid() || !strings.EqualFold(scope.APIType, "gemini") {
		return false
	}
	return !looksLikeWrappedSignature(sig)
}

func looksLikeWrappedSignature(signature string) bool {
	type envelope struct {
		Format string `json:"format"`
	}

	decoders := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	for _, decoder := range decoders {
		data, err := decoder.DecodeString(signature)
		if err != nil {
			continue
		}
		var payload envelope
		if err := json.Unmarshal(data, &payload); err != nil {
			continue
		}
		if payload.Format != "" {
			return true
		}
	}
	return false
}

func decodeThoughtSignature(sig string, scope *llms.SignatureScope) []byte {
	if sig == "" {
		return nil
	}
	if unwrapped, ok := llms.UnwrapScopedSignature(sig, scope); ok {
		sig = unwrapped
	} else if scope != nil && scope.Valid() && !isLegacyGeminiSignature(sig, scope) {
		return nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(sig); err == nil {
		return decoded
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(sig); err == nil {
		return decoded
	}
	return []byte(sig)
}

func mustDecodeThoughtSignatureBase64(sig string) []byte {
	decoded, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		panic(fmt.Sprintf("invalid Gemini dummy thought signature: %v", err))
	}
	return decoded
}

func shouldUseGemini31ProDummyThoughtSignature(model string) bool {
	return strings.Contains(strings.ToLower(model), "gemini-3.1-pro")
}

// Gemini 3.1 Pro rejects replayed tool/thinking parts without a thoughtSignature.
func decodeThoughtSignatureForPart(
	sig string,
	scope *llms.SignatureScope,
	model string,
	role llms.ChatMessageType,
	part llms.ContentPart,
) []byte {
	decoded := decodeThoughtSignature(sig, scope)
	if len(decoded) > 0 {
		return decoded
	}
	if strings.TrimSpace(sig) != "" || role != llms.ChatMessageTypeAI || !shouldUseGemini31ProDummyThoughtSignature(model) {
		return nil
	}
	switch part.(type) {
	case llms.ThinkingContent, llms.ToolCall:
		return append([]byte(nil), gemini31ProDummyThoughtSignature...)
	default:
		return nil
	}
}

func contentPartSignature(part llms.ContentPart) string {
	switch p := part.(type) {
	case llms.TextContent:
		return p.Signature
	case llms.ToolCall:
		return p.Signature
	case llms.ThinkingContent:
		return p.Signature
	case llms.ToolCallResponse:
		return p.Signature
	default:
		return ""
	}
}

func aggregateTokenDetails(tokenDetails []*genai.ModalityTokenCount) map[string]int64 {
	detail := make(map[string]int64)
	for _, tokenDetail := range tokenDetails {
		modality := string(tokenDetail.Modality)
		detail[modality] += int64(tokenDetail.TokenCount)
	}
	return detail
}

// convertParts converts between a sequence of langchain parts and genai parts.
func convertParts(
	parts []llms.ContentPart,
	toolIdNameMap map[string]string,
	signatureScope *llms.SignatureScope,
	model string,
	role llms.ChatMessageType,
) ([]*genai.Part, error) {
	convertedParts := make([]*genai.Part, 0, len(parts))
	for _, part := range parts {
		var out genai.Part
		thoughtSignature := decodeThoughtSignatureForPart(contentPartSignature(part), signatureScope, model, role, part)

		switch p := part.(type) {
		case llms.TextContent:
			out = genai.Part{Text: p.Text, ThoughtSignature: thoughtSignature}
		case llms.FileURLContent:
			if strings.HasPrefix(p.URL, "data:") {
				data, mimeType, err := util.ExtractBase64Data(p.URL)
				if err != nil {
					return nil, err
				}
				if p.MimeType != "" {
					mimeType = p.MimeType
				}
				out = genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
			} else {
				mimeType := p.MimeType
				if mimeType == "" {
					var e error
					mimeType, e = util.DetectMimeType(p.URL)
					if e != nil {
						return nil, e
					}
				}
				out = genai.Part{
					FileData: &genai.FileData{
						FileURI:  p.URL,
						MIMEType: mimeType,
					}}
			}
		case llms.BinaryContent:
			out = genai.Part{
				InlineData: &genai.Blob{
					MIMEType: p.MIMEType,
					Data:     p.Data,
				},
			}
		case llms.ImageURLContent:
			if strings.HasPrefix(p.URL, "data:") {
				data, mimeType, err := util.ExtractBase64Data(p.URL)
				if err != nil {
					return nil, err
				}
				out = genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
			} else {
				t := p.MimeType
				if t == "" {
					var e error
					t, e = util.DetectMimeType(p.URL)
					if e != nil {
						return nil, e
					}
				}
				out = genai.Part{
					FileData: &genai.FileData{
						FileURI:  p.URL,
						MIMEType: t,
					}}
			}
		case llms.ToolCall:
			toolIdNameMap[p.ID] = p.FunctionCall.Name
			fc := p.FunctionCall
			var argsMap map[string]any
			argsMap = parseArguments(fc.Arguments)
			out = genai.Part{
				FunctionCall: &genai.FunctionCall{
					Name: fc.Name,
					Args: argsMap,
				},
				ThoughtSignature: thoughtSignature,
			}
		case llms.ThinkingContent:
			out = genai.Part{
				Text:             p.Thinking,
				Thought:          true,
				ThoughtSignature: thoughtSignature,
			}
		case llms.ToolCallResponse:
			toolName := p.Name
			if toolName == "" && toolIdNameMap != nil && p.ToolCallID != "" {
				toolName = toolIdNameMap[p.ToolCallID]
			}
			response := map[string]any{"response": p.Content}
			if p.IsError != nil {
				if *p.IsError {
					response = map[string]any{"error": p.Content}
				} else {
					response = map[string]any{"output": p.Content}
				}
			}
			out = genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     toolName,
					Response: response,
				},
				ThoughtSignature: thoughtSignature,
			}
		}

		convertedParts = append(convertedParts, &out)
	}

	return convertedParts, nil
}
func parseArguments(args string) map[string]any {
	var argsMap map[string]any
	if err := json.Unmarshal([]byte(args), &argsMap); err == nil {
		return argsMap
	}
	//Exception rules
	if !strings.HasSuffix(args, "}") {
		args += "\"}"
	}
	if err := json.Unmarshal([]byte(args), &argsMap); err == nil {
		return argsMap
	}
	return argsMap
}

// convertContent converts between a langchain MessageContent and genai content.
func convertContent(
	content llms.MessageContent,
	toolIdNameMap map[string]string,
	signatureScope *llms.SignatureScope,
	model string,
) (*genai.Content, error) {
	parts, err := convertParts(content.Parts, toolIdNameMap, signatureScope, model, content.Role)
	if err != nil {
		return nil, err
	}

	c := &genai.Content{
		Parts: parts,
	}

	switch content.Role {
	case llms.ChatMessageTypeSystem:
		c.Role = RoleSystem
	case llms.ChatMessageTypeAI:
		c.Role = RoleModel
	case llms.ChatMessageTypeHuman:
		c.Role = RoleUser
	case llms.ChatMessageTypeGeneric:
		c.Role = RoleUser
	case llms.ChatMessageTypeTool:
		c.Role = RoleUser
	case llms.ChatMessageTypeFunction:
		fallthrough
	default:
		return nil, fmt.Errorf("role %v not supported", content.Role)
	}

	return c, nil
}

func generateContent(ctx context.Context,
	client *genai.Client,
	cfg *genai.GenerateContentConfig,
	messages []llms.MessageContent,
	opts *llms.CallOptions,
) (*llms.ContentResponse, error) {
	if len(messages) == 0 {
		return nil, errors.New("no messages provided")
	}
	contents := make([]*genai.Content, 0, len(messages))
	toolIdNameMap := make(map[string]string)
	for _, mc := range messages {
		content, err := convertContent(mc, toolIdNameMap, opts.SignatureScope, opts.Model)
		if err != nil {
			return nil, err
		}
		if mc.Role == RoleSystem {
			cfg.SystemInstruction = content
			continue
		}
		contents = append(contents, content)
	}

	if opts.StreamingFunc == nil {
		response, err := client.Models.GenerateContent(ctx, opts.Model, contents, cfg)
		if err != nil {
			return nil, err
		}
		return convertResponse(response, opts)
	}
	stream := client.Models.GenerateContentStream(ctx, opts.Model, contents, cfg)
	return convertAndStreamFromIterator(ctx, stream, opts)
}

func generateFromMessages(
	ctx context.Context,
	client *genai.Client,
	cfg *genai.GenerateContentConfig,
	messages []llms.MessageContent,
	opts *llms.CallOptions,
) (*llms.ContentResponse, error) {
	history := make([]*genai.Content, 0, len(messages))
	toolIdNameMap := make(map[string]string)
	for _, mc := range messages {
		content, err := convertContent(mc, toolIdNameMap, opts.SignatureScope, opts.Model)
		if err != nil {
			return nil, err
		}
		if mc.Role == RoleSystem {
			cfg.SystemInstruction = content
			continue
		}
		history = append(history, content)
	}
	if len(history) == 0 {
		return nil, errors.New("no messages provided")
	}
	userInput := history[len(history)-1]
	history = history[:len(history)-1]
	chat, err := client.Chats.Create(ctx, opts.Model, cfg, history)
	if err != nil {
		return nil, err
	}
	var userInputParts []genai.Part
	for _, part := range userInput.Parts {
		userInputParts = append(userInputParts, *part)
	}

	if opts.StreamingFunc == nil {
		resp, err := chat.SendMessage(ctx, userInputParts...)
		if err != nil {
			return nil, err
		}
		return convertResponse(resp, opts)
	} else {
		return convertAndStreamFromIterator(ctx, chat.SendMessageStream(ctx, userInputParts...), opts)
	}
}

// convertAndStreamFromIterator takes an iterator of GenerateContentResponse
// and produces a llms.ContentResponse reply from it, while streaming the
// resulting text into the opts-provided streaming function.
// Note that this is tricky in the face of multiple
// candidates, so this code assumes only a single candidate for now.
func convertAndStreamFromIterator(
	ctx context.Context,
	iter iter.Seq2[*genai.GenerateContentResponse, error],
	opts *llms.CallOptions,
) (*llms.ContentResponse, error) {
	emitReasoning := opts.Thinking != nil || opts.ReasoningEffort != ""
	includeContentParts := opts.Thinking != nil && opts.Thinking.EnableSignature
	mergedResp := &genai.GenerateContentResponse{
		Candidates:     []*genai.Candidate{{Content: &genai.Content{Parts: make([]*genai.Part, 0)}}},
		ResponseID:     "",
		ModelVersion:   "",
		PromptFeedback: nil,
		UsageMetadata:  nil,
	}
	for chunk, err := range iter {
		if err != nil {
			return nil, fmt.Errorf("error in stream mode: %w", err)
		}

		if err := promptFeedbackError(chunk); err != nil {
			return nil, err
		}

		if len(chunk.Candidates) != 1 {
			return nil, fmt.Errorf("expect single candidate in stream mode; got %v", len(chunk.Candidates))
		}

		if chunk.ResponseID != "" && mergedResp.ResponseID == "" {
			mergedResp.ResponseID = chunk.ResponseID
		}
		if chunk.PromptFeedback != nil && mergedResp.PromptFeedback == nil {
			mergedResp.PromptFeedback = chunk.PromptFeedback
		}
		if chunk.ModelVersion != "" && mergedResp.ModelVersion == "" {
			mergedResp.ModelVersion = chunk.ModelVersion
		}
		if chunk.CreateTime.IsZero() && !mergedResp.CreateTime.IsZero() {
			mergedResp.CreateTime = chunk.CreateTime
		}

		chunkCandidate := chunk.Candidates[0]
		mergedCandidate := mergedResp.Candidates[0]

		mergedCandidate.FinishReason = chunkCandidate.FinishReason
		mergedCandidate.SafetyRatings = chunkCandidate.SafetyRatings
		if chunkCandidate.CitationMetadata != nil {
			mergedCandidate.CitationMetadata = chunkCandidate.CitationMetadata
		}
		if chunk.UsageMetadata != nil {
			mergedResp.UsageMetadata = chunk.UsageMetadata
		}

		if chunkCandidate.FinishReason == genai.FinishReasonMalformedFunctionCall {
			mergedCandidate.FinishMessage += chunkCandidate.FinishMessage
			m := map[string]any{
				"refusal": chunkCandidate.FinishMessage,
				"loc":     "refusal",
			}
			text, _ := json.Marshal(m)
			if err := opts.StreamingFunc(ctx, text); err != nil {
				return nil, err
			}
		}
		if chunkCandidate.Content != nil {
			if len(chunkCandidate.Content.Parts) > 0 {
				mergedCandidate.Content.Parts = append(mergedCandidate.Content.Parts, chunkCandidate.Content.Parts...)
				for _, part := range chunkCandidate.Content.Parts {
					signature := ""
					if len(part.ThoughtSignature) > 0 {
						signature = encodeThoughtSignature(part.ThoughtSignature)
					}
					if part.Text != "" {
						if includeContentParts && part.Thought {
							value := map[string]any{
								"thinking":  part.Text,
								"signature": signature,
							}
							payload := map[string]any{
								"value": value,
								"loc":   "thinking_part",
							}
							data, e := json.Marshal(payload)
							if e != nil {
								return nil, e
							}
							if err := opts.StreamingFunc(ctx, data); err != nil {
								return nil, err
							}
						}
						if part.Thought && emitReasoning {
							m := map[string]any{
								"reasoning_content": part.Text,
								"loc":               "reasoning_content",
							}
							if includeContentParts {
								m["signature"] = signature
							}
							text, _ := json.Marshal(m)
							if err := opts.StreamingFunc(ctx, text); err != nil {
								return nil, err
							}
						} else {
							if includeContentParts {
								value := map[string]any{
									"text":      part.Text,
									"signature": signature,
								}
								payload := map[string]any{
									"value": value,
									"loc":   "text_part",
								}
								data, e := json.Marshal(payload)
								if e != nil {
									return nil, e
								}
								if err := opts.StreamingFunc(ctx, data); err != nil {
									return nil, err
								}
							} else {
								if err := opts.StreamingFunc(ctx, []byte(part.Text)); err != nil {
									return nil, err
								}
							}
						}
					}
					// 工具调用不能合并, 每次都是一个独立的返回，所以没有 id
					if part.FunctionCall != nil {
						argsBytes, e := json.Marshal(part.FunctionCall.Args)
						if e != nil {
							return nil, e
						}
						toolId := part.FunctionCall.ID
						if toolId == "" {
							toolId = "tooluse_" + randomToolID(22)
						}

						toolCall := []llms.ToolCall{{
							Type: "function",
							ID:   toolId,
							FunctionCall: &llms.FunctionCall{
								Name:      part.FunctionCall.Name,
								Arguments: string(argsBytes),
							},
						}}
						if includeContentParts {
							toolCall[0].Signature = signature
						}
						toolCallBytes, e := json.Marshal(toolCall)
						if e != nil {
							return nil, e
						}
						if err := opts.StreamingFunc(ctx, toolCallBytes); err != nil {
							return nil, err
						}
					}
					// generated image
					if part.InlineData != nil {
						fileData := map[string]any{
							"value": &llms.ContentChoiceFileData{
								Data:      part.InlineData.Data,
								MimeType:  part.InlineData.MIMEType,
								Signature: signature,
							},
							"loc": "file_data",
						}
						fileBytes, e := json.Marshal(fileData)
						if e != nil {
							return nil, e
						}
						if err := opts.StreamingFunc(ctx, fileBytes); err != nil {
							return nil, err
						}
					}
					if part.FileData != nil {
						fileData := map[string]any{
							"value": &llms.ContentChoiceFileData{
								FileUrl:   part.FileData.FileURI,
								MimeType:  part.FileData.MIMEType,
								Signature: signature,
							},
							"loc": "file_data",
						}
						fileBytes, e := json.Marshal(fileData)
						if e != nil {
							return nil, e
						}
						if err := opts.StreamingFunc(ctx, fileBytes); err != nil {
							return nil, err
						}
					}
				}
			}
		}
		if chunkCandidate.GroundingMetadata != nil {
			mergedCandidate.GroundingMetadata = chunkCandidate.GroundingMetadata
			metadata := &llms.ChatCompletionMessageMetadata{
				WebSearchQueries: chunkCandidate.GroundingMetadata.WebSearchQueries,
			}
			metadata.Annotations, metadata.SourceCitations = convertAnnotations(chunkCandidate.GroundingMetadata)
			if len(metadata.Annotations) > 0 || len(chunkCandidate.GroundingMetadata.WebSearchQueries) > 0 {
				text, _ := json.Marshal(
					map[string]any{
						"value": metadata,
						"loc":   "metadata",
					},
				)
				if err := opts.StreamingFunc(ctx, text); err != nil {
					return nil, err
				}
			}
		}
	}

	return convertResponse(mergedResp, opts)
}

func promptFeedbackError(response *genai.GenerateContentResponse) error {
	if response == nil || response.PromptFeedback == nil || response.PromptFeedback.BlockReason == "" {
		return nil
	}
	feedback := response.PromptFeedback
	message := strings.TrimSpace(feedback.BlockReasonMessage)
	if message == "" {
		return fmt.Errorf("prompt blocked: reason=%s", feedback.BlockReason)
	}
	return fmt.Errorf("prompt blocked: reason=%s message=%s", feedback.BlockReason, message)
}

func convertExtraHeader(cfg *genai.GenerateContentConfig, header http.Header) {
	extraBody := make(map[string]any)
	extraHeader := make(map[string][]string)
	// session id
	if sessionID := header.Get(llms.ExtraHeaderSessionID); sessionID != "" {
		extraBody[llms.ExtraHeaderSessionID] = sessionID
	}
	if requestType := header.Values(llms.ExtraHeaderXVertexAILLMRequestType); len(requestType) > 0 {
		extraHeader[llms.ExtraHeaderXVertexAILLMRequestType] = requestType
	}
	if sharedRequestType := header.Values(llms.ExtraHeaderXVertexAILLMSharedRequestType); len(sharedRequestType) > 0 {
		extraHeader[llms.ExtraHeaderXVertexAILLMSharedRequestType] = sharedRequestType
	}

	if cfg.HTTPOptions == nil {
		cfg.HTTPOptions = &genai.HTTPOptions{}
	}
	if cfg.HTTPOptions.Headers == nil {
		cfg.HTTPOptions.Headers = make(http.Header)
	}
	//extra body
	cfg.HTTPOptions.ExtrasRequestProvider = func(body map[string]any) map[string]any {
		if body == nil {
			body = make(map[string]any)
		}
		for k, v := range extraBody {
			body[k] = v
		}
		return body
	}
	//extra header
	for k, v := range extraHeader {
		for _, value := range v {
			cfg.HTTPOptions.Headers.Add(k, value)
		}
	}

}

func applyExtraBody(cfg *genai.GenerateContentConfig, extraBody map[string]any) {
	if len(extraBody) == 0 {
		return
	}
	if cfg.HTTPOptions == nil {
		cfg.HTTPOptions = &genai.HTTPOptions{}
	}
	if cfg.HTTPOptions.ExtraBody == nil {
		cfg.HTTPOptions.ExtraBody = make(map[string]any, len(extraBody))
	}
	for key, value := range extraBody {
		cfg.HTTPOptions.ExtraBody[key] = value
	}
}

func convertToolChoice(toolChoice any) (*genai.ToolConfig, error) {
	var funcCallConfig *genai.FunctionCallingConfig
	if toolChoice == nil {
		return nil, nil
	}

	if toolChoice, ok := toolChoice.(string); ok {
		switch toolChoice {
		case "none":
			funcCallConfig = &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeNone}
		case "any", "required":
			funcCallConfig = &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeAny}
		case "auto":
			funcCallConfig = &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeAuto}
		default:
			return nil, errors.New("unsupported tool choice")
		}
		return &genai.ToolConfig{FunctionCallingConfig: funcCallConfig}, nil
	}

	var llmsToolChoice *llms.ToolChoice
	if toolChoice, ok := toolChoice.(llms.ToolChoice); ok {
		llmsToolChoice = &toolChoice
	} else {
		if err := mapstructure.Decode(toolChoice, llmsToolChoice); err != nil {
			return nil, fmt.Errorf("failed to decode tool choice: %w", err)
		}
	}
	if llmsToolChoice != nil {
		if llmsToolChoice.Function == nil {
			funcCallConfig = &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeAuto}
		} else {
			funcCallConfig = &genai.FunctionCallingConfig{
				Mode:                 genai.FunctionCallingConfigModeAny,
				AllowedFunctionNames: []string{llmsToolChoice.Function.Name},
			}
		}
	}
	return &genai.ToolConfig{FunctionCallingConfig: funcCallConfig}, nil
}

// convertTools converts from a list of langchaingo tools to a list of genai
// tools.
func convertTools(tools []llms.Tool) ([]*genai.Tool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	var genAiTools []*genai.Tool
	functionDeclarations := make([]*genai.FunctionDeclaration, 0)
	for i, tool := range tools {
		if tool.Type != "function" {
			return nil, fmt.Errorf("tool [%d]: unsupported type %q, want 'function'", i, tool.Type)
		}
		// 官方支持的是 googleSearch, google_search 应该逐步弃用
		if tool.Function.Name == "google_search" || tool.Function.Name == "googleSearch" {
			genAiTools = append(genAiTools, &genai.Tool{
				GoogleSearch: &genai.GoogleSearch{},
			})
			continue
		}
		if tool.Function.Name == "google_search_retrieval" {
			genAiTools = append(genAiTools, &genai.Tool{
				GoogleSearchRetrieval: &genai.GoogleSearchRetrieval{
					DynamicRetrievalConfig: &genai.DynamicRetrievalConfig{Mode: genai.DynamicRetrievalConfigModeDynamic},
				},
			})
			continue
		}

		// We have a llms.FunctionDefinition in tool.Function, and we have to
		// convert it to genai.FunctionDeclaration
		genaiFuncDecl := &genai.FunctionDeclaration{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
		}

		// Expect the Parameters field to be a map[string]any, from which we will
		// extract properties to populate the schema.
		params, ok := tool.Function.Parameters.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("tool [%d]: unsupported type %T of Parameters", i, tool.Function.Parameters)
		}

		// extraFields为兼容逻辑，在convertTools时不转换maxItems等字段，因为这会导致tool过于复杂而被拒绝
		// TODO: 在上层修复之后这里的兼容逻辑去掉
		schema, err := convertToSchema(params, fmt.Sprintf("tool [%d]", i), false)
		if err != nil {
			return nil, err
		}
		genaiFuncDecl.Parameters = schema
		functionDeclarations = append(functionDeclarations, genaiFuncDecl)
	}
	if len(functionDeclarations) > 0 {
		genAiTools = append(genAiTools, &genai.Tool{
			FunctionDeclarations: functionDeclarations,
		})
	}
	return genAiTools, nil
}

// schemaWorkItem represents a work item in the schema conversion process
type schemaWorkItem struct {
	schemaMap map[string]any
	schema    *genai.Schema
	errPrefix string
}

// convertToSchema converts a map[string]any to a genai.Schema using iteration
func convertToSchema(schemaMap map[string]any, errPrefix string, convertExtraFields bool) (*genai.Schema, error) {
	rootSchema := &genai.Schema{}

	workQueue := []schemaWorkItem{{
		schemaMap: schemaMap,
		schema:    rootSchema,
		errPrefix: errPrefix,
	}}

	for len(workQueue) > 0 {
		item := workQueue[0]
		workQueue = workQueue[1:]

		nullableFromType := false
		if ty, ok := item.schemaMap["type"]; ok {
			types, nullable, err := parseSchemaTypes(ty, item.errPrefix)
			if err != nil {
				return nil, err
			}
			if len(types) == 1 {
				item.schema.Type = convertToolSchemaType(types[0])
			} else if len(types) > 1 {
				item.schema.AnyOf = make([]*genai.Schema, 0, len(types))
				for _, tyValue := range types {
					item.schema.AnyOf = append(item.schema.AnyOf, &genai.Schema{
						Type: convertToolSchemaType(tyValue),
					})
				}
			}
			nullableFromType = nullable
		}

		schemaForType := func(ty genai.Type) *genai.Schema {
			if item.schema.Type == ty {
				return item.schema
			}
			for _, schema := range item.schema.AnyOf {
				if schema.Type == ty {
					return schema
				}
			}
			return nil
		}

		// Handle properties if type is object
		if props, ok := item.schemaMap["properties"].(map[string]any); ok {
			targetSchema := schemaForType(genai.TypeObject)
			if targetSchema == nil {
				continue
			}
			if ordering, ok := item.schemaMap["propertyOrdering"].([]any); ok {
				orderingValues := make([]string, 0, len(ordering))
				for _, e := range ordering {
					eString, ok := e.(string)
					if !ok {
						return nil, fmt.Errorf("%s: expected string for propertyOrdering value", item.errPrefix)
					}
					orderingValues = append(orderingValues, eString)
				}
				targetSchema.PropertyOrdering = orderingValues
			}
			targetSchema.Properties = make(map[string]*genai.Schema)
			for propName, propValue := range props {
				valueMap, ok := propValue.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("%s, property [%v]: expect to find a value map", item.errPrefix, propName)
				}

				// Create new schema for property
				propSchema := &genai.Schema{}
				targetSchema.Properties[propName] = propSchema

				// Add to work queue
				workQueue = append(
					workQueue, schemaWorkItem{
						schemaMap: valueMap,
						schema:    propSchema,
						errPrefix: fmt.Sprintf("%s, property [%v]", item.errPrefix, propName),
					},
				)
			}
		}

		// Handle items if type is array
		if items, ok := item.schemaMap["items"].(map[string]any); ok {
			targetSchema := schemaForType(genai.TypeArray)
			if targetSchema == nil {
				continue
			}
			itemSchema := &genai.Schema{}
			targetSchema.Items = itemSchema

			// Add to work queue
			workQueue = append(
				workQueue, schemaWorkItem{
					schemaMap: items,
					schema:    itemSchema,
					errPrefix: fmt.Sprintf("%s, items", item.errPrefix),
				},
			)
		}

		// Handle required fields
		if required, ok := item.schemaMap["required"]; ok {
			targetSchema := item.schema
			if objectSchema := schemaForType(genai.TypeObject); objectSchema != nil && item.schema.Type != genai.TypeObject {
				targetSchema = objectSchema
			}
			if rs, ok := required.([]string); ok {
				targetSchema.Required = rs
			} else if ri, ok := required.([]any); ok {
				rs := make([]string, 0, len(ri))
				for _, r := range ri {
					rString, ok := r.(string)
					if !ok {
						return nil, fmt.Errorf("%s: expected string for required", item.errPrefix)
					}
					rs = append(rs, rString)
				}
				targetSchema.Required = rs
			} else {
				return nil, fmt.Errorf("%s: expected []string or []interface{} for required", item.errPrefix)
			}
		}

		// Handle description
		if desc, ok := item.schemaMap["description"]; ok {
			descString, ok := desc.(string)
			if !ok {
				return nil, fmt.Errorf("%s: expected string for description", item.errPrefix)
			}
			item.schema.Description = descString
		}

		if convertExtraFields {
			// Handle string fields
			if err := setStringField(item.schemaMap, "title", &item.schema.Title, item.errPrefix); err != nil {
				return nil, err
			}
			if err := setStringField(item.schemaMap, "format", &item.schema.Format, item.errPrefix); err != nil {
				return nil, err
			}
			if err := setStringField(item.schemaMap, "pattern", &item.schema.Pattern, item.errPrefix); err != nil {
				return nil, err
			}

			// Handle default
			if defaultVal, ok := item.schemaMap["default"]; ok {
				item.schema.Default = defaultVal
			}

			// Handle numeric constraints
			if err := setFloatField(item.schemaMap, "minimum", &item.schema.Minimum, item.errPrefix); err != nil {
				return nil, err
			}
			if err := setFloatField(item.schemaMap, "maximum", &item.schema.Maximum, item.errPrefix); err != nil {
				return nil, err
			}
			if err := setInt64Field(item.schemaMap, "minLength", &item.schema.MinLength, item.errPrefix); err != nil {
				return nil, err
			}
			if err := setInt64Field(item.schemaMap, "maxLength", &item.schema.MaxLength, item.errPrefix); err != nil {
				return nil, err
			}
			if err := setInt64Field(item.schemaMap, "minItems", &item.schema.MinItems, item.errPrefix); err != nil {
				return nil, err
			}
			if err := setInt64Field(item.schemaMap, "maxItems", &item.schema.MaxItems, item.errPrefix); err != nil {
				return nil, err
			}

			// Handle nullable
			if nullable, ok := item.schemaMap["nullable"]; ok {
				if nullableBool, ok := nullable.(bool); ok {
					item.schema.Nullable = &nullableBool
				}
			}
		}

		if nullableFromType {
			nullable := true
			item.schema.Nullable = &nullable
		}

		// Handle enum
		if enum, ok := item.schemaMap["enum"].([]any); ok {
			enumValues := make([]string, 0, len(enum))
			for _, e := range enum {
				eString, ok := e.(string)
				if !ok {
					return nil, fmt.Errorf("%s: expected string for enum value", item.errPrefix)
				}
				enumValues = append(enumValues, eString)
			}
			item.schema.Enum = enumValues
		}
	}

	return rootSchema, nil
}

// convertToolSchemaType converts a tool's schema type from its langchaingo
// representation (string) to a genai enum.
func convertToolSchemaType(ty string) genai.Type {
	switch ty {
	case "object":
		return genai.TypeObject
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	default:
		return genai.TypeUnspecified
	}
}

func parseSchemaTypes(raw any, errPrefix string) ([]string, bool, error) {
	switch v := raw.(type) {
	case string:
		types, nullable := filterNullTypes([]string{v})
		if len(types) == 0 {
			return nil, nullable, fmt.Errorf("%s: expected non-null type", errPrefix)
		}
		return types, nullable, nil
	case []string:
		types, nullable := filterNullTypes(v)
		if len(types) == 0 {
			return nil, nullable, fmt.Errorf("%s: expected non-null type", errPrefix)
		}
		return types, nullable, nil
	case []any:
		values := make([]string, 0, len(v))
		for _, t := range v {
			s, ok := t.(string)
			if !ok {
				return nil, false, fmt.Errorf("%s: expected string for type", errPrefix)
			}
			values = append(values, s)
		}
		types, nullable := filterNullTypes(values)
		if len(types) == 0 {
			return nil, nullable, fmt.Errorf("%s: expected non-null type", errPrefix)
		}
		return types, nullable, nil
	default:
		return nil, false, fmt.Errorf("%s: expected string for type", errPrefix)
	}
}

func filterNullTypes(values []string) ([]string, bool) {
	filtered := make([]string, 0, len(values))
	nullable := false
	for _, value := range values {
		if value == "null" {
			nullable = true
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered, nullable
}

// convertJSONSchemaToGenaiSchema converts a langchaingo JSONSchema (map[string]any) to genai.Schema
func convertJSONSchemaToGenaiSchema(jsonSchema map[string]any) (*genai.Schema, error) {
	if jsonSchema == nil {
		return nil, nil
	}

	// Handle both formats: direct schema and schema with name
	var schemaMap map[string]any
	if schema, hasSchema := jsonSchema["schema"].(map[string]any); hasSchema {
		// Format: {"name": "...", "schema": {...}}
		schemaMap = schema
	} else {
		// Format: direct schema {...}
		schemaMap = jsonSchema
	}

	return convertToSchema(schemaMap, "JSON schema", true)
}

// setStringField sets a string field in the schema if the key exists in the map
func setStringField(schemaMap map[string]any, key string, target *string, errPrefix string) error {
	value, ok := schemaMap[key]
	if !ok {
		return nil
	}

	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s: expected string for %s", errPrefix, key)
	}

	*target = str
	return nil
}

// setFloatField sets a float64 pointer field in the schema if the key exists in the map
func setFloatField(schemaMap map[string]any, key string, target **float64, errPrefix string) error {
	value, ok := schemaMap[key]
	if !ok {
		return nil
	}

	if floatVal, ok := value.(float64); ok {
		*target = &floatVal
		return nil
	}

	if intVal, ok := value.(int); ok {
		floatVal := float64(intVal)
		*target = &floatVal
		return nil
	}

	return fmt.Errorf("%s: expected number for %s", errPrefix, key)
}

// setInt64Field sets an int64 pointer field in the schema if the key exists in the map
func setInt64Field(schemaMap map[string]any, key string, target **int64, errPrefix string) error {
	value, ok := schemaMap[key]
	if !ok {
		return nil
	}

	if strVal, ok := value.(string); ok {
		// Parse string to int64 to maintain precision
		int64Val, err := strconv.ParseInt(strVal, 10, 64)
		if err != nil {
			return fmt.Errorf("%s: cannot parse string '%s' as int64 for %s", errPrefix, strVal, key)
		}
		*target = &int64Val
		return nil
	}

	if intVal, ok := value.(int64); ok {
		*target = &intVal
		return nil
	}

	if floatVal, ok := value.(float64); ok {
		int64Val := int64(floatVal)
		*target = &int64Val
		return nil
	}

	return fmt.Errorf("%s: expected number for %s", errPrefix, key)
}

func convertThinkingLevel(level string) genai.ThinkingLevel {
	switch level {
	case "low", "medium", "minimal":
		return genai.ThinkingLevelLow
	case "high":
		return genai.ThinkingLevelHigh
	}
	return genai.ThinkingLevelUnspecified
}

// randomToolID returns a random hex string of length n, used to assign IDs
// to streamed tool calls that arrive without one.
func randomToolID(n int) string {
	bytesLen := (n + 1) / 2
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should not fail; fall back to a fixed marker.
		return strings.Repeat("0", n)
	}
	return hex.EncodeToString(b)[:n]
}
