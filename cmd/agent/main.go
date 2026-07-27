package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/shoucheng/my-first-agent/domain/account"
	"github.com/shoucheng/my-first-agent/domain/llm"
	"github.com/shoucheng/my-first-agent/domain/video"
	"github.com/shoucheng/my-first-agent/infra/config"
	"github.com/shoucheng/my-first-agent/internal/tools/builtin"
	"github.com/shoucheng/my-first-agent/internal/tools/sandbox"
	"github.com/shoucheng/my-first-agent/pkg/types"
)

func main() {
	cfgPath := flag.String("config", "config/config.yaml", "path to YAML config")
	model := flag.String("model", "claude-3-5-sonnet-20241022", "model name to call")
	prompt := flag.String("prompt", "用一句中文介绍你自己。", "prompt to send")
	flag.Parse()

	ctx := context.Background()

	if err := config.Init(*cfgPath); err != nil {
		log.Fatalf("init config: %v", err)
	}

	account.Init(ctx)
	llm.Init()
	video.Init()

	settings := config.GetConfig()
	agentConfig := types.AgentConfig{MaxIterations: 10, Verbose: true}
	if settings != nil {
		agentConfig.MaxIterations = settings.Agent.MaxIterations
		agentConfig.Verbose = settings.Agent.Verbose
	}

	workDir, _ := os.Getwd()
	sb := sandbox.NewLocalSandbox(workDir)
	toolRegistry, err := builtin.NewBuiltinRegistry(sb)
	if err != nil {
		log.Fatalf("init tools: %v", err)
	}

	myAgent, err := NewAgent(llm.Default(), *model, toolRegistry, agentConfig)
	if err != nil {
		log.Fatalf("init agent: %v", err)
	}

	resp, err := myAgent.Run(ctx, *prompt)
	if err != nil {
		log.Fatalf("run agent: %v", err)
	}
	fmt.Println(resp)
}
