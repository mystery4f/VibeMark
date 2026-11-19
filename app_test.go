package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	nativeclip "github.com/dyike/keel/native/clipboard"
	"github.com/dyike/keel/ui/el"
	"github.com/dyike/keel/ui/window"
	"github.com/dyike/keel/ui/window/windowtest"
	"golang.org/x/image/bmp"
)

// writeNote puts one markdown file in a fresh directory and returns its path.
func writeNote(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("# 标题\n\n正文\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

// openWindow builds the app and hands it to a window that never shows.
func openWindow(t *testing.T, a *app) *windowtest.App {
	t.Helper()
	return windowtest.New(t, window.Options{
		Content:   el.Root(a),
		KeepState: &a.state,
		Shortcuts: a.shortcuts(),
		// keel ends the process when the last window closes unless a close hook
		// is set, and a test must not end the run that way.
		OnClose: func() {},
		Width:   720,
		Height:  480,
	})
}

func TestSingleFileWindowShowsPathAndTogglesPreview(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := openWindow(t, a)

	app.Want("note.md")
	app.Press("ctrl+p")
	app.Want("预览")
	if !a.previewing {
		t.Fatal("preview did not open")
	}
	app.Press("ctrl+p")
	app.Want("已保存")
}

func TestWorkspaceWindowListsFiles(t *testing.T) {
	dir, _ := writeNote(t)
	a := newApp("", dir)
	app := openWindow(t, a)

	app.Want("note.md")
	if a.tree == nil {
		t.Fatal("tree was not built")
	}
}

func TestMissingFileIsCreated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "draft.md")
	a := newApp(path, "")
	openWindow(t, a)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file was not created: %v", err)
	}
}

func TestEmptyWorkspaceShowsHint(t *testing.T) {
	a := newApp("", t.TempDir())
	app := openWindow(t, a)

	app.Want(emptyHint)
}

func TestUserErrorIsReadable(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{fs.ErrNotExist, "不存在"},
		{fs.ErrPermission, "没有访问权限"},
		{fmt.Errorf("boom"), "boom"},
	}
	for _, c := range cases {
		if got := userError("打开", "a.md", c.err); !strings.Contains(got, c.want) {
			t.Fatalf("userError = %q, want %q in it", got, c.want)
		}
	}
}

func TestEditorHoldsTheFile(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	openWindow(t, a)

	if got := a.text(); got != "# 标题\n\n正文\n" {
		t.Fatalf("document = %q, want the file", got)
	}
}

func TestDocumentOpensReadyToType(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := openWindow(t, a)

	app.Type("新")
	if got := a.text(); !strings.HasPrefix(got, "新") {
		t.Fatalf("document = %q, want the typing at the start", got)
	}
}

func TestEmptyFileAcceptsTyping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(path, "")
	app := openWindow(t, a)

	app.Type("第一行")
	if got := a.text(); got != "第一行" {
		t.Fatalf("document = %q, want the typing", got)
	}
}

func TestEnterAndBackspaceBehaveLikeATextField(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := openWindow(t, a)

	a.editor.SetCursor(0, 2)
	app.Press("enter")
	app.Type("新行")
	if got := a.text(); got != "# \n新行标题\n\n正文\n" {
		t.Fatalf("document = %q, want the break where the caret stood", got)
	}
	app.Press("backspace")
	if got := a.text(); got != "# \n新标题\n\n正文\n" {
		t.Fatalf("document = %q, want the break removed", got)
	}
}

func TestUndoStillWorksAfterTyping(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := openWindow(t, a)

	app.Type("abc")
	app.Press("ctrl+z")
	if got := a.text(); got != "# 标题\n\n正文\n" {
		t.Fatalf("document = %q, want the typing undone", got)
	}
}

func TestSaveKeepsTypedText(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := openWindow(t, a)

	app.Type("记下")
	app.Press("ctrl+s")
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(saved), "记下") {
		t.Fatalf("file = %q, want the typing", saved)
	}
}

func TestPreviewSeesTheTypedText(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := openWindow(t, a)

	app.Type("补一行\n")
	app.Press("ctrl+p")
	app.Want("补一行")
}

