package llm

import (
	"context"
	"fmt"

	"github.com/shoucheng/my-first-agent/pkg/types"
)

// Client LLM 客户端
type Client struct {
	config Config
	// 这里将添加实际的 LLM 客户端实现
}

// NewClient 创建新的 LLM 客户端
func NewClient(config Config) (*Client, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	return &Client{
		config: config,
	}, nil
}

// Generate 生成文本
func (c *Client) Generate(ctx context.Context, messages []types.Message) (string, error) {
	// TODO: 实现实际的 LLM 调用逻辑
	return "", fmt.Errorf("not implemented yet")
}

// GenerateWithOptions 使用自定义选项生成文本
func (c *Client) GenerateWithOptions(ctx context.Context, messages []types.Message, opts types.GenerateOptions) (string, error) {
	// TODO: 实现实际的 LLM 调用逻辑
	return "", fmt.Errorf("not implemented yet")
}
