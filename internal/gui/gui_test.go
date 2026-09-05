package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetDrives(t *testing.T) {
	drives := GetDrives()
	if len(drives) == 0 {
		t.Fatalf("expected at least 1 drive, got 0")
	}
	t.Logf("Found %d drives: %+v", len(drives), drives[0])
}

func TestServerAPI(t *testing.T) {
	tempDir := t.TempDir()
	srv, err := NewServer(tempDir, tempDir)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer srv.Close()

	// Test /api/info
	req := httptest.NewRequest("GET", "/api/info", nil)
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var info map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("failed to decode /api/info response: %v", err)
	}
	if info["version"] == "" {
		t.Errorf("expected version to be non-empty")
	}

	// Test /api/drives
	reqDrives := httptest.NewRequest("GET", "/api/drives", nil)
	wDrives := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wDrives, reqDrives)
	if wDrives.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wDrives.Code)
	}

	// Test /api/list
	reqList := httptest.NewRequest("GET", "/api/list?path="+tempDir, nil)
	wList := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wList.Code)
	}

	var listResp ListResponse
	if err := json.Unmarshal(wList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode /api/list response: %v", err)
	}
	if listResp.Path == "" {
		t.Errorf("expected path to be non-empty")
	}

	// Test /api/mkdir in tempDir
	err = os.WriteFile(tempDir+"/sample.txt", []byte("hello world"), 0o644)
	if err != nil {
		t.Fatalf("failed to create sample file: %v", err)
	}

	reqFile := httptest.NewRequest("GET", "/api/file?path="+tempDir+"/sample.txt", nil)
	wFile := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wFile, reqFile)
	if wFile.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wFile.Code)
	}
}