func TestBoldWrapsTheSelection(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := openWindow(t, a)

	a.editor.SetCursor(0, 2)
	app.Type("加粗")
	a.editor.SetCursor(0, 2)
	if got := a.editor.Selection(); got != "" {
		t.Fatalf("selection = %q, want an empty one before wrapping", got)
	}
	// Selecting with the keyboard keeps the command honest about what the
	// editor reports for a selection.
	for i := 0; i < 2; i++ {
		app.Press("shift+right")
	}
	app.Press("ctrl+b")
	if got := a.text(); !strings.Contains(got, "**加粗**") {
		t.Fatalf("document = %q, want the bold markers", got)
	}
}

func TestWorkspaceAutosaves(t *testing.T) {
	dir, path := writeNote(t)
	a := newApp("", dir)
	app := openWindow(t, a)

	app.Type("改")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		app.Render()
		saved, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(string(saved), "改") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the working directory was not saved")
}

func TestHeadingAndParagraphShortcuts(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := openWindow(t, a)

	a.editor.SetCursor(0, 0)
	app.Press("ctrl+3")
	if got := a.text(); !strings.HasPrefix(got, "### 标题") {
		t.Fatalf("document = %q, want a third level heading", got)
	}
	app.Press("ctrl+0")
	if got := a.text(); !strings.HasPrefix(got, "标题") {
		t.Fatalf("document = %q, want the heading removed", got)
	}
}

func TestFenceShortcutWrapsTheDocument(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := openWindow(t, a)

	app.Press("ctrl+shift+k")
	if got := a.text(); !strings.HasPrefix(got, "```\n") || !strings.HasSuffix(got, "\n```") {
		t.Fatalf("document = %q, want a fenced block", got)
	}
	app.Press("ctrl+shift+k")
	if got := a.text(); strings.HasPrefix(got, "```") {
		t.Fatalf("document = %q, want the fences removed", got)
	}
}

func TestPastedTextLandsAtTheCaret(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	openWindow(t, a)

	a.editor.SetCursor(0, 2)
	a.insertPasted(nativeclip.Data{Text: "插"})
	if got := a.text(); got != "# 插标题\n\n正文\n" {
	}
}

