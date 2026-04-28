package llms

// ContextOption 定义上下文选项，用于各种增强功能的配置
// 这里不用 Type 来表示主要是考虑到如果后续要增加其他的 config，会导致后续的解析变得更复杂，所有的 config 都要找到上一个不是 config 的
// 消息类型
type ContextOption struct {
	CacheConfig *CacheConfig `json:"cache,omitempty"`
	// ...
	// TraceConfig

	Extra map[string]interface{} `json:"config,omitempty"`
}

func (co ContextOption) isPart() {}

type CacheConfig struct {
	Type string `json:"type,omitempty"`
}

func NewCacheOption() ContextOption {
	return ContextOption{
		CacheConfig: &CacheConfig{Type: "default"},
	}
}
