package memory

import (
	"context"
	"sync"

	"github.com/shoucheng/my-first-agent/pkg/types"
)

// BufferMemory 简单的缓冲记忆实现
type BufferMemory struct {
	mu       sync.RWMutex
	messages []types.Message
	maxSize  int // 最大保存消息数，0 表示无限制
}

// NewBufferMemory 创建新的缓冲记忆
func NewBufferMemory(maxSize int) *BufferMemory {
	return &BufferMemory{
		messages: make([]types.Message, 0),
		maxSize:  maxSize,
	}
}

// Add 添加消息到记忆
func (m *BufferMemory) Add(ctx context.Context, message types.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = append(m.messages, message)

	// 如果超过最大大小，移除最旧的消息
	if m.maxSize > 0 && len(m.messages) > m.maxSize {
		m.messages = m.messages[len(m.messages)-m.maxSize:]
	}

	return nil
}

// GetHistory 获取历史消息
func (m *BufferMemory) GetHistory(ctx context.Context) ([]types.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本以避免并发修改
	history := make([]types.Message, len(m.messages))
	copy(history, m.messages)

	return history, nil
}

// Clear 清空记忆
func (m *BufferMemory) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = make([]types.Message, 0)
	return nil
}
