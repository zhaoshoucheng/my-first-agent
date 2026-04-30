package llm

import (
	"context"
	"fmt"
	"sync"

	"github.com/shoucheng/my-first-agent/domain/account"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
)

// Service 是 LLM 模块对外的唯一入口。
//
// 内部维护两个东西：
//   - accountSvc：账号源（可注入；默认是 account.Default()）
//   - clients   ：按账号名缓存的 langchaingo Model 池（双锁懒加载，并发安全）
//
// 外部调用方（如 agent）只需要持有 *Service，运行期通过
// GenerateContent(ctx, model, messages, opts...) 即可发起对话。
type Service struct {
	mu      sync.RWMutex
	clients map[string]llms.Model
}

// NewService 构造一个独立的 LLM 服务实例。一般不直接调用 — 业务代码应
func NewService() *Service {
	return &Service{
		clients: make(map[string]llms.Model),
	}
}

// clientFor 是包内通用的"取账号→拿/建 client"路径，供 Client 与
// GenerateContent 复用。缓存键是账号名（同一账号的不同 model 共享一个 client，
// 具体 model 通过 CallOption 在每次调用时传入）。
func (s *Service) clientFor(ctx context.Context, acc *account.Account) (llms.Model, error) {
	// fast path: 已缓存
	s.mu.RLock()
	if m, ok := s.clients[acc.Name]; ok {
		s.mu.RUnlock()
		return m, nil
	}
	s.mu.RUnlock()

	// slow path: 构造 + 写缓存
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.clients[acc.Name]; ok { // double-check
		return m, nil
	}
	m, err := newClient(ctx, acc)
	if err != nil {
		return nil, fmt.Errorf("llm.Service: build client for %q: %w", acc.Name, err)
	}
	s.clients[acc.Name] = m
	return m, nil
}

// GenerateContent 是 LLM 服务的主入口：
//
//  1. 按 model 名路由到一个账号（providerForModel + pickAccountForModel）
//  2. 取/建该账号的 langchaingo client（带缓存）
//  3. 把 model 名作为 CallOption 拼到 opts 头部，转发 GenerateContent
//
// 调用方只需要关心 model 名和 messages，不必感知账号、Provider、客户端缓存。
//
// model 名会被放到 opts 的最前面，调用方如果想覆盖（同一个 client 临时换一个
// 模型）可以再追加 llms.WithModel(...) — 后追加的同名 option 会覆盖前面的。
func (s *Service) GenerateContent(
	ctx context.Context,
	model string,
	messages []llms.MessageContent,
	opts ...llms.CallOption,
) (*llms.ContentResponse, error) {
	acc, err := account.Default().PickAccountForModel(model)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(ctx, acc)
	if err != nil {
		return nil, err
	}
	all := append([]llms.CallOption{llms.WithModel(model)}, opts...)
	return client.GenerateContent(ctx, messages, all...)
}
