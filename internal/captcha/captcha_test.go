package captcha

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/nfnt/resize"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"oldbeggar-refactor/internal/config"
)

func TestCleanText(t *testing.T) {
	cases := map[string]string{
		"ab3d":       "ab3d",
		" 8 2 4 6 ":  "8246",
		"你好, world!": "你好world",
		"\n\t\r":     "",
		"O0lI1":      "O0lI1",
	}
	for input, want := range cases {
		if got := cleanText(input); got != want {
			t.Errorf("cleanText(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDecodeCTC(t *testing.T) {
	// 索引 0 为 blank；连续重复合并；越界索引忽略。
	indices := []int64{0, 1, 1, 1, 0, 2, 0, 0, 3, 3, 0, 2, int64(len(charset) + 5)}
	got := decode(indices)
	want := charset[1] + charset[2] + charset[3] + charset[2]
	if got != want {
		t.Errorf("decode = %q, want %q", got, want)
	}
}

func TestParseHTTPResult(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`8246`, "8246"},
		{`{"result":"ab3d"}`, "ab3d"},
		{`{"result":{"text":" x9y "}}`, "x9y"},
		{`{"data":"7788"}`, "7788"},
		{`{"data":{"text":"ok1"}}`, "ok1"},
		{`{"foo":"bar"}`, ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := parseHTTPResult([]byte(c.body)); got != c.want {
			t.Errorf("parseHTTPResult(%q) = %q, want %q", c.body, got, c.want)
		}
	}
}

// renderCaptchaText 用基础字体渲染一张白底黑字的验证码风格图片。
// 先以原始分辨率绘制，再最近邻放大，使文字占据大部分画面。
func renderCaptchaText(t *testing.T, text string) []byte {
	t.Helper()

	const scale = 5

	small := image.NewRGBA(image.Rect(0, 0, len(text)*7+2, 13+2))
	for y := 0; y < small.Bounds().Dy(); y++ {
		for x := 0; x < small.Bounds().Dx(); x++ {
			small.Set(x, y, color.White)
		}
	}

	drawer := &font.Drawer{
		Dst:  small,
		Src:  image.Black,
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(1), Y: fixed.I(1 + 10)},
	}
	drawer.DrawString(text)

	big := resize.Resize(scale*uint(small.Bounds().Dx()), scale*uint(small.Bounds().Dy()), small, resize.NearestNeighbor)

	var buf bytes.Buffer
	if err := png.Encode(&buf, big); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// TestOnnxRecognizerRenderedText 用本地模型识别渲染出的文字图片。
// 模型文件缺失时跳过（资产分发方式见 README “验证码识别”）。
func TestOnnxRecognizerRenderedText(t *testing.T) {
	enabled := true
	rec, err := New(config.CaptchaConfig{Enabled: &enabled, Engine: "onnx", ModelFile: modelPath(t)}, nil)
	if err != nil {
		t.Fatalf("new recognizer: %v", err)
	}

	text, err := rec.Recognize(context.Background(), renderCaptchaText(t, "8246"))
	if err != nil {
		t.Fatalf("recognize: %v", err)
	}
	if text != "8246" {
		t.Errorf("recognize rendered text = %q, want %q", text, "8246")
	}
}
