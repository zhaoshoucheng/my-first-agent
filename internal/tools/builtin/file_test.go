package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shoucheng/my-first-agent/internal/tools/sandbox"
)

// fileTools 在临时目录上构造一套基于本地 sandbox 的 file 工具。
func fileTools(t *testing.T) (sandbox.Sandbox, string) {
	t.Helper()
	dir := t.TempDir()
	return sandbox.NewLocalSandbox(dir), dir
}

func TestFileWriteAndRead(t *testing.T) {
	sb, dir := fileTools(t)
	ctx := context.Background()
	path := filepath.Join(dir, "note.txt")

	w := NewFileWriteText(sb)
	if _, err := w.Execute(ctx, map[string]any{
		"abs_path": path,
		"content":  "hello\nworld",
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	// append_newline 默认 true，写入时应自动补一个换行。
	data, _ := os.ReadFile(path)
	if string(data) != "hello\nworld\n" {
		t.Fatalf("on-disk content = %q", string(data))
	}

	r := NewFileRead(sb)
	got, err := r.Execute(ctx, map[string]any{"abs_path": path})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "hello\nworld\n" {
		t.Errorf("read content = %q", got)
	}
}

func TestFileRead_LineRange(t *testing.T) {
	sb, dir := fileTools(t)
	ctx := context.Background()
	path := filepath.Join(dir, "lines.txt")
	os.WriteFile(path, []byte("a\nb\nc\nd\ne"), 0o644)

	r := NewFileRead(sb)
	// [1,3) → 取 b、c 两行（0-based，左闭右开）。
	got, err := r.Execute(ctx, map[string]any{
		"abs_path":   path,
		"line_range": []any{float64(1), float64(3)},
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "b\nc" {
		t.Errorf("line_range [1,3) = %q, want \"b\\nc\"", got)
	}
}

func TestFileRead_MissingPath(t *testing.T) {
	sb, _ := fileTools(t)
	r := NewFileRead(sb)
	if _, err := r.Execute(context.Background(), map[string]any{}); err == nil {
		t.Error("expected error when abs_path missing")
	}
}

func TestFileReplaceText(t *testing.T) {
	sb, dir := fileTools(t)
	ctx := context.Background()
	path := filepath.Join(dir, "code.txt")
	os.WriteFile(path, []byte("foo bar foo"), 0o644)

	repl := NewFileReplaceText(sb)

	// old_str 出现多次且未开启 replace_all → 应报错，文件不变。
	if _, err := repl.Execute(ctx, map[string]any{
		"abs_path": path, "old_str": "foo", "new_str": "X",
	}); err == nil {
		t.Error("expected error for ambiguous replace without replace_all")
	}

	// 开启 replace_all → 全部替换。
	if _, err := repl.Execute(ctx, map[string]any{
		"abs_path": path, "old_str": "foo", "new_str": "X", "replace_all": true,
	}); err != nil {
		t.Fatalf("replace_all: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "X bar X" {
		t.Errorf("after replace_all = %q", string(data))
	}

	// old_str 不存在 → 报错。
	if _, err := repl.Execute(ctx, map[string]any{
		"abs_path": path, "old_str": "nope", "new_str": "Y",
	}); err == nil {
		t.Error("expected error when old_str not found")
	}
}

func TestFileAppendText(t *testing.T) {
	sb, dir := fileTools(t)
	ctx := context.Background()
	path := filepath.Join(dir, "log.txt")

	a := NewFileAppendText(sb)
	// 文件不存在时 append 等价于创建。
	if _, err := a.Execute(ctx, map[string]any{
		"abs_path": path, "content": "line1", "append_newline": true,
	}); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if _, err := a.Execute(ctx, map[string]any{
		"abs_path": path, "content": "line2", "append_newline": false,
	}); err != nil {
		t.Fatalf("append 2: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "line1\nline2" {
		t.Errorf("appended content = %q", string(data))
	}
	if !strings.HasPrefix(string(data), "line1") {
		t.Errorf("unexpected content %q", string(data))
	}
}
