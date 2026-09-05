package vfs

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

// tarEntry reprezentuje pojedynczy, zindeksowany wpis archiwum TAR.
// Zawartość plików regularnych jest trzymana w pamięci, dzięki czemu
// możliwy jest wielokrotny, swobodny odczyt (strumienie bzip2/gzip nie
// pozwalają na przewijanie).
type tarEntry struct {
	name    string // znormalizowana ścieżka wewnątrz archiwum, np. "dir/sub/plik.txt"
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	data    []byte // nil dla katalogów i wpisów niebędących plikami regularnymi
}

// compression określa sposób kompresji strumienia TAR.
type compression int

const (
	compNone compression = iota
	compGzip
	compBzip2
)

// TarVFS udostępnia zawartość archiwum TAR (opcjonalnie skompresowanego
// gzip lub bzip2) poprzez interfejs VFS. Archiwum jest tylko do odczytu.
type TarVFS struct {
	archivePath string
	subPath     string
	entries     map[string]*tarEntry    // zindeksowane wpisy z archiwum
	subDirs     map[string][]*FileEntry // katalog -> lista dzieci
}

// Kontrola zgodności z interfejsem VFS na etapie kompilacji.
var _ VFS = (*TarVFS)(nil)

// errTarReadOnly zwraca standardowy błąd dla operacji modyfikujących.
func errTarReadOnly() error {
	return fmt.Errorf("archiwum TAR jest tylko do odczytu")
}

// NewTarVFS otwiera archiwum TAR, rozpoznaje jego kompresję i buduje
// w pamięci indeks plików oraz katalogów wirtualnych.
func NewTarVFS(archivePath string) (*TarVFS, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("nie można otworzyć archiwum %q: %w", archivePath, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("nie można odczytać informacji o archiwum %q: %w", archivePath, err)
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("%q nie jest plikiem archiwum", archivePath)
	}

	// Wykrycie formatu: najpierw rozszerzenie, w razie potrzeby sygnatura.
	sig := make([]byte, 4)
	n, err := io.ReadFull(f, sig)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("nie można odczytać nagłówka archiwum %q: %w", archivePath, err)
	}
	sig = sig[:n]
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("nie można przewinąć archiwum %q: %w", archivePath, err)
	}

	var r io.Reader = f
	switch detectCompression(archivePath, sig) {
	case compGzip:
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("nieprawidłowy strumień gzip w %q: %w", archivePath, err)
		}
		defer gz.Close()
		r = gz
	case compBzip2:
		r = bzip2.NewReader(f)
	}

	v := &TarVFS{
		archivePath: archivePath,
		entries:     make(map[string]*tarEntry),
		subDirs:     make(map[string][]*FileEntry),
	}

	if err := v.scan(r); err != nil {
		return nil, err
	}
	v.buildTree(fi.ModTime())

	return v, nil
}

// detectCompression rozpoznaje kompresję po rozszerzeniu, a jeśli to nie
// wystarczy — po sygnaturze pierwszych bajtów pliku.
func detectCompression(name string, sig []byte) compression {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"), strings.HasSuffix(lower, ".taz"):
		return compGzip
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz"),
		strings.HasSuffix(lower, ".tbz2"), strings.HasSuffix(lower, ".tb2"):
		return compBzip2
	}

	// Fallback: sygnatura pliku.
	if len(sig) >= 2 && sig[0] == 0x1f && sig[1] == 0x8b {
		return compGzip
	}
	if len(sig) >= 3 && sig[0] == 'B' && sig[1] == 'Z' && sig[2] == 'h' {
		return compBzip2
	}
	return compNone
}

// scan przechodzi przez wszystkie wpisy archiwum i zapisuje je w indeksie.
func (t *TarVFS) scan(r io.Reader) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("błąd odczytu archiwum %q: %w", t.archivePath, err)
		}

		name := normalizeArchivePath(hdr.Name)
		if name == "" {
			continue // wpis wskazujący na korzeń archiwum
		}

		info := hdr.FileInfo()
		e := &tarEntry{
			name:    name,
			size:    hdr.Size,
			mode:    info.Mode(),
			modTime: hdr.ModTime,
			isDir:   hdr.Typeflag == tar.TypeDir || info.IsDir(),
		}

		if !e.isDir && info.Mode().IsRegular() {
			data, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("błąd odczytu pliku %q z archiwum: %w", name, err)
			}
			e.data = data
			e.size = int64(len(data))
		}
		if e.isDir {
			e.size = 0
			e.mode |= os.ModeDir
		}

		t.entries[name] = e
	}
	return nil
}

