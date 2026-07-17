package account

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProvider_IsValid(t *testing.T) {
	valid := []Provider{ProviderAnthropic, ProviderAzureOpenAI, ProviderAwsBedrock, ProviderGcpVertexAI}
	for _, p := range valid {
		if !p.IsValid() {
			t.Errorf("%q should be valid", p)
		}
	}
	if Provider("nope").IsValid() {
		t.Error("unknown provider should be invalid")
	}
}

func TestAccount_Validate(t *testing.T) {
	cases := []struct {
		name    string
		acc     Account
		wantErr error
	}{
		{"valid", Account{Name: "a", Provider: ProviderAnthropic, Credential: Credential{APIKey: "k"}}, nil},
		{"empty name", Account{Provider: ProviderAnthropic, Credential: Credential{APIKey: "k"}}, ErrEmptyName},
		{"unknown provider", Account{Name: "a", Provider: "x", Credential: Credential{APIKey: "k"}}, ErrUnknownProvider},
		{"missing key", Account{Name: "a", Provider: ProviderAnthropic}, ErrMissingAPIKey},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.acc.Validate(); err != tc.wantErr {
				t.Errorf("Validate() = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// --- FileLoader ---

func writeJSON(t *testing.T, dir, file, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFileLoader_Load(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "anthropic.json", `{"name":"anth","provider":"anthropic","credential":{"api_key":"k1"}}`)
	writeJSON(t, dir, "azure.json", `{"name":"az","provider":"azure-openai","credential":{"api_key":"k2"}}`)
	writeJSON(t, dir, "readme.txt", `not an account`) // 非 .json 应被忽略

	accs, err := NewFileLoader(dir).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(accs) != 2 {
		t.Fatalf("loaded %d accounts, want 2", len(accs))
	}
}

func TestFileLoader_EmptyDir(t *testing.T) {
	if _, err := NewFileLoader("").Load(context.Background()); err == nil {
		t.Error("expected error for empty dir")
	}
	if _, err := NewFileLoader(filepath.Join(t.TempDir(), "missing")).Load(context.Background()); err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

func TestFileLoader_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "a.json", `{"name":"dup","provider":"anthropic","credential":{"api_key":"k1"}}`)
	writeJSON(t, dir, "b.json", `{"name":"dup","provider":"anthropic","credential":{"api_key":"k2"}}`)
	if _, err := NewFileLoader(dir).Load(context.Background()); err == nil {
		t.Error("expected error on duplicate account name")
	}
}

func TestFileLoader_InvalidAccount(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "bad.json", `{"name":"x","provider":"anthropic"}`) // 缺 api_key
	if _, err := NewFileLoader(dir).Load(context.Background()); err == nil {
		t.Error("expected validation error for account missing api_key")
	}
}

// --- Service ---

func acc(name string, p Provider) *Account {
	return &Account{Name: name, Provider: p, Credential: Credential{APIKey: "k"}}
}

func TestNewServiceFromAccounts(t *testing.T) {
	// 重复名应报错。
	if _, err := newServiceFromAccounts([]*Account{acc("a", ProviderAnthropic), acc("a", ProviderAnthropic)}); err == nil {
		t.Error("expected error on duplicate name")
	}
	// nil 账号应报错。
	if _, err := newServiceFromAccounts([]*Account{nil}); err == nil {
		t.Error("expected error on nil account")
	}

	svc, err := newServiceFromAccounts([]*Account{acc("b", ProviderAnthropic), acc("a", ProviderAzureOpenAI)})
	if err != nil {
		t.Fatalf("newServiceFromAccounts: %v", err)
	}
	if svc.Count() != 2 {
		t.Errorf("Count() = %d, want 2", svc.Count())
	}
	// Names() 应排序。
	names := svc.Names()
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("Names() = %v, want [a b]", names)
	}
	if _, err := svc.Get("missing"); err == nil {
		t.Error("expected error for unknown account")
	}
}

func TestProviderForModel(t *testing.T) {
	cases := []struct {
		model string
		want  Provider
		ok    bool
	}{
		{"claude-opus-4-7", ProviderAnthropic, true},
		{"gpt-4o", ProviderAzureOpenAI, true},
		{"o1-preview", ProviderAzureOpenAI, true},
		{"o3-mini", ProviderAzureOpenAI, true},
		{"gemini-2.5-pro", ProviderGcpVertexAI, true},
		{"CLAUDE-3", ProviderAnthropic, true}, // 大小写不敏感
		{"llama-3", "", false},
	}
	for _, tc := range cases {
		got, err := providerForModel(tc.model)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("providerForModel(%q) = (%q,%v), want %q", tc.model, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Errorf("providerForModel(%q) expected error", tc.model)
		}
	}
}

func TestPickAccountForModel(t *testing.T) {
	svc, _ := newServiceFromAccounts([]*Account{
		acc("anth-2", ProviderAnthropic),
		acc("anth-1", ProviderAnthropic),
		acc("azure-1", ProviderAzureOpenAI),
	})

	// claude-* 应路由到 anthropic，并选排序后第一个匹配的账号（anth-1）。
	got, err := svc.PickAccountForModel("claude-3-5-sonnet")
	if err != nil {
		t.Fatalf("PickAccountForModel: %v", err)
	}
	if got.Name != "anth-1" {
		t.Errorf("picked %q, want first matching provider account anth-1", got.Name)
	}

	// gpt-* 路由到 azure。
	got, err = svc.PickAccountForModel("gpt-4o")
	if err != nil || got.Name != "azure-1" {
		t.Errorf("gpt-4o picked %v err=%v, want azure-1", got, err)
	}

	// gemini-* 无对应账号 → 报错。
	if _, err := svc.PickAccountForModel("gemini-2.5-pro"); err == nil {
		t.Error("expected error when no account matches provider")
	}

	// 未知模型前缀 → 路由失败。
	if _, err := svc.PickAccountForModel("llama-3"); err == nil {
		t.Error("expected error for unroutable model")
	}
}
