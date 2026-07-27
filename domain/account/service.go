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
//   - claude-*                       → anthropic
//   - gpt-* / o1-* / o3-* / sora-*   → azure-openai（sora 走 azure 上的 sora 部署）
//   - gemini-* / veo-*               → gcp-vertex-ai
//   - seedance-* / dreamina-*        → modelark
//
// 后续如果路由规则变复杂（按 region、按 tag、按显式映射表等），
// 把这里换成一个可配置的 router 即可，不影响 Service 的对外签名。
func providerForModel(model string) (Provider, error) {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "claude-"):
		return ProviderAnthropic, nil
	case strings.HasPrefix(m, "gpt-"), strings.HasPrefix(m, "o1-"), strings.HasPrefix(m, "o3-"), strings.HasPrefix(m, "sora-"):
		return ProviderAzureOpenAI, nil
	case strings.HasPrefix(m, "gemini-"), strings.HasPrefix(m, "veo-"):
		return ProviderGcpVertexAI, nil
	case strings.HasPrefix(m, "seedance-"), strings.HasPrefix(m, "dreamina-"):
		return ProviderModelArk, nil
	default:
		return "", fmt.Errorf("llm: cannot route model %q to a known provider", model)
	}
}

// PickAccountForModel 根据 model 名找一个可用账号：先推断 Provider，
// 再到账号集合里挑一个匹配该 Provider 的账号。
//
// 选择规则（解决"同一 Provider 有多个账号"的歧义，例如 azure-openai 同时被
// gpt 文本和 sora 视频使用）：
//  1. 该 Provider 只有一个账号 → 直接用；
//  2. 多个时，优先按账号名与 model 名匹配：名字完全相等 > 账号名是 model 前缀
//     （如账号 sora-2 服务 model sora-2-pro）> model 是账号名前缀；
//  3. 都匹配不上 → 退回按名字排序的第一个（此时是有歧义的，靠给账号起名规避）。
//
// 建议把账号名起成对应的 model 名（如账号 sora-2 服务 sora-* 视频），即可稳定命中。
func (s *Service) PickAccountForModel(model string) (*Account, error) {
	want, err := providerForModel(model)
	if err != nil {
		return nil, err
	}
	var candidates []*Account
	for _, name := range s.Names() {
		acc, err := s.Get(name)
		if err != nil {
			continue
		}
		if acc.Provider == want {
			candidates = append(candidates, acc)
		}
	}
	switch len(candidates) {
	case 0:
		return nil, fmt.Errorf("llm: no account configured for provider %q (model %q)", want, model)
	case 1:
		return candidates[0], nil
	}
	if best := bestNameMatch(model, candidates); best != nil {
		return best, nil
	}
	return candidates[0], nil
}

// bestNameMatch 在同 Provider 的多个账号里，按账号名与 model 名的贴合度挑一个：
// 完全相等 > 账号名是 model 前缀 > model 是账号名前缀；都不沾则返回 nil。
func bestNameMatch(model string, accs []*Account) *Account {
	ml := strings.ToLower(model)
	var prefixAccModel, prefixModelAcc *Account
	for _, a := range accs {
		nl := strings.ToLower(a.Name)
		switch {
		case nl == ml:
			return a // 完全相等，最强匹配
		case prefixAccModel == nil && strings.HasPrefix(ml, nl):
			prefixAccModel = a
		case prefixModelAcc == nil && strings.HasPrefix(nl, ml):
			prefixModelAcc = a
		}
	}
	if prefixAccModel != nil {
		return prefixAccModel
	}
	return prefixModelAcc
}
