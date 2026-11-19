# VibeMark

English | [简体中文](README.zh-CN.md)

A small WYSIWYG Markdown editor for the desktop, built in Go on the
[keel](https://github.com/dyike/keel) UI framework. It is meant to be usable as
`$EDITOR`: point `git config core.editor` at it and commits open a real editor
instead of a terminal buffer.

## How it renders

The document is one text editor. `internal/mddoc` turns the Markdown into
decorations, so headings are bold in the accent colour, bold and italic text
looks bold and italic, and the markers fade to 40% instead of disappearing.
`Ctrl+P` shows the fully typeset page.

The trade-off is recorded in
[`docs/adr/0003-one-editor-with-decorations.md`](docs/adr/0003-one-editor-with-decorations.md):
keel decorations cannot hide characters or change glyph width, so headings keep
the body size and the markers stay visible. Keeping one editor keeps typing,
undo, selection and input methods native, which matters more than exact
typography.

## Modes

- `vibemark <file>` — single file, blocking, for `$EDITOR`. Saves on `Ctrl+S`
  only; closing with unsaved changes exits `1` so git aborts the commit.
- `vibemark --dir <dir>` — browse a directory of Markdown files with a tree on
  the left. Saves automatically 300 ms after you stop typing.
- `vibemark` — reopens the last directory, or the current working directory.

Exit codes for the single-file mode: `0` saved, `1` cancelled, `2` could not
open or write (message on stderr).

## Shortcuts

| Chord | Action |
| --- | --- |
| `Ctrl+V` | Paste text, files or a screenshot: pictures are copied into the temp folder and shown, other files become links |
| `Ctrl+S` | Save |
| `Ctrl+P` | Toggle the full-screen preview |
| `Ctrl+Shift+L` | Toggle the file tree |
| `Ctrl+B` / `Ctrl+I` | Bold / italic around the selection |
| ``Ctrl+Shift+` `` | Inline code around the selection |
| `Alt+Shift+5` | Strikethrough around the selection |
| `Ctrl+K` | Wrap the selection as a link |
| `Ctrl+1` ... `Ctrl+6` | Heading level 1 ... 6 |
| `Ctrl+0` | Back to a plain paragraph |
| `Ctrl+Shift+K` | Toggle a fenced code block |
| `Ctrl+Shift+Q` | Blockquote |
| `Ctrl+Shift+[` / `Ctrl+Shift+]` | Ordered / bullet list |

The chords follow Typora's defaults, so muscle memory carries over. Typing,
`Enter`, `Shift+Enter`, `Backspace`, `Delete`, `Ctrl+A` and `Ctrl+Z` are the
editor's own, so they behave the way they do in any text box.

## Pictures

A pasted picture is written to `%TEMP%\vibemark` (override with
`VIBEMARK_MEDIA`), never beside the note, and the link is absolute:

```markdown
![shot.png](C:/Users/you/AppData/Local/Temp/vibemark/shot.png)
```

The folder is the system temp directory, so Windows may clear it: copy a picture
into the note's own folder if it has to outlive a reboot.

Everything else is read from disk as written. A relative path is looked up
beside the document first, then in the working directory, then where the editor
started, and `file://` URLs, `<angle brackets>` and `%20` escapes all work. The
renderer reads local files only; a remote `https://` source is skipped instead
of fetched, so opening a note never reaches the network. A picture it cannot
read is named on the status bar.

## Use it as git's editor

```sh
go build -o vibemark.exe .        # or: keel build
# put it on PATH, then:
git config --global core.editor vibemark
```

## Development

```sh
go build ./...
go test ./...       # UI tests run headless through keel's windowtest
gofmt -l .          # .githooks/pre-commit checks this, plus vet and test
```

`windowtest` synthesises keys but never touches the real keyboard or clipboard,
so the parts that depend on the OS are checked with keel's own automation
server, `keel-mcp` (installed by `go install
github.com/dyike/keel/cmd/keel-mcp@v0.4.11`, configured in
[`.pi/mcp.json`](.pi/mcp.json)):

```sh
go build -o .tmp/vibemark.exe .
# then, through the MCP tools:
#   launch  {"command": "…/.tmp/vibemark.exe --dir .tmp/demo"}
#   click   {"text": "正文"}
#   press_key {"key": "backspace"}
#   press_key {"key": "ctrl+v"}
```

It drives the real window off-screen (or with `visible: true`), so cross-block
deleting, clipboard pasting and autosave are verified end to end instead of
being assumed.

Enable the hooks once per clone: `git config core.hooksPath .githooks`.

Design records live in [`docs/design/vibemark-mvp.md`](docs/design/vibemark-mvp.md),
[`GLOSSARY.md`](GLOSSARY.md), and [`docs/adr/`](docs/adr/).

## Known limits

- Markers stay visible, only faded, and headings keep the body size: keel
  decorations cannot hide characters or change glyph width.
- The line-number gutter and the scrollbar belong to the editor widget and
  cannot be switched off.
- A command that rewrites the document (bold, heading, code fence) restarts the
  undo history, because the editor exposes no way to write its buffer.
- Pictures arrive through the clipboard only: there is no file dialog and no
  drag and drop.
