// Package sora 是视频生成的 azure-openai 后端（Azure 上部署的 Sora 模型
// sora-2 / sora-2-pro），用标准库 net/http 直连 Azure OpenAI 的 video API：
//
//   - 提交：POST {base}/openai/v1/videos?api-version=…            （multipart/form-data）→ {id, status}
//   - 轮询：GET  {base}/openai/v1/videos/{id}?api-version=…       → {status, error}
//   - 取件：GET  {base}/openai/v1/videos/{id}/content?api-version=… → 视频二进制
//
// 鉴权用账号 Credential.APIKey 作为 Bearer token；base URL 取 Credential.BaseURL
// （必填，形如 https://<resource>.openai.azure.com），api-version 取 Credential.APIVersion。
//
// Sora 的参数模型与 Veo 不同：分辨率+比例合成一个 size 串（WxH），时长只接受
// 4/8/12 秒，且原生自带音频、无音频开关。为了让同一个 spec 能驱动 Sora，本后端把
// resolution+aspect_ratio 换算成 size、把 duration_seconds 归一到允许档位，其余
// Veo 专属字段（seed / generate_audio / negative_prompt / reference_images / fps …）
// 一律忽略。首帧图作为 input_reference 二进制部件上传（Azure v1 视频端点与 OpenAI
// schema 兼容）；结果通过 content 端点取回字节，塞进 operation 的 VideoBytes。
package sora

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/shoucheng/my-first-agent/domain/account"
	"google.golang.org/genai"
)

const (
	endpointCreate   = "/openai/v1/videos"
	endpointRetrieve = "/openai/v1/videos/%s"
	endpointDownload = "/openai/v1/videos/%s/content"

	// defaultAPIVersion 是 Azure v1 视频端点（/openai/v1/videos）预览特性要求的
	// api-version 值。credential.api_version 为空时用它兜底。
	defaultAPIVersion = "preview"
)

// soraSeconds 是 Sora 允许的时长档位（秒）。
var soraSeconds = []int32{4, 8, 12}

// wxhRe 匹配显式的 "宽x高" 分辨率写法，如 1280x720。
var wxhRe = regexp.MustCompile(`^\d+x\d+$`)

// Client 是 Sora（azure-openai）视频生成客户端。
type Client struct {
	baseURL    string
	apiKey     string
	apiVersion string
	http       *http.Client
}

// New 用账号凭据构造一个 Sora 客户端。base_url 必填（Azure 资源终端点）。
func New(acc *account.Account) (*Client, error) {
	base := strings.TrimRight(acc.Credential.BaseURL, "/")
	if base == "" {
		return nil, fmt.Errorf("sora: account %q requires credential.base_url (azure endpoint)", acc.Name)
	}
	apiVersion := acc.Credential.APIVersion
	if apiVersion == "" {
		apiVersion = defaultAPIVersion
	}
	return &Client{
		baseURL:    base,
		apiKey:     acc.Credential.APIKey,
		apiVersion: apiVersion,
		http:       http.DefaultClient,
	}, nil
}

type videoJob struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// urlFor 拼出带 api-version 的完整 url。
func (c *Client) urlFor(path string) string {
	u := c.baseURL + path
	if c.apiVersion != "" {
		u += "?api-version=" + url.QueryEscape(c.apiVersion)
	}
	return u
}

