package gui

import (
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"myc/internal/operations"
	"myc/internal/version"
	"myc/internal/vfs"
)

//go:embed web/*
var WebFS embed.FS

type Server struct {
	listener   net.Listener
	port       int
	initialL   string
	initialR   string
	server     *http.Server
	lastActive time.Time
	mu         sync.Mutex
	stopChan   chan struct{}
}

type FileItemJSON struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Mode      string `json:"mode"`
	ModTime   string `json:"mod_time"`
	IsDir     bool   `json:"is_dir"`
	IsArchive bool   `json:"is_archive"`
	Ext       string `json:"ext"`
}

type ListResponse struct {
	Path      string         `json:"path"`
	IsArchive bool           `json:"is_archive"`
	TotalSize int64          `json:"total_size"`
	FreeSize  int64          `json:"free_size"`
	Items     []FileItemJSON `json:"items"`
}

func NewServer(leftPath, rightPath string) (*Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("nie udało się uruchomić serwera lokalnego: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	s := &Server{
		listener:   listener,
		port:       port,
		initialL:   leftPath,
		initialR:   rightPath,
		lastActive: time.Now(),
		stopChan:   make(chan struct{}),
	}

	mux := http.NewServeMux()

	// Static frontend
	webSub, err := fs.Sub(WebFS, "web")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(webSub)))
	}

	// API routes
	mux.HandleFunc("/api/info", s.handleInfo)
	mux.HandleFunc("/api/drives", s.handleDrives)
	mux.HandleFunc("/api/list", s.handleList)
	mux.HandleFunc("/api/file", s.handleFile)
	mux.HandleFunc("/api/save", s.handleSave)
	mux.HandleFunc("/api/mkdir", s.handleMkdir)
	mux.HandleFunc("/api/delete", s.handleDelete)
	mux.HandleFunc("/api/copy", s.handleCopy)
	mux.HandleFunc("/api/move", s.handleMove)
	mux.HandleFunc("/api/rename", s.handleRename)
	mux.HandleFunc("/api/multi-rename/preview", s.handleMultiRenamePreview)
	mux.HandleFunc("/api/multi-rename/apply", s.handleMultiRenameApply)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/compare", s.handleCompare)
	mux.HandleFunc("/api/sync", s.handleSync)
	mux.HandleFunc("/api/split", s.handleSplit)
	mux.HandleFunc("/api/join", s.handleJoin)
	mux.HandleFunc("/api/exec", s.handleExec)
	mux.HandleFunc("/api/heartbeat", s.handleHeartbeat)
	mux.HandleFunc("/api/exit", s.handleExit)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	return s, nil
}

func (s *Server) Port() int {
	return s.port
}

func (s *Server) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", s.port)
}

func (s *Server) Start() error {
	return s.server.Serve(s.listener)
}

func (s *Server) Close() error {
	return s.server.Close()
}

func (s *Server) StopChan() <-chan struct{} {
	return s.stopChan
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.lastActive = time.Now()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleExit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "bye"})
	go func() {
		time.Sleep(200 * time.Millisecond)
		select {
		case <-s.stopChan:
		default:
			close(s.stopChan)
		}
	}()
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":      version.Version,
		"initial_left": s.initialL,
		"initial_right": s.initialR,
	})
}

func (s *Server) handleDrives(w http.ResponseWriter, r *http.Request) {
	drives := GetDrives()
	writeJSON(w, http.StatusOK, drives)
}

