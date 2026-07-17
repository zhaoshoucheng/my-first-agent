// Package event 定义 Alembic 的事件流：agent 的一切行为都是事件。
// 上下文、展示、持久化都从这一条流派生，见 docs/architecture/agent-core.md §3.1。
package event

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Type 事件类型
type Type string

const (
	TypeUserMessage      Type = "user_message"      // 用户输入，触发迭代
	TypeActionStart      Type = "action_start"      // 一次工具调用决定（思考+动作合一）
	TypeObservation      Type = "observation"       // 工具执行结果，触发下一轮迭代
	TypePlanUpdate       Type = "plan_update"       // 计划创建/修订/推进
	TypeAssistantMessage Type = "assistant_message" // 给用户的正式答复
	TypeStatus           Type = "status"            // 过程提示，仅外化不进上下文
	TypeTaskDone         Type = "task_done"         // 任务结束
)

// IsTrigger 是否为"迭代触发器"：仅 user_message 和 observation 会驱动下一轮循环。
// 这取代了 for 循环——循环的"下一圈"由事件驱动，而非控制流驱动。
func (t Type) IsTrigger() bool {
	return t == TypeUserMessage || t == TypeObservation
}

// task_done 的结束原因
const (
	ReasonDone          = "done"           // 正常交付
	ReasonMaxIterations = "max_iterations" // 迭代上限
	ReasonInterrupted   = "interrupted"    // 用户打断 / ctx 取消
	ReasonError         = "error"          // 不可修复错误
)

// Event 事件流的基本单元
type Event struct {
	ID      string
	TaskID  string
	Type    Type
	Payload any // 类型化负载，按 Type 取断言，见下方 *Payload 类型
	Time    time.Time
}

var seq atomic.Int64

// New 构造事件。ID 暂为进程内自增序号，持久化方案定型后再换。
func New(taskID string, typ Type, payload any) Event {
	return Event{
		ID:      fmt.Sprintf("evt_%d", seq.Add(1)),
		TaskID:  taskID,
		Type:    typ,
		Payload: payload,
		Time:    time.Now(),
	}
}

// UserMessagePayload user_message 负载
type UserMessagePayload struct {
	Text string
}

// ActionStartPayload action_start 负载
type ActionStartPayload struct {
	Thought    string // 模型本轮的思考文本
	ToolCallID string
	Tool       string
	Input      string // 工具参数（原始 JSON）
}

// ObservationPayload observation 负载。失败也是事件，不是异常。
type ObservationPayload struct {
	ToolCallID string
	Tool       string
	Content    string
	IsError    bool
}

// PlanUpdatePayload plan_update 负载。
// M5 计划层落地后携带完整计划；当前只有紧凑文本摘要（供上下文注入）。
type PlanUpdatePayload struct {
	Summary string
}

// AssistantMessagePayload assistant_message 负载
type AssistantMessagePayload struct {
	Text string
}

// StatusPayload status 负载
type StatusPayload struct {
	Text string
}

// TaskDonePayload task_done 负载
type TaskDonePayload struct {
	Reason string // 见 Reason* 常量
}
