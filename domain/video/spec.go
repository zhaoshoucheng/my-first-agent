package video

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/genai"
	"gopkg.in/yaml.v3"
)

// Spec 是"用一个文件描述一次视频生成请求"的用户可写载体。
//
// 它把 Request + genai.GenerateVideosConfig 的字段摊平成对人/AI 友好的形状，
// 支持 JSON 或 YAML（按扩展名判断）。图片元素既可给本地文件 path，也可给 url
// （gs:// 或 http(s)://）。字段含义与取值见 docs/video-request-spec.md。
//
// 可选数值/布尔用指针，以区分"未设置"与"零值"（例如 generate_audio: false 与不写）。
type Spec struct {
	Model  string `json:"model"  yaml:"model"`  // 模型名，空则用配置里的 default_model
	Prompt string `json:"prompt" yaml:"prompt"` // 文本提示词

	// 图片元素
	Image           *ImageRef  `json:"image,omitempty"            yaml:"image,omitempty"`            // 首帧（图生视频）
	LastFrame       *ImageRef  `json:"last_frame,omitempty"       yaml:"last_frame,omitempty"`       // 尾帧
	ReferenceImages []RefImage `json:"reference_images,omitempty" yaml:"reference_images,omitempty"` // 参考图（带 role）

	// 生成参数（对应 genai.GenerateVideosConfig）
	Resolution       string `json:"resolution,omitempty"        yaml:"resolution,omitempty"`        // 720p | 1080p | 1280x720 | 1920x1080
	AspectRatio      string `json:"aspect_ratio,omitempty"      yaml:"aspect_ratio,omitempty"`      // 16:9 | 9:16
	DurationSeconds  *int32 `json:"duration_seconds,omitempty"  yaml:"duration_seconds,omitempty"`  // 时长（秒）
	GenerateAudio    *bool  `json:"generate_audio,omitempty"    yaml:"generate_audio,omitempty"`    // 是否生成音频（Veo 3 支持）
	Seed             *int32 `json:"seed,omitempty"              yaml:"seed,omitempty"`              // RNG 种子
	FPS              *int32 `json:"fps,omitempty"               yaml:"fps,omitempty"`               // 帧率
	NumberOfVideos   int32  `json:"number_of_videos,omitempty"  yaml:"number_of_videos,omitempty"`  // 输出视频数
	NegativePrompt   string `json:"negative_prompt,omitempty"   yaml:"negative_prompt,omitempty"`   // 反向提示
	PersonGeneration string `json:"person_generation,omitempty" yaml:"person_generation,omitempty"` // dont_allow | allow_adult
	EnhancePrompt    bool   `json:"enhance_prompt,omitempty"    yaml:"enhance_prompt,omitempty"`    // 是否启用提示词改写
	OutputGCSURI     string `json:"output_gcs_uri,omitempty"    yaml:"output_gcs_uri,omitempty"`    // Veo 结果直接写入的 GCS 桶
}

// ImageRef 是一个图片元素：path（本地文件）与 url（gs:// 或 http(s)://）二选一。
type ImageRef struct {
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	URL  string `json:"url,omitempty"  yaml:"url,omitempty"`
	MIME string `json:"mime,omitempty" yaml:"mime,omitempty"` // 可选，缺省按扩展名推断
}

// RefImage 是带 role 的参考图。role 取值 ASSET | STYLE（仅 Veo 用；modelark 一律当参考图）。
type RefImage struct {
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	URL  string `json:"url,omitempty"  yaml:"url,omitempty"`
	MIME string `json:"mime,omitempty" yaml:"mime,omitempty"`
	Role string `json:"role,omitempty" yaml:"role,omitempty"`
}

// LoadSpec 从 JSON/YAML 文件读取一个 Spec 并组装成 Request。
// 扩展名 .json 走 JSON，.yaml/.yml 走 YAML；其它扩展名先试 YAML（兼容 JSON）。
func LoadSpec(path string) (*Request, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("video: read spec %q: %w", path, err)
	}
	var spec Spec
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("video: parse json spec %q: %w", path, err)
		}
	default:
		// yaml.v3 也能解析 JSON，作为通用兜底。
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("video: parse yaml spec %q: %w", path, err)
		}
	}
	return spec.ToRequest()
}

// ToRequest 把 Spec 组装成 Request（读取本地图片文件、拼装 genai config）。
func (s *Spec) ToRequest() (*Request, error) {
	req := &Request{Model: s.Model, Prompt: s.Prompt}

	img, err := loadImageRef(s.Image)
	if err != nil {
		return nil, fmt.Errorf("video: image: %w", err)
	}
	req.Image = img

	cfg := &genai.GenerateVideosConfig{
		Resolution:       s.Resolution,
		AspectRatio:      s.AspectRatio,
		DurationSeconds:  s.DurationSeconds,
		GenerateAudio:    s.GenerateAudio,
		Seed:             s.Seed,
		FPS:              s.FPS,
		NumberOfVideos:   s.NumberOfVideos,
		NegativePrompt:   s.NegativePrompt,
		PersonGeneration: s.PersonGeneration,
		EnhancePrompt:    s.EnhancePrompt,
		OutputGCSURI:     s.OutputGCSURI,
	}

	lastFrame, err := loadImageRef(s.LastFrame)
	if err != nil {
		return nil, fmt.Errorf("video: last_frame: %w", err)
	}
	cfg.LastFrame = lastFrame

	for i, ref := range s.ReferenceImages {
		image, err := loadImage(ref.Path, ref.URL, ref.MIME)
		if err != nil {
			return nil, fmt.Errorf("video: reference_images[%d]: %w", i, err)
		}
		if image == nil {
			continue
		}
		cfg.ReferenceImages = append(cfg.ReferenceImages, &genai.VideoGenerationReferenceImage{
			Image:         image,
			ReferenceType: referenceType(ref.Role),
		})
	}

	req.Config = cfg
	return req, nil
}

// loadImageRef 把一个 *ImageRef 转成 genai.Image；ref 为 nil 或空则返回 nil。
func loadImageRef(ref *ImageRef) (*genai.Image, error) {
	if ref == nil {
		return nil, nil
	}
	return loadImage(ref.Path, ref.URL, ref.MIME)
}

// loadImage 由 path（读本地文件字节）或 url（gs:// / http(s)://，存进 GCSURI）构造图片。
// 两者都空返回 nil，nil。两者都填以 path 为准。
func loadImage(path, url, mime string) (*genai.Image, error) {
	switch {
	case path != "":
		bytes, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", path, err)
		}
		if mime == "" {
			mime = mimeFromExt(path)
		}
		return &genai.Image{ImageBytes: bytes, MIMEType: mime}, nil
	case url != "":
		return &genai.Image{GCSURI: url, MIMEType: mime}, nil
	default:
		return nil, nil
	}
}

func referenceType(role string) genai.VideoGenerationReferenceType {
	if strings.EqualFold(role, "STYLE") {
		return genai.VideoGenerationReferenceTypeStyle
	}
	return genai.VideoGenerationReferenceTypeAsset
}

func mimeFromExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