func resolveVFS(rawPath string) (vfs.VFS, string, func(), error) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		cwd, _ := os.Getwd()
		rawPath = cwd
	}

	if strings.Contains(rawPath, "::/") {
		parts := strings.SplitN(rawPath, "::/", 2)
		archivePath := parts[0]
		subPath := parts[1]
		lower := strings.ToLower(archivePath)

		if strings.HasSuffix(lower, ".zip") {
			zv, err := vfs.NewZipVFS(archivePath)
			if err != nil {
				return nil, "", nil, err
			}
			if subPath != "" {
				_ = zv.SetPath(subPath)
			}
			return zv, zv.Path(), func() { _ = zv.Close() }, nil
		}

		tv, err := vfs.NewTarVFS(archivePath)
		if err != nil {
			return nil, "", nil, err
		}
		if subPath != "" {
			_ = tv.SetPath(subPath)
		}
		return tv, tv.Path(), func() { _ = tv.Close() }, nil
	}

	lower := strings.ToLower(rawPath)
	if info, err := os.Stat(rawPath); err == nil && !info.IsDir() {
		if strings.HasSuffix(lower, ".zip") {
			zv, err := vfs.NewZipVFS(rawPath)
			if err != nil {
				return nil, "", nil, err
			}
			return zv, zv.Path(), func() { _ = zv.Close() }, nil
		}
		if strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.bz2") {
			tv, err := vfs.NewTarVFS(rawPath)
			if err != nil {
				return nil, "", nil, err
			}
			return tv, tv.Path(), func() { _ = tv.Close() }, nil
		}
	}

	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		absPath = rawPath
	}
	lv, err := vfs.NewLocalVFS(absPath)
	if err != nil {
		return nil, "", nil, err
	}
	return lv, lv.Path(), func() { _ = lv.Close() }, nil
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	v, resolvedPath, cleanup, err := resolveVFS(reqPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Błąd otwarcia ścieżki: %v", err))
		return
	}
	if cleanup != nil {
		defer cleanup()
	}

	entries, err := v.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd listowania zawartości: %v", err))
		return
	}

	items := make([]FileItemJSON, 0, len(entries))
	for _, e := range entries {
		ext := ""
		if !e.IsDir && !e.IsArchive {
			ext = strings.TrimPrefix(filepath.Ext(e.Name), ".")
		}
		items = append(items, FileItemJSON{
			Name:      e.Name,
			Path:      e.Path,
			Size:      e.Size,
			Mode:      e.Mode.String(),
			ModTime:   e.ModTime.Format("2006-01-02 15:04"),
			IsDir:     e.IsDir,
			IsArchive: e.IsArchive,
			Ext:       ext,
		})
	}

	var totalSpace, freeSpace int64
	if !v.IsArchive() {
		totalSpace, freeSpace = getDiskSpace(resolvedPath)
	}

	writeJSON(w, http.StatusOK, ListResponse{
		Path:      resolvedPath,
		IsArchive: v.IsArchive(),
		TotalSize: totalSpace,
		FreeSize:  freeSpace,
		Items:     items,
	})
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeError(w, http.StatusBadRequest, "Brak parametru path")
		return
	}

	v, _, cleanup, err := resolveVFS(filepath.Dir(filePath))
	if err == nil && v.IsArchive() && cleanup != nil {
		defer cleanup()
	}

	var reader io.ReadCloser
	var size int64
	if v != nil && v.IsArchive() {
		baseName := filepath.Base(filePath)
		reader, err = v.Open(baseName)
		if st, errStat := v.Stat(baseName); errStat == nil {
			size = st.Size
		}
	} else {
		f, errOpen := os.Open(filePath)
		if errOpen != nil {
			err = errOpen
		} else {
			reader = f
			if st, errStat := f.Stat(); errStat == nil {
				size = st.Size()
			}
		}
	}

	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("Nie można odczytać pliku: %v", err))
		return
	}
	defer reader.Close()

	// Read up to 2MB for preview
	maxRead := int64(2 * 1024 * 1024)
	buf := make([]byte, maxRead)
	n, _ := io.ReadFull(reader, buf)
	data := buf[:n]

	// Check if binary
	isBinary := false
	for _, b := range data[:min(len(data), 1024)] {
		if b == 0 {
			isBinary = true
			break
		}
	}

	resp := map[string]any{
		"path":      filePath,
		"size":      size,
		"is_binary": isBinary,
	}

	if isBinary {
		hexStr := hex.Dump(data[:min(len(data), 32*1024)])
		resp["hex"] = hexStr
	} else {
		resp["content"] = string(data)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	if strings.Contains(req.Path, "::/") {
		writeError(w, http.StatusBadRequest, "Zapis wewnątrz archiwum nie jest bezpośrednio obsługiwany")
		return
	}

	if err := os.WriteFile(req.Path, []byte(req.Content), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd zapisu pliku: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	target := filepath.Join(req.Path, req.Name)
	if err := os.MkdirAll(target, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd tworzenia katalogu: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "created", "path": target})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	for _, p := range req.Paths {
		if err := operations.Delete(p); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd usuwania %s: %v", p, err))
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sources     []string `json:"sources"`
		Destination string   `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	for _, src := range req.Sources {
		info, err := os.Stat(src)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("Nie znaleziono źródła %s: %v", src, err))
			return
		}
		dst := filepath.Join(req.Destination, filepath.Base(src))
		if info.IsDir() {
			if err := operations.CopyDir(src, dst, nil); err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd kopiowania folderu %s: %v", src, err))
				return
			}
		} else {
			if err := operations.CopyFile(src, dst, nil); err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd kopiowania pliku %s: %v", src, err))
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "copied"})
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sources     []string `json:"sources"`
		Destination string   `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	for _, src := range req.Sources {
		dst := filepath.Join(req.Destination, filepath.Base(src))
		if err := operations.Move(src, dst); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd przenoszenia %s: %v", src, err))
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "moved"})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPath string `json:"old_path"`
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	dir := filepath.Dir(req.OldPath)
	newPath := filepath.Join(dir, req.NewName)
	if err := os.Rename(req.OldPath, newPath); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd zmiany nazwy: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "renamed", "new_path": newPath})
}

