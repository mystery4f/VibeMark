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

	"github.com/mystery4f/vibemark/internal/mddoc"
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
	a, app := benchApp(b, "one", benchDoc(20))
	_ = a
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Type("x")
	}
}

func BenchmarkFrameDocument(b *testing.B) {
	for _, lines := range []int{200, 2000, 20000} {
		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			_, app := benchApp(b, "doc", benchDoc(lines))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				app.Render()
			}
		})
	}
}

func BenchmarkFramePreview(b *testing.B) {
	a, app := benchApp(b, "doc", benchDoc(200))
	a.previewing = true
	a.previewStale = true
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.Render()
	}
}

func BenchmarkDecorations(b *testing.B) {
	doc := benchDoc(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mddoc.Decorations(doc)
	}
}
