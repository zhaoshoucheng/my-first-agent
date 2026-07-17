package tools

import (
	"context"
	"fmt"
	"testing"

	llmstypes "github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
)

// echoTool 是一个最小的可控 Tool 实现，用来驱动 registry 的解析/执行路径。
type echoTool struct {
	name     string
	required []string
	props    map[string]any
	execErr  error
}

func (e *echoTool) Name() string        { return e.name }
func (e *echoTool) Description() string { return "echo" }
func (e *echoTool) Parameters() JSONSchema {
	return ObjectSchema(e.props, e.required...)
}
func (e *echoTool) Execute(_ context.Context, args map[string]any) (string, error) {
	if e.execErr != nil {
		return "", e.execErr
	}
	return fmt.Sprintf("%v", args["text"]), nil
}

func newEchoTool() *echoTool {
	return &echoTool{
		name:     "echo",
		required: []string{"text"},
		props:    map[string]any{"text": StringProperty("text to echo")},
	}
}

func toolCall(id, name, args string) llmstypes.ToolCall {
	return llmstypes.ToolCall{
		ID:           id,
		Type:         "function",
		FunctionCall: &llmstypes.FunctionCall{Name: name, Arguments: args},
	}
}

func TestRegistry_RegisterDuplicateAndEmpty(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(nil); err == nil {
		t.Error("expected error registering nil tool")
	}
	if err := r.Register(newEchoTool()); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := r.Register(newEchoTool()); err == nil {
		t.Error("expected error registering duplicate name")
	}
}

func TestRegistry_ParseToolCall(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newEchoTool())

	cases := []struct {
		name string
		call llmstypes.ToolCall
		want ParseStatus
	}{
		{"ok", toolCall("1", "echo", `{"text":"hi"}`), ParseStatusOK},
		{"unknown tool", toolCall("2", "ghost", `{}`), ParseStatusUnknown},
		{"missing required", toolCall("3", "echo", `{}`), ParseStatusWrongArgs},
		{"unknown arg", toolCall("4", "echo", `{"text":"hi","extra":1}`), ParseStatusWrongArgs},
		{"wrong type", toolCall("5", "echo", `{"text":123}`), ParseStatusWrongArgs},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.ParseToolCall(tc.call)
			if got.Status != tc.want {
				t.Errorf("status = %q, want %q (err=%v)", got.Status, tc.want, got.Err)
			}
		})
	}
}

func TestRegistry_ParseToolCall_NoFunctionPayload(t *testing.T) {
	r := NewRegistry()
	got := r.ParseToolCall(llmstypes.ToolCall{ID: "x"})
	if got.Status != ParseStatusWrongArgs {
		t.Errorf("status = %q, want wrong_args", got.Status)
	}
}

// 损坏的 JSON 应被 jsonrepair 修复后正常解析。
func TestRegistry_ParseToolCall_RepairsBrokenJSON(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newEchoTool())
	got := r.ParseToolCall(toolCall("1", "echo", `{"text":"hi",}`)) // 尾随逗号
	if got.Status != ParseStatusOK {
		t.Fatalf("status = %q, err=%v", got.Status, got.Err)
	}
	if got.Arguments["text"] != "hi" {
		t.Errorf("args = %v", got.Arguments)
	}
}

func TestRegistry_ExecuteToolCall(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newEchoTool())

	// 正常执行。
	res := r.ExecuteToolCall(context.Background(), toolCall("1", "echo", `{"text":"hello"}`))
	if res.IsError {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if res.Content != "hello" {
		t.Errorf("content = %q", res.Content)
	}

	// 解析失败（未知工具）应转成 IsError 结果，而不是 panic。
	res = r.ExecuteToolCall(context.Background(), toolCall("2", "ghost", `{}`))
	if !res.IsError {
		t.Error("expected IsError for unknown tool")
	}
}

func TestRegistry_ExecuteToolCall_ExecError(t *testing.T) {
	r := NewRegistry()
	tool := newEchoTool()
	tool.execErr = fmt.Errorf("boom")
	_ = r.Register(tool)

	res := r.ExecuteToolCall(context.Background(), toolCall("1", "echo", `{"text":"x"}`))
	if !res.IsError || res.Content != "boom" {
		t.Errorf("expected error result with message, got %+v", res)
	}
}

func TestRegistry_Definitions_Sorted(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&echoTool{name: "zebra", props: map[string]any{}})
	_ = r.Register(&echoTool{name: "alpha", props: map[string]any{}})

	defs := r.Definitions()
	if len(defs) != 2 {
		t.Fatalf("got %d defs", len(defs))
	}
	if defs[0].Function.Name != "alpha" || defs[1].Function.Name != "zebra" {
		t.Errorf("definitions not sorted: %s, %s", defs[0].Function.Name, defs[1].Function.Name)
	}
}

func TestAsFloat(t *testing.T) {
	cases := []struct {
		in   any
		ok   bool
		want float64
	}{
		{float64(1.5), true, 1.5},
		{int(3), true, 3},
		{int64(4), true, 4},
		{"nope", false, 0},
		{nil, false, 0},
	}
	for _, tc := range cases {
		got, ok := AsFloat(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("AsFloat(%v) = (%v,%v), want (%v,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
