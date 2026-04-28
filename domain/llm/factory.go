package llm

import (
	"context"
	"fmt"

	"github.com/shoucheng/my-first-agent/domain/account"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms/anthropic"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms/openai"
)

// newClient 把一个 Account 实例化为 langchaingo 的 llms.Model。
//
// 这是包私有 helper，由 Service 在懒加载时调用；外部不应直接调用。
//
// 第一版只实现 anthropic 与 azure-openai；其它 Provider 返回 not-implemented。
func newClient(_ context.Context, acc *account.Account) (llms.Model, error) {
	if err := acc.Validate(); err != nil {
		return nil, fmt.Errorf("llm.newClient: invalid account %q: %w", acc.Name, err)
	}
	switch acc.Provider {
	case account.ProviderAnthropic:
		return newAnthropic(acc)
	case account.ProviderAzureOpenAI:
		return newAzureOpenAI(acc)
	case account.ProviderAwsBedrock, account.ProviderGcpVertexAI:
		return nil, fmt.Errorf("llm.newClient: provider %q is not implemented yet", acc.Provider)
	default:
		return nil, fmt.Errorf("llm.newClient: unknown provider %q", acc.Provider)
	}
}

func newAnthropic(acc *account.Account) (llms.Model, error) {
	opts := []anthropic.Option{
		anthropic.WithAPIKey(acc.Credential.APIKey),
	}
	if acc.Credential.BaseURL != "" {
		opts = append(opts, anthropic.WithBaseURL(acc.Credential.BaseURL))
	}
	return anthropic.New(opts...)
}

func newAzureOpenAI(acc *account.Account) (llms.Model, error) {
	cred := acc.Credential
	if cred.BaseURL == "" {
		return nil, fmt.Errorf("azure-openai account %q: credential.base_url is required", acc.Name)
	}
	if cred.APIVersion == "" {
		return nil, fmt.Errorf("azure-openai account %q: credential.api_version is required", acc.Name)
	}
	if cred.DeploymentName == "" {
		return nil, fmt.Errorf("azure-openai account %q: credential.deployment_name is required", acc.Name)
	}
	opts := []openai.Option{
		openai.WithToken(cred.APIKey),
		openai.WithAPIType(openai.APITypeAzure),
		openai.WithBaseURL(cred.BaseURL),
		openai.WithAPIVersion(cred.APIVersion),
		openai.WithModel(cred.DeploymentName),
	}
	return openai.New(opts...)
}
