//go:build !windows

package gui

import (
	"os"
	"path/filepath"
	"syscall"
)

func getDiskSpace(path string) (int64, int64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	return total, free
}

func GetDrives() []DriveInfo {
	var drives []DriveInfo
	seen := make(map[string]bool)

	addDrive := func(letter, path, name string) {
		if path == "" || seen[path] {
			return
		}
		if _, err := os.Stat(path); err != nil {
			return
		}
		seen[path] = true
		total, free := getDiskSpace(path)
		drives = append(drives, DriveInfo{
			Letter:    letter,
			Path:      path,
			Name:      name,
			TotalSize: total,
			FreeSize:  free,
		})
	}

	addDrive("/", "/", "/ (Root)")
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		addDrive("~", home, "~ (Home)")
	}

	for _, p := range []string{"/media", "/mnt", "/tmp"} {
		addDrive(filepath.Base(p), p, p)
	}

	if user := os.Getenv("USER"); user != "" {
		for _, base := range []string{"/media/" + user, "/run/media/" + user} {
			if entries, err := os.ReadDir(base); err == nil {
				for _, e := range entries {
					if e.IsDir() {
						full := filepath.Join(base, e.Name())
						addDrive(e.Name(), full, e.Name())
					}
				}
			}
		}
	}

	return drives
}
