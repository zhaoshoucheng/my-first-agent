package builtin

import "testing"

func TestBuildVideoRequest(t *testing.T) {
	args := map[string]any{
		"prompt":           "a cat surfing",
		"model":            "sora-2",
		"resolution":       "1080p",
		"aspect_ratio":     "16:9",
		"duration_seconds": float64(8), // JSON 数字解码为 float64
		"generate_audio":   true,
		"seed":             float64(42),
		"negative_prompt":  "blurry",
	}
	req, err := buildVideoRequest(args)
	if err != nil {
		t.Fatalf("buildVideoRequest: %v", err)
	}
	if req.Model != "sora-2" || req.Prompt != "a cat surfing" {
		t.Errorf("model/prompt = %q/%q", req.Model, req.Prompt)
	}
	c := req.Config
	if c.Resolution != "1080p" || c.AspectRatio != "16:9" || c.NegativePrompt != "blurry" {
		t.Errorf("config strings wrong: %+v", c)
	}
	if c.DurationSeconds == nil || *c.DurationSeconds != 8 {
		t.Errorf("duration = %v want 8", c.DurationSeconds)
	}
	if c.Seed == nil || *c.Seed != 42 {
		t.Errorf("seed = %v want 42", c.Seed)
	}
	if c.GenerateAudio == nil || !*c.GenerateAudio {
		t.Errorf("generate_audio = %v want true", c.GenerateAudio)
	}
	if req.Image != nil {
		t.Errorf("image should be nil when image_path absent")
	}
}

func TestBuildVideoRequest_PromptRequired(t *testing.T) {
	if _, err := buildVideoRequest(map[string]any{"prompt": "  "}); err == nil {
		t.Error("expected error for blank prompt")
	}
	if _, err := buildVideoRequest(map[string]any{}); err == nil {
		t.Error("expected error for missing prompt")
	}
}

func TestGenerateVideo_Schema(t *testing.T) {
	v := NewGenerateVideo()
	if v.Name() != "generate_video" {
		t.Errorf("name = %q", v.Name())
	}
	// 必填字段应为 prompt
	req, _ := v.Parameters()["required"].([]any)
	if len(req) != 1 || req[0] != "prompt" {
		t.Errorf("required = %v want [prompt]", req)
	}
}
