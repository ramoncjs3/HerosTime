// Package captcha 提供登录图形验证码的自动识别能力。
//
// 支持两种引擎：
//   - onnx：纯 Go 加载 ddddocr common.onnx 模型（gonnx 推理，无 cgo 依赖），离线识别；
//   - http：将图片 base64 提交到远程识别服务（兼容 ddddocr-server 风格响应）。
//
// 模型的输入输出契约与 Python 版 ddddocr 保持一致：
// 输入 [1,1,64,W] 灰度图（Lanczos3 等比缩放到高 64，(v/255-0.5)/0.5 归一化），
// 输出 int64 类别索引，经 CTC 贪心解码映射到 charset 字符表。
package captcha

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/advancedclimatesystems/gonnx"
	"github.com/nfnt/resize"
	"gorgonia.org/tensor"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/httputil"
)

// Recognizer 识别一张验证码图片并返回文本。
type Recognizer interface {
	Recognize(ctx context.Context, imageBytes []byte) (string, error)
}

// New 根据配置创建识别器；配置关闭时返回 nil, nil。
func New(cfg config.CaptchaConfig, httpClient *httputil.Client) (Recognizer, error) {
	if !cfg.IsEnabled() {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Engine)) {
	case "", "onnx", "ddddocr":
		return &onnxRecognizer{cfg: cfg}, nil
	case "http":
		if strings.TrimSpace(cfg.Endpoint) == "" {
			return nil, fmt.Errorf("captcha.endpoint is required when engine=http")
		}
		return &httpRecognizer{cfg: cfg, http: httpClient}, nil
	default:
		return nil, fmt.Errorf("unsupported captcha engine %q", cfg.Engine)
	}
}

// onnxRecognizer 懒加载 gonnx 模型；gonnx 推理非并发安全，识别过程加锁。
type onnxRecognizer struct {
	cfg config.CaptchaConfig

	mu    sync.Mutex
	once  sync.Once
	model *gonnx.Model
	err   error
}

func (r *onnxRecognizer) Recognize(ctx context.Context, imageBytes []byte) (string, error) {
	r.once.Do(func() {
		r.model, r.err = gonnx.NewModelFromFile(resolveAssetPath(r.cfg.ModelFile))
	})
	if r.err != nil {
		return "", fmt.Errorf("init captcha ocr (model=%s, see README \"验证码识别\" for asset setup): %w", r.cfg.ModelFile, r.err)
	}

	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return "", fmt.Errorf("decode captcha image: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	text, err := r.classify(img)
	if err != nil {
		return "", fmt.Errorf("captcha ocr classify: %w", err)
	}
	text = cleanText(text)
	if text == "" {
		return "", fmt.Errorf("captcha ocr returned empty text")
	}
	return text, nil
}

// classify 预处理图片并运行模型，CTC 解码后返回原始文本。
func (r *onnxRecognizer) classify(img image.Image) (string, error) {
	bounds := img.Bounds()
	if bounds.Dy() == 0 {
		return "", fmt.Errorf("image height is 0")
	}

	// 与 Python 官方模型一致：按高度 64 等比缩放（Lanczos3 对齐 PIL LANCZOS）。
	width := int(float64(bounds.Dx()) * (64.0 / float64(bounds.Dy())))
	if width <= 0 {
		return "", fmt.Errorf("invalid target width %d", width)
	}
	resized := resize.Resize(uint(width), 64, img, resize.Lanczos3)

	inputName := "input1"
	if names := r.model.InputNames(); len(names) > 0 {
		inputName = names[0]
	}
	input := tensor.New(tensor.WithShape(1, 1, 64, width), tensor.Of(tensor.Float32))
	copy(input.Data().([]float32), preprocess(resized, width))

	outputs, err := r.model.Run(gonnx.Tensors{inputName: input})
	if err != nil {
		return "", fmt.Errorf("run model: %w", err)
	}

	var out tensor.Tensor
	if names := r.model.OutputNames(); len(names) > 0 {
		out = outputs[names[0]]
	}
	if out == nil {
		for _, candidate := range outputs {
			if candidate != nil {
				out = candidate
				break
			}
		}
	}
	if out == nil {
		return "", fmt.Errorf("model returned no output tensor")
	}
	return decode(outputIndices(out)), nil
}

// preprocess 将缩放后的图片转成模型输入用的 float32 序列（按行优先展平）。
//
// 与 Python 官方模型的预处理保持一致：
//  1. 转灰度（PIL 的 'L' 通道，即 ITU-R 601 亮度加权，Go 用 color.GrayModel 等价实现）
//  2. 像素值 /255 归一到 [0,1]
//  3. 再做 (x-0.5)/0.5，映射到 [-1,1]
func preprocess(img image.Image, width int) []float32 {
	bounds := img.Bounds()
	data := make([]float32, 64*width)
	i := 0
	for y := 0; y < 64; y++ {
		for x := 0; x < width; x++ {
			c := color.GrayModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.Gray)
			v := float32(c.Y) / 255.0
			data[i] = (v - 0.5) / 0.5
			i++
		}
	}
	return data
}

