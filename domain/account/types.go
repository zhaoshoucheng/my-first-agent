// Package account 是账号服务：负责账号的实体定义、加载与运行期查询。
//
// 账号是 LLM Provider 的访问主体——一个 Account 携带一份凭据，
// 用它可以调用对应 Provider 上的所有可用模型。
//
// 由于 Provider/Credential 是账号的内在属性，它们定义在本包中（而不是
// domain/llm），既符合分层语义，也避免了 account ↔ llm 之间的 import 循环。
package account

// Provider 提供 LLM 服务的厂商。
//
// 命名沿用 llm-router 工程的取值，便于跨工程对齐。
type Provider string

const (
	ProviderAnthropic   Provider = "anthropic"     // anthropic 官方
	ProviderAzureOpenAI Provider = "azure-openai"  // 微软 azure 上的 openai 部署
	ProviderAwsBedrock  Provider = "aws-bedrock"   // aws bedrock（暂未实现）
	ProviderGcpVertexAI Provider = "gcp-vertex-ai" // gcp vertex（暂未实现）
)

// IsValid 是否为已知 Provider。
func (p Provider) IsValid() bool {
	switch p {
	case ProviderAnthropic, ProviderAzureOpenAI, ProviderAwsBedrock, ProviderGcpVertexAI:
		return true
	}
	return false
}

// Credential 凭据通用结构。不同 Provider 用到的字段不同，由 LLM 工厂按需读取：
//
//   - anthropic     : APIKey [+ BaseURL]
//   - azure-openai  : APIKey + BaseURL + APIVersion + DeploymentName
//   - aws-bedrock   : APIKey 形如 "<accessKeyId>/<secretAccessKey>" + Region
//   - gcp-vertex-ai : APIKey 为 GCP 服务账号 JSON 的 base64 + Region + ProjectID
type Credential struct {
	APIKey         string `json:"api_key"`
	BaseURL        string `json:"base_url,omitempty"`
	APIVersion     string `json:"api_version,omitempty"`
	DeploymentName string `json:"deployment_name,omitempty"` // azure 部署名
	Region         string `json:"region,omitempty"`
	ProjectID      string `json:"project_id,omitempty"`
	Organization   string `json:"organization,omitempty"`
}
