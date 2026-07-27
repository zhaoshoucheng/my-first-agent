package builtin

import (
	"testing"

	"github.com/shoucheng/my-first-agent/internal/tools/sandbox"
)

// TestNewBuiltinRegistry 验证所有内置工具都被成功注册（而不是只注册了第一个）。
func TestNewBuiltinRegistry(t *testing.T) {
	sb := sandbox.NewLocalSandbox(t.TempDir())
	registry, err := NewBuiltinRegistry(sb)
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}

	// 注册表里登记的全部工具，应当与 register.go 列出的内置工具一一对应。
	want := []string{
		"calculator",
		"shell_exec", "shell_view",
		"file_read", "file_write_text", "file_replace_text", "file_append_text",
		"browser_navigate", "browser_view", "browser_click",
		"browser_input", "browser_scroll_up", "browser_scroll_down",
		"omni_search",
		"generate_video",
	}

	defs := registry.Definitions()
	if len(defs) != len(want) {
		t.Errorf("registered %d tools, want %d", len(defs), len(want))
	}

	for _, name := range want {
		if _, ok := registry.Get(name); !ok {
			t.Errorf("tool %q was not registered", name)
		}
	}
}
