package llm

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/shoucheng/my-first-agent/domain/account"
	"github.com/shoucheng/my-first-agent/infra/config"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
)

func TestMain(m *testing.M) {
	cfPath := os.Getenv("CONFIG_PATH")
	err := config.Init(cfPath)
	if err != nil {
		panic(err)
	}
	conf := config.GetConfig()
	conf.Account.Source.File.Dir = os.Getenv("CONFIG_ACCOUNT_DIR")
	account.Init(context.Background())
	Init()
	code := m.Run()
	os.Exit(code)
}

// fakeModel 是一个最小的 llms.Model 实现，用来在不联网的前提下测试
// Service.GenerateContent 的派发 / 路由 / 选项透传等行为。
//
// 它会把每次调用的入参完整存下来，并按调用方注入的 generate 函数返回结果，
// 从而方便在测试里断言 "调用方传进来的 model / messages / opts 是不是被
// 正确转发到了底层 client"。
type fakeModel struct {
	mu sync.Mutex

	// 配置：返回值；如果 generate 不为 nil，优先用它。
	resp *llms.ContentResponse
	err  error
	// 自定义返回函数，可以根据调用入参动态构造返回值。
	generate func(ctx context.Context, messages []llms.MessageContent, opts []llms.CallOption) (*llms.ContentResponse, error)

	// 录像：把每次调用的入参原样录下来。
	calls []fakeCall
}

type fakeCall struct {
	messages []llms.MessageContent
	opts     []llms.CallOption
	// resolved 是 opts 全部 apply 之后的快照，断言时直接看这个最方便。
	resolved llms.CallOptions
}

func (f *fakeModel) Call(ctx context.Context, prompt string, opts ...llms.CallOption) (string, error) {
	resp, err := f.GenerateContent(ctx, []llms.MessageContent{
		{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextContent{Text: prompt}}},
	}, opts...)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Content, nil
}

func (f *fakeModel) GenerateContent(
	ctx context.Context,
	messages []llms.MessageContent,
	opts ...llms.CallOption,
) (*llms.ContentResponse, error) {
	resolved := llms.CallOptions{}
	for _, o := range opts {
		o(&resolved)
	}

	f.mu.Lock()
	f.calls = append(f.calls, fakeCall{messages: messages, opts: opts, resolved: resolved})
	f.mu.Unlock()

	if f.generate != nil {
		return f.generate(ctx, messages, opts)
	}
	return f.resp, f.err
}

func (f *fakeModel) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeModel) lastCall() fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[len(f.calls)-1]
}

// testAccount 简化测试账号的构造。
func testAccount(name string, p account.Provider) *account.Account {
	return &account.Account{
		Name:       name,
		Provider:   p,
		Credential: account.Credential{APIKey: "test-key"},
	}
}

// helper：返回一个 ContentResponse 让 mock 直接吐出来。
func textResponse(text string) *llms.ContentResponse {
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: text}},
	}
}

// ---------------------------------------------------------------------------
// 三个 Provider 的派发测试
// ---------------------------------------------------------------------------