// Submit 以 multipart 方式创建一个视频生成任务，返回只带 id 的未完成 operation。
func (c *Client) Submit(ctx context.Context, model, prompt string, image *genai.Image, config *genai.GenerateVideosConfig) (*genai.GenerateVideosOperation, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	if err := mw.WriteField("prompt", prompt); err != nil {
		return nil, err
	}
	if err := mw.WriteField("model", model); err != nil {
		return nil, err
	}

	resolution, aspect := "", ""
	if config != nil {
		resolution, aspect = config.Resolution, config.AspectRatio
	}

	// 首帧图：适配成 Sora 允许尺寸（要求 input_reference 像素宽高精确等于 size），
	// 并据此确定 size；无图时 size 由 resolution+aspect 换算。
	var imgData []byte
	var imgMIME, size string
	if image != nil {
		raw, err := imageBytes(ctx, c.http, image)
		if err != nil {
			return nil, fmt.Errorf("sora: input image: %w", err)
		}
		if imgData, imgMIME, size, err = fitToSora(raw, model, resolution); err != nil {
			return nil, fmt.Errorf("sora: fit input image: %w", err)
		}
	} else {
		size = mapSize(model, resolution, aspect)
	}

	if size != "" {
		if err := mw.WriteField("size", size); err != nil {
			return nil, err
		}
	}
	if config != nil {
		if secs := mapSeconds(config.DurationSeconds); secs != "" {
			if err := mw.WriteField("seconds", secs); err != nil {
				return nil, err
			}
		}
	}

	if imgData != nil {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="input_reference"; filename="input_reference%s"`, extFromMIME(imgMIME)))
		h.Set("Content-Type", imgMIME)
		part, err := mw.CreatePart(h)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(imgData); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.urlFor(endpointCreate), &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	job, err := c.doJob(req)
	if err != nil {
		return nil, err
	}
	return &genai.GenerateVideosOperation{Name: job.ID}, nil
}

// Poll 查询任务状态；完成时通过 content 端点取回字节塞进 operation。
func (c *Client) Poll(ctx context.Context, op *genai.GenerateVideosOperation) (*genai.GenerateVideosOperation, error) {
	if op == nil || op.Name == "" {
		return nil, fmt.Errorf("sora: poll requires a video id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.urlFor(fmt.Sprintf(endpointRetrieve, op.Name)), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	job, err := c.doJob(req)
	if err != nil {
		return nil, err
	}
	if job.Error != nil {
		return nil, fmt.Errorf("sora: video %s failed: %s - %s", op.Name, job.Error.Code, job.Error.Message)
	}

	switch strings.ToLower(job.Status) {
	case "succeeded", "completed":
		videoBytes, err := c.downloadContent(ctx, op.Name)
		if err != nil {
			return nil, err
		}
		op.Done = true
		op.Response = &genai.GenerateVideosResponse{
			GeneratedVideos: []*genai.GeneratedVideo{
				{Video: &genai.Video{VideoBytes: videoBytes, MIMEType: "video/mp4"}},
			},
		}
		return op, nil
	case "failed", "cancelled", "canceled":
		return nil, fmt.Errorf("sora: video %s failed (status=%s)", op.Name, job.Status)
	default:
		// queued / preprocessing / running：保持未完成，交由上层按间隔重试。
		op.Done = false
		return op, nil
	}
}

// downloadContent 从 content 端点取回视频二进制。
func (c *Client) downloadContent(ctx context.Context, id string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.urlFor(fmt.Sprintf(endpointDownload, id)), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sora: download content: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sora: download content http %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

// doJob 执行请求并解析成 videoJob。
func (c *Client) doJob(req *http.Request) (*videoJob, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sora: request %s: %w", req.URL.Path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sora: %s http %d: %s", req.URL.Path, resp.StatusCode, string(data))
	}
	var job videoJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("sora: decode response: %w", err)
	}
	if job.ID == "" {
		return nil, fmt.Errorf("sora: empty video id in response: %s", string(data))
	}
	return &job, nil
}

// imageBytes 取图片字节：优先内联字节；否则 http(s) url 拉取（gs:// 等无法直读则报错）。
func imageBytes(ctx context.Context, hc *http.Client, img *genai.Image) ([]byte, error) {
	if len(img.ImageBytes) > 0 {
		return img.ImageBytes, nil
	}
	if strings.HasPrefix(img.GCSURI, "http://") || strings.HasPrefix(img.GCSURI, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, img.GCSURI, nil)
		if err != nil {
			return nil, err
		}
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("fetch %q http %d", img.GCSURI, resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}
	return nil, fmt.Errorf("image has no bytes and url is not http(s): %q", img.GCSURI)
}

// mapSize 把 resolution + aspect_ratio 换算成该模型允许的 Sora size（WxH）。
//
//   - resolution 若已是 "宽x高"：按其方向/档位归一到该模型允许的尺寸（避免直接透传
//     出不被支持的值，如 sora-2 上的 1792x1024）；
//   - 否则按 aspect_ratio（9:16 竖 / 其余横）与 resolution（含 1080/1024/1792 → 高清档）
//     选允许尺寸（高清档仅 sora-2-pro 生效）；
//   - 两者都为空返回 ""（交给 API 默认）。
func mapSize(model, resolution, aspect string) string {
	if wxhRe.MatchString(resolution) {
		w, h := parseWxH(resolution)
		tw, th := pickSize(model, h > w, maxInt(w, h) > 1280)
		return fmt.Sprintf("%dx%d", tw, th)
	}
	if resolution == "" && aspect == "" {
		return ""
	}
	portrait := aspect == "9:16"
	high := strings.Contains(resolution, "1080") || strings.Contains(resolution, "1792") || strings.Contains(resolution, "1024")
	tw, th := pickSize(model, portrait, high)
	return fmt.Sprintf("%dx%d", tw, th)
}

// parseWxH 解析 "宽x高"；解析失败返回 0,0（会被 pickSize 当作横屏标清处理）。
func parseWxH(s string) (int, int) {
	i := strings.IndexByte(s, 'x')
	if i < 0 {
		return 0, 0
	}
	w, _ := strconv.Atoi(s[:i])
	h, _ := strconv.Atoi(s[i+1:])
	return w, h
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// mapSeconds 把 duration_seconds 归一到 Sora 允许档位（4/8/12）的最近值。
func mapSeconds(d *int32) string {
	if d == nil {
		return ""
	}
	best := soraSeconds[0]
	for _, v := range soraSeconds[1:] {
		if abs32(v-*d) < abs32(best-*d) {
			best = v
		}
	}
	return strconv.Itoa(int(best))
}

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// extFromMIME 由图片 MIME 推一个文件扩展名，用于 multipart 部件的 filename。
func extFromMIME(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	default:
		return ""
	}
}
