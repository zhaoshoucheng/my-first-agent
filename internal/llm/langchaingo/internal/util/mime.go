package util

import (
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// 常见文件扩展名对应的MIME类型映射，补充mime包可能不包含的类型
var additionalMimeTypes = map[string]string{
	".mp4":   "video/mp4",
	".webm":  "video/webm",
	".webp":  "image/webp",
	".svg":   "image/svg+xml",
	".avif":  "image/avif",
	".heic":  "image/heic",
	".heif":  "image/heif",
	".md":    "text/markdown",
	".yaml":  "application/x-yaml",
	".yml":   "application/x-yaml",
	".toml":  "application/toml",
	".wasm":  "application/wasm",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".woff":  "font/woff",
	".woff2": "font/woff2",
}

// DetectMimeType 检测URL内容的MIME类型
// 首先尝试通过HTTP头获取，如果失败则通过URL的文件扩展名判断
func DetectMimeType(url string) (string, error) {
	if mimeType, err := getMimeTypeFromHTTP(url); err == nil && mimeType != "" && mimeType != "unknown type" {
		return mimeType, nil
	}
	return getMimeTypeFromExtension(url), nil
}

func getMimeTypeFromHTTP(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; my-first-agent/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP error! Status code: %d", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return "unknown type", nil
	}
	return strings.TrimSpace(strings.Split(contentType, ";")[0]), nil
}

func getMimeTypeFromExtension(url string) string {
	urlPath := url
	if idx := strings.IndexByte(urlPath, '?'); idx >= 0 {
		urlPath = urlPath[:idx]
	}
	if idx := strings.IndexByte(urlPath, '#'); idx >= 0 {
		urlPath = urlPath[:idx]
	}
	ext := filepath.Ext(filepath.Base(urlPath))
	if ext == "" {
		return "application/octet-stream"
	}
	if mimeType, ok := additionalMimeTypes[strings.ToLower(ext)]; ok {
		return mimeType
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return "application/octet-stream"
	}
	return strings.TrimSpace(strings.Split(mimeType, ";")[0])
}
