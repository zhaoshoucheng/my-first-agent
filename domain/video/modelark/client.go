// Package modelark 是视频生成的 modelark 后端（seedance / dreamina 系列模型），
// 用标准库 net/http 直连其 REST API：
//
//   - 提交：POST {base}/api/v3/contents/generations/tasks          → {id}
//   - 轮询：GET  {base}/api/v3/contents/generations/tasks/{id}      → {status, error, content}
//
// 鉴权用账号 Credential.APIKey 作为 Bearer token；base URL 可由 Credential.BaseURL
// 覆盖，默认 https://ark.ap-southeast.bytepluses.com。
//
// 为了与 veo 后端共用上层编排，本后端把 task id 映射进 genai.GenerateVideosOperation
// 的 Name，把结果 url 映射进 Response.GeneratedVideos[i].Video.URI，尾帧 url 存入
// Metadata。图片输入统一编码成 base64 data URL 传入。
package modelark

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shoucheng/my-first-agent/domain/account"
	"google.golang.org/genai"
)

const (
	defaultBaseURL      = "https://ark.ap-southeast.bytepluses.com"
	createTaskPath      = "/api/v3/contents/generations/tasks"
	retrieveTaskPathFmt = "/api/v3/contents/generations/tasks/%s"

	contentTypeText     = "text"
	contentTypeImageURL = "image_url"

	roleReferenceImage = "reference_image"
	roleLastFrame      = "last_frame"

	// metadataKeyLastFrame 是尾帧 url 在 operation.Metadata 中的键。
	metadataKeyLastFrame = "modelark_last_frame_urls"
)

// Client 是 modelark 视频生成客户端。
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New 用账号凭据构造一个 modelark 客户端。
func New(acc *account.Account) (*Client, error) {
	base := strings.TrimRight(acc.Credential.BaseURL, "/")
	if base == "" {
		base = defaultBaseURL
	}
	return &Client{
		baseURL: base,
		apiKey:  acc.Credential.APIKey,
		http:    http.DefaultClient,
	}, nil
}

// --- 请求 / 响应 结构 ---

type urlValue struct {
	URL string `json:"url"`
}

