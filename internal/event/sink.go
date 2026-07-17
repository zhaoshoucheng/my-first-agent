package event

import "fmt"

// Sink 事件外化出口。展示形态未定（终端直出 / 前端渲染），先收口成接口：
// 核心循环只管 Handle，换实现不动循环。
type Sink interface {
	Handle(e Event)
}

// ConsoleSink 最简终端外化（占位实现）。
type ConsoleSink struct{}

func (ConsoleSink) Handle(e Event) {
	switch p := e.Payload.(type) {
	case UserMessagePayload:
		fmt.Printf("> %s\n", p.Text)
	case ActionStartPayload:
		if p.Thought != "" {
			fmt.Printf("· %s\n", p.Thought)
		}
		fmt.Printf("[%s] %s\n", p.Tool, p.Input)
	case ObservationPayload:
		mark := "✓"
		if p.IsError {
			mark = "✗"
		}
		fmt.Printf("%s %s: %s\n", mark, p.Tool, p.Content)
	case PlanUpdatePayload:
		fmt.Printf("◇ 计划：%s\n", p.Summary)
	case AssistantMessagePayload:
		fmt.Printf("\n%s\n", p.Text)
	case StatusPayload:
		fmt.Printf("… %s\n", p.Text)
	case TaskDonePayload:
		fmt.Printf("— task %s (%s)\n", e.TaskID, p.Reason)
	default:
		fmt.Printf("(%s)\n", e.Type)
	}
}
