// Package veo 是视频生成的 gcp-vertex-ai 后端，用 google.golang.org/genai SDK
// 走 Vertex AI backend 调 Veo 模型。凭据取自账号 Credential：
//
//   - APIKey    : base64 编码的 GCP 服务账号 JSON
//   - ProjectID : GCP 项目 ID
//   - Region    : GCP 区域
//
// 只支持文生视频、图生首帧、尾帧（config.LastFrame）与参数（分辨率 / 比例 / 时长 /
// 音频 / 种子 / 反向提示）。参考图 roles 不受 Veo 支持——那是 modelark 后端的能力。
package veo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"cloud.google.com/go/auth/credentials"
	"github.com/shoucheng/my-first-agent/domain/account"
	"google.golang.org/genai"
)

// gcpCloudPlatformScope 是 Vertex AI 需要的 OAuth scope。
const gcpCloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// Client 封装一个 genai Vertex AI 客户端。
type Client struct {
	genai *genai.Client
}

// New 用账号凭据构造一个 Veo 客户端。
func New(ctx context.Context, acc *account.Account) (*Client, error) {
	cred := acc.Credential
	if cred.ProjectID == "" || cred.Region == "" {
		return nil, fmt.Errorf("veo: account %q requires credential.project_id and credential.region", acc.Name)
	}
	jsonCred, err := decodeBase64Credential(cred.APIKey)
	if err != nil {
		return nil, fmt.Errorf("veo: account %q invalid base64 credential: %w", acc.Name, err)
	}
	detected, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes:          []string{gcpCloudPlatformScope},
		CredentialsJSON: jsonCred,
	})
	if err != nil {
		return nil, fmt.Errorf("veo: account %q detect credentials: %w", acc.Name, err)
	}
	g, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:     genai.BackendVertexAI,
		Project:     cred.ProjectID,
		Location:    cred.Region,
		Credentials: detected,
	})
	if err != nil {
		return nil, fmt.Errorf("veo: account %q new genai client: %w", acc.Name, err)
	}
	return &Client{genai: g}, nil
}

// Submit 提交一次生成任务。只有 image（首帧）会被 Veo 使用；尾帧与参数经 config 传入。
func (c *Client) Submit(ctx context.Context, model, prompt string, image *genai.Image, config *genai.GenerateVideosConfig) (*genai.GenerateVideosOperation, error) {
	return c.genai.Models.GenerateVideos(ctx, model, prompt, image, config)
}

// Poll 查询一次任务状态。Done=false 时原样返回；后端报错时把 operation.Error 转成 error。
func (c *Client) Poll(ctx context.Context, op *genai.GenerateVideosOperation) (*genai.GenerateVideosOperation, error) {
	op, err := c.genai.Operations.GetVideosOperation(ctx, op, nil)
	if err != nil {
		return nil, err
	}
	if op.Error != nil {
		bs, _ := json.Marshal(op.Error)
		return nil, fmt.Errorf("veo: operation failed: %s", string(bs))
	}
	if op.Done {
		if err := raiFailure(op); err != nil {
			return nil, err
		}
	}
	return op, nil
}

// raiFailure 在任务完成时检查内容安全过滤：被过滤或产物为空都视为终态失败。
func raiFailure(op *genai.GenerateVideosOperation) error {
	resp := op.Response
	if resp == nil || len(resp.GeneratedVideos) == 0 {
		return errors.New("veo: no video generated (possibly filtered by content safety)")
	}
	if len(resp.RAIMediaFilteredReasons) > 0 {
		return fmt.Errorf("veo: filtered by content safety: %v", resp.RAIMediaFilteredReasons)
	}
	if resp.RAIMediaFilteredCount > 0 {
		return fmt.Errorf("veo: %d media filtered by content safety", resp.RAIMediaFilteredCount)
	}
	return nil
}

// decodeBase64Credential 先按标准 base64 再按 raw base64 解，兼容两种编码写法。
func decodeBase64Credential(s string) ([]byte, error) {
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}
