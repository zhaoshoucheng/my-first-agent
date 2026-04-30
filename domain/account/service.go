package account

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/shoucheng/my-first-agent/infra/config"
)

// Service 账号服务：在启动时通过 SourceConfig 选定的 Loader 把账号读到内存中，
type Service struct {
	accounts map[string]*Account
}

// 任何配置错误、加载错误或账号校验错误都会导致构造失败。
func NewService(ctx context.Context, cfg config.SourceConfig) (*Service, error) {
	loader, err := NewLoader(cfg)
	if err != nil {
		return nil, fmt.Errorf("account.NewService: %w", err)
	}
	accounts, err := loader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("account.NewService: load: %w", err)
	}
	return newServiceFromAccounts(accounts)
}

// newServiceFromAccounts 是一个内部 helper，便于测试时直接传入账号集合。
func newServiceFromAccounts(accounts []*Account) (*Service, error) {
	m := make(map[string]*Account, len(accounts))
	for _, a := range accounts {
		if a == nil {
			return nil, fmt.Errorf("account.Service: nil account in input")
		}
		if _, dup := m[a.Name]; dup {
			return nil, fmt.Errorf("account.Service: duplicate account name %q", a.Name)
		}
		m[a.Name] = a
	}
	return &Service{accounts: m}, nil
}

// Get 按名取账号。账号不存在返回错误。
func (s *Service) Get(name string) (*Account, error) {
	a, ok := s.accounts[name]
	if !ok {
		return nil, fmt.Errorf("account.Service: unknown account %q", name)
	}
	return a, nil
}

// Names 返回所有账号名（已排序，便于稳定输出）。
func (s *Service) Names() []string {
	names := make([]string, 0, len(s.accounts))
	for n := range s.accounts {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Count 当前注册的账号数量。
func (s *Service) Count() int {
	return len(s.accounts)
}

// providerForModel 按 model 名前缀推断它属于哪个 Provider。
//
// 这是一份内置的最小路由规则，覆盖目前已实现的三个 Provider：
//   - claude-*               → anthropic
//   - gpt-* / o1-* / o3-*    → azure-openai
//   - gemini-*               → gcp-vertex-ai
//
// 后续如果路由规则变复杂（按 region、按 tag、按显式映射表等），
// 把这里换成一个可配置的 router 即可，不影响 Service 的对外签名。
func providerForModel(model string) (Provider, error) {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "claude-"):
		return ProviderAnthropic, nil
	case strings.HasPrefix(m, "gpt-"), strings.HasPrefix(m, "o1-"), strings.HasPrefix(m, "o3-"):
		return ProviderAzureOpenAI, nil
	case strings.HasPrefix(m, "gemini-"):
		return ProviderGcpVertexAI, nil
	default:
		return "", fmt.Errorf("llm: cannot route model %q to a known provider", model)
	}
}

// PickAccountForModel 根据 model 名找一个可用账号：先推断 Provider，
// 再到给定账号集合里选第一个匹配该 Provider 的账号。
//
// 选第一个是当下的简化策略；多账号场景下未来可加权重 / 限流 / 故障转移。
func (s *Service) PickAccountForModel(model string) (*Account, error) {
	want, err := providerForModel(model)
	if err != nil {
		return nil, err
	}
	for _, name := range s.Names() {
		acc, err := s.Get(name)
		if err != nil {
			continue
		}
		if acc.Provider == want {
			return acc, nil
		}
	}
	return nil, fmt.Errorf("llm: no account configured for provider %q (model %q)", want, model)
}
