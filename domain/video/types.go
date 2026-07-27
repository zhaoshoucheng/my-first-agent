// Package video 是视频生成服务：对上给出统一的 GenerateVideo 入口，对下按账号
// Provider 分发到不同后端（veo 走 gcp-vertex-ai，seedance/dreamina 走 modelark）。
//
// 设计上尽量复用 google.golang.org/genai 的类型作为接口通用载体：
//   - genai.Image                    — 图片输入（首帧 / 尾帧 / 参考图）
//   - genai.GenerateVideosConfig     — 生成参数（分辨率 / 比例 / 时长 / 音频 / 种子 …）
//   - genai.GenerateVideosOperation  — 后端返回的长任务句柄（含轮询结果）
//
// modelark 后端不用 genai SDK，但把自己的任务 id / 结果映射进上述 genai 类型，
// 这样 Service 的编排逻辑（提交 → 轮询 → 下载 → 落盘）对两个后端是同一套。
package video

import "google.golang.org/genai"

// Request 是一次视频生成请求。
//
// 功能档位：
//   - A 档（全做）：文生视频、图生首帧、尾帧、参数（resolution / ratio /
//     duration / generate_audio / seed）——参数经 Config 承载。
//   - B 档（做）：参考图 roles——经 Config.ReferenceImages 承载。
//   - C 档（不做）：视频生视频、参考音频——类型留字段，遇不支持组合直接报错。
type Request struct {
	Model  string       // 模型名，空则由 Service 回退到配置的默认模型
	Prompt string       // 文本提示词
	Image  *genai.Image // 可选：图生视频的首帧

	// Config 承载所有生成参数与进阶图片输入：
	//   - LastFrame       尾帧
	//   - ReferenceImages 参考图（带 role：ASSET / STYLE）
	//   - Resolution / AspectRatio / DurationSeconds / GenerateAudio / Seed
	//   - NegativePrompt
	Config *genai.GenerateVideosConfig
}

// Result 是一次视频生成的最终产物。
type Result struct {
	Model     string // 实际使用的模型名
	LocalPath string // 落盘后的本地文件路径
	SourceURI string // 后端返回的来源地址（GCS URI 或 http url），可能为空（纯字节回传时）
}
