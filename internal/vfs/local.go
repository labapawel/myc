package vfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LocalVFS implements VFS for the host filesystem.
type LocalVFS struct {
	currentPath string
}

// NewLocalVFS creates a new local filesystem VFS rooted at initialPath.
func NewLocalVFS(initialPath string) (*LocalVFS, error) {
	if initialPath == "" {
		var err error
		initialPath, err = os.Getwd()
		if err != nil {
			initialPath = "."
		}
	}
	abs, err := filepath.Abs(initialPath)
	if err != nil {
		abs = initialPath
	}
	return &LocalVFS{currentPath: abs}, nil
}

func (l *LocalVFS) Path() string {
	return l.currentPath
}

func (l *LocalVFS) SetPath(p string) error {
	abs, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s nie jest katalogiem", abs)
	}
	l.currentPath = abs
	return nil
}

func (l *LocalVFS) Parent() error {
	parent := filepath.Dir(l.currentPath)
	if parent == l.currentPath || (len(l.currentPath) == 3 && l.currentPath[1] == ':') {
		return nil // already at root
	}
	return l.SetPath(parent)
}

func (l *LocalVFS) List() ([]*FileEntry, error) {
	entries, err := os.ReadDir(l.currentPath)
	if err != nil {
		return nil, err
	}

	var result []*FileEntry

	// Add parent entry if not at root
	parent := filepath.Dir(l.currentPath)
	isRoot := parent == l.currentPath || (len(l.currentPath) == 3 && l.currentPath[1] == ':')
	if !isRoot {
		result = append(result, &FileEntry{
			Name:    "..",
			Path:    parent,
			IsDir:   true,
			ModTime: l.statModTime(l.currentPath),
		})
	}

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := e.Name()
		fullPath := filepath.Join(l.currentPath, name)
		isArchive := IsArchiveFile(name)

		result = append(result, &FileEntry{
			Name:      name,
			Path:      fullPath,
			Size:      info.Size(),
			Mode:      info.Mode(),
			ModTime:   info.ModTime(),
			IsDir:     e.IsDir(),
			IsArchive: isArchive,
		})
	}

	// Sort: ".." first, then directories, then files alphabetically (case-insensitive)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Name == ".." {
			return true
		}
		if result[j].Name == ".." {
			return false
		}
		if result[i].IsDir && !result[j].IsDir {
			return true
		}
		if !result[i].IsDir && result[j].IsDir {
			return false
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

func (l *LocalVFS) Stat(name string) (*FileEntry, error) {
	p := filepath.Join(l.currentPath, name)
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	return &FileEntry{
		Name:      info.Name(),
		Path:      p,
		Size:      info.Size(),
		Mode:      info.Mode(),
		ModTime:   info.ModTime(),
		IsDir:     info.IsDir(),
		IsArchive: IsArchiveFile(info.Name()),
	}, nil
}

func (l *LocalVFS) Open(name string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(l.currentPath, name))
}

func (l *LocalVFS) Create(name string) (io.WriteCloser, error) {
	return os.Create(filepath.Join(l.currentPath, name))
}

func (l *LocalVFS) Mkdir(name string) error {
	return os.MkdirAll(filepath.Join(l.currentPath, name), 0755)
}

func (l *LocalVFS) Remove(name string) error {
	return os.Remove(filepath.Join(l.currentPath, name))
}

func (l *LocalVFS) RemoveAll(name string) error {
	return os.RemoveAll(filepath.Join(l.currentPath, name))
}

func (l *LocalVFS) Rename(oldName, newName string) error {
	oldP := filepath.Join(l.currentPath, oldName)
	newP := filepath.Join(l.currentPath, newName)
	return os.Rename(oldP, newP)
}

func (l *LocalVFS) IsArchive() bool {
	return false
}

func (l *LocalVFS) Close() error {
	return nil
}

func (l *LocalVFS) statModTime(p string) time.Time {
	if info, err := os.Stat(p); err == nil {
		return info.ModTime()
	}
	return time.Now()
}

// IsArchiveFile checks if the filename has an archive extension.
func IsArchiveFile(filename string) bool {
	lower := strings.ToLower(filename)
	exts := []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tbz2", ".rar", ".7z"}
	for _, ext := range exts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
