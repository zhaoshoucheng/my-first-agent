package memory

// MemoryType 记忆类型
type MemoryType string

const (
	// MemoryTypeBuffer 缓冲记忆 - 保存最近的 N 条消息
	MemoryTypeBuffer MemoryType = "buffer"

	// MemoryTypeSummary 摘要记忆 - 对历史消息进行摘要
	MemoryTypeSummary MemoryType = "summary"

	// MemoryTypeVector 向量记忆 - 使用向量数据库存储和检索
	MemoryTypeVector MemoryType = "vector"
)

// Config 记忆配置
type Config struct {
	Type    MemoryType
	MaxSize int // 对于 Buffer 类型，限制保存的消息数量
}
