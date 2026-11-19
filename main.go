// Command vibemark opens one markdown file or one directory in a window and
// returns git a meaningful exit code.
//
//	vibemark <file>        edit one file (git's core.editor / $EDITOR)
//	vibemark --dir <path>  edit every markdown file under a directory
//	vibemark               the directory of the last run, else the working one
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"

	"github.com/dyike/keel/ui/el"
	"github.com/dyike/keel/ui/window"
)

//go:embed appicon.png
var icon []byte

// exit codes: 0 saved, 1 the user left unsaved text (git aborts the commit),
// 2 the file or directory could not be opened, with the reason on stderr.
const exitUnsaved = 1

func main() {
	window.SetIcon(icon)

	dir := flag.String("dir", "", "directory whose markdown files to edit")
	flag.Parse()

	path := flag.Arg(0)
	if path != "" && *dir != "" {
		fmt.Fprintln(os.Stderr, "vibemark: give a file or --dir, not both")
		os.Exit(2)
	}

	a := newApp(path, *dir)
	window.Open(window.Options{
		Title:     "VibeMark",
		Width:     720,
		Height:    480,
		MinWidth:  480,
		MinHeight: 320,
		Content:   el.Root(a),
		KeepState: &a.state,
		Shortcuts: a.shortcuts(),
		OnClose:   a.onClose,
	})
	window.Main()
}

// onClose runs after the window is gone, which is the only hook left: keel does
// not let a view cancel a close. Single-file mode therefore turns unsaved text
// into exit code 1, and git reads that as "abort the commit".
func (a *app) onClose() {
	if a.mode == modeSingle && a.dirty {
		os.Exit(exitUnsaved)
	}
}
