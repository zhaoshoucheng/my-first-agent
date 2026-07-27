package sora

import "testing"

func TestMapSize(t *testing.T) {
	cases := []struct {
		model, res, aspect, want string
	}{
		// sora-2：只有 720p 两档，高清请求一律降到 720p
		{"sora-2", "1280x720", "", "1280x720"},
		{"sora-2", "1920x1080", "", "1280x720"},  // 显式高清 WxH → 归一到 720p
		{"sora-2", "", "", ""},                    // 都空 → 交给 API 默认
		{"sora-2", "720p", "16:9", "1280x720"},    // 横屏
		{"sora-2", "1080p", "16:9", "1280x720"},   // sora-2 无 1080 档 → 720p
		{"sora-2", "720p", "9:16", "720x1280"},    // 竖屏
		{"sora-2", "1080p", "9:16", "720x1280"},   // sora-2 竖屏也只 720p
		{"sora-2", "", "9:16", "720x1280"},        // 只给比例
		// sora-2-pro：支持 1080 档
		{"sora-2-pro", "1080p", "16:9", "1792x1024"},
		{"sora-2-pro", "1080p", "9:16", "1024x1792"},
		{"sora-2-pro", "1920x1080", "", "1792x1024"}, // 显式高清 WxH 保留
		{"sora-2-pro", "720p", "16:9", "1280x720"},   // 标清仍 720p
	}
	for _, c := range cases {
		if got := mapSize(c.model, c.res, c.aspect); got != c.want {
			t.Errorf("mapSize(%q,%q,%q)=%q want %q", c.model, c.res, c.aspect, got, c.want)
		}
	}
}

func TestMapSeconds(t *testing.T) {
	i32 := func(v int32) *int32 { return &v }
	cases := []struct {
		in   *int32
		want string
	}{
		{nil, ""},        // 不设 → 交给 API 默认
		{i32(4), "4"},
		{i32(8), "8"},
		{i32(12), "12"},
		{i32(6), "4"},    // 6 距 4 与 8 等距，取先者 4
		{i32(7), "8"},    // 更近 8
		{i32(100), "12"}, // 超范围 → 最大档
		{i32(1), "4"},    // 低于范围 → 最小档
	}
	for _, c := range cases {
		if got := mapSeconds(c.in); got != c.want {
			t.Errorf("mapSeconds(%v)=%q want %q", c.in, got, c.want)
		}
	}
}
