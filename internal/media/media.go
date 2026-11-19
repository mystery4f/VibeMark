// Package media stores the pictures a reader pastes into a document. They live
// in one folder under the system temp directory, so a note never grows a folder
// of copied pictures beside it, and markdown links to them by absolute path.
package media

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
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

// Root is the folder pasted pictures go to: VIBEMARK_MEDIA when it is set, so a
// test or a profile can point it somewhere it owns, and the temp directory
// otherwise.
func Root() string {
	if dir := os.Getenv("VIBEMARK_MEDIA"); dir != "" {
		return dir
	}
	temp := os.TempDir()
	// Windows hands out the short 8.3 name for the user folder, which would end
	// up spelled that way inside the markdown; the long name reads better.
	if long, err := filepath.EvalSymlinks(temp); err == nil {
		temp = long
	}
	return filepath.Join(temp, "vibemark")
}

// Encode makes clipboard or file bytes readable by the markdown renderer, which
// decodes PNG, JPEG, GIF and WebP. Windows hands over screenshots as a bitmap,
// so a BMP is re-encoded as PNG and the returned name follows.
func Encode(name string, data []byte) (string, []byte, error) {
	if !strings.EqualFold(filepath.Ext(name), ".bmp") {
		return name, data, nil
	}
	img, err := decodeBMP(data)
	if err != nil {
		return "", nil, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return "", nil, err
	}
	return strings.TrimSuffix(name, filepath.Ext(name)) + ".png", out.Bytes(), nil
}

// DecodeBMP reads a bitmap, the one picture format the markdown renderer does
// not decode itself.
func DecodeBMP(data []byte) (image.Image, error) {
	return decodeBMP(data)
}

// Copy copies src into root and returns the path markdown links to: absolute,
// so a document outside root still finds the picture.
func Copy(root, src string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	return Save(root, filepath.Base(src), data)
}

// Save writes data as name inside root and returns the absolute path markdown
// links to. A name already taken gets a counter, so a second paste never
// overwrites the first picture.
func Save(root, name string, data []byte) (string, error) {
	if name == "" {
		name = "pasted"
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	target, err := freeName(root, filepath.Base(name))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", err
	}
	return filepath.ToSlash(target), nil
}

// Name is the label a reader sees for a linked file: its base name.
func Name(path string) string {
	return filepath.Base(path)
}

// freeName returns the first free path for base inside root, adding -1, -2 and
// so on before the extension when the name is taken.
func freeName(root, base string) (string, error) {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	candidate := filepath.Join(root, base)
	for i := 0; ; i++ {
		_, err := os.Stat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				return candidate, nil
			}
			return "", err
		}
		candidate = filepath.Join(root, fmt.Sprintf("%s-%d%s", stem, i+1, ext))
	}
}
