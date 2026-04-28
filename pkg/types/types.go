// Package types 定义跨包共享的基础类型与接口。
//
// LLM 调用相关的类型 (Model, MessageContent, ContentResponse 等) 不再在此处重复定义，
// 统一使用 internal/llm/langchaingo/llms 中的 vendored 类型。
package types

import (
	"context"

	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
)

// Tool 工具接口
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input string) (string, error)
}

// Memory 记忆接口
//
// 直接采用 langchaingo 的 MessageContent 作为消息载体，
// 这样 Memory 历史可以直接喂给 model.GenerateContent。
type Memory interface {
	Add(ctx context.Context, message llms.MessageContent) error
	GetHistory(ctx context.Context) ([]llms.MessageContent, error)
	Clear(ctx context.Context) error
}

// AgentStep 智能体执行步骤
type AgentStep struct {
	Thought     string // 思考过程
	Action      string // 要执行的动作
	ActionInput string // 动作输入
	Observation string // 观察结果
}

// AgentConfig 智能体配置
type AgentConfig struct {
	MaxIterations int     // 最大迭代次数
	Temperature   float32 // LLM 温度参数
	Verbose       bool    // 是否输出详细日志
	Model         string  // 使用的模型名称
}
