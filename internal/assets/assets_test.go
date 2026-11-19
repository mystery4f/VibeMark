package assets

import (
	"os"
	"path/filepath"
	"testing"
)

// Save puts the bytes in the assets folder and reports the path markdown uses.
func TestSaveWritesThePicture(t *testing.T) {
	dir := t.TempDir()

	got, err := Save(dir, "shot.png", []byte("pixels"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "assets/shot.png"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	data, err := os.ReadFile(filepath.Join(dir, "assets", "shot.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "pixels" {
		t.Fatalf("file = %q", data)
	}
}

// A second paste with the same name must not overwrite the first picture.
func TestSaveKeepsBothNames(t *testing.T) {
	dir := t.TempDir()

	first, err := Save(dir, "shot.png", []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Save(dir, "shot.png", []byte("two"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("both pastes went to %q", first)
	}
	if want := "assets/shot-1.png"; second != want {
		t.Fatalf("second = %q, want %q", second, want)
	}
	if got := read(t, filepath.Join(dir, "assets", "shot.png")); got != "one" {
		t.Fatalf("first picture = %q", got)
	}
}

// Copy brings an existing file in and keeps its name.
func TestCopyKeepsTheName(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "diagram.png")
	if err := os.WriteFile(src, []byte("webp-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Copy(dir, src)
	if err != nil {
		t.Fatal(err)
	}
	if want := "assets/diagram.png"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if read(t, filepath.Join(dir, "assets", "diagram.png")) != "webp-bytes" {
		t.Fatal("the copy lost its bytes")
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
