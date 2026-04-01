package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/shoucheng/my-first-agent/internal/agent"
	"github.com/shoucheng/my-first-agent/internal/llm"
	"github.com/shoucheng/my-first-agent/internal/memory"
	"github.com/shoucheng/my-first-agent/internal/tools"
	"github.com/shoucheng/my-first-agent/pkg/types"
)

func main() {
	ctx := context.Background()

	// 从环境变量加载配置
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is not set")
	}

	// 创建 LLM 客户端
	llmClient, err := llm.NewClient(llm.Config{
		Provider:    llm.ProviderOpenAI,
		APIKey:      apiKey,
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   2000,
	})
	if err != nil {
		log.Fatalf("Failed to create LLM client: %v", err)
	}

	// 创建记忆
	memory := memory.NewBufferMemory(10)

	// 创建工具注册表
	toolRegistry := tools.NewRegistry()

	// 注册工具
	if err := toolRegistry.Register(tools.NewCalculator()); err != nil {
		log.Fatalf("Failed to register calculator tool: %v", err)
	}

	// 创建智能体
	agentConfig := types.AgentConfig{
		MaxIterations: 10,
		Temperature:   0.7,
		Verbose:       true,
		Model:         "gpt-4",
	}

	myAgent, err := agent.New(llmClient, memory, toolRegistry, agentConfig)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 运行智能体
	question := "What is 25 * 4 + 10?"
	fmt.Printf("Question: %s\n", question)

	response, err := myAgent.Run(ctx, question)
	if err != nil {
		log.Fatalf("Agent execution failed: %v", err)
	}

	fmt.Printf("\nFinal Answer: %s\n", response)
}
