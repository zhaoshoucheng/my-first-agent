package video

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shoucheng/my-first-agent/domain/account"
	"github.com/shoucheng/my-first-agent/infra/config"
	"google.golang.org/genai"
)

// 默认轮询间隔，用于配置缺省时兜底。
const defaultPollInterval = 10 * time.Second

// Service 是视频生成模块对外的唯一入口。
//
// 内部按账号名缓存后端 Client（双锁懒加载，并发安全），对上暴露 GenerateVideo：
// 按 model 名路由到账号 → 取/建 client → 提交任务 → 轮询到完成 → 下载落盘 → 返回结果。
type Service struct {
	mu      sync.RWMutex
	clients map[string]Client
}

// NewService 构造一个独立的视频生成服务实例。
func NewService() *Service {
	return &Service{
		clients: make(map[string]Client),
	}
}

// clientFor 取账号对应的后端 client（带缓存，缓存键是账号名）。
func (s *Service) clientFor(ctx context.Context, acc *account.Account) (Client, error) {
	s.mu.RLock()
	if c, ok := s.clients[acc.Name]; ok {
		s.mu.RUnlock()
		return c, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.clients[acc.Name]; ok { // double-check
		return c, nil
	}
	c, err := newClient(ctx, acc)
	if err != nil {
		return nil, fmt.Errorf("video.Service: build client for %q: %w", acc.Name, err)
	}
	s.clients[acc.Name] = c
	return c, nil
}

// GenerateVideo 是视频生成的主入口：提交任务、轮询到完成、下载落盘并返回结果。
//
// req.Model 为空时回退到配置的默认模型。调用方只需关心 model / prompt / 图片 / 参数，
// 不必感知账号、Provider、后端差异。
func (s *Service) GenerateVideo(ctx context.Context, req *Request) (*Result, error) {
	if req == nil {
		return nil, fmt.Errorf("video: nil request")
	}
	vcfg := videoSettings()
	model := req.Model
	if model == "" {
		model = vcfg.DefaultModel
	}
	if model == "" {
		return nil, fmt.Errorf("video: no model specified and no default_model configured")
	}

	acc, err := account.Default().PickAccountForModel(model)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(ctx, acc)
	if err != nil {
		return nil, err
	}

	log.Printf("[video] submitting task (model=%q, account=%q)...", model, acc.Name)
	op, err := client.Submit(ctx, model, req.Prompt, req.Image, req.Config)
	if err != nil {
		return nil, fmt.Errorf("video: submit %q: %w", model, err)
	}
	log.Printf("[video] submitted, task=%q — polling for result", op.Name)

	op, err = s.poll(ctx, client, op, pollInterval(vcfg))
	if err != nil {
		return nil, err
	}

	video, err := firstVideo(op)
	if err != nil {
		return nil, err
	}
	log.Printf("[video] task %q done, downloading result...", op.Name)
	localPath, err := s.download(ctx, video, model, op, vcfg.OutputDir)
	if err != nil {
		return nil, err
	}
	log.Printf("[video] saved to %s", localPath)
	return &Result{Model: model, LocalPath: localPath, SourceURI: video.URI}, nil
}

// poll 按间隔轮询直到任务完成或上下文取消，每轮打印一次心跳，避免长时间无输出。
func (s *Service) poll(ctx context.Context, client Client, op *genai.GenerateVideosOperation, interval time.Duration) (*genai.GenerateVideosOperation, error) {
	start := time.Now()
	for attempt := 1; ; attempt++ {
		var err error
		op, err = client.Poll(ctx, op)
		if err != nil {
			return nil, err
		}
		if op.Done {
			return op, nil
		}
		log.Printf("[video] task %q still running (attempt %d, elapsed %s), next check in %s",
			op.Name, attempt, elapsedRounded(start), interval)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// elapsedRounded 返回自 start 起、按秒取整的耗时，便于日志阅读。
func elapsedRounded(start time.Time) time.Duration {
	return time.Since(start).Round(time.Second)
}

// download 把生成的视频写到 outputDir 下，返回本地文件路径。
// 优先用内联字节（veo 不设 OutputGCSURI 时走这条）；否则按 http(s) url 下载（modelark）。
func (s *Service) download(ctx context.Context, video *genai.Video, model string, op *genai.GenerateVideosOperation, outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("video: mkdir %q: %w", outputDir, err)
	}
	path := filepath.Join(outputDir, outputFilename(model, op))

	var data []byte
	switch {
	case len(video.VideoBytes) > 0:
		data = video.VideoBytes
	case strings.HasPrefix(video.URI, "http://"), strings.HasPrefix(video.URI, "https://"):
		var err error
		if data, err = httpGet(ctx, video.URI); err != nil {
			return "", fmt.Errorf("video: download %q: %w", video.URI, err)
		}
	default:
		return "", fmt.Errorf("video: cannot download result (uri=%q, no inline bytes)", video.URI)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("video: write %q: %w", path, err)
	}
	return path, nil
}

func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// firstVideo 从完成的 operation 中取第一个视频。
func firstVideo(op *genai.GenerateVideosOperation) (*genai.Video, error) {
	if op.Response == nil || len(op.Response.GeneratedVideos) == 0 || op.Response.GeneratedVideos[0].Video == nil {
		return nil, fmt.Errorf("video: operation completed but returned no video")
	}
	return op.Response.GeneratedVideos[0].Video, nil
}

// outputFilename 由 model 与任务标识拼一个文件系统安全的 .mp4 文件名。
func outputFilename(model string, op *genai.GenerateVideosOperation) string {
	id := op.Name
	if i := strings.LastIndexAny(id, "/\\"); i >= 0 {
		id = id[i+1:]
	}
	id = sanitize(id)
	if id == "" {
		id = fmt.Sprintf("%d", time.Now().Unix())
	}
	return sanitize(model) + "-" + id + ".mp4"
}

// sanitize 把非字母数字、下划线、连字符、点的字符替换成下划线。
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			return r
		default:
			return '_'
		}
	}, s)
}

// videoSettings 读全局配置里的 video 段；未初始化时返回零值。
func videoSettings() config.VideoSettings {
	if c := config.GetConfig(); c != nil {
		return c.Video
	}
	return config.VideoSettings{}
}

func pollInterval(vcfg config.VideoSettings) time.Duration {
	if vcfg.PollIntervalS > 0 {
		return time.Duration(vcfg.PollIntervalS) * time.Second
	}
	return defaultPollInterval
}
