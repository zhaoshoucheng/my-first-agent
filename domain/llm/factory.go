package llm

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/shoucheng/my-first-agent/domain/account"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms/anthropic"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms/googleai"
	googlegenai "github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms/googleai/genai"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms/openai"
)

// newClient 把一个 Account 实例化为 langchaingo 的 llms.Model。
//
// 这是包私有 helper，由 Service 在懒加载时调用；外部不应直接调用。
func newClient(ctx context.Context, acc *account.Account) (llms.Model, error) {
	if err := acc.Validate(); err != nil {
		return nil, fmt.Errorf("llm.newClient: invalid account %q: %w", acc.Name, err)
	}
	switch acc.Provider {
	case account.ProviderAnthropic:
		return newAnthropic(acc)
	case account.ProviderAzureOpenAI:
		return newAzureOpenAI(acc)
	case account.ProviderGcpVertexAI:
		return newGemini(ctx, acc)
	case account.ProviderAwsBedrock:
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

// newGemini 构造 Gemini / Vertex AI 客户端。
//
// 账号 Credential 复用通用字段：
//   - 仅设置 APIKey  → 走 Gemini Developer API
//   - 同时设置 ProjectID + Region → 走 Vertex AI（APIKey 这时被解释为
//     base64 编码的 GCP 服务账号 JSON，参考 account.types.go 的注释）
func newGemini(ctx context.Context, acc *account.Account) (llms.Model, error) {
	cred := acc.Credential
	opts := []googleai.Option{}
	useVertex := cred.ProjectID != "" && cred.Region != ""
	if useVertex {
		opts = append(opts,
			googleai.WithCloudProject(cred.ProjectID),
			googleai.WithCloudLocation(cred.Region),
		)
		// APIKey 在 vertex 模式下被解释为 base64 编码的服务账号 JSON。
		if cred.APIKey != "" {
			decoded, err := decodeBase64Credential(cred.APIKey)
			if err != nil {
				return nil, fmt.Errorf("gcp-vertex-ai account %q: invalid base64 credential: %w", acc.Name, err)
			}
			opts = append(opts, googleai.WithCredentialsJSON(decoded))
		}
	} else {
		opts = append(opts, googleai.WithAPIKey(cred.APIKey))
	}
	return googlegenai.New(ctx, opts...)
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

func decodeBase64Credential(s string) ([]byte, error) {
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}
