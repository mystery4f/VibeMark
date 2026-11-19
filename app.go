package main

import (
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/dyike/keel/ui/core"
	"github.com/dyike/keel/ui/el"
	"github.com/dyike/keel/ui/kit"
	"github.com/dyike/keel/ui/markdown"
	"github.com/dyike/keel/ui/theme"

	"github.com/mystery4f/vibemark/internal/mddoc"
	"github.com/mystery4f/vibemark/internal/workspace"
)

// appMode is which of the two shapes the window shows.
type appMode int

const (
	modeWorkspace appMode = iota // a directory tree plus the editor
	modeSingle                   // one file, no tree
)

// emptyHint is the zero-data text a workspace shows before a file is open.
const emptyHint = "这个目录里还没有 Markdown 文件"

// appState is what keel persists between runs through KeepState.
type appState struct {
	Dir  string
	Path string
}

// app is the whole window: one editor, one optional tree, one status bar.
type app struct {
	state appState

	mode appMode
	dir  string
	path string

	editor  *kit.CodeEditorView
	decor   *kit.CodeDecorationCollection
	tree    *kit.TreeView
	bar     *kit.StatusBarView
	preview *markdown.Doc

	hasFiles bool // the working directory holds at least one markdown file

	saved       string
	dirty       bool
	previewing  bool
	pending     bool // decorations are stale and must be rebuilt
	saveArmed   bool // a debounce timer is running
	status      string
	statusColor *color.NRGBA

	ready bool
}

// newApp builds the app for one file (path) or one directory (dir). Exactly one
// of them is non-empty when the command line named it.
func newApp(path, dir string) *app {
	a := &app{path: path, dir: dir, bar: kit.StatusBar(), preview: markdown.New("")}
	if path != "" {
		a.mode = modeSingle
	} else {
		a.mode = modeWorkspace
	}
	return a
}

// once loads the document and builds the widgets on the first frame, after keel
// has restored the persisted state.
func (a *app) once() {
	if a.ready {
		return
	}
	a.ready = true

	a.editor = kit.CodeEditor("").
		Language("markdown").
		Fill().
		SoftWrap(true).
		AutoClose(true).
		OnChange(a.changed)
	a.decor = a.editor.Decorations()

	if a.mode == modeWorkspace && a.dir == "" {
		a.dir = a.state.Dir
	}
	if a.mode == modeSingle && a.path == "" {
		a.path = a.state.Path
	}

	switch a.mode {
	case modeSingle:
		a.openFile()
	case modeWorkspace:
		if a.dir != "" {
			a.loadDir()
		}
	}
}

// openFile creates the file when it does not exist yet and loads it. A failure
// leaves the window empty and reports through the status bar.
func (a *app) openFile() {
	name := filepath.Base(a.path)
	if err := workspace.Ensure(a.path); err != nil {
		a.fail(userError("打开", name, err))
		return
	}
	content, err := workspace.Read(a.path)
	if err != nil {
		a.fail(userError("读取", name, err))
		return
	}
	a.load(content)
	a.state.Path = a.path
	a.status = ""
}

// load makes content the saved text and shows it in the editor.
func (a *app) load(content string) {
	a.saved = content
	a.dirty = false
	a.editor.Name(filepath.Base(a.path))
	a.editor.SetValue(content)
	a.editor.SetCursor(0, 0)
	a.applyDecorations(content)
}

// loadDir scans the directory into the tree and opens its first file.
func (a *app) loadDir() {
	roots, err := workspace.Scan(a.dir)
	if err != nil {
		a.fail(userError("扫描目录", a.dir, err))
		return
	}
	a.tree = kit.Tree(treeNodes(roots)...).OnChange(a.open)
	a.hasFiles = len(roots) > 0
	a.state.Dir = a.dir
	if first := firstFile(roots); first != "" {
		a.open(first)
	}
}

// open switches the editor to the file the tree selected.
func (a *app) open(path string) {
	content, err := workspace.Read(path)
	if err != nil {
		a.fail(userError("打开", filepath.Base(path), err))
		return
	}
	a.path = path
	a.load(content)
}

// changed runs on every editor change. Decorations wait for the next frame so a
// burst of keystrokes costs one parse.
func (a *app) changed(text string) {
	a.dirty = text != a.saved
	a.pending = true
	if a.mode == modeWorkspace {
		a.scheduleSave()
	}
}

// scheduleSave writes 300ms after the last keystroke. Working-directory mode has
// no unsaved marker, so the write must follow the typing closely.
func (a *app) scheduleSave() {
	if a.saveArmed {
		return
	}
	a.saveArmed = true
	time.AfterFunc(300*time.Millisecond, func() {
		core.Update(func() {
			a.saveArmed = false
			if a.dirty {
				a.save()
			}
		})
	})
}

// save writes the editor text through a temporary file. A failure keeps the
// dirty flag so the next attempt still has data to write.
func (a *app) save() {
	if a.path == "" {
		return
	}
	if err := workspace.WriteAtomic(a.path, a.editor.Value()); err != nil {
		a.fail(userError("保存", filepath.Base(a.path), err))
		return
	}
	a.saved = a.editor.Value()
	a.dirty = false
	a.status = ""
	a.statusColor = nil
}

// userError turns a filesystem error into text the reader can act on.
func userError(op, name string, err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf("%s %s 失败：文件或目录不存在", op, name)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Sprintf("%s %s 失败：没有访问权限", op, name)
	}
	return fmt.Sprintf("%s %s 失败：%v", op, name, err)
}

