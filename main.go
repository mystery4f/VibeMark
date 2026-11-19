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
	"io"
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

	path, dir, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "vibemark: %v\n", err)
		os.Exit(2)
	}

	a := newApp(path, dir)
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

// parseArgs turns the command line into one file or one directory. A caller that
// cannot be satisfied — unknown flag, or a file and --dir at once — gets an
// error instead of a window, because git reads exit code 2 as "the editor
// failed" and stops the commit.
func parseArgs(args []string) (path, dir string, err error) {
	fs := flag.NewFlagSet("vibemark", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dirFlag := fs.String("dir", "", "directory whose markdown files to edit")
	if parseErr := fs.Parse(args); parseErr != nil {
		return "", "", parseErr
	}
	path = fs.Arg(0)
	if fs.NArg() > 1 {
		// flag.Parse stops at the first positional argument, so anything after
		// it never reached the flag set; refuse it instead of dropping it.
		return "", "", fmt.Errorf("unexpected arguments: %v", fs.Args()[1:])
	}
	if path != "" && *dirFlag != "" {
		return "", "", fmt.Errorf("give a file or --dir, not both")
	}
	return path, *dirFlag, nil
}
