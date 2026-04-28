package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/shoucheng/my-first-agent/domain/llm"
	"github.com/shoucheng/my-first-agent/infra/config"
	"github.com/shoucheng/my-first-agent/internal/agent"
	"github.com/shoucheng/my-first-agent/internal/memory"
	"github.com/shoucheng/my-first-agent/internal/tools"
	"github.com/shoucheng/my-first-agent/pkg/types"
)

// 基础示例：从 yaml 配置初始化 LLM 服务，再装配出一个 agent。
func main() {
	ctx := context.Background()

	// 步骤 1: 读 yaml 配置，存入全局单例。
	if err := config.Init("config/config.yaml"); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	cfg := config.Get()

	// 步骤 2: 初始化 LLM 服务（内部初始化账号服务）。
	llmSvc, err := llm.NewService(ctx, cfg.LLM)
	if err != nil {
		log.Fatalf("初始化 LLM 服务失败: %v", err)
	}

	// 步骤 3: memory + tools。
	mem := memory.NewBufferMemory(10)
	toolRegistry := tools.NewRegistry()
	if err := toolRegistry.Register(tools.NewCalculator()); err != nil {
		log.Fatalf("注册计算器工具失败: %v", err)
	}

	// 步骤 4: 装配 agent。
	myAgent, err := agent.New(llmSvc, mem, toolRegistry, types.AgentConfig{
		MaxIterations: 5,
		Temperature:   0.7,
		Verbose:       true,
	})
	if err != nil {
		log.Fatalf("创建智能体失败: %v", err)
	}

	// 步骤 5: 运行。
	for _, question := range []string{
		"计算 15 * 8",
		"100 除以 4 是多少?",
	} {
		fmt.Printf("\n问题: %s\n", question)
		resp, err := myAgent.Run(ctx, question)
		if err != nil {
			log.Printf("执行失败: %v", err)
			continue
		}
		fmt.Printf("答案: %s\n", resp)
		fmt.Println(strings.Repeat("-", 50))
	}
}
