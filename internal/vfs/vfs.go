package vfs

import (
	"io"
	"os"
	"time"
)

// FileEntry represents a file or directory item in a panel.
type FileEntry struct {
	Name      string
	Path      string
	Size      int64
	Mode      os.FileMode
	ModTime   time.Time
	IsDir     bool
	IsArchive bool
	Selected  bool
}

// VFS is the virtual file system interface.
type VFS interface {
	Path() string
	SetPath(path string) error
	List() ([]*FileEntry, error)
	Stat(name string) (*FileEntry, error)
	Open(name string) (io.ReadCloser, error)
	Create(name string) (io.WriteCloser, error)
	Mkdir(name string) error
	Remove(name string) error
	RemoveAll(name string) error
	Rename(oldName, newName string) error
	Parent() error
	IsArchive() bool
	Close() error
}
