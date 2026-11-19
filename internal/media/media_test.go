package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/bmp"
)

// Save writes the bytes under the root and returns the absolute path markdown
// links to.
func TestSaveWritesUnderRoot(t *testing.T) {
	root := t.TempDir()

	got, err := Save(root, "shot.png", []byte("pixels"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.ToSlash(filepath.Join(root, "shot.png")); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if !filepath.IsAbs(filepath.FromSlash(got)) {
		t.Fatalf("path = %q, want an absolute one", got)
	}
	if read(t, filepath.Join(root, "shot.png")) != "pixels" {
		t.Fatal("the bytes did not land")
	}
}

// A second paste with the same name must not overwrite the first picture.
func TestSaveKeepsBothNames(t *testing.T) {
	root := t.TempDir()

	first, err := Save(root, "shot.png", []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Save(root, "shot.png", []byte("two"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.ToSlash(filepath.Join(root, "shot-1.png")); second != want {
		t.Fatalf("second = %q, want %q", second, want)
	}
	if read(t, filepath.Join(root, "shot.png")) != "one" {
		t.Fatal("the first picture was overwritten")
	}
	if first == second {
		t.Fatalf("both pastes went to %q", first)
	}
}

// Copy brings an existing file in and keeps its name.
func TestCopyKeepsTheName(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(t.TempDir(), "diagram.png")
	if err := os.WriteFile(src, []byte("picture-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Copy(root, src)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.ToSlash(filepath.Join(root, "diagram.png")); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if read(t, filepath.Join(root, "diagram.png")) != "picture-bytes" {
		t.Fatal("the copy lost its bytes")
	}
}

// Root honours VIBEMARK_MEDIA so a test or profile can pick the folder.
func TestRootPrefersTheOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VIBEMARK_MEDIA", dir)

	if got := Root(); got != dir {
		t.Fatalf("Root() = %q, want %q", got, dir)
	}
}

// Image tells the renderer's formats from everything else.
func TestImage(t *testing.T) {
	cases := map[string]bool{
		"a.png": true, "a.JPG": true, "a.jpeg": true, "a.gif": true,
		"a.webp": true, "a.bmp": true, "a.svg": false, "a.txt": false, "a": false,
	}
	for name, want := range cases {
		if got := Image(name); got != want {
			t.Errorf("Image(%q) = %v, want %v", name, got, want)
		}
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A bitmap from the clipboard is re-encoded as PNG, because the markdown
// renderer decodes PNG, JPEG, GIF and WebP only.
func TestEncodeTurnsABitmapIntoPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 6, 4))
	for x := 0; x < 6; x++ {
		img.Set(x, 1, color.RGBA{R: 9, G: 200, B: 30, A: 255})
	}
	var in bytes.Buffer
	if err := bmp.Encode(&in, img); err != nil {
		t.Fatal(err)
	}

	name, data, err := Encode("pasted-1.bmp", in.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if want := "pasted-1.png"; name != want {
		t.Fatalf("name = %q, want %q", name, want)
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(data)); err != nil || format != "png" {
		t.Fatalf("format = %q, err = %v", format, err)
	}
}

// Everything else is stored untouched.
func TestEncodeLeavesOtherFormatsAlone(t *testing.T) {
	name, data, err := Encode("shot.png", []byte("pixels"))
	if err != nil {
		t.Fatal(err)
	}
	if name != "shot.png" || string(data) != "pixels" {
		t.Fatalf("name = %q, data = %q", name, data)
	}
}

// fieldBMP builds a 32 bit bitmap the way the Windows clipboard does: a 40 byte
// header and BI_BITFIELDS masks after it, which golang.org/x/image/bmp refuses.
func fieldBMP(width, height int, fill color.NRGBA) []byte {
	dib := 40
	masks := 12
	offset := 14 + dib + masks
	stride := width * 4
	file := make([]byte, offset+stride*height)
	file[0], file[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(file[2:], uint32(len(file)))
	binary.LittleEndian.PutUint32(file[10:], uint32(offset))
	binary.LittleEndian.PutUint32(file[14:], uint32(dib))
	binary.LittleEndian.PutUint32(file[18:], uint32(width))
	binary.LittleEndian.PutUint32(file[22:], uint32(height))
	binary.LittleEndian.PutUint16(file[26:], 1)
	binary.LittleEndian.PutUint16(file[28:], 32)
	binary.LittleEndian.PutUint32(file[30:], 3) // BI_BITFIELDS
	maskAt := 14 + dib
	binary.LittleEndian.PutUint32(file[maskAt:], 0x00ff0000)
	binary.LittleEndian.PutUint32(file[maskAt+4:], 0x0000ff00)
	binary.LittleEndian.PutUint32(file[maskAt+8:], 0x000000ff)
	pixel := uint32(fill.B) | uint32(fill.G)<<8 | uint32(fill.R)<<16
	for y := 0; y < height; y++ {
		line := file[offset+y*stride:]
		for x := 0; x < width; x++ {
			binary.LittleEndian.PutUint32(line[x*4:], pixel)
		}
	}
	return file
}

// A clipboard screenshot is a bit-field bitmap: it must decode, and pasting must
// store it as PNG because the renderer cannot read BMP.
func TestEncodeDecodesABitFieldBitmap(t *testing.T) {
	name, data, err := Encode("pasted-1.bmp", fieldBMP(12, 9, color.NRGBA{R: 230, G: 120, B: 20, A: 255}))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if want := "pasted-1.png"; name != want {
		t.Fatalf("name = %q, want %q", name, want)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "png" {
		t.Fatalf("format = %q, err = %v", format, err)
	}
	if cfg.Width != 12 || cfg.Height != 9 {
		t.Fatalf("size = %dx%d, want 12x9", cfg.Width, cfg.Height)
	}
}

// The colour survives the round trip, so a screenshot does not turn black.
func TestDecodeBMPKeepsTheColour(t *testing.T) {
	img, err := decodeBMP(fieldBMP(2, 2, color.NRGBA{R: 230, G: 120, B: 20, A: 255}))
	if err != nil {
		t.Fatalf("decodeBMP: %v", err)
	}
	r, g, b, _ := img.At(0, 0).RGBA()
	if got := [3]uint32{r >> 8, g >> 8, b >> 8}; got != [3]uint32{230, 120, 20} {
		t.Fatalf("colour = %v, want [230 120 20]", got)
	}
}