// fail records an error message for the status bar.
func (a *app) fail(msg string) {
	pal := theme.Current()
	c := pal.Danger
	a.status = msg
	a.statusColor = &c
}

// applyDecorations rebuilds the decoration set from the current source.
func (a *app) applyDecorations(src string) {
	if a.decor == nil {
		return
	}
	a.decor.Clear()
	a.decor.Append(mddoc.Decorations(src)...)
}

// Render draws the window: tree, editor or preview, then the status bar.
func (a *app) Render(cx *el.Context) el.Element {
	a.once()
	if a.pending {
		a.pending = false
		a.applyDecorations(a.editor.Value())
	}

	pal := theme.Current()
	body := el.Div().Row().Grow().HFull()
	if a.mode == modeWorkspace && !a.previewing && a.tree != nil && a.hasFiles {
		body.Child(el.Div().W(el.Dp(240)).HFull().Border(1, pal.Border).ScrollY().Child(a.tree.Render(cx)))
	}
	switch {
	case a.mode == modeWorkspace && a.path == "":
		body.Child(el.Div().Grow().HFull().P(24).Items(el.Start).Justify(el.Start).
			Child(el.Text(emptyHint).TextColor(pal.Muted)))
	case a.previewing:
		a.preview.SetSource(a.editor.Value())
		body.Child(el.Div().Grow().HFull().ScrollY().P(24).Child(a.preview.Render(cx)))
	default:
		body.Child(el.Div().Grow().HFull().Child(a.editor.Render(cx)))
	}

	bar := a.bar.Left(textView{text: a.title(), color: pal.Muted})
	if state := a.stateText(); state != "" {
		c := pal.Muted
		if a.statusColor != nil {
			c = *a.statusColor
		}
		bar.Right(textView{text: state, color: c})
	}

	root := el.Div().Col().WFull().HFull()
	return root.Child(body, bar.Render(cx))
}

// title is the path shown at the left of the status bar.
func (a *app) title() string {
	if a.path == "" {
		if a.dir != "" {
			return a.dir
		}
		return "未打开文件"
	}
	if a.mode == modeWorkspace && a.dir != "" {
		if rel, err := filepath.Rel(a.dir, a.path); err == nil {
			return rel
		}
	}
	return filepath.Base(a.path)
}

// stateText is the right-hand status: an error, the preview flag, or the save
// state (only single-file mode has unsaved text worth showing).
func (a *app) stateText() string {
	switch {
	case a.status != "":
		return a.status
	case a.previewing:
		return "预览"
	case a.mode == modeSingle && a.dirty:
		return "未保存"
	case a.path != "":
		return "已保存"
	}
	return ""
}

// shortcuts returns the key bindings the window registers.
func (a *app) shortcuts() map[string]func() {
	return map[string]func(){
		"ctrl+b":               func() { a.wrap("**", "**") },
		"ctrl+i":               func() { a.wrap("*", "*") },
		"ctrl+e":               func() { a.wrap("`", "`") },
		"ctrl+k":               func() { a.wrap("[", "](url)") },
		"ctrl+s":               a.save,
		"ctrl+p":               func() { a.previewing = !a.previewing },
		"ctrl+1":               func() { a.prefix("# ") },
		"ctrl+2":               func() { a.prefix("## ") },
		"ctrl+3":               func() { a.prefix("### ") },
		"ctrl+shift+8":         func() { a.prefix("- ") },
		"ctrl+shift+9":         func() { a.prefix("1. ") },
		"ctrl+shift+backslash": func() { a.prefix("> ") },
	}
}

// wrap toggles an inline marker around the selection.
func (a *app) wrap(open, close string) {
	line, col := a.editor.Cursor()
	src := a.editor.Value()
	out, caret := mddoc.WrapSel(src, mddoc.Pos{Line: line, Col: col}, a.editor.Selection(), open, close)
	a.replace(out, caret)
}

// prefix sets the block marker of the caret's line.
func (a *app) prefix(marker string) {
	line, col := a.editor.Cursor()
	out, caret := mddoc.PrefixLine(a.editor.Value(), mddoc.Pos{Line: line, Col: col}, marker)
	a.replace(out, caret)
}

// replace writes the edited source back and puts the caret where the command
// left it.
func (a *app) replace(src string, caret mddoc.Pos) {
	a.editor.SetValue(src)
	a.editor.SetCursor(caret.Line, caret.Col)
	a.changed(src)
	a.pending = true
}

// treeNodes converts the scanned directory into keel tree nodes.
func treeNodes(nodes []*workspace.Node) []*kit.TreeNode {
	out := make([]*kit.TreeNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, &kit.TreeNode{ID: n.ID, Label: n.Label, Children: treeNodes(n.Children)})
	}
	return out
}

// firstFile returns the first markdown file in the tree, depth first.
func firstFile(nodes []*workspace.Node) string {
	for _, n := range nodes {
		if !n.Dir {
			return n.Path
		}
		if p := firstFile(n.Children); p != "" {
			return p
		}
	}
	return ""
}

// textView is the status bar's small text item. kit.StatusBarView.Left and
// Right take views rather than strings, so the label keeps its own token color
// and never collapses into the overflow menu.
type textView struct {
	text  string
	color color.NRGBA
}

func (t textView) Render(cx *el.Context) el.Element {
	return el.Text(t.text).TextSize(12).TextColor(t.color)
}