func (s *Server) handleMultiRenamePreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Files       []string `json:"files"`
		Find        string   `json:"find"`
		Replace     string   `json:"replace"`
		Prefix      string   `json:"prefix"`
		Suffix      string   `json:"suffix"`
		StartNum    int      `json:"start_num"`
		Digits      int      `json:"digits"`
		ToLowerCase bool     `json:"to_lower"`
		ToUpperCase bool     `json:"to_upper"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	rule := operations.RenameRule{
		Find:          req.Find,
		Replace:       req.Replace,
		Prefix:        req.Prefix,
		Suffix:        req.Suffix,
		CounterStart:  req.StartNum,
		CounterStep:   1,
		CounterDigits: req.Digits,
		ToLowerCase:   req.ToLowerCase,
		ToUpperCase:   req.ToUpperCase,
	}

	pairs := operations.PreviewRename(req.Files, rule)
	writeJSON(w, http.StatusOK, pairs)
}

func (s *Server) handleMultiRenameApply(w http.ResponseWriter, r *http.Request) {
	var pairs []operations.RenamePair
	if err := json.NewDecoder(r.Body).Decode(&pairs); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	if err := operations.ExecuteRename(pairs); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd wykonania zamiany nazw: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "renamed"})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RootDir       string `json:"root_dir"`
		Pattern       string `json:"pattern"`
		Content       string `json:"content"`
		CaseSensitive bool   `json:"case_sensitive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	filter := operations.SearchFilter{
		NamePattern:   req.Pattern,
		Content:       req.Content,
		CaseSensitive: req.CaseSensitive,
	}

	var matches []operations.SearchMatch
	var mu sync.Mutex

	_ = operations.SearchFiles(req.RootDir, filter, func(m operations.SearchMatch) {
		mu.Lock()
		if len(matches) < 200 { // limit to 200 results
			matches = append(matches, m)
		}
		mu.Unlock()
	}, nil)

	writeJSON(w, http.StatusOK, matches)
}

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File1 string `json:"file1"`
		File2 string `json:"file2"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	res, err := operations.CompareFiles(req.File1, req.File2)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd porównywania: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LeftDir  string `json:"left_dir"`
		RightDir string `json:"right_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	items, err := operations.CompareDirectories(req.LeftDir, req.RightDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd analizy synchronizacji: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleSplit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File      string `json:"file"`
		DstDir    string `json:"dst_dir"`
		ChunkSize int64  `json:"chunk_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	if req.ChunkSize <= 0 {
		req.ChunkSize = 1457664 // standard floppy
	}

	parts, err := operations.SplitFile(req.File, req.DstDir, req.ChunkSize, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd podziału pliku: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "split", "parts": parts})
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FirstPart string `json:"first_part"`
		DstDir    string `json:"dst_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	outPath, err := operations.JoinFiles(req.FirstPart, req.DstDir, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Błąd scalania plików: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "joined", "out_path": outPath})
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cmd string `json:"cmd"`
		Cwd string `json:"cwd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Niepoprawne dane JSON")
		return
	}

	var cmd *exec.Cmd
	if filepath.Separator == '\\' {
		cmd = exec.Command("cmd.exe", "/c", req.Cmd)
	} else {
		cmd = exec.Command("sh", "-c", req.Cmd)
	}
	if req.Cwd != "" {
		cmd.Dir = req.Cwd
	}

	out, err := cmd.CombinedOutput()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"output":    string(out),
		"exit_code": exitCode,
		"error":     fmt.Sprint(err),
	})
}
