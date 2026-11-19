package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	nativeclip "github.com/dyike/keel/native/clipboard"
	"github.com/dyike/keel/ui/core"
	"github.com/dyike/keel/ui/el"
	"github.com/dyike/keel/ui/kit"
	"github.com/dyike/keel/ui/markdown"
	"github.com/dyike/keel/ui/theme"
	"github.com/mystery4f/vibemark/internal/mddoc"
	"github.com/mystery4f/vibemark/internal/media"
	"github.com/mystery4f/vibemark/internal/workspace"
)

type appMode int

const (
	modeWorkspace appMode = iota // a directory tree plus the editor
	modeSingle                   // one file, no tree
)

const (
	// emptyHint is the zero-data text a workspace shows before a file is open.
	emptyHint = "这个目录里还没有 Markdown 文件"
	// saveDelay is how long typing rests before a working directory writes.
	saveDelay = 300 * time.Millisecond
)

// appState is what keel persists between runs through KeepState.
type appState struct {
	Dir  string
	Path string
}

// app is the whole window: the document in one editor, an optional tree, and a
// status bar. The editor holds the markdown source and decorations style it in
// place, so what the reader types is what the file says.
type app struct {
	state appState

	mode     appMode
	dir      string
	path     string
	mediaDir string // where a pasted picture is written, never beside the note

	editor       *kit.CodeEditorView
	decor        *kit.CodeDecorationCollection
	tree         *kit.TreeView
	bar          *kit.StatusBarView
	preview      *markdown.Doc
	previewStale bool // the preview has to parse the document again

	hasFiles bool // the working directory holds at least one markdown file

	saved       string
	dirty       bool
	previewing  bool
	pending     bool        // decorations are stale and must be rebuilt
	saveArmed   atomic.Bool // a debounce timer is running
	saveDue     atomic.Bool // the debounce elapsed; the write runs in the next frame
	status      string
	statusColor *color.NRGBA
	treeHidden  bool // the sidebar is folded away

	ready bool
}

// newApp builds the app for one file (path) or one directory (dir). Exactly one
// of them is non-empty when the command line named it.
func newApp(path, dir string) *app {
	a := &app{path: path, dir: dir, bar: kit.StatusBar(), mediaDir: media.Root()}
	if path != "" {
		a.mode = modeSingle
	} else {
		a.mode = modeWorkspace
	}
	return a
}

