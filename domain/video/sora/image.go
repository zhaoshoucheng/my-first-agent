package sora

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // 注册 gif 解码
	_ "image/jpeg" // 注册 jpeg 解码
	"image/png"
	"math"
	"strconv"
	"strings"
)

// isPro 判断是否为 sora-2-pro（pro 才支持 1080 档尺寸）。
func isPro(model string) bool {
	return strings.Contains(strings.ToLower(model), "pro")
}

// pickSize 按模型能力、方向（竖/横）、清晰度档位选一个 Sora 允许的输出尺寸。
//
// 各模型允许的分辨率（来自 API 约束）：
//   - sora-2      ：720x1280 / 1280x720（仅 720p 两档）
//   - sora-2-pro  ：额外支持 1024x1792 / 1792x1024（1080 档）
//
// 因此 high 档只在 pro 上生效；sora-2 上无论是否高清都落到 720p 两档。
func pickSize(model string, portrait, high bool) (int, int) {
	if portrait {
		if high && isPro(model) {
			return 1024, 1792
		}
		return 720, 1280
	}
	if high && isPro(model) {
		return 1792, 1024
	}
	return 1280, 720
}

// fitToSora 把首帧图适配成 Sora 可接受的尺寸：
//
//	Azure Sora 要求 input_reference 的像素宽高必须精确等于请求的 size。用户的原图
//	（如分镜稿）通常不是 Sora 允许的尺寸，直接传会报 "Inpaint image must match the
//	requested width and height"。这里按原图宽高比选最贴近的允许尺寸（避免拉伸失真），
//	再 cover-crop + 双线性缩放到该尺寸，重新编码为 PNG。仅用标准库，不引第三方依赖。
//
// resolution 只用来选清晰度档位（含 1080/1792/1024 视为高清档，仅 pro 生效）。
// 返回：适配后的图片字节、MIME、以及对应的 size 串（"宽x高"）。支持 png/jpeg/gif 输入。
func fitToSora(src []byte, model, resolution string) ([]byte, string, string, error) {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, "", "", fmt.Errorf("decode image (png/jpeg/gif supported): %w", err)
	}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		return nil, "", "", fmt.Errorf("image has zero dimension")
	}
	tw, th := chooseSoraSize(model, sw, sh, resolution)

	dst := scaleBilinear(img, coverRect(sw, sh, tw, th), tw, th)

	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, "", "", fmt.Errorf("encode png: %w", err)
	}
	size := strconv.Itoa(tw) + "x" + strconv.Itoa(th)
	return out.Bytes(), "image/png", size, nil
}

// chooseSoraSize 按原图宽高比（竖/横）与清晰度档位、模型能力选一个允许尺寸。
func chooseSoraSize(model string, sw, sh int, resolution string) (int, int) {
	portrait := sh > sw
	high := containsAny(resolution, "1080", "1792", "1024")
	return pickSize(model, portrait, high)
}

// coverRect 计算原图中与目标宽高比一致、居中的最大子矩形（用于 cover-crop）。
func coverRect(sw, sh, tw, th int) image.Rectangle {
	ta := float64(tw) / float64(th)
	sa := float64(sw) / float64(sh)
	if sa > ta {
		// 原图更宽 → 裁两侧
		cw := int(float64(sh) * ta)
		x0 := (sw - cw) / 2
		return image.Rect(x0, 0, x0+cw, sh)
	}
	// 原图更高 → 裁上下
	ch := int(float64(sw) / ta)
	y0 := (sh - ch) / 2
	return image.Rect(0, y0, sw, y0+ch)
}

// scaleBilinear 把 src 的 sr 子矩形双线性缩放到 tw×th 的 RGBA 图。
func scaleBilinear(src image.Image, sr image.Rectangle, tw, th int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	srW, srH := sr.Dx(), sr.Dy()
	for dy := 0; dy < th; dy++ {
		fy := (float64(dy)+0.5)*float64(srH)/float64(th) - 0.5
		y0 := int(math.Floor(fy))
		wy := fy - float64(y0)
		for dx := 0; dx < tw; dx++ {
			fx := (float64(dx)+0.5)*float64(srW)/float64(tw) - 0.5
			x0 := int(math.Floor(fx))
			wx := fx - float64(x0)

			r00, g00, b00, a00 := sampleAt(src, sr, x0, y0)
			r10, g10, b10, a10 := sampleAt(src, sr, x0+1, y0)
			r01, g01, b01, a01 := sampleAt(src, sr, x0, y0+1)
			r11, g11, b11, a11 := sampleAt(src, sr, x0+1, y0+1)

			dst.SetRGBA(dx, dy, color.RGBA{
				R: lerp2(r00, r10, r01, r11, wx, wy),
				G: lerp2(g00, g10, g01, g11, wx, wy),
				B: lerp2(b00, b10, b01, b11, wx, wy),
				A: lerp2(a00, a10, a01, a11, wx, wy),
			})
		}
	}
	return dst
}

// sampleAt 取 sr 子矩形内局部坐标 (lx,ly)（越界则夹取边缘）的像素，返回 8bit RGBA。
func sampleAt(src image.Image, sr image.Rectangle, lx, ly int) (r, g, b, a uint8) {
	if lx < 0 {
		lx = 0
	} else if lx >= sr.Dx() {
		lx = sr.Dx() - 1
	}
	if ly < 0 {
		ly = 0
	} else if ly >= sr.Dy() {
		ly = sr.Dy() - 1
	}
	cr, cg, cb, ca := src.At(sr.Min.X+lx, sr.Min.Y+ly).RGBA() // 0..65535
	return uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), uint8(ca >> 8)
}

// lerp2 对四个角做双线性插值。
func lerp2(v00, v10, v01, v11 uint8, wx, wy float64) uint8 {
	top := float64(v00)*(1-wx) + float64(v10)*wx
	bot := float64(v01)*(1-wx) + float64(v11)*wx
	return uint8(math.Round(top*(1-wy) + bot*wy))
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
