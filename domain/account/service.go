package account

import (
	"context"
	"fmt"
	"sort"
)

// Service 账号服务：在启动时通过 SourceConfig 选定的 Loader 把账号读到内存中，
// 之后对外提供按名查询、列表等能力。
//
// 当前实现是一次性快照（不热更新）。后续如果要支持热更新，
// 在此包内加一个 Reload(ctx) 方法即可，对外 API 不变。
//
// 并发安全。
type Service struct {
	accounts map[string]*Account
}

// 任何配置错误、加载错误或账号校验错误都会导致构造失败。
func NewService(ctx context.Context, cfg SourceConfig) (*Service, error) {
	loader, err := cfg.NewLoader()
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
