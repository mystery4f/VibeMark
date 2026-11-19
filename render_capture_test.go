package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dyike/keel/ui/el"
	"github.com/dyike/keel/ui/theme"
	"github.com/dyike/keel/ui/window"
	"github.com/dyike/keel/ui/window/windowtest"
)

// TestCaptureRenders writes PNG renders of both modes for design review. It only
// runs when VIBEMARK_RENDERS names the output directory, so the ordinary test
// run stays free of file writes.
func TestCaptureRenders(t *testing.T) {
	out := os.Getenv("VIBEMARK_RENDERS")
	if out == "" {
		t.Skip("set VIBEMARK_RENDERS to a directory to capture renders")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	notes := filepath.Join(t.TempDir(), "note.md")
	write(t, notes, "# 标题\n\n正文里 **粗体** 与 *斜体*，还有 `代码`。\n\n- [x] 已完成\n- [ ] 待办\n\n> 引用\n\n```go\nfunc main() {}\n```\n")

	cases := []struct {
		name   string
		mode   *string
		dir    *string
		width  int
		height int
	}{
		{"single-file-1024x768", &notes, nil, 1024, 768},
		{"single-file-720x480", &notes, nil, 720, 480},
		{"workspace-1024x768", nil, dirOf(notes), 1024, 768},
		{"workspace-empty-1024x768", nil, emptyDir(t), 1024, 768},
		{"single-file-dark-1024x768", &notes, nil, 1024, 768},
	}
	for i, c := range cases {
		if c.name == "single-file-dark-1024x768" {
			theme.Apply(theme.Dark())
		} else if i == 0 {
			theme.Apply(theme.Light())
		}
		a := newApp(pathOr(c.mode), dirOr(c.dir))
		app := windowtest.New(t, window.Options{
			Title:     "VibeMark",
			Width:     c.width,
			Height:    c.height,
			Content:   el.Root(a),
			KeepState: &a.state,
			Shortcuts: a.shortcuts(),
		})
		app.Resize(c.width, c.height)
		app.Render()
		png := app.Screenshot()
		if len(png) == 0 {
			t.Fatalf("%s: empty screenshot", c.name)
		}
		if err := os.WriteFile(filepath.Join(out, c.name+".png"), png, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func pathOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func dirOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func emptyDir(t *testing.T) *string {
	d := t.TempDir()
	return &d
}

func dirOf(path string) *string {
	d := filepath.Dir(path)
	return &d
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCaptureActiveBlock renders the one state a screenshot can review: a block
// open as its own source with the rest of the document still laid out.
func TestCaptureActiveBlock(t *testing.T) {
	out := os.Getenv("VIBEMARK_RENDERS")
	if out == "" {
		t.Skip("set VIBEMARK_RENDERS to a directory to capture renders")
	}
	notes := filepath.Join(t.TempDir(), "note.md")
	write(t, notes, "# 标题\n\n正文里 **粗体** 与 *斜体*，还有 `代码`。\n\n- [x] 已完成\n- [ ] 待办\n\n> 引用\n")

	a := newApp(notes, "")
	app := windowtest.New(t, window.Options{
		Title:     "VibeMark",
		Width:     1024,
		Height:    768,
		Content:   el.Root(a),
		KeepState: &a.state,
		Shortcuts: a.shortcuts(),
	})
	app.Render()
	a.activate(1)
	app.Render()
	png := app.Screenshot()
	if len(png) == 0 {
		t.Fatal("empty screenshot")
	}
	if err := os.WriteFile(filepath.Join(out, "single-file-active-block-1024x768.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCapturePicture writes a render of a note that embeds a local picture, so
// the image path can be checked without a real window.
func TestCapturePicture(t *testing.T) {
	out := os.Getenv("VIBEMARK_RENDERS")
	if out == "" {
		t.Skip("set VIBEMARK_RENDERS to a directory to capture renders")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "assets", "dot.png"), pngBytes(t))
	notes := filepath.Join(dir, "note.md")
	write(t, notes, "# 图\n\n![dot](assets/dot.png)\n\n后面一段\n")

	a := newApp(notes, "")
	app := windowtest.New(t, window.Options{Width: 640, Height: 480, Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})
	app.Render()
	if err := os.WriteFile(filepath.Join(out, "picture-640x480.png"), app.Screenshot(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pngBytes is a small solid picture, enough to prove the renderer decodes it.
func pngBytes(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 0x2f, G: 0x6f, B: 0xed, A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
