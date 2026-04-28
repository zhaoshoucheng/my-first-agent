package llms

import (
	"strings"

	"github.com/spf13/cast"
)

type CompletionTokensDetails struct {
	ReasoningTokens          int              `json:"reasoning_tokens"`
	AcceptedPredictionTokens int              `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int              `json:"rejected_prediction_tokens"`
	CompletionTokensModality map[string]int64 `json:"completion_tokens_modality,omitempty"`
}

type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`

	// anthropic 的 cache token 信息
	CacheCreationInputTokens int64            `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int64            `json:"cache_read_input_tokens,omitempty"`
	PromptTokensModality     map[string]int64 `json:"prompt_tokens_modality,omitempty"`
}

type Usage struct {
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	TrafficType      string `json:"traffic_type,omitempty"`

	// 下面两个暂时不支持
	// PromptTokensDetails is the details of the prompt tokens.
	PromptTokensDetails PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
	// CompletionTokensDetails is the details of the completion tokens.
	CompletionTokensDetails CompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

func ExtractUsage(generationInfo map[string]any) Usage {
	usage := Usage{}
	if len(generationInfo) == 0 {
		return usage
	}

	// 兼容 anthropic/openai/vertex 多个返回值
	if val, ok := generationInfo["input_tokens"]; ok {
		usage.PromptTokens = cast.ToInt(val)
	} else if val, ok := generationInfo["PromptTokens"]; ok {
		usage.PromptTokens = cast.ToInt(val)
	} else if val, ok := generationInfo["InputTokens"]; ok {
		usage.PromptTokens = cast.ToInt(val)
	}

	if val, ok := generationInfo["CompletionTokens"]; ok {
		usage.CompletionTokens = cast.ToInt(val)
	} else if val, ok := generationInfo["OutputTokens"]; ok {
		usage.CompletionTokens = cast.ToInt(val)
	} else if val, ok := generationInfo["output_tokens"]; ok {
		usage.CompletionTokens = cast.ToInt(val)
	}

	if val, ok := generationInfo["CachedTokens"]; ok {
		usage.PromptTokensDetails.CachedTokens = cast.ToInt(val)
	} else if val, ok := generationInfo["cached_tokens"]; ok {
		usage.PromptTokensDetails.CachedTokens = cast.ToInt(val)
	}

	if val, ok := generationInfo["cache_read_input_tokens"]; ok {
		usage.PromptTokensDetails.CacheReadInputTokens = cast.ToInt64(val)
	}

	if val, ok := generationInfo["cache_creation_input_tokens"]; ok {
		usage.PromptTokensDetails.CacheCreationInputTokens = cast.ToInt64(val)
	}

	// 给 azure openai 使用
	if promptTokens, ok := generationInfo["PromptTokensDetails"].(map[string]int); ok {
		usage.PromptTokensDetails.CachedTokens = promptTokens["CachedTokens"]
	}

	//thinking token记录
	if val, ok := generationInfo["thoughts_tokens"]; ok {
		usage.CompletionTokensDetails.ReasoningTokens = cast.ToInt(val)
	}
	// OpenAI reasoning tokens from CompletionTokensDetails
	if completionDetails, ok := generationInfo["CompletionTokensDetails"].(map[string]int); ok {
		if reasoningTokens, exists := completionDetails["ReasoningTokens"]; exists {
			usage.CompletionTokensDetails.ReasoningTokens = reasoningTokens
		}
	}
	if val, ok := generationInfo["accepted_prediction_tokens"]; ok {
		usage.CompletionTokensDetails.AcceptedPredictionTokens = cast.ToInt(val)
	}
	if val, ok := generationInfo["rejected_prediction_tokens"]; ok {
		usage.CompletionTokensDetails.RejectedPredictionTokens = cast.ToInt(val)
	}

	//多模态token记录
	if val, ok := generationInfo["input_tokens_detail"]; ok {
		inputTokenDetail := cast.ToStringMapInt64(val)
		for modality, tokens := range inputTokenDetail {
			if usage.PromptTokensDetails.PromptTokensModality == nil {
				usage.PromptTokensDetails.PromptTokensModality = make(map[string]int64)
			}
			usage.PromptTokensDetails.PromptTokensModality[strings.ToLower(modality)] = tokens
		}
	}
	if val, ok := generationInfo["output_tokens_detail"]; ok {
		outputTokenDetail := cast.ToStringMapInt64(val)
		for modality, tokens := range outputTokenDetail {
			if usage.CompletionTokensDetails.CompletionTokensModality == nil {
				usage.CompletionTokensDetails.CompletionTokensModality = make(map[string]int64)
			}
			usage.CompletionTokensDetails.CompletionTokensModality[strings.ToLower(modality)] = tokens
		}
	}
	if val, ok := generationInfo["traffic_type"]; ok {
		usage.TrafficType = strings.ToLower(cast.ToString(val))
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	// claude
	// - prompt_tokens：不计入缓存系统的 token
	// - cache_creation_input_tokens: 缓存写
	// - cache_read_input_tokens：缓存读
	// total_tokens = prompt_tokens + cache_creation_input_tokens + cache_read_input_tokens
	//
	// gemini / gpt
	// 	prompt_tokens:  全部的输入 token
	// cached_tokens：全部输入 token 里面有缓存读的部分

	// gpt gemini 全部对齐为claude的模式
	if usage.PromptTokensDetails.CachedTokens > 0 {
		// FIXME: 暂时兼容，找出为什么会有cached_tokens > 0 而prompt_tokens为0的情况
		if usage.PromptTokens >= usage.PromptTokensDetails.CachedTokens {
			usage.PromptTokens = usage.PromptTokens - usage.PromptTokensDetails.CachedTokens
		}
		usage.PromptTokensDetails.CacheReadInputTokens = int64(usage.PromptTokensDetails.CachedTokens)
	}
	return usage
}
