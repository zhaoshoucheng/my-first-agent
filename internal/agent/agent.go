package agent

import (
	"context"
	"fmt"

	"github.com/shoucheng/my-first-agent/internal/tools"
	"github.com/shoucheng/my-first-agent/pkg/types"
)

// Agent 智能体
type Agent struct {
	llm      types.LLMClient
	memory   types.Memory
	tools    *tools.Registry
	config   types.AgentConfig
	executor *Executor
}

// New 创建新的智能体
func New(llm types.LLMClient, memory types.Memory, toolRegistry *tools.Registry, config types.AgentConfig) (*Agent, error) {
	if llm == nil {
		return nil, fmt.Errorf("llm client is required")
	}

	agent := &Agent{
		llm:    llm,
		memory: memory,
		tools:  toolRegistry,
		config: config,
	}

	// 创建执行器
	agent.executor = NewExecutor(agent)

	return agent, nil
}

// Run 运行智能体
func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	// 添加用户输入到记忆
	if a.memory != nil {
		if err := a.memory.Add(ctx, types.Message{
			Role:    "user",
			Content: input,
		}); err != nil {
			return "", fmt.Errorf("failed to add message to memory: %w", err)
		}
	}

	// 使用执行器运行智能体
	response, err := a.executor.Execute(ctx, input)
	if err != nil {
		return "", err
	}

	// 添加响应到记忆
	if a.memory != nil {
		if err := a.memory.Add(ctx, types.Message{
			Role:    "assistant",
			Content: response,
		}); err != nil {
			return "", fmt.Errorf("failed to add response to memory: %w", err)
		}
	}

	return response, nil
}

// GetLLM 获取 LLM 客户端
func (a *Agent) GetLLM() types.LLMClient {
	return a.llm
}

// GetMemory 获取记忆
func (a *Agent) GetMemory() types.Memory {
	return a.memory
}

// GetTools 获取工具注册表
func (a *Agent) GetTools() *tools.Registry {
	return a.tools
}

// GetConfig 获取配置
func (a *Agent) GetConfig() types.AgentConfig {
	return a.config
}
