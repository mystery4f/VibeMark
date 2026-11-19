package media

import (
	"os"
	"path/filepath"
	"testing"
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
