package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/shoucheng/my-first-agent/pkg/types"
)

// ReActAgent ReAct (Reasoning and Acting) 模式的智能体
type ReActAgent struct {
	*Agent
}

// NewReActAgent 创建 ReAct 智能体
func NewReActAgent(agent *Agent) *ReActAgent {
	return &ReActAgent{
		Agent: agent,
	}
}

// Run 运行 ReAct 智能体
func (r *ReActAgent) Run(ctx context.Context, input string) (string, error) {
	maxIterations := r.config.MaxIterations
	if maxIterations == 0 {
		maxIterations = 10
	}

	steps := []types.AgentStep{}

	for i := 0; i < maxIterations; i++ {
		if r.config.Verbose {
			fmt.Printf("\n=== Iteration %d ===\n", i+1)
		}

		// TODO: 实现 ReAct 循环
		// 1. 构建包含历史步骤的提示词
		// 2. 调用 LLM 获取下一步思考和行动
		// 3. 解析 Thought, Action, Action Input
		// 4. 执行 Action
		// 5. 获取 Observation
		// 6. 检查是否得到 Final Answer

		step := types.AgentStep{
			Thought:     "",
			Action:      "",
			ActionInput: "",
			Observation: "",
		}

		steps = append(steps, step)
	}

	return "", fmt.Errorf("ReAct agent not fully implemented yet")
}

// parseReActOutput 解析 ReAct 格式的输出
func parseReActOutput(output string) (thought, action, actionInput string, isFinal bool, finalAnswer string) {
	lines := strings.Split(output, "\n")

	thoughtRegex := regexp.MustCompile(`(?i)^Thought:\s*(.+)`)
	actionRegex := regexp.MustCompile(`(?i)^Action:\s*(.+)`)
	actionInputRegex := regexp.MustCompile(`(?i)^Action Input:\s*(.+)`)
	finalAnswerRegex := regexp.MustCompile(`(?i)^Final Answer:\s*(.+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if matches := thoughtRegex.FindStringSubmatch(line); len(matches) > 1 {
			thought = matches[1]
		} else if matches := actionRegex.FindStringSubmatch(line); len(matches) > 1 {
			action = matches[1]
		} else if matches := actionInputRegex.FindStringSubmatch(line); len(matches) > 1 {
			actionInput = matches[1]
		} else if matches := finalAnswerRegex.FindStringSubmatch(line); len(matches) > 1 {
			isFinal = true
			finalAnswer = matches[1]
		}
	}

	return
}
