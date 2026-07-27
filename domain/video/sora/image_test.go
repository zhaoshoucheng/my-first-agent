package sora

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// makePNG 生成一张纯色 w×h 的 png 字节。
func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestFitToSora(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		w, h       int
		resolution string
		wantSize   string
	}{
		// sora-2：只 720p
		{"sora2 landscape", "sora-2", 1920, 1080, "", "1280x720"},
		{"sora2 landscape high->720", "sora-2", 1920, 1080, "1080p", "1280x720"},
		{"sora2 portrait", "sora-2", 1080, 1920, "", "720x1280"},
		{"sora2 portrait high->720", "sora-2", 1080, 1920, "1080p", "720x1280"},
		{"sora2 square -> landscape", "sora-2", 800, 800, "", "1280x720"},
		{"sora2 odd size", "sora-2", 1234, 567, "", "1280x720"},
		// sora-2-pro：支持 1080 档
		{"pro landscape high", "sora-2-pro", 1920, 1080, "1080p", "1792x1024"},
		{"pro portrait high", "sora-2-pro", 1080, 1920, "1080p", "1024x1792"},
		{"pro landscape standard", "sora-2-pro", 1920, 1080, "", "1280x720"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := makePNG(t, c.w, c.h)
			out, mime, size, err := fitToSora(src, c.model, c.resolution)
			if err != nil {
				t.Fatalf("fitToSora: %v", err)
			}
			if size != c.wantSize {
				t.Errorf("size=%q want %q", size, c.wantSize)
			}
			if mime != "image/png" {
				t.Errorf("mime=%q want image/png", mime)
			}
			// 解码输出，确认像素尺寸精确等于 size（Sora 的硬性要求）。
			img, _, err := image.Decode(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("decode output: %v", err)
			}
			b := img.Bounds()
			want := c.wantSize
			got := ""
			got += itoa(b.Dx()) + "x" + itoa(b.Dy())
			if got != want {
				t.Errorf("output dims=%s want %s", got, want)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
