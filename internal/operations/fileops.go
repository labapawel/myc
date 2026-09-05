package operations

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ProgressFn reports bytes copied and total bytes.
type ProgressFn func(currentBytes, totalBytes int64, currentFile string)

// CopyFile copies a single file from src to dst.
func CopyFile(src, dst string, onProgress ProgressFn) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("nie można skopiować elementu nieregularnego: %s", src)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Ensure target directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	totalSize := srcInfo.Size()
	buf := make([]byte, 64*1024)
	var written int64

	for {
		nr, er := srcFile.Read(buf)
		if nr > 0 {
			nw, ew := dstFile.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
				if onProgress != nil {
					onProgress(written, totalSize, filepath.Base(src))
				}
			}
			if ew != nil {
				return ew
			}
		}
		if er != nil {
			if er != io.EOF {
				return er
			}
			break
		}
	}

	return nil
}

// CopyDir recursively copies a directory tree.
func CopyDir(src, dst string, onProgress ProgressFn) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath, onProgress); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath, onProgress); err != nil {
				return err
			}
		}
	}

	return nil
}

// Move moves or renames a file or directory.
func Move(src, dst string) error {
	// Try atomic rename first
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// Cross-device link or unsupported: fallback to copy + remove
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := CopyDir(src, dst, nil); err != nil {
			return err
		}
		return os.RemoveAll(src)
	}

	if err := CopyFile(src, dst, nil); err != nil {
		return err
	}
	return os.Remove(src)
}

// Delete removes a file or directory recursively.
func Delete(path string) error {
	return os.RemoveAll(path)
}
