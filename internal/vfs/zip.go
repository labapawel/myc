package vfs

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"
)

// ZipVFS allows browsing inside a .zip file as if it were a directory.
type ZipVFS struct {
	archivePath string
	subPath     string
	reader      *zip.ReadCloser
}

// NewZipVFS opens a zip file at archivePath and mounts it as a VFS.
func NewZipVFS(archivePath string) (*ZipVFS, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	return &ZipVFS{
		archivePath: archivePath,
		subPath:     "",
		reader:      r,
	}, nil
}

func (z *ZipVFS) Path() string {
	if z.subPath == "" {
		return z.archivePath + "::/"
	}
	return z.archivePath + "::/" + z.subPath
}

func (z *ZipVFS) SetPath(p string) error {
	p = strings.TrimPrefix(p, z.archivePath+"::/")
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	z.subPath = p
	return nil
}

func (z *ZipVFS) Parent() error {
	if z.subPath == "" {
		return nil
	}
	z.subPath = path.Dir(z.subPath)
	if z.subPath == "." {
		z.subPath = ""
	}
	return nil
}

func (z *ZipVFS) List() ([]*FileEntry, error) {
	seenDirs := make(map[string]bool)
	var result []*FileEntry

	// Parent entry
	result = append(result, &FileEntry{
		Name:    "..",
		Path:    "..",
		IsDir:   true,
		ModTime: time.Now(),
	})

	prefix := z.subPath
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	for _, f := range z.reader.File {
		name := f.Name
		if !strings.HasPrefix(name, prefix) {
			continue
		}

		rel := strings.TrimPrefix(name, prefix)
		if rel == "" {
			continue
		}

		parts := strings.Split(strings.TrimSuffix(rel, "/"), "/")
		entryName := parts[0]

		if len(parts) > 1 || strings.HasSuffix(rel, "/") {
			// Subdirectory
			if !seenDirs[entryName] {
				seenDirs[entryName] = true
				result = append(result, &FileEntry{
					Name:      entryName,
					Path:      prefix + entryName,
					IsDir:     true,
					ModTime:   f.Modified,
					IsArchive: false,
				})
			}
		} else {
			// File
			result = append(result, &FileEntry{
				Name:      entryName,
				Path:      name,
				Size:      int64(f.UncompressedSize64),
				Mode:      f.Mode(),
				ModTime:   f.Modified,
				IsDir:     false,
				IsArchive: IsArchiveFile(entryName),
			})
		}
	}

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

func (z *ZipVFS) Stat(name string) (*FileEntry, error) {
	target := name
	if z.subPath != "" {
		target = path.Join(z.subPath, name)
	}
	for _, f := range z.reader.File {
		clean := strings.TrimSuffix(f.Name, "/")
		if clean == target {
			return &FileEntry{
				Name:    name,
				Path:    f.Name,
				Size:    int64(f.UncompressedSize64),
				Mode:    f.Mode(),
				ModTime: f.Modified,
				IsDir:   f.FileInfo().IsDir(),
			}, nil
		}
	}
	return nil, fmt.Errorf("nie znaleziono w archiwum: %s", name)
}

func (z *ZipVFS) Open(name string) (io.ReadCloser, error) {
	target := name
	if z.subPath != "" {
		target = path.Join(z.subPath, name)
	}
	for _, f := range z.reader.File {
		if f.Name == target || strings.TrimSuffix(f.Name, "/") == target {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("nie znaleziono w archiwum: %s", name)
}

func (z *ZipVFS) Create(name string) (io.WriteCloser, error) {
	return nil, fmt.Errorf("archiwum ZIP jest tylko do odczytu")
}

func (z *ZipVFS) Mkdir(name string) error {
	return fmt.Errorf("archiwum ZIP jest tylko do odczytu")
}

func (z *ZipVFS) Remove(name string) error {
	return fmt.Errorf("archiwum ZIP jest tylko do odczytu")
}

func (z *ZipVFS) RemoveAll(name string) error {
	return fmt.Errorf("archiwum ZIP jest tylko do odczytu")
}

func (z *ZipVFS) Rename(oldName, newName string) error {
	return fmt.Errorf("archiwum ZIP jest tylko do odczytu")
}

func (z *ZipVFS) IsArchive() bool {
	return true
}

func (z *ZipVFS) Close() error {
	if z.reader != nil {
		return z.reader.Close()
	}
	return nil
}

// ExtractFile extracts a single file from zip into an io.Writer.
func (z *ZipVFS) ExtractFile(entryPath string, dst io.Writer) error {
	rc, err := z.Open(entryPath)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(dst, rc)
	return err
}

// ReadAllBytes reads the content of a file in the zip into a byte slice.
func (z *ZipVFS) ReadAllBytes(name string) ([]byte, error) {
	rc, err := z.Open(name)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(rc)
	return buf.Bytes(), err
}
