package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/shoucheng/my-first-agent/domain/account"
	"github.com/shoucheng/my-first-agent/domain/llm"
	"github.com/shoucheng/my-first-agent/infra/config"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
)

func main() {
	cfgPath := flag.String("config", "config/config.yaml", "path to YAML config")
	model := flag.String("model", "claude-3-5-sonnet-20241022", "model name to call")
	prompt := flag.String("prompt", "用一句中文介绍你自己。", "prompt to send")
	flag.Parse()

	ctx := context.Background()

	// 1. 读 yaml 配置（infra），存入全局单例。
	if err := config.Init(*cfgPath); err != nil {
		log.Fatalf("init config: %v", err)
	}

	// 2. 各 domain 模块独立 Init，按依赖顺序：
	account.Init(ctx)

	if err := llm.Init(ctx); err != nil {
		log.Fatalf("init llm: %v", err)
	}

	if len(llm.Default().Accounts()) == 0 {
		log.Fatalf("no accounts loaded")
	}

	// 3. 调一次模型：根据 model 名路由账号 → 建 client → GenerateContent。
	resp, err := llm.Default().GenerateContent(ctx, *model, []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextContent{Text: *prompt}},
		},
	})
	if err != nil {
		log.Fatalf("generate: %v", err)
	}
	if len(resp.Choices) == 0 {
		log.Fatalf("empty response")
	}
	fmt.Println(resp.Choices[0].Content)
}
