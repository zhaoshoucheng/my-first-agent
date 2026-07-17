package task

// PhaseStatus 阶段状态
type PhaseStatus string

const (
	PhaseTodo  PhaseStatus = "todo"
	PhaseDoing PhaseStatus = "doing"
	PhaseDone  PhaseStatus = "done"
)

// Phase 计划中的一个阶段
type Phase struct {
	ID     int
	Title  string
	Status PhaseStatus
}

// Plan 任务计划，由 agent 通过元工具（plan_update / plan_advance）维护。
type Plan struct {
	Phases  []Phase
	Current int // 当前阶段下标
}
