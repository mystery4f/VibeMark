// Package assets stores the files a reader pastes into a document. Copies live
// in an assets directory next to the document, so a note and its pictures stay
// together when the folder moves.
package assets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// imageExt lists the extensions the markdown renderer decodes.
var imageExt = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".bmp":  true,
}

// Image reports whether name is a picture the renderer can draw.
func Image(name string) bool {
	return imageExt[strings.ToLower(filepath.Ext(name))]
}

// Dir is the assets directory of the document in dir.
func Dir(dir string) string {
	return filepath.Join(dir, "assets")
}

// Copy copies src into the assets directory of dir and returns the path to
// write into markdown, slash separated and relative to dir.
func Copy(dir, src string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	return Save(dir, filepath.Base(src), data)
}

// Save writes data as name inside the assets directory of dir and returns the
// path to write into markdown, slash separated and relative to dir. A name
// already taken gets a counter, so a second paste never overwrites the first.
func Save(dir, name string, data []byte) (string, error) {
	if name == "" {
		name = "pasted"
	}
	folder := Dir(dir)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", err
	}
	target := freeName(folder, filepath.Base(name))
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join("assets", filepath.Base(target))), nil
}

// Name is the label a reader sees for a linked file: its base name.
func Name(path string) string {
	return filepath.Base(path)
}

// freeName returns the first free path for base inside folder, adding -1, -2
// and so on before the extension when the name is taken.
func freeName(folder, base string) string {
	candidate := filepath.Join(folder, base)
	if !exists(candidate) {
		return candidate
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		candidate = filepath.Join(folder, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if !exists(candidate) {
			return candidate
		}
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
