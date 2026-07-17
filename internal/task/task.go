// Package task 管理任务与其事件历史：task_id 是上下文存取的唯一键。
// 一次任务 = 一个 task_id = 一条只追加的事件流，多轮对话即向同一 task 追加。
package task

import (
	"context"

	"github.com/shoucheng/my-first-agent/internal/event"
)

// Task 一次任务
type Task struct {
	ID     string
	Events []event.Event // 只追加
	Plan   *Plan         // 当前计划，可为 nil
}

// Store 任务存取接口。当前只有内存实现；
// 将来换成文件/SQLite 实现即获得持久化与 resume，循环代码不动。
type Store interface {
	Create(ctx context.Context) (*Task, error)
	Get(ctx context.Context, taskID string) (*Task, error)
	Append(ctx context.Context, taskID string, events ...event.Event) error
}
