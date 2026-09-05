package operations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RenameRule defines how file names should be transformed.
type RenameRule struct {
	Find        string
	Replace     string
	Prefix      string
	Suffix      string
	CounterStart int
	CounterStep  int
	CounterDigits int
	ToLowerCase bool
	ToUpperCase bool
}

// RenamePair represents an original file and its proposed new name.
type RenamePair struct {
	OldPath string
	NewPath string
	OldName string
	NewName string
}

// PreviewRename generates the list of renamed pairs without performing the changes.
func PreviewRename(files []string, rule RenameRule) []RenamePair {
	var pairs []RenamePair
	counter := rule.CounterStart
	if counter == 0 {
		counter = 1
	}
	step := rule.CounterStep
	if step == 0 {
		step = 1
	}
	digits := rule.CounterDigits
	if digits == 0 {
		digits = 1
	}

	for _, p := range files {
		dir := filepath.Dir(p)
		base := filepath.Base(p)
		ext := filepath.Ext(base)
		nameWithoutExt := strings.TrimSuffix(base, ext)

		newName := nameWithoutExt

		// Search and replace
		if rule.Find != "" {
			newName = strings.ReplaceAll(newName, rule.Find, rule.Replace)
		}

		// Prefix and Suffix
		if rule.Prefix != "" {
			newName = rule.Prefix + newName
		}
		if rule.Suffix != "" {
			newName = newName + rule.Suffix
		}

		// Counter placeholder support: [C] or [N]
		if strings.Contains(newName, "[C]") || strings.Contains(newName, "[N]") {
			formatStr := fmt.Sprintf("%%0%dd", digits)
			numStr := fmt.Sprintf(formatStr, counter)
			newName = strings.ReplaceAll(newName, "[C]", numStr)
			newName = strings.ReplaceAll(newName, "[N]", numStr)
			counter += step
		}

		// Case changes
		if rule.ToLowerCase {
			newName = strings.ToLower(newName)
			ext = strings.ToLower(ext)
		} else if rule.ToUpperCase {
			newName = strings.ToUpper(newName)
			ext = strings.ToUpper(ext)
		}

		finalName := newName + ext
		pairs = append(pairs, RenamePair{
			OldPath: p,
			NewPath: filepath.Join(dir, finalName),
			OldName: base,
			NewName: finalName,
		})
	}

	return pairs
}

// ExecuteRename applies the rename operations.
func ExecuteRename(pairs []RenamePair) error {
	for _, pair := range pairs {
		if pair.OldPath != pair.NewPath {
			if err := os.Rename(pair.OldPath, pair.NewPath); err != nil {
				return fmt.Errorf("błąd zmiany nazwy '%s' na '%s': %w", pair.OldName, pair.NewName, err)
			}
		}
	}
	return nil
}