type content struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *urlValue `json:"image_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type createTaskRequest struct {
	Model           string    `json:"model"`
	Content         []content `json:"content"`
	Resolution      string    `json:"resolution,omitempty"`
	Ratio           string    `json:"ratio,omitempty"`
	Duration        *int32    `json:"duration,omitempty"`
	GenerateAudio   *bool     `json:"generate_audio,omitempty"`
	Seed            *int32    `json:"seed,omitempty"`
	ReturnLastFrame bool      `json:"return_last_frame,omitempty"`
}

type createTaskResponse struct {
	ID string `json:"id"`
}

type taskError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type taskContent struct {
	VideoURL     string `json:"video_url,omitempty"`
	LastFrameURL string `json:"last_frame_url,omitempty"`
}

type taskResponse struct {
	ID      string       `json:"id"`
	Status  string       `json:"status"`
	Error   *taskError   `json:"error,omitempty"`
	Content *taskContent `json:"content,omitempty"`
}

// Submit 提交一次生成任务，返回一个只带 task id 的未完成 operation。
func (c *Client) Submit(ctx context.Context, model, prompt string, image *genai.Image, config *genai.GenerateVideosConfig) (*genai.GenerateVideosOperation, error) {
	body, err := buildCreateRequest(model, prompt, image, config)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("modelark: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+createTaskPath, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("modelark: create task: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("modelark: create task http %d: %s", resp.StatusCode, string(data))
	}
	var out createTaskResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("modelark: decode create response: %w", err)
	}
	if out.ID == "" {
		return nil, fmt.Errorf("modelark: empty task id in response: %s", string(data))
	}
	return &genai.GenerateVideosOperation{Name: out.ID}, nil
}

// Poll 查询一次任务状态并把结果映射进 operation。未完成时返回 Done=false 的 operation。
func (c *Client) Poll(ctx context.Context, op *genai.GenerateVideosOperation) (*genai.GenerateVideosOperation, error) {
	if op == nil || op.Name == "" {
		return nil, fmt.Errorf("modelark: poll requires a task id")
	}
	url := c.baseURL + fmt.Sprintf(retrieveTaskPathFmt, op.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("modelark: retrieve task: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("modelark: retrieve task http %d: %s", resp.StatusCode, string(data))
	}
	var task taskResponse
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("modelark: decode task response: %w", err)
	}

	switch strings.ToLower(task.Status) {
	case "succeeded", "success", "completed", "done":
		if task.Content == nil || task.Content.VideoURL == "" {
			return nil, fmt.Errorf("modelark: task %s succeeded but returned no video_url", op.Name)
		}
		op.Done = true
		op.Response = &genai.GenerateVideosResponse{
			GeneratedVideos: []*genai.GeneratedVideo{
				{Video: &genai.Video{URI: task.Content.VideoURL, MIMEType: "video/mp4"}},
			},
		}
		if task.Content.LastFrameURL != "" {
			if op.Metadata == nil {
				op.Metadata = map[string]any{}
			}
			op.Metadata[metadataKeyLastFrame] = []string{task.Content.LastFrameURL}
		}
		return op, nil
	case "failed", "failure", "cancelled", "canceled":
		return nil, failureError(op.Name, task.Error)
	default:
		// 排队 / 运行中：保持未完成，交由上层按间隔重试。
		op.Done = false
		return op, nil
	}
}

func failureError(id string, e *taskError) error {
	if e == nil {
		return fmt.Errorf("modelark: task %s failed", id)
	}
	return fmt.Errorf("modelark: task %s failed: %s - %s", id, e.Code, e.Message)
}

// buildCreateRequest 把统一入参组装成 modelark 的请求体。
//
// content 顺序：prompt 文本 → 首帧图（role 空）→ 参考图（role=reference_image）→
// 尾帧图（role=last_frame）。图片一律编码为 base64 data URL。
func buildCreateRequest(model, prompt string, image *genai.Image, config *genai.GenerateVideosConfig) (*createTaskRequest, error) {
	req := &createTaskRequest{
		Model:           model,
		ReturnLastFrame: true,
		Content:         []content{{Type: contentTypeText, Text: prompt}},
	}

	if image != nil {
		url, err := imageToURL(image)
		if err != nil {
			return nil, fmt.Errorf("modelark: first-frame image: %w", err)
		}
		req.Content = append(req.Content, content{Type: contentTypeImageURL, ImageURL: &urlValue{URL: url}})
	}

	if config != nil {
		for i, ref := range config.ReferenceImages {
			if ref == nil || ref.Image == nil {
				continue
			}
			url, err := imageToURL(ref.Image)
			if err != nil {
				return nil, fmt.Errorf("modelark: reference image %d: %w", i, err)
			}
			req.Content = append(req.Content, content{Type: contentTypeImageURL, ImageURL: &urlValue{URL: url}, Role: roleReferenceImage})
		}
		if config.LastFrame != nil {
			url, err := imageToURL(config.LastFrame)
			if err != nil {
				return nil, fmt.Errorf("modelark: last-frame image: %w", err)
			}
			req.Content = append(req.Content, content{Type: contentTypeImageURL, ImageURL: &urlValue{URL: url}, Role: roleLastFrame})
		}
		req.Resolution = normalizeResolution(config.Resolution)
		req.Ratio = config.AspectRatio
		req.Duration = config.DurationSeconds
		req.GenerateAudio = config.GenerateAudio
		req.Seed = config.Seed
	}
	return req, nil
}

// imageToURL 把 genai.Image 转成 modelark 可消费的 url：
// 有字节则编码为 base64 data URL；否则退回已有的 GCS/http url。
func imageToURL(img *genai.Image) (string, error) {
	if len(img.ImageBytes) > 0 {
		mime := img.MIMEType
		if mime == "" {
			mime = "image/jpeg"
		}
		return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(img.ImageBytes)), nil
	}
	if img.GCSURI != "" {
		return img.GCSURI, nil
	}
	return "", fmt.Errorf("image has neither bytes nor uri")
}

// normalizeResolution 把常见分辨率写法归一到 modelark 的 p 后缀词表；未知值原样透传。
func normalizeResolution(res string) string {
	switch res {
	case "1280x720", "720":
		return "720p"
	case "1920x1080", "1080":
		return "1080p"
	case "854x480", "480":
		return "480p"
	default:
		return res
	}
}
