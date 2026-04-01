package main

import (
	"context"
	"fmt"
	"log"

	"github.com/shoucheng/my-first-agent/internal/agent"
	"github.com/shoucheng/my-first-agent/internal/llm"
	"github.com/shoucheng/my-first-agent/internal/memory"
	"github.com/shoucheng/my-first-agent/internal/tools"
	"github.com/shoucheng/my-first-agent/pkg/types"
)

// 这是一个基础示例，展示如何创建和使用智能体
func main() {
	ctx := context.Background()

	// 步骤 1: 创建 LLM 客户端
	llmClient, err := llm.NewClient(llm.Config{
		Provider:    llm.ProviderOpenAI,
		APIKey:      "your-api-key-here", // 在实际使用中应该从环境变量读取
		Model:       "gpt-4",
		Temperature: 0.7,
	})
	if err != nil {
		log.Fatalf("创建 LLM 客户端失败: %v", err)
	}

	// 步骤 2: 创建记忆系统
	mem := memory.NewBufferMemory(10) // 保留最近 10 条消息

	// 步骤 3: 创建工具注册表并注册工具
	toolRegistry := tools.NewRegistry()

	calculator := tools.NewCalculator()
	if err := toolRegistry.Register(calculator); err != nil {
		log.Fatalf("注册计算器工具失败: %v", err)
	}

	// 步骤 4: 配置智能体
	config := types.AgentConfig{
		MaxIterations: 5,
		Temperature:   0.7,
		Verbose:       true,
		Model:         "gpt-4",
	}

	// 步骤 5: 创建智能体
	myAgent, err := agent.New(llmClient, mem, toolRegistry, config)
	if err != nil {
		log.Fatalf("创建智能体失败: %v", err)
	}

	// 步骤 6: 运行智能体
	questions := []string{
		"计算 15 * 8",
		"100 除以 4 是多少?",
	}

	for _, question := range questions {
		fmt.Printf("\n问题: %s\n", question)

		response, err := myAgent.Run(ctx, question)
		if err != nil {
			log.Printf("执行失败: %v", err)
			continue
		}

		fmt.Printf("答案: %s\n", response)
		fmt.Println(strings.Repeat("-", 50))
	}
}