// TestGenerateContent_RoutingPerProvider 覆盖三个平台的最基本派发：
// 给 model 名 → 选对账号 → 调用对应 mock client。
func TestGenerateContent_RoutingPerProvider(t *testing.T) {
	cases := []struct {
		name        string
		model       string
		wantAccount string
	}{
		{"anthropic", "claude-opus-4-7", "claude-account"},
		{"azure-openai", "gpt-5-5", "azure-account"},
		//	{"gemini", "gemini-2.5-pro", "gemini-account"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			anthMock := &fakeModel{resp: textResponse("anth-ok")}
			azureMock := &fakeModel{resp: textResponse("azure-ok")}
			geminiMock := &fakeModel{resp: textResponse("gemini-ok")}

			svc := Default()
			resp, err := svc.GenerateContent(context.Background(), tc.model,
				[]llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "hi")},
			)
			if err != nil {
				t.Fatalf("GenerateContent: %v", err)
			}

			// 只有目标账号的 mock 应该收到调用。
			byAccount := map[string]*fakeModel{
				"claude-account": anthMock,
				"azure-account":  azureMock,
				"gemini-account": geminiMock,
			}
			for accName, mock := range byAccount {
				want := 0
				if accName == tc.wantAccount {
					want = 1
				}
				if got := mock.callCount(); got != want {
					t.Errorf("account %q: got %d calls, want %d", accName, got, want)
				}
			}

			// 模型名必须以 CallOption 形式透传到底层 client。
			last := byAccount[tc.wantAccount].lastCall()
			if last.resolved.Model != tc.model {
				t.Errorf("Model option: got %q, want %q", last.resolved.Model, tc.model)
			}

			// 返回值应原样回传。
			if len(resp.Choices) != 1 || !strings.HasSuffix(resp.Choices[0].Content, "-ok") {
				t.Errorf("response not propagated, got %+v", resp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 函数调用 / Tool use 透传测试
// ---------------------------------------------------------------------------

// TestGenerateContent_FunctionCalling 验证 Tools / ToolChoice 等 CallOption
// 能被透传到底层 client，并且 ToolCall 形式的响应能原样回到调用方。
func TestGenerateContent_FunctionCalling(t *testing.T) {
	weatherTool := llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "get_weather",
			Description: "get current weather of a city",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string"},
				},
				"required": []string{"city"},
			},
		},
	}

	for _, tc := range []struct {
		name        string
		model       string
		accountName string
		provider    account.Provider
	}{
		{"anthropic-tool-call", "claude-3-5-sonnet", "anth", account.ProviderAnthropic},
		{"azure-openai-tool-call", "gpt-4o", "azure", account.ProviderAzureOpenAI},
		{"gemini-tool-call", "gemini-2.5-pro", "gem", account.ProviderGcpVertexAI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := &fakeModel{
				generate: func(_ context.Context, _ []llms.MessageContent, opts []llms.CallOption) (*llms.ContentResponse, error) {
					// 解析 opts，看 Tools 是否到位。
					resolved := llms.CallOptions{}
					for _, o := range opts {
						o(&resolved)
					}
					if len(resolved.Tools) != 1 || resolved.Tools[0].Function.Name != "get_weather" {
						return nil, errors.New("tools not propagated")
					}
					if resolved.ToolChoice != "auto" {
						return nil, errors.New("tool_choice not propagated")
					}
					return &llms.ContentResponse{
						Choices: []*llms.ContentChoice{{
							ToolCalls: []llms.ToolCall{{
								ID:   "call_1",
								Type: "function",
								FunctionCall: &llms.FunctionCall{
									Name:      "get_weather",
									Arguments: `{"city":"Beijing"}`,
								},
							}},
						}},
					}, nil
				},
			}
			svc := Default()

			resp, err := svc.GenerateContent(context.Background(), tc.model,
				[]llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "weather in Beijing?")},
				llms.WithTools([]llms.Tool{weatherTool}),
				llms.WithToolChoice("auto"),
			)
			if err != nil {
				t.Fatalf("GenerateContent: %v", err)
			}

			if len(resp.Choices) != 1 || len(resp.Choices[0].ToolCalls) != 1 {
				t.Fatalf("unexpected response: %+v", resp)
			}
			tc1 := resp.Choices[0].ToolCalls[0]
			if tc1.FunctionCall.Name != "get_weather" || tc1.FunctionCall.Arguments != `{"city":"Beijing"}` {
				t.Errorf("tool call wrong: %+v", tc1)
			}

			// 第二轮：把 tool 执行结果以 ToolCallResponse 的形式喂回去，
			// 确认 messages 被原样传给底层 client。
			mock.generate = func(_ context.Context, msgs []llms.MessageContent, _ []llms.CallOption) (*llms.ContentResponse, error) {
				if len(msgs) != 2 {
					return nil, errors.New("expected 2 messages on followup")
				}
				last := msgs[len(msgs)-1]
				if last.Role != llms.ChatMessageTypeTool {
					return nil, errors.New("expected last message role=tool")
				}
				if _, ok := last.Parts[0].(llms.ToolCallResponse); !ok {
					return nil, errors.New("expected last part to be ToolCallResponse")
				}
				return textResponse("Sunny."), nil
			}
			follow, err := svc.GenerateContent(context.Background(), tc.model,
				[]llms.MessageContent{
					llms.TextParts(llms.ChatMessageTypeHuman, "weather in Beijing?"),
					{
						Role: llms.ChatMessageTypeTool,
						Parts: []llms.ContentPart{
							llms.ToolCallResponse{ToolCallID: "call_1", Name: "get_weather", Content: `{"temp":"22C"}`},
						},
					},
				},
			)
			if err != nil {
				t.Fatalf("followup GenerateContent: %v", err)
			}
			if follow.Choices[0].Content != "Sunny." {
				t.Errorf("followup content wrong: %q", follow.Choices[0].Content)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 多模态（图片输入）透传测试
// ---------------------------------------------------------------------------

// TestGenerateContent_MultiModal 验证 ImageURL / Binary 类型的 ContentPart
// 能完整透传，不被 Service 改写或丢弃。
func TestGenerateContent_MultiModal(t *testing.T) {
	for _, tc := range []struct {
		name        string
		model       string
		accountName string
		provider    account.Provider
	}{
		{"anthropic-mm", "claude-3-5-sonnet", "anth", account.ProviderAnthropic},
		{"azure-mm", "gpt-4o", "azure", account.ProviderAzureOpenAI},
		{"gemini-mm", "gemini-2.5-pro", "gem", account.ProviderGcpVertexAI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := &fakeModel{resp: textResponse("looks like a cat")}
			svc := Default()

			imgURL := "https://example.com/cat.jpg"
			messages := []llms.MessageContent{{
				Role: llms.ChatMessageTypeHuman,
				Parts: []llms.ContentPart{
					llms.TextContent{Text: "what is in this image?"},
					llms.ImageURLContent{URL: imgURL, MimeType: "image/jpeg"},
					llms.BinaryContent{MIMEType: "image/png", Data: []byte{0x89, 0x50, 0x4E, 0x47}},
				},
			}}

			if _, err := svc.GenerateContent(context.Background(), tc.model, messages); err != nil {
				t.Fatalf("GenerateContent: %v", err)
			}

			last := mock.lastCall()
			if len(last.messages) != 1 || len(last.messages[0].Parts) != 3 {
				t.Fatalf("messages not transparently passed: %+v", last.messages)
			}
			if img, ok := last.messages[0].Parts[1].(llms.ImageURLContent); !ok || img.URL != imgURL {
				t.Errorf("image part lost or rewritten: %+v", last.messages[0].Parts[1])
			}
			if bin, ok := last.messages[0].Parts[2].(llms.BinaryContent); !ok || bin.MIMEType != "image/png" {
				t.Errorf("binary part lost or rewritten: %+v", last.messages[0].Parts[2])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 路由 / 缓存 / 错误处理 / 并发
// ---------------------------------------------------------------------------

func TestGenerateContent_UnknownModel(t *testing.T) {
	svc := Default()
	_, err := svc.GenerateContent(context.Background(), "llama-3", nil)
	if err == nil {
		t.Fatal("expected error for unknown model prefix")
	}
	if !strings.Contains(err.Error(), "cannot route model") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestGenerateContent_NoAccountForProvider(t *testing.T) {
	// 只配 anthropic，但请求 gpt-* → azure，找不到账号。
	svc := Default()
	_, err := svc.GenerateContent(context.Background(), "gpt-4o", nil)
	if err == nil {
		t.Fatal("expected error when no account matches provider")
	}
	if !strings.Contains(err.Error(), "no account configured") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestGenerateContent_ClientCachedAcrossCalls(t *testing.T) {
	mock := &fakeModel{resp: textResponse("ok")}
	svc := Default()

	for i := 0; i < 3; i++ {
		if _, err := svc.GenerateContent(context.Background(), "claude-3-5-sonnet", nil); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if got := mock.callCount(); got != 3 {
		t.Errorf("expected 3 calls into the cached client, got %d", got)
	}
	// 缓存里仍然只有 1 个 entry。
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	if len(svc.clients) != 1 {
		t.Errorf("expected 1 cached client, got %d", len(svc.clients))
	}
}

func TestGenerateContent_PicksFirstMatchingAccount(t *testing.T) {
	// 两个 anthropic 账号；按 Names() 排序后第一个被命中。
	mock1 := &fakeModel{resp: textResponse("from-1")}
	mock2 := &fakeModel{resp: textResponse("from-2")}
	svc := Default()

	resp, err := svc.GenerateContent(context.Background(), "claude-3-5-sonnet", nil)
	if err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}
	if resp.Choices[0].Content != "from-1" {
		t.Errorf("expected first sorted account (anth-1) to be picked, got %q", resp.Choices[0].Content)
	}
	if mock1.callCount() != 1 || mock2.callCount() != 0 {
		t.Errorf("call counts wrong: mock1=%d mock2=%d", mock1.callCount(), mock2.callCount())
	}
}

func TestGenerateContent_OptionsAppendable(t *testing.T) {
	// 调用方在 opts 末尾再追加 WithModel(other) 应能覆盖 Service 自动注入的 model。
	mock := &fakeModel{resp: textResponse("ok")}
	svc := Default()

	if _, err := svc.GenerateContent(context.Background(), "claude-3-5-sonnet", nil,
		llms.WithTemperature(0.7),
		llms.WithMaxTokens(128),
		llms.WithModel("claude-override"),
	); err != nil {
		t.Fatalf("GenerateContent: %v", err)
	}
	last := mock.lastCall()
	if last.resolved.Model != "claude-override" {
		t.Errorf("later WithModel should override default, got %q", last.resolved.Model)
	}
	if last.resolved.Temperature != 0.7 || last.resolved.MaxTokens != 128 {
		t.Errorf("opts not preserved: %+v", last.resolved)
	}
}

func TestGenerateContent_ConcurrentSafe(t *testing.T) {
	mock := &fakeModel{resp: textResponse("ok")}
	svc := Default()

	const N = 64
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_, _ = svc.GenerateContent(context.Background(), "claude-3-5-sonnet", nil)
		}()
	}
	wg.Wait()

	if mock.callCount() != N {
		t.Errorf("expected %d concurrent calls, got %d", N, mock.callCount())
	}
}
