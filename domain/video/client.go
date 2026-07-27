package video

import (
	"context"
	"fmt"

	"github.com/shoucheng/my-first-agent/domain/account"
	"github.com/shoucheng/my-first-agent/domain/video/modelark"
	"github.com/shoucheng/my-first-agent/domain/video/sora"
	"github.com/shoucheng/my-first-agent/domain/video/veo"
	"google.golang.org/genai"
)

// Client 是视频生成后端的统一抽象。两个后端（veo / modelark）各自实现：
//
//   - Submit 提交一次生成任务，立即返回一个未完成的任务句柄；
//   - Poll   用任务句柄查询一次最新状态，Done=true 时 Response / Error 之一可用。
//
// 接口签名直接用 genai 原生类型，这样 veo 子包（本就用 genai SDK）无需任何适配即可
// 满足本接口，而 modelark 子包只要匹配同样的方法签名即可（Go 接口是结构化的，子包
// 不必反向 import 本包，从而避免 import 环）。modelark 把自己的 task id 映射到
// operation.Name，把结果 url 映射到 operation.Response.GeneratedVideos[i].Video.URI。
type Client interface {
	Submit(ctx context.Context, model, prompt string, image *genai.Image, config *genai.GenerateVideosConfig) (*genai.GenerateVideosOperation, error)
	Poll(ctx context.Context, op *genai.GenerateVideosOperation) (*genai.GenerateVideosOperation, error)
}

// newClient 把一个 Account 实例化为对应后端的 Client。
//
// 这是包私有 helper，由 Service 在懒加载时调用；外部不应直接调用。
func newClient(ctx context.Context, acc *account.Account) (Client, error) {
	if err := acc.Validate(); err != nil {
		return nil, fmt.Errorf("video.newClient: invalid account %q: %w", acc.Name, err)
	}
	switch acc.Provider {
	case account.ProviderGcpVertexAI:
		return veo.New(ctx, acc)
	case account.ProviderModelArk:
		return modelark.New(acc)
	case account.ProviderAzureOpenAI:
		return sora.New(acc)
	default:
		return nil, fmt.Errorf("video.newClient: provider %q does not support video generation", acc.Provider)
	}
}