// buildTree dopisuje brakujące katalogi wirtualne oraz buduje mapę dzieci.
func (t *TarVFS) buildTree(fallbackTime time.Time) {
	names := make([]string, 0, len(t.entries))
	for name := range t.entries {
		names = append(names, name)
	}

	// Katalogi, które nie mają własnego nagłówka w archiwum.
	for _, name := range names {
		for dir := parentDir(name); dir != ""; dir = parentDir(dir) {
			if _, ok := t.entries[dir]; ok {
				continue
			}
			t.entries[dir] = &tarEntry{
				name:    dir,
				mode:    os.ModeDir | 0o755,
				modTime: fallbackTime,
				isDir:   true,
			}
		}
	}

	t.subDirs = make(map[string][]*FileEntry, len(t.entries)+1)
	t.subDirs[""] = nil // korzeń istnieje zawsze
	for name, e := range t.entries {
		if e.isDir {
			if _, ok := t.subDirs[name]; !ok {
				t.subDirs[name] = nil // katalog może być pusty
			}
		}
		parent := parentDir(name)
		t.subDirs[parent] = append(t.subDirs[parent], t.toFileEntry(e))
	}

	for _, children := range t.subDirs {
		sortEntries(children)
	}
}

// sortEntries sortuje wpisy: najpierw katalogi, potem pliki, alfabetycznie.
func sortEntries(entries []*FileEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

// toFileEntry konwertuje wpis archiwum na FileEntry.
func (t *TarVFS) toFileEntry(e *tarEntry) *FileEntry {
	return &FileEntry{
		Name:      path.Base(e.name),
		Path:      t.archivePath + "::/" + e.name,
		Size:      e.size,
		Mode:      e.mode,
		ModTime:   e.modTime,
		IsDir:     e.isDir,
		IsArchive: !e.isDir && IsArchiveFile(e.name),
	}
}

// normalizeArchivePath sprowadza ścieżkę z archiwum do postaci
// "a/b/c" (bez wiodących i końcowych ukośników, bez "./" i "..").
func normalizeArchivePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if idx := strings.Index(p, "::/"); idx >= 0 {
		p = p[idx+len("::/"):]
	}
	p = strings.Trim(p, "/")
	if p == "" || p == "." {
		return ""
	}
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return strings.Trim(p, "/")
}

// parentDir zwraca katalog nadrzędny dla znormalizowanej ścieżki
// ("" oznacza korzeń archiwum).
func parentDir(p string) string {
	if p == "" {
		return ""
	}
	dir := path.Dir(p)
	if dir == "." || dir == "/" {
		return ""
	}
	return dir
}

// resolve zamienia nazwę względną (lub bezwzględną wewnątrz archiwum)
// na znormalizowaną ścieżkę w archiwum.
func (t *TarVFS) resolve(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if idx := strings.Index(name, "::/"); idx >= 0 {
		name = name[idx+len("::/"):]
	}
	if strings.HasPrefix(name, "/") {
		return normalizeArchivePath(name)
	}
	if name == "" || name == "." {
		return t.subPath
	}
	if name == ".." {
		return parentDir(t.subPath)
	}
	if t.subPath == "" {
		return normalizeArchivePath(name)
	}
	return normalizeArchivePath(t.subPath + "/" + name)
}

// Path zwraca aktualną ścieżkę wirtualną w postaci "archiwum::/podkatalog".
func (t *TarVFS) Path() string {
	return t.archivePath + "::/" + t.subPath
}

// SetPath ustawia bieżący katalog wewnątrz archiwum.
func (t *TarVFS) SetPath(p string) error {
	if t.entries == nil {
		return fmt.Errorf("archiwum jest zamknięte")
	}

	target := normalizeArchivePath(p)
	if target == "" {
		t.subPath = ""
		return nil
	}

	e, ok := t.entries[target]
	if !ok {
		return fmt.Errorf("katalog %q nie istnieje w archiwum %s", target, t.archivePath)
	}
	if !e.isDir {
		return fmt.Errorf("%q nie jest katalogiem", target)
	}

	t.subPath = target
	return nil
}

