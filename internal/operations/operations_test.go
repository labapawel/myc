package operations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyAndMove(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "source.txt")
	dstFile := filepath.Join(tempDir, "copy.txt")
	movedFile := filepath.Join(tempDir, "moved.txt")

	content := []byte("Testing operations copy and move")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Test CopyFile
	if err := CopyFile(srcFile, dstFile, nil); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	data, err := os.ReadFile(dstFile)
	if err != nil || string(data) != string(content) {
		t.Fatalf("Copy content mismatch")
	}

	// Test Move
	if err := Move(dstFile, movedFile); err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	if _, err := os.Stat(dstFile); !os.IsNotExist(err) {
		t.Errorf("expected dstFile to no longer exist after move")
	}
	if _, err := os.Stat(movedFile); err != nil {
		t.Errorf("expected movedFile to exist")
	}
}

func TestMultiRename(t *testing.T) {
	files := []string{
		"/path/photo_01.jpg",
		"/path/photo_02.jpg",
	}

	rule := RenameRule{
		Find:          "photo",
		Replace:       "vacation_[C]",
		CounterStart:  10,
		CounterStep:   5,
		CounterDigits: 3,
	}

	pairs := PreviewRename(files, rule)
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}

	if pairs[0].NewName != "vacation_010_01.jpg" {
		t.Errorf("expected vacation_010_01.jpg, got %s", pairs[0].NewName)
	}
	if pairs[1].NewName != "vacation_015_02.jpg" {
		t.Errorf("expected vacation_015_02.jpg, got %s", pairs[1].NewName)
	}
}

func TestCompareFiles(t *testing.T) {
	tempDir := t.TempDir()
	f1 := filepath.Join(tempDir, "f1.txt")
	f2 := filepath.Join(tempDir, "f2.txt")
	f3 := filepath.Join(tempDir, "f3.txt")

	os.WriteFile(f1, []byte("line1\nline2\nline3\n"), 0644)
	os.WriteFile(f2, []byte("line1\nline2\nline3\n"), 0644)
	os.WriteFile(f3, []byte("line1\nDIFF2\nline3\n"), 0644)

	resIdentical, err := CompareFiles(f1, f2)
	if err != nil || !resIdentical.Identical {
		t.Errorf("expected f1 and f2 to be identical")
	}

	resDiff, err := CompareFiles(f1, f3)
	if err != nil || resDiff.Identical {
		t.Errorf("expected f1 and f3 to be different")
	}
	if resDiff.DiffLine != 2 {
		t.Errorf("expected diff at line 2, got %d", resDiff.DiffLine)
	}
}

func TestFindDuplicates(t *testing.T) {
	tempDir := t.TempDir()
	f1 := filepath.Join(tempDir, "orig.txt")
	f2 := filepath.Join(tempDir, "copy1.txt")
	f3 := filepath.Join(tempDir, "different.txt")

	content := []byte("exact same bytes in f1 and f2")
	os.WriteFile(f1, content, 0644)
	os.WriteFile(f2, content, 0644)
	os.WriteFile(f3, []byte("unique bytes here"), 0644)

	groups, err := FindDuplicates(tempDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}
	if len(groups[0].Files) != 2 {
		t.Fatalf("expected 2 duplicate files in group, got %d", len(groups[0].Files))
	}
}
