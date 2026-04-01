package llm

// Provider LLM 提供商类型
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderAzure     Provider = "azure"
)

// Config LLM 客户端配置
type Config struct {
	Provider    Provider
	APIKey      string
	Model       string
	Endpoint    string // 可选，用于 Azure 等
	Temperature float32
	MaxTokens   int
}