// Parent przechodzi o jeden poziom wyżej w drzewie archiwum.
func (t *TarVFS) Parent() error {
	if t.entries == nil {
		return fmt.Errorf("archiwum jest zamknięte")
	}
	if t.subPath == "" {
		return fmt.Errorf("jesteś w katalogu głównym archiwum %s", t.archivePath)
	}
	t.subPath = parentDir(t.subPath)
	return nil
}

// List zwraca zawartość bieżącego katalogu; poza korzeniem dodaje wpis "..".
func (t *TarVFS) List() ([]*FileEntry, error) {
	if t.entries == nil {
		return nil, fmt.Errorf("archiwum jest zamknięte")
	}

	children, ok := t.subDirs[t.subPath]
	if !ok {
		return nil, fmt.Errorf("katalog %q nie istnieje w archiwum %s", t.subPath, t.archivePath)
	}

	result := make([]*FileEntry, 0, len(children)+1)
	if t.subPath != "" {
		parent := parentDir(t.subPath)
		result = append(result, &FileEntry{
			Name:    "..",
			Path:    t.archivePath + "::/" + parent,
			Mode:    os.ModeDir | 0o755,
			ModTime: time.Time{},
			IsDir:   true,
		})
	}

	// Kopie, aby modyfikacje po stronie UI (np. Selected) nie psuły indeksu.
	for _, c := range children {
		cp := *c
		result = append(result, &cp)
	}
	return result, nil
}

// Stat zwraca informacje o wskazanym wpisie archiwum.
func (t *TarVFS) Stat(name string) (*FileEntry, error) {
	if t.entries == nil {
		return nil, fmt.Errorf("archiwum jest zamknięte")
	}

	target := t.resolve(name)
	if target == "" {
		// Korzeń archiwum.
		return &FileEntry{
			Name:      path.Base(t.archivePath),
			Path:      t.archivePath + "::/",
			Mode:      os.ModeDir | 0o755,
			IsDir:     true,
			IsArchive: true,
		}, nil
	}

	e, ok := t.entries[target]
	if !ok {
		return nil, fmt.Errorf("wpis %q nie istnieje w archiwum %s", target, t.archivePath)
	}
	return t.toFileEntry(e), nil
}

// Open zwraca strumień odczytu z zawartością pliku w archiwum.
func (t *TarVFS) Open(name string) (io.ReadCloser, error) {
	if t.entries == nil {
		return nil, fmt.Errorf("archiwum jest zamknięte")
	}

	target := t.resolve(name)
	if target == "" {
		return nil, fmt.Errorf("nieprawidłowa nazwa pliku: %q", name)
	}

	e, ok := t.entries[target]
	if !ok {
		return nil, fmt.Errorf("plik %q nie istnieje w archiwum %s", target, t.archivePath)
	}
	if e.isDir {
		return nil, fmt.Errorf("%q jest katalogiem", target)
	}

	return io.NopCloser(bytes.NewReader(e.data)), nil
}

// Create — archiwum jest tylko do odczytu.
func (t *TarVFS) Create(name string) (io.WriteCloser, error) {
	return nil, errTarReadOnly()
}

// Mkdir — archiwum jest tylko do odczytu.
func (t *TarVFS) Mkdir(name string) error { return errTarReadOnly() }

// Remove — archiwum jest tylko do odczytu.
func (t *TarVFS) Remove(name string) error { return errTarReadOnly() }

// RemoveAll — archiwum jest tylko do odczytu.
func (t *TarVFS) RemoveAll(name string) error { return errTarReadOnly() }

// Rename — archiwum jest tylko do odczytu.
func (t *TarVFS) Rename(oldName, newName string) error { return errTarReadOnly() }

// IsArchive informuje, że VFS reprezentuje archiwum.
func (t *TarVFS) IsArchive() bool { return true }

// Close zwalnia pamięć zajmowaną przez indeks archiwum.
func (t *TarVFS) Close() error {
	t.entries = nil
	t.subDirs = nil
	t.subPath = ""
	return nil
}