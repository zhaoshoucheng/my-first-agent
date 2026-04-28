package llm

import (
	"fmt"
	"strings"

	"github.com/shoucheng/my-first-agent/domain/account"
)

// providerForModel 按 model 名前缀推断它属于哪个 Provider。
//
// 这是一份内置的最小路由规则，覆盖目前已实现的两个 Provider：
//   - claude-*               → anthropic
//   - gpt-* / o1-* / o3-*    → azure-openai
//
// 后续如果路由规则变复杂（按 region、按 tag、按显式映射表等），
// 把这里换成一个可配置的 router 即可，不影响 Service 的对外签名。
func providerForModel(model string) (account.Provider, error) {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "claude-"):
		return account.ProviderAnthropic, nil
	case strings.HasPrefix(m, "gpt-"), strings.HasPrefix(m, "o1-"), strings.HasPrefix(m, "o3-"):
		return account.ProviderAzureOpenAI, nil
	default:
		return "", fmt.Errorf("llm: cannot route model %q to a known provider", model)
	}
}

// pickAccountForModel 根据 model 名找一个可用账号：先推断 Provider，
// 再到账号集合里选第一个匹配该 Provider 的账号。
//
// 选第一个是当下的简化策略；多账号场景下未来可加权重 / 限流 / 故障转移。
func pickAccountForModel(model string) (*account.Account, error) {
	want, err := providerForModel(model)
	if err != nil {
		return nil, err
	}
	svc := account.Default()
	for _, name := range svc.Names() {
		acc, err := svc.Get(name)
		if err != nil {
			continue
		}
		if acc.Provider == want {
			return acc, nil
		}
	}
	return nil, fmt.Errorf("llm: no account configured for provider %q (model %q)", want, model)
}
