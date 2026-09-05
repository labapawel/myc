package operations

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

// DuplicateGroup represents a set of files that share the same content.
type DuplicateGroup struct {
	Hash  string
	Size  int64
	Files []string
}

// FindDuplicates scans dir and returns groups of duplicate files.
func FindDuplicates(dir string, onProgress func(scannedFiles int)) ([]DuplicateGroup, error) {
	bySize := make(map[int64][]string)
	var scanned int

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable files
		}
		if info.Mode().IsRegular() && info.Size() > 0 {
			bySize[info.Size()] = append(bySize[info.Size()], path)
			scanned++
			if onProgress != nil && scanned%100 == 0 {
				onProgress(scanned)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var duplicateGroups []DuplicateGroup

	// Check hashes only for sizes with more than 1 file
	for size, files := range bySize {
		if len(files) < 2 {
			continue
		}

		byHash := make(map[string][]string)
		for _, f := range files {
			h, err := computeFileHash(f)
			if err != nil {
				continue
			}
			byHash[h] = append(byHash[h], f)
		}

		for hash, matches := range byHash {
			if len(matches) > 1 {
				duplicateGroups = append(duplicateGroups, DuplicateGroup{
					Hash:  hash,
					Size:  size,
					Files: matches,
				})
			}
		}
	}

	return duplicateGroups, nil
}

func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
