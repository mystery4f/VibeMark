package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dyike/keel/ui/el"
	"github.com/dyike/keel/ui/window"
	"github.com/dyike/keel/ui/window/windowtest"
)

// benchDoc builds a document of n paragraphs mixing the markup the renderer
// has to handle, so a benchmark measures real work.
func benchDoc(n int) string {
	var b strings.Builder
	b.WriteString("# 标题\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "第 %d 段有 **粗体** 与 *斜体* 还有 `代码` 和 [链接](x.md)。\n二行内容。\n\n", i)
		if i%5 == 0 {
			b.WriteString("- 列表项一\n- 列表项二\n\n> 引用一行\n\n```go\nfmt.Println(1)\n```\n\n")
		}
	}
	return b.String()
}

// benchList builds one block of n list items: a checklist or a long code block
// has that shape, and it is the case the editor's height bound exists for.
func benchList(n int) string {
	var b strings.Builder
	b.WriteString("# 标题\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "- 第 %d 项内容\n", i)
	}
	b.WriteString("\n正文。\n")
	return b.String()
}

func benchApp(tb testing.TB, name, doc string) (*app, *windowtest.App) {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), name)
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		tb.Fatal(err)
	}
	a := newApp(path, "")
	app := windowtest.New(tb, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})
	app.Render()
	return a, app
}

// BenchmarkTypeKeystroke is the latency one typed character costs: the editor
// runs the change handler and the window draws the frame that follows.
func BenchmarkTypeKeystroke(b *testing.B) {
	a, app := benchApp(b, "many.md", benchDoc(200))
	a.editor.SetCursor(0, 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Type("x")
	}
	b.ReportAllocs()
}

// BenchmarkFrameManyBlocks draws a page of many short blocks.
func BenchmarkFrameManyBlocks(b *testing.B) {
	_, app := benchApp(b, "many.md", benchDoc(200))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Render()
	}
	b.ReportAllocs()
}

// BenchmarkFrameLongBlock draws one block far longer than the editor box, the
// case where an unbounded editor measured tens of milliseconds per frame.
func BenchmarkFrameLongBlock(b *testing.B) {
	for _, n := range []int{200, 500, 900} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			a, app := benchApp(b, "list.md", benchList(n))
			a.activate(1)
			app.Render()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				app.Render()
			}
			b.ReportAllocs()
		})
	}
}

// BenchmarkFrameSourceMode draws the whole document in one editor buffer.
func BenchmarkFrameSourceMode(b *testing.B) {
	a, app := benchApp(b, "many.md", benchDoc(200))
	a.toggleSource()
	app.Render()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Render()
	}
	b.ReportAllocs()
}

// BenchmarkFramePreview draws the preview of the whole document.
func BenchmarkFramePreview(b *testing.B) {
	a, app := benchApp(b, "many.md", benchDoc(200))
	a.previewing = true
	a.previewStale = true
	app.Render()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Render()
	}
	b.ReportAllocs()
}