// outputIndices 将模型输出张量统一转成 int64 索引序列。
func outputIndices(out tensor.Tensor) []int64 {
	switch data := out.Data().(type) {
	case []int64:
		return data
	case []int32:
		indices := make([]int64, len(data))
		for i, v := range data {
			indices[i] = int64(v)
		}
		return indices
	case []float32:
		indices := make([]int64, len(data))
		for i, v := range data {
			indices[i] = int64(v)
		}
		return indices
	default:
		return nil
	}
}

// decode 对模型输出的类别索引做 CTC 贪心解码：
// 折叠连续重复项、去掉空白符（索引 0），再映射到字符表。
func decode(indices []int64) string {
	var sb []rune
	last := int64(-1)
	for _, item := range indices {
		if item == last {
			continue
		}
		last = item
		if item != 0 && int(item) < len(charset) {
			sb = append(sb, []rune(charset[item])...)
		}
	}
	return string(sb)
}

// httpRecognizer 将图片提交到远程识别服务。
type httpRecognizer struct {
	cfg  config.CaptchaConfig
	http *httputil.Client
}

func (r *httpRecognizer) Recognize(ctx context.Context, imageBytes []byte) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"image": base64.StdEncoding.EncodeToString(imageBytes),
	})
	if err != nil {
		return "", fmt.Errorf("encode captcha request: %w", err)
	}
	resp, err := r.http.Post(ctx, r.cfg.Endpoint, "application/json", payload, nil)
	if err != nil {
		return "", fmt.Errorf("captcha http recognize: %w", err)
	}
	text := parseHTTPResult(resp.Body)
	if text == "" {
		return "", fmt.Errorf("captcha http endpoint returned empty result: %s", strings.TrimSpace(string(resp.Body)))
	}
	return text, nil
}

// parseHTTPResult 兼容多种常见响应格式：
// 纯文本、{"result":"xxxx"}、{"result":{"text":"xxxx"}}、{"data":"xxxx"}。
func parseHTTPResult(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}
	if trimmed[0] != '{' {
		return cleanText(trimmed)
	}
	var generic struct {
		Result json.RawMessage `json:"result"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &generic); err != nil {
		return ""
	}
	for _, raw := range []json.RawMessage{generic.Result, generic.Data} {
		if len(raw) == 0 {
			continue
		}
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			if cleaned := cleanText(text); cleaned != "" {
				return cleaned
			}
		}
		var nested struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &nested); err == nil {
			if cleaned := cleanText(nested.Text); cleaned != "" {
				return cleaned
			}
		}
	}
	return ""
}

// cleanText 去掉空白与常见干扰字符，只保留字母数字与中文。
func cleanText(text string) string {
	var builder strings.Builder
	for _, r := range text {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r >= 0x4e00 && r <= 0x9fff:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// resolveAssetPath 优先使用配置路径；不存在时回退到可执行文件目录下的 ocr/ 同名文件。
func resolveAssetPath(path string) string {
	if path == "" {
		return path
	}
	if fileExists(path) {
		return path
	}
	if exe, err := os.Executable(); err == nil {
		alt := filepath.Join(filepath.Dir(exe), "ocr", filepath.Base(path))
		if fileExists(alt) {
			return alt
		}
	}
	return path
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
