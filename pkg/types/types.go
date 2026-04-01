package types

import "context"

// Message 表示对话消息
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// ToolInput 工具输入
type ToolInput struct {
	Name   string
	Args   map[string]interface{}
}

// ToolOutput 工具输出
type ToolOutput struct {
	Name   string
	Result string
	Error  error
}

// AgentStep 智能体执行步骤
type AgentStep struct {
	Thought    string      // 思考过程
	Action     string      // 要执行的动作
	ActionInput string     // 动作输入
	Observation string     // 观察结果
}

// AgentConfig 智能体配置
type AgentConfig struct {
	MaxIterations int     // 最大迭代次数
	Temperature   float32 // LLM 温度参数
	Verbose       bool    // 是否输出详细日志
	Model         string  // 使用的模型名称
}

// Tool 工具接口
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input string) (string, error)
}

// LLMClient LLM 客户端接口
type LLMClient interface {
	Generate(ctx context.Context, messages []Message) (string, error)
	GenerateWithOptions(ctx context.Context, messages []Message, opts GenerateOptions) (string, error)
}

// GenerateOptions LLM 生成选项
type GenerateOptions struct {
	Temperature float32
	MaxTokens   int
	StopWords   []string
}

// Memory 记忆接口
type Memory interface {
	Add(ctx context.Context, message Message) error
	GetHistory(ctx context.Context) ([]Message, error)
	Clear(ctx context.Context) error
}