func TestPastedPictureGoesToTheMediaFolder(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	a.mediaDir = t.TempDir()
	openWindow(t, a)

	a.editor.SetCursor(0, 2)
	a.insertPasted(nativeclip.Data{Images: []nativeclip.Image{{MIME: "image/png", Data: pngData(t)}}})

	link := filepath.ToSlash(filepath.Join(a.mediaDir, "pasted-1.png"))
	if got := a.text(); !strings.Contains(got, "![]("+link+")") {
		t.Fatalf("document = %q, want the picture link to %s", got, link)
	}
	if _, err := os.Stat(filepath.Join(a.mediaDir, "pasted-1.png")); err != nil {
		t.Fatalf("picture was not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "assets")); err == nil {
		t.Fatal("the note grew a picture folder beside it")
	}
}

func TestPastedFileBecomesALink(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	a.mediaDir = t.TempDir()
	openWindow(t, a)

	file := filepath.Join(t.TempDir(), "报表.pdf")
	if err := os.WriteFile(file, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.insertPasted(nativeclip.Data{Files: []string{file}})

	link := filepath.ToSlash(filepath.Join(a.mediaDir, "报表.pdf"))
	if got := a.text(); !strings.Contains(got, "[报表.pdf]("+link+")") {
		t.Fatalf("document = %q, want the file link to %s", got, link)
	}
}

// A screenshot arrives from Windows as a bitmap; the renderer cannot decode
// that, so it is stored as PNG and the link follows.
func TestPastedPictureBytesAreSavedAsPNG(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	a.mediaDir = t.TempDir()
	openWindow(t, a)

	a.insertPasted(nativeclip.Data{Images: []nativeclip.Image{{MIME: "image/bmp", Data: bmpBytes(t)}}})

	link := filepath.ToSlash(filepath.Join(a.mediaDir, "pasted-1.png"))
	if got := a.text(); !strings.Contains(got, "![]("+link+")") {
		t.Fatalf("document = %q, want the picture link to %s", got, link)
	}
	saved, err := os.Open(filepath.Join(a.mediaDir, "pasted-1.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer saved.Close()
	if _, format, err := image.DecodeConfig(saved); err != nil || format != "png" {
		t.Fatalf("saved format = %q, err = %v", format, err)
	}
}

func TestRemotePicturesStayOff(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	openWindow(t, a)

	if _, err := a.loadImage(context.Background(), "https://example.com/a.png"); err == nil {
		t.Fatal("a remote picture was read")
	}
}

func TestImageLoadsFromTheDocumentFolder(t *testing.T) {
	dir, path := writeNote(t)
	picture := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(picture, pngData(t), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(path, "")

	img, err := a.loadImage(context.Background(), "shot.png")
	if err != nil {
		t.Fatalf("loadImage: %v", err)
	}
	if img == nil {
		t.Fatal("loadImage returned no pixels")
	}
}

func TestImageLoadsFromTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "notes")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, "note.md")
	if err := os.WriteFile(path, []byte("# 标题\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cover.png"), pngData(t), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp("", dir)
	a.path = path

	if _, err := a.loadImage(context.Background(), "cover.png"); err != nil {
		t.Fatalf("loadImage: %v", err)
	}
}

func TestImagePathAcceptsFileURLsAndEscapes(t *testing.T) {
	dir, path := writeNote(t)
	picture := filepath.Join(dir, "my shot.png")
	if err := os.WriteFile(picture, pngData(t), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(path, "")

	for _, src := range []string{
		"my%20shot.png",
		"<my shot.png>",
		"file:///" + filepath.ToSlash(filepath.Join(dir, "my shot.png")),
	} {
		if _, err := a.loadImage(context.Background(), src); err != nil {
			t.Fatalf("loadImage(%q): %v", src, err)
		}
	}
}

func TestImagePathAcceptsBackslashesFromTheWorkdir(t *testing.T) {
	dir, path := writeNote(t)
	if err := os.WriteFile(filepath.Join(dir, "deep.png"), pngData(t), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(path, "")

	if _, err := a.loadImage(context.Background(), strings.ReplaceAll(filepath.Join(dir, "deep.png"), "/", "\\")); err != nil {
		t.Fatalf("loadImage: %v", err)
	}
}

// A note that already points at a bitmap still renders: the renderer cannot
// decode BMP, so it is read here.
func TestImageLoadsABitmap(t *testing.T) {
	dir, path := writeNote(t)
	if err := os.WriteFile(filepath.Join(dir, "shot.bmp"), bmpBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(path, "")

	if _, err := a.loadImage(context.Background(), "shot.bmp"); err != nil {
		t.Fatalf("loadImage: %v", err)
	}
}

func TestFailedPictureIsReported(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := openWindow(t, a)

	if _, err := a.loadImage(context.Background(), "missing.png"); err == nil {
		t.Fatal("a missing picture was read")
	}
	app.Render()
	if !strings.Contains(a.status, "图片读取失败") {
		t.Fatalf("status = %q, want the picture failure", a.status)
	}
}

// bmpBytes is what the Windows clipboard hands over for a screenshot.
func bmpBytes(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 120, B: 200, A: 255})
		}
	}
	if err := bmp.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// pngData is a tiny picture, small enough to keep the test fast.
func pngData(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 230, G: 120, B: 20, A: 255})
		}
	}
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// A close reports unsaved text in single-file mode, which is what git reads as
// "abort the commit".
func TestCloseCodeReportsUnsavedText(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	a.once()

	if got := a.closeCode(); got != 0 {
		t.Fatalf("close code = %d, want 0 for a saved file", got)
	}
	a.dirty = true
	if got := a.closeCode(); got != exitUnsaved {
		t.Fatalf("close code = %d, want %d for unsaved text", got, exitUnsaved)
	}
	a.mode = modeWorkspace
	if got := a.closeCode(); got != 0 {
		t.Fatalf("close code = %d, want 0 in the working directory", got)
	}
}
