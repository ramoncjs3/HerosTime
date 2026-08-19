package captcha

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/advancedclimatesystems/gonnx"

	"oldbeggar-refactor/internal/config"
)

// modelPath 返回本地 ddddocr common.onnx 的路径，不存在时跳过。
func modelPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "ocr", "common.onnx")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("model not available at %s", path)
	}
	return path
}

// TestCaptchaModelOpsProbe 列出模型使用的算子与 opset，验证 gonnx 兼容性。
func TestCaptchaModelOpsProbe(t *testing.T) {
	raw, err := os.ReadFile(modelPath(t))
	if err != nil {
		t.Fatalf("read model: %v", err)
	}
	mp, err := gonnx.ModelProtoFromBytes(raw)
	if err != nil {
		t.Fatalf("parse model proto: %v", err)
	}
	for _, imp := range mp.GetOpsetImport() {
		t.Logf("opset domain=%q version=%d", imp.GetDomain(), imp.GetVersion())
	}

	ops := map[string]int{}
	for _, node := range mp.Graph.GetNode() {
		ops[node.GetOpType()]++
	}
	names := make([]string, 0, len(ops))
	for name := range ops {
		names = append(names, name)
	}
	sort.Strings(names)
	t.Logf("op types (%d): %s", len(names), strings.Join(names, ", "))
	for _, name := range names {
		t.Logf("  %-20s x%d", name, ops[name])
	}

	t.Logf("graph inputs: %v", mp.Graph.InputNames())
	for name, shape := range mp.Graph.InputShapes() {
		t.Logf("  input %s shape: %v", name, shape)
	}
	t.Logf("graph outputs: %v", mp.Graph.OutputNames())
	for name, shape := range mp.Graph.OutputShapes() {
		t.Logf("  output %s shape: %v", name, shape)
	}
}

// TestCaptchaModelGraphProbe 打印全部节点与属性，用于适配 gonnx。
func TestCaptchaModelGraphProbe(t *testing.T) {
	raw, err := os.ReadFile(modelPath(t))
	if err != nil {
		t.Fatalf("read model: %v", err)
	}
	mp, err := gonnx.ModelProtoFromBytes(raw)
	if err != nil {
		t.Fatalf("parse model proto: %v", err)
	}
	for i, node := range mp.Graph.GetNode() {
		attrs := make([]string, 0)
		for _, attr := range node.GetAttribute() {
			attrs = append(attrs, attr.GetName())
		}
		t.Logf("%3d %-18s in=%v out=%v attrs=%v",
			i, node.GetOpType(), node.GetInput(), node.GetOutput(), attrs)
	}
}

// TestCaptchaModelRunProbe 实际加载并运行模型，验证 gonnx 支持全部算子。
func TestCaptchaModelRunProbe(t *testing.T) {
	model, err := gonnx.NewModelFromFile(modelPath(t))
	if err != nil {
		t.Fatalf("load model with gonnx: %v", err)
	}
	t.Logf("input names: %v", model.InputNames())
	t.Logf("output names: %v", model.OutputNames())
}

// TestCaptchaInferProbe 端到端推理一张合成图片，验证全流程算子正确运行。
func TestCaptchaInferProbe(t *testing.T) {
	enabled := true
	rec, err := New(config.CaptchaConfig{Enabled: &enabled, Engine: "onnx", ModelFile: modelPath(t)}, nil)
	if err != nil {
		t.Fatalf("new recognizer: %v", err)
	}

	// 合成一张带明暗条纹的 120x40 图片，编码为 PNG。
	img := image.NewRGBA(image.Rect(0, 0, 120, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 120; x++ {
			v := uint8((x*7 + y*13) % 256)
			img.Set(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	start := time.Now()
	text, err := rec.Recognize(context.Background(), buf.Bytes())
	elapsed := time.Since(start)
	if err != nil {
		// 噪声图解码为空文本属于预期，只要推理链路跑通即可。
		if strings.Contains(err.Error(), "empty text") {
			t.Logf("inference ran ok in %s, decoded to empty text (expected for noise image)", elapsed)
			return
		}
		t.Fatalf("recognize: %v", err)
	}
	t.Logf("inference took %s, result=%q", elapsed, text)
}
