// Package workspace reads and writes the markdown files of one directory.
//
// Nothing here keeps state between calls: the editor owns the source text and
// asks this package only to scan, read and write.
package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Node is one entry of the file tree. ID is the absolute path, which is unique
// inside a tree and stable across renders.
type Node struct {
	ID, Label string
	Path      string
	Dir       bool
	Children  []*Node
}

// Scan returns every markdown file under root as a tree of directories. Hidden
// directories, node_modules and dist are skipped.
func Scan(root string) ([]*Node, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		if err == nil {
			err = fs.ErrNotExist
		}
		return nil, err
	}

	var files []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if p == root {
				return walkErr
			}
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist") {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(name), ".md") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	dirs := map[string]*Node{}
	roots := []*Node{}
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			continue
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		parent := ""
		var bucket *[]*Node
		if len(parts) == 1 {
			bucket = &roots
		}
		for i, part := range parts[:len(parts)-1] {
			key := parent + part
			node, ok := dirs[key]
			if !ok {
				node = &Node{ID: filepath.Join(root, key), Label: part, Path: filepath.Join(root, key), Dir: true}
				dirs[key] = node
				if parent == "" {
					roots = append(roots, node)
				} else if p, ok := dirs[parent[:len(parent)-1]]; ok {
					p.Children = append(p.Children, node)
				}
			}
			if i == len(parts)-2 {
				bucket = &node.Children
			}
			parent = key + "/"
		}
		if bucket != nil {
			*bucket = append(*bucket, &Node{ID: file, Label: parts[len(parts)-1], Path: file})
		}
	}

	var sortNodes func(nodes []*Node)
	sortNodes = func(nodes []*Node) {
		sort.SliceStable(nodes, func(i, j int) bool {
			if nodes[i].Dir != nodes[j].Dir {
				return nodes[i].Dir
			}
			return strings.ToLower(nodes[i].Label) < strings.ToLower(nodes[j].Label)
		})
		for _, n := range nodes {
			sortNodes(n.Children)
		}
	}
	sortNodes(roots)
	return roots, nil
}

// Read returns the file's text unchanged, so saving without an edit is byte
// identical.
func Read(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteAtomic writes content through a temporary file in the same directory and
// renames it into place, so an interrupted save cannot truncate the file.
func WriteAtomic(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".vibemark-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// Ensure creates an empty file when path does not exist yet. A missing parent
// directory is an error: the caller reports it instead of guessing a location.
func Ensure(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(path)
	if info, statErr := os.Stat(parent); statErr != nil || !info.IsDir() {
		if statErr != nil {
			return statErr
		}
		return fs.ErrNotExist
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}
