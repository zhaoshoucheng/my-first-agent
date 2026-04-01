package tools

import (
	"fmt"
	"sync"

	"github.com/shoucheng/my-first-agent/pkg/types"
)

// Registry 工具注册表
type Registry struct {
	mu    sync.RWMutex
	tools map[string]types.Tool
}

// NewRegistry 创建新的工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]types.Tool),
	}
}

// Register 注册工具
func (r *Registry) Register(tool types.Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}

	r.tools[name] = tool
	return nil
}

// Get 获取工具
func (r *Registry) Get(name string) (types.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return tool, nil
}

// List 列出所有工具
func (r *Registry) List() []types.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]types.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	return tools
}

// GetDescriptions 获取所有工具的描述（用于提示词）
func (r *Registry) GetDescriptions() string {
	tools := r.List()
	desc := "Available tools:\n"
	for _, tool := range tools {
		desc += fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description())
	}
	return desc
}