func (a *app) once() {
	if a.ready {
		return
	}
	a.ready = true
	a.preview = a.newDoc("")

	a.editor = kit.CodeEditor("").
		Language("markdown").
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

// load makes content the saved text of the window.
func (a *app) load(content string) {
	a.saved = content
	a.dirty = false
	a.editor.Name(filepath.Base(a.path))
	a.editor.SetValue(content)
	a.editor.SetCursor(0, 0)
	a.pending = true
	a.previewStale = true
	// A file opens ready to edit: git hands the editor a file and expects the
	// next keystroke to land in it.
	a.editor.Focus()
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

// open switches the window to the file the tree selected, saving pending edits
// of the current one first.
func (a *app) open(path string) {
	a.save()
	content, err := workspace.Read(path)
	if err != nil {
		a.fail(userError("打开", filepath.Base(path), err))
		return
	}
	a.path = path
	a.load(content)
}

// text is the document as it stands, which is exactly what the editor holds.
func (a *app) text() string {
	return a.editor.Value()
}

// changed records an edit: the dirty flag, the decoration rebuild that the next
// frame does, and the debounce that writes the file.
func (a *app) changed(value string) {
	a.dirty = value != a.saved
	a.pending = true
	a.previewStale = true
	if a.mode == modeWorkspace {
		a.scheduleSave()
	}
}

func (a *app) scheduleSave() {
	if a.mode != modeWorkspace || !a.saveArmed.CompareAndSwap(false, true) {
		return
	}
	time.AfterFunc(saveDelay, func() {
		// The write itself runs in a frame, where the editor buffer belongs to
		// this goroutine; asking for one only needs an invalidation.
		a.saveArmed.Store(false)
		a.saveDue.Store(true)
		core.Update(func() {})
	})
}

// save writes the document through a temporary file. A failure keeps the dirty
// flag so the next attempt still has data to write.
func (a *app) save() {
	if a.path == "" {
		return
	}
	full := a.text()
	a.dirty = full != a.saved
	if !a.dirty {
		a.status = ""
		a.statusColor = nil
		return
	}
	if err := workspace.WriteAtomic(a.path, full); err != nil {
		a.fail(userError("保存", filepath.Base(a.path), err))
		return
	}
	a.saved = full
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

// applyDecorations rebuilds the styling from the document text.
func (a *app) applyDecorations(src string) {
	if a.decor == nil {
		return
	}
	a.decor.Clear()
	a.decor.Append(mddoc.Decorations(src)...)
}

// Render draws the window: tree, document, then the status bar.
func (a *app) Render(cx *el.Context) el.Element {
	a.once()
	// A save that came due while the window was idle runs here, in a frame.
	if a.saveDue.Swap(false) && a.dirty {
		a.save()
	}
	if a.pending {
		a.pending = false
		a.applyDecorations(a.text())
	}

	pal := theme.Current()
	body := el.Div().Row().Grow().HFull()
	if a.mode == modeWorkspace && !a.previewing && !a.treeHidden && a.tree != nil && a.hasFiles {
		body.Child(el.Div().W(el.Dp(240)).HFull().Border(1, pal.Border).ScrollY().Child(a.tree.Render(cx)))
	}
	switch {
	case a.mode == modeWorkspace && a.path == "":
		body.Child(el.Div().Grow().HFull().P(24).Items(el.Start).Justify(el.Start).
			Child(el.Text(emptyHint).TextColor(pal.Muted)))
	case a.previewing:
		// Parsing the document once per change instead of once per frame keeps
		// a long file from re-parsing while the preview just sits there.
		if a.previewStale {
			a.previewStale = false
			a.preview.SetSource(a.text())
		}
		body.Child(el.Div().Grow().HFull().ScrollY().P(24).Child(a.preview.Render(cx)))
	default:
		left := float32(0)
		if a.mode == modeWorkspace && a.path == "" {
			left = 24
		}
		body.Child(el.Div().Grow().HFull().Px(24 + left).Py(12).
			Child(el.Div().WFull().Grow().HFull().Child(a.editor.Fill().Render(cx))))
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
	// The chords follow Typora's defaults, so muscle memory carries over. Enter,
	// Shift+Enter, Backspace and Delete are left to the editor, which is what
	// makes them behave the way they do in any other text field.
	return map[string]func(){
		"ctrl+b":       func() { a.wrap("**", "**") },
		"ctrl+i":       func() { a.wrap("*", "*") },
		"ctrl+shift+`": func() { a.wrap("`", "`") },
		"alt+shift+5":  func() { a.wrap("~~", "~~") },
		"ctrl+k":       func() { a.wrap("[", "](url)") },
		"ctrl+v":       a.paste,
		"ctrl+s":       a.save,
		"ctrl+1":       func() { a.prefix("# ") },
		"ctrl+2":       func() { a.prefix("## ") },
		"ctrl+3":       func() { a.prefix("### ") },
		"ctrl+4":       func() { a.prefix("#### ") },
		"ctrl+5":       func() { a.prefix("##### ") },
		"ctrl+6":       func() { a.prefix("###### ") },
		"ctrl+0":       func() { a.prefix("") },
		"ctrl+shift+k": a.codeFence,
		"ctrl+shift+q": func() { a.prefix("> ") },
		"ctrl+shift+[": func() { a.prefix("1. ") },
		"ctrl+shift+]": func() { a.prefix("- ") },
		"ctrl+p":       a.togglePreview,
		"ctrl+shift+l": a.toggleTree,
	}
}

// wrap toggles an inline marker around the selection.
func (a *app) wrap(open, close string) {
	if a.previewing {
		return
	}
	out, caret := mddoc.WrapSel(a.editor.Value(), a.caret(), a.editor.Selection(), open, close)
	a.replace(out, caret)
}

// prefix sets the line marker of the caret's line.
func (a *app) prefix(marker string) {
	if a.previewing {
		return
	}
	out, caret := mddoc.PrefixLine(a.editor.Value(), a.caret(), marker)
	a.replace(out, caret)
}

// togglePreview shows the whole document rendered and comes back.
func (a *app) togglePreview() {
	a.save()
	a.previewing = !a.previewing
	if a.previewing {
		a.previewStale = true
	}
}

// toggleTree folds the sidebar away in the working-directory mode.
func (a *app) toggleTree() {
	a.treeHidden = !a.treeHidden
}

// codeFence turns the document into a fenced code block, and takes the fences
// off again when it already is one.
func (a *app) codeFence() {
	if a.previewing {
		return
	}
	lines := strings.Split(a.editor.Value(), "\n")
	if strings.HasPrefix(strings.TrimLeft(lines[0], " \t"), "```") {
		lines = lines[1:]
		if n := len(lines); n > 0 && strings.HasPrefix(strings.TrimLeft(lines[n-1], " \t"), "```") {
			lines = lines[:n-1]
		}
		a.replace(strings.Join(lines, "\n"), mddoc.Pos{Line: 0, Col: 0})
		return
	}
	body := strings.Join(lines, "\n")
	a.replace("```\n"+body+"\n```", mddoc.Pos{Line: 1, Col: 0})
}

// caret is where the reader is, in the rune coordinates mddoc works in.
func (a *app) caret() mddoc.Pos {
	line, col := a.editor.Cursor()
	return mddoc.Pos{Line: line, Col: col}
}

// replace writes an edited document back and puts the caret where the command
// left it. keel offers no way to write into the buffer, so the undo history
// starts again after a command.
func (a *app) replace(src string, caret mddoc.Pos) {
	a.editor.SetValue(src)
	a.editor.SetCursor(caret.Line, caret.Col)
	a.changed(src)
	a.previewStale = true
}

// newDoc builds a rendered document whose pictures load from the note's own
// folder, so a relative ![](x.png) resolves next to it.
func (a *app) newDoc(text string) *markdown.Doc {
	return markdown.New(text).ImageLoader(a.loadImage)
}

// docDir is the folder the open document lives in.
func (a *app) docDir() string {
	if a.path != "" {
		return filepath.Dir(a.path)
	}
	return a.dir
}

// loadImage reads a picture for the renderer. Remote sources stay off, so a
// note never reaches the network by accident.
func (a *app) loadImage(ctx context.Context, src string) (image.Image, error) {
	path, err := a.imagePath(src)
	if err != nil {
		a.reportImageError(src, err)
		return nil, err
	}
	// The markdown renderer decodes PNG, JPEG, GIF and WebP; a bitmap has to be
	// read here.
	if strings.EqualFold(filepath.Ext(path), ".bmp") {
		data, err := os.ReadFile(path)
		if err == nil {
			var img image.Image
			if img, err = media.DecodeBMP(data); err == nil {
				return img, nil
			}
		}
		a.reportImageError(src, err)
		return nil, err
	}
	img, err := markdown.DecodeImage(ctx, path)
	if err != nil {
		a.reportImageError(src, err)
		return nil, err
	}
	return img, nil
}

// imagePath turns the source written in markdown into a path on disk. A reader
// pastes all sorts: file URLs, percent escapes, <angle brackets> around spaces,
// and paths that are relative to the working directory rather than the note.
func (a *app) imagePath(src string) (string, error) {
	raw := strings.TrimSpace(src)
	raw = strings.TrimSuffix(strings.TrimPrefix(raw, "<"), ">")
	switch {
	case strings.HasPrefix(raw, "data:"):
		return "", fmt.Errorf("%s: inline data is not read", src)
	case strings.HasPrefix(raw, "file://"):
		u, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		raw = u.Path
		// file:///D:/pics/a.png keeps a slash in front of the drive letter.
		if len(raw) > 2 && raw[0] == '/' && raw[2] == ':' {
			raw = raw[1:]
		}
	case strings.Contains(raw, "://"):
		return "", fmt.Errorf("%s: only local pictures are read", src)
	}
	if unescaped, err := url.PathUnescape(raw); err == nil {
		raw = unescaped
	}
	// A path pasted from Explorer arrives with backslashes; treat those as
	// separators rather than as markdown escapes.
	path := filepath.FromSlash(strings.ReplaceAll(raw, "\\", "/"))
	if filepath.IsAbs(path) {
		return path, nil
	}
	for _, base := range []string{a.docDir(), a.dir, cwd()} {
		if base == "" {
			continue
		}
		candidate := filepath.Join(base, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	// Nothing found: decode the path the note suggests, so the message names
	// the place the reader expected the picture to be.
	return filepath.Join(a.docDir(), path), nil
}

// cwd is the directory the editor was started in. A note often names a picture
// relative to the project it lives in, which is that directory.
func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

// reportImageError says on the status bar which picture could not be read. The
// loader runs off the UI thread, so the state changes inside core.Update.
func (a *app) reportImageError(src string, err error) {
	if err == nil {
		return
	}
	core.Update(func() {
		a.status = "图片读取失败：" + media.Name(src)
		a.statusColor = nil
	})
}

// paste puts the clipboard into the document at the caret: pictures are copied
// to the media folder and shown, other files become markdown links, and plain
// text lands as it is.
func (a *app) paste() {
	if a.previewing {
		return
	}
	nativeclip.Read(func(data nativeclip.Data, err error) {
		if err != nil {
			core.Update(func() { a.fail(userError("粘贴", "剪贴板", err)) })
			return
		}
		core.Update(func() { a.insertPasted(data) })
	})
}

// insertPasted writes one clipboard snapshot at the caret.
func (a *app) insertPasted(data nativeclip.Data) {
	if a.previewing {
		return
	}
	// The window shortcut consumes the key, so the editor never runs its own
	// clipboard read: plain text has to be inserted here too.
	var parts []string
	for _, file := range data.Files {
		path, err := a.storePicture(filepath.Base(file), readFile(file))
		if err != nil {
			path, err = media.Copy(a.mediaDir, file)
		}
		if err != nil {
			a.fail(userError("粘贴", media.Name(file), err))
			continue
		}
		if media.Image(file) {
			parts = append(parts, "!["+media.Name(path)+"]("+path+")")
			continue
		}
		parts = append(parts, "["+media.Name(file)+"]("+path+")")
	}
	// A picture held as bytes, e.g. a screenshot, arrives without a file name.
	for i, pic := range data.Images {
		name := fmt.Sprintf("pasted-%d%s", i+1, pasteExt(pic.MIME))
		path, err := a.storePicture(name, pic.Data)
		if err != nil {
			a.fail(userError("粘贴", name, err))
			continue
		}
		parts = append(parts, "![]("+path+")")
	}
	text := data.Text
	if len(parts) > 0 {
		text = strings.Join(parts, "\n")
	}
	if text == "" {
		return
	}
	out, caret := mddoc.Insert(a.editor.Value(), a.caret(), text)
	a.replace(out, caret)
}

// storePicture writes one picture into the media folder after making sure the
// renderer can decode it. A name the encoder changes, ending in .png instead of
// .bmp, is the one returned.
func (a *app) storePicture(name string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("%s: no bytes", name)
	}
	name, data, err := media.Encode(name, data)
	if err != nil {
		return "", err
	}
	return media.Save(a.mediaDir, name, data)
}

// readFile is os.ReadFile, kept tiny so the paste loop reads as one idea.
func readFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// pasteExt maps a clipboard media type to the extension the saved file gets.
func pasteExt(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/bmp":
		return ".bmp"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
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
