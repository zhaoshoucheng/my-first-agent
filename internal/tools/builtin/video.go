package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shoucheng/my-first-agent/domain/video"
	"github.com/shoucheng/my-first-agent/internal/tools"
	"google.golang.org/genai"
)

// GenerateVideo 把视频生成模块（domain/video）包装成一个可供大模型调用的工具。
//
// 依赖 video.Default()（需在进程启动时 video.Init()）；后端按 model 名前缀自动路由：
// veo-*（vertex-ai）/ sora-*（azure-openai）/ seedance-*|dreamina-*（modelark）。
type GenerateVideo struct{}

func NewGenerateVideo() *GenerateVideo { return &GenerateVideo{} }

func (v *GenerateVideo) Name() string { return "generate_video" }

func (v *GenerateVideo) Description() string {
	return tools.NormalizeDescription(
		"Generate a video from a text prompt, optionally guided by a first-frame image.",
		"The backend is chosen by model name: veo-* (Google Veo), sora-2/sora-2-pro (OpenAI/Azure Sora),",
		"seedance-*/dreamina-* (ModelArk). Submits the job, polls until done, downloads the result,",
		"and returns JSON with the local file path. Some params only apply to certain backends and are otherwise ignored.",
	)
}

func (v *GenerateVideo) Parameters() tools.JSONSchema {
	return tools.ObjectSchema(map[string]any{
		"prompt":           tools.StringProperty("Text prompt describing the video to generate."),
		"model":            tools.StringProperty("Model name, e.g. veo-3.0-generate-001 / sora-2 / seedance-1-0-pro. Empty uses the configured default_model."),
		"image_path":       tools.StringProperty("Optional local image file path used as the first frame (image-to-video)."),
		"resolution":       tools.StringProperty("Optional resolution: 720p / 1080p, or a WxH string like 1280x720."),
		"aspect_ratio":     tools.StringProperty("Optional aspect ratio: 16:9 (landscape) or 9:16 (portrait)."),
		"duration_seconds": tools.IntegerProperty("Optional duration in seconds."),
		"generate_audio":   map[string]any{"type": "boolean", "description": "Whether to generate audio (Veo 3; Sora has native audio; ignored elsewhere)."},
		"seed":             tools.IntegerProperty("Optional RNG seed for reproducibility."),
		"negative_prompt":  tools.StringProperty("Optional negative prompt describing what to avoid (Veo)."),
	}, "prompt")
}

func (v *GenerateVideo) Execute(ctx context.Context, args map[string]any) (string, error) {
	req, err := buildVideoRequest(args)
	if err != nil {
		return "", err
	}
	res, err := video.Default().GenerateVideo(ctx, req)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(map[string]string{
		"model":      res.Model,
		"local_path": res.LocalPath,
		"source_uri": res.SourceURI,
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// buildVideoRequest 把工具入参组装成 video.Request（与后端无关，便于单测）。
func buildVideoRequest(args map[string]any) (*video.Request, error) {
	prompt := strings.TrimSpace(stringArg(args, "prompt"))
	if prompt == "" {
		return nil, fmt.Errorf("generate_video: prompt is required")
	}
	req := &video.Request{
		Model:  stringArg(args, "model"),
		Prompt: prompt,
		Config: &genai.GenerateVideosConfig{
			Resolution:     stringArg(args, "resolution"),
			AspectRatio:    stringArg(args, "aspect_ratio"),
			NegativePrompt: stringArg(args, "negative_prompt"),
		},
	}
	if d, ok := int32Arg(args, "duration_seconds"); ok {
		req.Config.DurationSeconds = &d
	}
	if s, ok := int32Arg(args, "seed"); ok {
		req.Config.Seed = &s
	}
	if b, ok := args["generate_audio"].(bool); ok {
		req.Config.GenerateAudio = &b
	}
	if path := stringArg(args, "image_path"); path != "" {
		img, err := loadImageFile(path)
		if err != nil {
			return nil, err
		}
		req.Image = img
	}
	return req, nil
}

func stringArg(args map[string]any, key string) string {
	s, _ := args[key].(string)
	return s
}

func int32Arg(args map[string]any, key string) (int32, bool) {
	f, ok := tools.AsFloat(args[key])
	if !ok {
		return 0, false
	}
	return int32(f), true
}

func loadImageFile(path string) (*genai.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("generate_video: read image %q: %w", path, err)
	}
	return &genai.Image{ImageBytes: data, MIMEType: imageMIME(path)}, nil
}

func imageMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
