package vfs

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalVFS(t *testing.T) {
	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "test1.txt")
	if err := os.WriteFile(file1, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(tempDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	vfs, err := NewLocalVFS(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := vfs.List()
	if err != nil {
		t.Fatal(err)
	}

	foundSub := false
	foundFile := false
	for _, e := range entries {
		if e.Name == "sub" && e.IsDir {
			foundSub = true
		}
		if e.Name == "test1.txt" && !e.IsDir {
			foundFile = true
		}
	}

	if !foundSub {
		t.Errorf("expected to find directory 'sub'")
	}
	if !foundFile {
		t.Errorf("expected to find file 'test1.txt'")
	}
}

func TestZipVFS(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.zip")

	// Create test zip
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	f1, err := zw.Create("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	f1.Write([]byte("content inside zip"))

	f2, err := zw.Create("folder/nested.txt")
	if err != nil {
		t.Fatal(err)
	}
	f2.Write([]byte("nested content"))

	zw.Close()

	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	zvfs, err := NewZipVFS(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zvfs.Close()

	entries, err := zvfs.List()
	if err != nil {
		t.Fatal(err)
	}

	foundHello := false
	foundFolder := false
	for _, e := range entries {
		if e.Name == "hello.txt" {
			foundHello = true
		}
		if e.Name == "folder" && e.IsDir {
			foundFolder = true
		}
	}

	if !foundHello {
		t.Errorf("expected hello.txt in zip root")
	}
	if !foundFolder {
		t.Errorf("expected folder in zip root")
	}

	// Read content
	data, err := zvfs.ReadAllBytes("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "content inside zip" {
		t.Errorf("unexpected content: %s", string(data))
	}
}
