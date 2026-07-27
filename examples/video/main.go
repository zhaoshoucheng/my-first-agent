// examples/video 是视频生成模块的端到端驱动：
//
//	config.Init → account.Init → video.Init → GenerateVideo → 打印落盘路径
//
// 用法：
//
//	# 用命令行 flag 传参
//	go run ./examples/video -model veo-3.0-generate-001 -prompt "a cat surfing a wave"
//	go run ./examples/video -model seedance-1-0-pro -prompt "..." -image ./first.jpg
//
//	# 用一个文件描述完整请求（含各种元素与参数），字段说明见 docs/video-request-spec.md
//	go run ./examples/video -spec ./examples/video/spec.example.json
//
// 账号凭据来自账号目录下的账号文件（provider 为 gcp-vertex-ai 或 modelark）。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/shoucheng/my-first-agent/domain/account"
	"github.com/shoucheng/my-first-agent/domain/video"
	"github.com/shoucheng/my-first-agent/infra/config"
	"google.golang.org/genai"
)

func main() {
	cfgPath := flag.String("config", "config/config.yaml", "path to YAML config")
	specPath := flag.String("spec", "", "path to a JSON/YAML request spec (see docs/video-request-spec.md)")
	model := flag.String("model", "", "video model name (empty = config default_model)")
	prompt := flag.String("prompt", "A cat surfing a wave, cinematic.", "text prompt")
	imagePath := flag.String("image", "", "optional first-frame image file (image-to-video)")
	resolution := flag.String("resolution", "", "optional resolution, e.g. 720p / 1080p / 1280x720")
	ratio := flag.String("ratio", "", "optional aspect ratio, e.g. 16:9 / 9:16")
	duration := flag.Int("duration", 0, "optional duration in seconds (0 = backend default)")
	audio := flag.Bool("audio", false, "generate audio along with the video")
	seed := flag.Int("seed", 0, "optional RNG seed (0 = random)")
	flag.Parse()

	ctx := context.Background()

	if err := config.Init(*cfgPath); err != nil {
		log.Fatalf("init config: %v", err)
	}
	account.Init(ctx)
	video.Init()

	// -spec 优先：用文件描述完整请求；否则回退到命令行 flag。
	var req *video.Request
	if *specPath != "" {
		var err error
		if req, err = video.LoadSpec(*specPath); err != nil {
			log.Fatalf("load spec: %v", err)
		}
	} else {
		req = &video.Request{
			Model:  *model,
			Prompt: *prompt,
			Config: buildConfig(*resolution, *ratio, *duration, *audio, *seed),
		}
		if *imagePath != "" {
			img, err := loadImage(*imagePath)
			if err != nil {
				log.Fatalf("load image: %v", err)
			}
			req.Image = img
		}
	}

	fmt.Printf("submitting video generation (model=%q)...\n", req.Model)
	res, err := video.Default().GenerateVideo(ctx, req)
	if err != nil {
		log.Fatalf("generate video: %v", err)
	}
	fmt.Printf("done.\n  model:  %s\n  saved:  %s\n  source: %s\n", res.Model, res.LocalPath, res.SourceURI)
}

func buildConfig(resolution, ratio string, duration int, audio bool, seed int) *genai.GenerateVideosConfig {
	cfg := &genai.GenerateVideosConfig{
		Resolution:  resolution,
		AspectRatio: ratio,
	}
	if duration > 0 {
		d := int32(duration)
		cfg.DurationSeconds = &d
	}
	if audio {
		a := true
		cfg.GenerateAudio = &a
	}
	if seed > 0 {
		s := int32(seed)
		cfg.Seed = &s
	}
	return cfg
}

func loadImage(path string) (*genai.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &genai.Image{ImageBytes: data, MIMEType: mimeFromPath(path)}, nil
}

func mimeFromPath(path string) string {
	switch {
	case hasSuffix(path, ".png"):
		return "image/png"
	case hasSuffix(path, ".webp"):
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func hasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}
